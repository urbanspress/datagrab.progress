package dbstudio

import (
	"context"
	"database/sql"
	"os"
	"strconv"
	"testing"
	"time"

	"groxy-desk/internal/security"
	"groxy-desk/internal/testfixture"
)

func TestPinnedPostgresLogicalReplay(t *testing.T) {
	portText := os.Getenv("GROXY_DESK_TEST_POSTGRES_PORT")
	if portText == "" {
		t.Skip("requires a disposable PostgreSQL 18 database and its pg_dump client")
	}
	port, err := strconv.ParseUint(portText, 10, 16)
	if err != nil {
		t.Fatal(err)
	}
	ssh := testfixture.PinnedSSHWithSFTP(t, uint32(port), testfixture.ForwardLoopback(uint32(port)))
	m := NewManager(t.TempDir(), nil, nil)
	defer m.CloseAll()
	tunnel, err := m.Tunnels().OpenTunnel(OpenTunnelRequest{Engine: "postgres", SSHHost: "127.0.0.1", SSHPort: ssh.Port,
		KnownHostsPath: ssh.KnownHosts, PinnedFingerprint: ssh.Fingerprint, RemoteDBHost: "127.0.0.1", RemoteDBPort: portText})
	if err != nil {
		t.Fatal(err)
	}
	target := TargetSpec{Engine: "postgres", DBName: "desk_fixture", DBUser: "postgres", DBPassword: "desk-fixture-only", TunnelID: tunnel.TunnelID}
	scope, err := m.networkScopeFor(target, "restore")
	if err != nil {
		t.Fatal(err)
	}
	db, _, _, err := m.openTarget(target)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	execute := func(script string) {
		t.Helper()
		if _, err := db.Exec(script); err != nil {
			t.Fatal(err)
		}
	}
	execute("CREATE SCHEMA desk_replay")
	defer execute("DROP SCHEMA desk_replay CASCADE")
	execute(`CREATE TABLE desk_replay.prices(id bigint PRIMARY KEY, amount numeric(12,2), note text);
		CREATE VIEW desk_replay.priced AS SELECT id,amount FROM desk_replay.prices;
		CREATE FUNCTION desk_replay.changed() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN NEW.note = 'changed'; RETURN NEW; END $$;
		CREATE TRIGGER changed BEFORE UPDATE ON desk_replay.prices FOR EACH ROW EXECUTE FUNCTION desk_replay.changed();
		INSERT INTO desk_replay.prices VALUES(9007199254740993,0.00,NULL)`)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	err = withPinnedMaintenanceLock(ctx, scope, func(remove *bool) error {
		return m.withPostgresRestoreFence(ctx, target, scope, remove, func(ctx context.Context, conn *sql.Conn) error {
			tx, err := conn.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
			if err != nil {
				return err
			}
			defer tx.Rollback()
			saved, err := postgresLogicalImage(ctx, conn, scope.database, false)
			if err != nil {
				return err
			}
			_, err = tx.ExecContext(ctx, `UPDATE desk_replay.prices SET amount=2.50;
				DROP VIEW desk_replay.priced; CREATE TABLE desk_replay.later(id int);`)
			if err != nil {
				return err
			}
			if err := replayPostgresImage(ctx, conn, scope.database, security.SHA256Hex(saved), saved); security.ErrorStatus(err, 0) != 409 {
				t.Fatalf("expected conflict after schema and data changes, got %v", err)
			}
			changed, err := postgresLogicalImage(ctx, conn, scope.database, false)
			if err != nil {
				return err
			}
			if err := replayPostgresImage(ctx, conn, scope.database, security.SHA256Hex(changed), saved); err != nil {
				return err
			}
			t.Logf("verified reserved-connection logical replay SHA-256 %s", security.SHA256Hex(saved))
			return tx.Commit()
		})
	})
	if err != nil {
		t.Fatal(err)
	}
	var price string
	if err := db.QueryRow("SELECT amount FROM desk_replay.priced WHERE id=9007199254740993").Scan(&price); err != nil || price != "0.00" {
		t.Fatalf("post-commit view returned %q, %v", price, err)
	}
}
