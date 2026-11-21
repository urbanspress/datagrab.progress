package dbstudio

import (
	"bytes"
	"context"
	"crypto/subtle"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"groxy-desk/internal/security"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgproto3"
	"github.com/jackc/pgx/v5/stdlib"
)

// pg_dump normally authenticates a new database connection, which is
// impossible once ALLOW_CONNECTIONS is false. Give this one child access to
// the already reserved backend. The password is generated per invocation;
// neither the database password nor its cancellation key leaves Desk.
func dumpReservedPostgres(ctx context.Context, conn *sql.Conn, database string, clean bool) ([]byte, error) {
	path, err := exec.LookPath("pg_dump")
	if err != nil {
		return nil, writeFenceUnavailable("Install the PostgreSQL 18 client tools to use full logical recovery.", err)
	}
	if !filepath.IsAbs(path) {
		return nil, writeFenceUnavailable("The PostgreSQL client directory must have an absolute path.", nil)
	}
	token, err := security.RandomTokenHex(32)
	if err != nil {
		return nil, err
	}
	restrictKey, err := security.RandomTokenHex(24)
	if err != nil {
		return nil, err
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	defer listener.Close()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	port := fmt.Sprint(listener.Addr().(*net.TCPAddr).Port)
	args := []string{"--host=127.0.0.1", "--port=" + port, "--username=desk_dump", "--no-password",
		"--dbname=" + database, "--encoding=UTF8", "--inserts", "--restrict-key=" + restrictKey}
	if clean {
		args = append(args, "--clean", "--if-exists")
	}
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Dir = filepath.Dir(path)
	// Ignore libpq environment settings that can redirect a connection or
	// change dump semantics. The child never receives Desk's DB credentials.
	for _, item := range os.Environ() {
		if !strings.HasPrefix(strings.ToUpper(item), "PG") {
			cmd.Env = append(cmd.Env, item)
		}
	}
	cmd.Env = append(cmd.Env, "PGPASSWORD="+token, "PGSSLMODE=disable", "PGGSSENCMODE=disable", "PGCLIENTENCODING=UTF8")
	var out limitedDump
	cmd.Stdout = &out
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
		listener.Close()
	}()
	stop := context.AfterFunc(ctx, func() { listener.Close() })
	defer stop()
	client, err := listener.Accept()
	listener.Close()
	if err == nil {
		stopClient := context.AfterFunc(ctx, func() { client.Close() })
		err = conn.Raw(func(driver any) error {
			pg := driver.(*stdlib.Conn).Conn()
			if err := pg.DeallocateAll(ctx); err != nil {
				return err
			}
			return servePostgresDump(ctx, client, pg.PgConn(), database, token)
		})
		stopClient()
		client.Close()
	}
	if err != nil {
		cancel()
	}
	if err = errors.Join(err, <-done); err != nil {
		return nil, fmt.Errorf("PostgreSQL logical dump failed: %w", err)
	}
	// Remove exactly the two client-only commands at the boundaries. Never
	// scan or rewrite SQL literals, procedural bodies or row values.
	raw := out.buf.Bytes()
	opening := []byte("\\restrict " + restrictKey)
	closing := []byte("\\unrestrict " + restrictKey)
	prefix, body, found := bytes.Cut(raw, opening)
	header := bytes.ReplaceAll(bytes.TrimSpace(prefix), []byte("\r\n"), []byte("\n"))
	if !found || !bytes.Equal(header, []byte("--\n-- PostgreSQL database dump\n--")) {
		return nil, errors.New("unrecognized pg_dump header")
	}
	body, found = bytes.CutSuffix(bytes.TrimSpace(body), closing)
	if !found {
		return nil, errors.New("incomplete PostgreSQL logical dump")
	}
	return bytes.TrimSpace(body), nil
}

type limitedDump struct{ buf bytes.Buffer }

func (w *limitedDump) Write(p []byte) (int, error) {
	if len(p) > maxNetworkSnapshotBytes-w.buf.Len() {
		return 0, errors.New("logical dump exceeds the recovery size limit")
	}
	return w.buf.Write(p)
}

func servePostgresDump(ctx context.Context, client net.Conn, pg *pgconn.PgConn, database, token string) error {
	local := pgproto3.NewBackend(client, client)
	local.SetMaxBodyLen(maxNetworkSnapshotBytes)
	first, err := local.ReceiveStartupMessage()
	if err != nil {
		return err
	}
	startup, ok := first.(*pgproto3.StartupMessage)
	if !ok || startup.Parameters["database"] != database || startup.Parameters["user"] != "desk_dump" {
		return errors.New("unexpected PostgreSQL dump client")
	}
	local.Send(&pgproto3.AuthenticationCleartextPassword{})
	if err := local.Flush(); err != nil {
		return err
	}
	message, err := local.Receive()
	password, ok := message.(*pgproto3.PasswordMessage)
	if err != nil || !ok || subtle.ConstantTimeCompare([]byte(password.Password), []byte(token)) != 1 {
		return errors.New("invalid PostgreSQL dump credential")
	}
	local.Send(&pgproto3.AuthenticationOk{})
	for _, key := range []string{"server_version", "server_encoding", "client_encoding", "standard_conforming_strings", "integer_datetimes", "DateStyle"} {
		local.Send(&pgproto3.ParameterStatus{Name: key, Value: pg.ParameterStatus(key)})
	}
	local.Send(&pgproto3.ReadyForQuery{TxStatus: pg.TxStatus()})
	if err := local.Flush(); err != nil {
		return err
	}
	remote := pg.Frontend()
	inDeskTx := pg.TxStatus() == 'T'
	for {
		msg, err := local.Receive()
		if err != nil {
			return err
		}
		switch input := msg.(type) {
		case *pgproto3.Terminate:
			return nil
		case *pgproto3.Parse, *pgproto3.Bind, *pgproto3.Describe, *pgproto3.Execute, *pgproto3.Close:
			remote.Send(msg)
			continue
		case *pgproto3.Query:
			// Desk owns the outer transaction and needs the dump to see
			// its uncommitted replay. Answer pg_dump's transaction setup
			// locally; all its catalog and data reads use the same backend.
			if inDeskTx && (input.String == "BEGIN" || input.String == "SET TRANSACTION ISOLATION LEVEL REPEATABLE READ, READ ONLY") {
				tag := strings.Fields(input.String)[0]
				local.Send(&pgproto3.CommandComplete{CommandTag: []byte(tag)})
				local.Send(&pgproto3.ReadyForQuery{TxStatus: 'T'})
				if err := local.Flush(); err != nil {
					return err
				}
				continue
			}
			remote.Send(msg)
		case *pgproto3.Sync:
			remote.Send(msg)
		default:
			return fmt.Errorf("unsupported pg_dump message %T", msg)
		}
		if err := remote.Flush(); err != nil {
			return err
		}
		for {
			reply, err := pg.ReceiveMessage(ctx)
			if err != nil {
				return err
			}
			local.Send(reply)
			if err := local.Flush(); err != nil {
				return err
			}
			if _, ready := reply.(*pgproto3.ReadyForQuery); ready {
				break
			}
		}
	}
}
