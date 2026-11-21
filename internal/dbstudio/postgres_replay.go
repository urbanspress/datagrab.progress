package dbstudio

import (
	"bytes"
	"context"
	"database/sql"
	"errors"

	"groxy-desk/internal/security"

	"github.com/jackc/pgx/v5/stdlib"
)

// Capture and replay run inside a Desk-owned transaction. pg_dump modifies
// session settings during startup. A savepoint restores the caller's settings
// before the next query, including the search path and relation restrictions.
func postgresLogicalImage(ctx context.Context, conn *sql.Conn, database string, clean bool) ([]byte, error) {
	if _, err := conn.ExecContext(ctx, "SAVEPOINT desk_dump_settings"); err != nil {
		return nil, err
	}
	img, err := dumpReservedPostgres(ctx, conn, database, clean)
	if err != nil {
		return nil, err
	}
	if _, err := conn.ExecContext(ctx, "ROLLBACK TO SAVEPOINT desk_dump_settings"); err != nil {
		return nil, err
	}
	_, err = conn.ExecContext(ctx, "RELEASE SAVEPOINT desk_dump_settings")
	return img, err
}

// pg_dump --clean emits the current objects' dependency-ordered teardown
// between the standard setup and the ordinary dump. Verify the complete
// prefix and suffix before using that teardown. This also removes objects
// added since the saved dump, which replaying the saved --clean dump misses.
func postgresTeardown(plain, clean []byte) ([]byte, error) {
	const marker = "SET row_security = off;"
	setup, contents, ok := bytes.Cut(plain, []byte(marker))
	if !ok {
		return nil, errors.New("unrecognized PostgreSQL dump setup")
	}
	afterSetup, ok := bytes.CutPrefix(clean, plain[:len(setup)+len(marker)])
	if !ok {
		return nil, errors.New("PostgreSQL dump setup changed")
	}
	drop, ok := bytes.CutSuffix(bytes.TrimSpace(afterSetup), bytes.TrimSpace(contents))
	if !ok {
		return nil, errors.New("PostgreSQL clean dump differs outside the object teardown")
	}
	return bytes.TrimSpace(drop), nil
}

func execPostgresDump(ctx context.Context, conn *sql.Conn, script []byte) error {
	// The native dump uses INSERT statements, so PostgreSQL's simple query
	// protocol can parse the entire script including dollar-quoted functions.
	// No client meta-commands or shell are involved.
	return conn.Raw(func(driver any) error {
		pg := driver.(*stdlib.Conn).Conn().PgConn()
		return pg.Exec(ctx, string(script)).Close()
	})
}

func replayPostgresImage(ctx context.Context, conn *sql.Conn, database, baseline string, saved []byte) error {
	current, err := postgresLogicalImage(ctx, conn, database, false)
	if err != nil {
		return err
	}
	if security.SHA256Hex(current) != baseline {
		return changedNetworkTarget()
	}
	clean, err := postgresLogicalImage(ctx, conn, database, true)
	if err != nil {
		return err
	}
	drop, err := postgresTeardown(current, clean)
	if err != nil {
		return err
	}
	if err := execPostgresDump(ctx, conn, drop); err != nil {
		return err
	}
	if err := execPostgresDump(ctx, conn, saved); err != nil {
		return err
	}
	actual, err := postgresLogicalImage(ctx, conn, database, false)
	if err != nil {
		return err
	}
	if security.SHA256Hex(actual) != security.SHA256Hex(saved) {
		return errors.New("the replayed PostgreSQL database differs from the saved logical dump")
	}
	return nil
}
