package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"groxy-desk/internal/deskexec"
	"groxy-desk/internal/groxybridge"
	"groxy-desk/internal/security"
	"groxy-desk/internal/testfixture"

	"github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5/pgconn"
)

// TestPinnedNetworkRecoveryAPI uses the same disposable fixtures as the
// catalog tests and exercises the public HTTP routes over a pinned SSH tunnel.
func TestPinnedNetworkRecoveryAPI(t *testing.T) {
	for _, fixture := range []struct{ engine, portEnv, user string }{
		{"postgres", "GROXY_DESK_TEST_POSTGRES_PORT", "postgres"},
		{"mariadb", "GROXY_DESK_TEST_MARIADB_PORT", "root"},
	} {
		t.Run(fixture.engine, func(t *testing.T) {
			portValue := os.Getenv(fixture.portEnv)
			if portValue == "" {
				t.Skipf("%s is required for the disposable database fixture", fixture.portEnv)
			}
			port, err := strconv.ParseUint(portValue, 10, 16)
			if err != nil || port == 0 {
				t.Fatalf("invalid fixture port %q", portValue)
			}
			const userPassword, database = "desk-fixture-only", "desk_fixture"
			address := net.JoinHostPort("127.0.0.1", portValue)
			driver := "pgx"
			dsn := (&url.URL{Scheme: "postgres", User: url.UserPassword(fixture.user, userPassword),
				Host: address, Path: "/" + database, RawQuery: "sslmode=disable"}).String()
			if fixture.engine == "mariadb" {
				driver = "mysql"
				cfg := mysql.NewConfig()
				cfg.User, cfg.Passwd, cfg.Net, cfg.Addr, cfg.DBName = fixture.user, userPassword, "tcp", address, database
				dsn = cfg.FormatDSN()
			}
			db, err := sql.Open(driver, dsn)
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			db.SetMaxIdleConns(0)
			deadline := time.Now().Add(20 * time.Second)
			for {
				err := db.Ping()
				if err == nil {
					break
				}
				if time.Now().After(deadline) {
					t.Fatalf("waiting for disposable %s fixture: %v", fixture.engine, err)
				}
				time.Sleep(100 * time.Millisecond)
			}
			external := func(statement string) {
				t.Helper()
				if _, err := db.Exec(statement); err != nil {
					t.Fatal(err)
				}
			}
			external("CREATE TABLE desk_http_recovery (id BIGINT PRIMARY KEY, price DECIMAL(12,2) NOT NULL, note VARCHAR(40))")
			defer external("DROP TABLE desk_http_recovery")
			external("CREATE UNIQUE INDEX desk_http_price ON desk_http_recovery(price)")
			external("INSERT INTO desk_http_recovery VALUES (9007199254740993,0.00,NULL),(9007199254740994,9.00,'')")

			t.Run("native_controls", func(t *testing.T) {
				if fixture.engine == "postgres" {
					probePostgresWriteFence(t, db, dsn)
					return
				}
				probeMariaDBWriteFence(t, db)
				probeMariaDBSwap(t, db)
			})

			srv, workspace, _ := setupTestServer(t)
			session := srv.Session()
			post := func(path string, body map[string]any, expected int, csrf string) map[string]any {
				t.Helper()
				res, out, raw := doAPIReq(t, http.MethodPost, session.URL+path, session.BearerToken, csrf, body)
				if res.StatusCode != expected {
					t.Fatalf("%s returned %d, want %d: %s", path, res.StatusCode, expected, raw)
				}
				return out
			}
			sshEndpoint := testfixture.PinnedSSHWithSFTP(t, uint32(port), testfixture.ForwardLoopback(uint32(port)))
			tunnel := post("/api/db/tunnels/open", map[string]any{
				"engine": fixture.engine, "ssh_host": "127.0.0.1", "ssh_port": sshEndpoint.Port,
				"known_hosts_path": sshEndpoint.KnownHosts, "pinned_fingerprint": sshEndpoint.Fingerprint,
				"remote_db_host": "127.0.0.1", "remote_db_port": portValue,
			}, http.StatusOK, session.CSRFToken)["tunnel"].(map[string]any)
			target := map[string]any{"engine": fixture.engine, "tunnel_id": tunnel["tunnel_id"], "db_name": database,
				"db_user": fixture.user, "db_password": userPassword}
			if fixture.engine == "postgres" && os.Getenv("GROXY_DESK_TEST_PG_LOGICAL") == "1" {
				t.Run("full_logical_http", func(t *testing.T) {
					verifyFullPostgresAPI(t, db, dsn, tunnel["tunnel_id"].(string), post, session.CSRFToken, workspace)
				})
			}
			assertFenceUnavailable := func(path string, req map[string]any) {
				t.Helper()
				out := post(path, req, http.StatusConflict, session.CSRFToken)
				if out["error"].(map[string]any)["code"] != security.CodeWriteFenceUnavailable || out["approval_token"] != nil {
					t.Fatalf("%s did not reject the unfenced write before approval: %v", path, out)
				}
			}
			for _, path := range []string{"/api/db/mutate/plan", "/api/db/mutate/apply"} {
				assertFenceUnavailable(path, map[string]any{"target": target, "sql": "DELETE FROM desk_http_recovery"})
				assertFenceUnavailable(path, map[string]any{"target": target, "sql": "ALTER TABLE desk_http_recovery ADD COLUMN missing_fence INT"})
			}
			if fixture.engine == "mariadb" {
				other, err := db.Conn(context.Background())
				if err != nil {
					t.Fatal(err)
				}
				defer other.Close()
				if _, err := other.ExecContext(context.Background(), "USE information_schema"); err != nil {
					t.Fatal(err)
				}
				var before, after uint64
				if err := other.QueryRowContext(context.Background(), "SELECT CONNECTION_ID()").Scan(&before); err != nil {
					t.Fatal(err)
				}
				assertFenceUnavailable("/api/db/mutate/plan", map[string]any{"target": target,
					"sql": "ALTER TABLE desk_http_recovery ADD COLUMN pending INT; ALTER TABLE desk_http_recovery ADD COLUMN pending INT"})
				var dbName string
				if err := other.QueryRowContext(context.Background(), "SELECT DATABASE(), CONNECTION_ID()").Scan(&dbName, &after); err != nil || before != after || dbName != "information_schema" {
					t.Fatalf("unrelated MariaDB connection changed: %s %d/%d, %v", dbName, before, after, err)
				}
			}
			for _, path := range []string{"/api/db/import/plan", "/api/db/import/apply"} {
				for _, name := range []string{"data.csv", "data.xlsx", "data.json", "data.sql"} {
					assertFenceUnavailable(path, map[string]any{"target": target, "path": name, "table": "desk_http_recovery"})
				}
			}
			var untouched int
			if err := db.QueryRow("SELECT COUNT(*) FROM desk_http_recovery").Scan(&untouched); err != nil || untouched != 2 {
				t.Fatalf("unfenced requests affected the target: %d rows, %v", untouched, err)
			}
			edit := map[string]any{"table": "desk_http_recovery", "column": "price", "key": map[string]any{"id": "9007199254740993"},
				"before": "0.00", "value": "1.00", "value_kind": "decimal"}
			const restorePlan, restoreApply = "/api/db/snapshots/restore/plan", "/api/db/snapshots/restore"
			applyEdit := func() (map[string]any, string, string, []byte) {
				t.Helper()
				req := map[string]any{"target": target, "edit": edit}
				plan := post("/api/db/mutate/plan", req, http.StatusOK, session.CSRFToken)
				req["approval_token"] = plan["approval_token"]
				snapshot := plan["snapshot"].(map[string]any)
				id := snapshot["snapshot_id"].(string)
				path := filepath.Join(srv.stateDir, "snapshots", id+".bak")
				bytes, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				var backup struct {
					Table json.RawMessage `json:"data_and_schema"`
				}
				if err := json.Unmarshal(bytes, &backup); err != nil || len(backup.Table) == 0 {
					t.Fatalf("missing snapshot table: %v", err)
				}
				if snapshot["before_sha256"] != security.SHA256Hex(bytes) {
					t.Fatal("snapshot metadata did not match stored bytes")
				}
				post("/api/db/mutate/apply", req, http.StatusCreated, session.CSRFToken)
				post("/api/db/mutate/apply", req, http.StatusConflict, session.CSRFToken)
				return map[string]any{"target": target, "snapshot_id": id}, security.SHA256Hex(backup.Table), path, bytes
			}

			restore, digest, _, _ := applyEdit()
			plan := post(restorePlan, restore, http.StatusOK, session.CSRFToken)
			restore["approval_token"] = plan["approval_token"]
			post(restoreApply, restore, http.StatusForbidden, "")
			var restored map[string]any
			if fixture.engine == "postgres" {
				verifyPostgresRestoreAdmission(t, db, dsn, func() {
					restored = post(restoreApply, restore, http.StatusCreated, session.CSRFToken)
				})
			} else {
				restored = post(restoreApply, restore, http.StatusCreated, session.CSRFToken)
			}
			if restored["restored_sha256"] != digest {
				t.Fatalf("restore SHA-256 %v, want %s", restored["restored_sha256"], digest)
			}
			post(restoreApply, restore, http.StatusConflict, session.CSRFToken)

			restore, digest, path, backup := applyEdit()
			plan = post(restorePlan, restore, http.StatusOK, session.CSRFToken)
			restore["approval_token"] = plan["approval_token"]
			external("UPDATE desk_http_recovery SET note='external' WHERE id=9007199254740994")
			post(restoreApply, restore, http.StatusConflict, session.CSRFToken)
			post(restorePlan, restore, http.StatusConflict, session.CSRFToken)
			restore["force"] = true
			plan = post(restorePlan, restore, http.StatusOK, session.CSRFToken)
			restore["baseline_sha256"] = plan["baseline_sha256"]
			post(restoreApply, restore, http.StatusForbidden, session.CSRFToken)
			restore["approval_token"] = plan["approval_token"]
			external("INSERT INTO desk_http_recovery VALUES (9007199254740995,4.00,'unreviewed')")
			post(restoreApply, restore, http.StatusConflict, session.CSRFToken)
			plan = post(restorePlan, restore, http.StatusOK, session.CSRFToken)
			restore["approval_token"], restore["baseline_sha256"] = plan["approval_token"], plan["baseline_sha256"]
			external("ALTER TABLE desk_http_recovery ADD COLUMN later_ddl VARCHAR(40)")
			post(restoreApply, restore, http.StatusConflict, session.CSRFToken)
			post(restorePlan, restore, http.StatusConflict, session.CSRFToken)
			external("ALTER TABLE desk_http_recovery DROP COLUMN later_ddl")
			plan = post(restorePlan, restore, http.StatusOK, session.CSRFToken)
			restore["approval_token"], restore["baseline_sha256"] = plan["approval_token"], plan["baseline_sha256"]

			if err := os.WriteFile(path, []byte("corrupt snapshot"), 0o600); err != nil {
				t.Fatal(err)
			}
			post(restoreApply, restore, http.StatusUnprocessableEntity, session.CSRFToken)
			var count int
			if err := db.QueryRow("SELECT COUNT(*) FROM desk_http_recovery").Scan(&count); err != nil || count != 3 {
				t.Fatalf("restore touched rows after a conflict or corruption: row count %d, %v", count, err)
			}
			var externalNote string
			if err := db.QueryRow("SELECT note FROM desk_http_recovery WHERE id=9007199254740994").Scan(&externalNote); err != nil || externalNote != "external" {
				t.Fatalf("external update lost after rejected restore: note %q, %v", externalNote, err)
			}
			if err := os.WriteFile(path, backup, 0o600); err != nil {
				t.Fatal(err)
			}
			restored = post(restoreApply, restore, http.StatusCreated, session.CSRFToken)
			if restored["restored_sha256"] != digest || restored["forced"] != true {
				t.Fatalf("forced restore did not verify the saved table: %v", restored)
			}
			recordPath := filepath.Join(srv.stateDir, "snapshots", restore["snapshot_id"].(string)+".json")
			recordData, err := os.ReadFile(recordPath)
			if err != nil {
				t.Fatal(err)
			}
			var record groxybridge.SnapshotRecord
			if err := json.Unmarshal(recordData, &record); err != nil ||
				!record.Restored || !record.ForcedRestore || record.RestoredAt == "" || record.RestoredSHA256 != digest {
				t.Fatalf("forced restore was not recorded with its verified checksum: %+v, %v", record, err)
			}
			post(restoreApply, restore, http.StatusConflict, session.CSRFToken)
			var price string
			var note sql.NullString
			err = db.QueryRow("SELECT COUNT(*) FROM desk_http_recovery").Scan(&count)
			if err != nil || count != 2 {
				t.Fatalf("snapshot row count %d: %v", count, err)
			}
			err = db.QueryRow("SELECT price,note FROM desk_http_recovery WHERE id=9007199254740993").Scan(&price, &note)
			if err != nil || price != "0.00" || note.Valid {
				t.Fatalf("restored price %q and null status %v: %v", price, note.Valid, err)
			}
			err = db.QueryRow("SELECT note FROM desk_http_recovery WHERE id=9007199254740994").Scan(&note)
			if err != nil || !note.Valid || note.String != "" {
				t.Fatalf("empty string changed: %v, %v", note, err)
			}
			if fixture.engine == "postgres" {
				external("CREATE ROLE desk_restore_unprivileged LOGIN PASSWORD 'desk-fixture-only'")
				defer external("DROP ROLE desk_restore_unprivileged")
				external("GRANT SELECT, UPDATE ON desk_http_recovery TO desk_restore_unprivileged")
				defer external("REVOKE SELECT, UPDATE ON desk_http_recovery FROM desk_restore_unprivileged")
				target["db_user"] = "desk_restore_unprivileged"
				req, _, _, _ := applyEdit()
				plan := post(restorePlan, req, http.StatusOK, session.CSRFToken)
				req["approval_token"] = plan["approval_token"]
				assertFenceUnavailable(restoreApply, req)
				if err := db.QueryRow("SELECT price FROM desk_http_recovery WHERE id=9007199254740993").Scan(&price); err != nil || price != "1.00" {
					t.Fatalf("unprivileged recovery changed the table or disabled connections: %q, %v", price, err)
				}
			}
		})
	}
	if os.Getenv("GROXY_DESK_TEST_MARIADB_LEAST_PRIV_PORT") != "" || os.Getenv("GROXY_DESK_TEST_MARIADB_PORT") != "" || os.Getenv("GROXY_DESK_TEST_MARIADB_DOCKER") == "1" {
		t.Run("mariadb_least_privilege", func(t *testing.T) {
			verifyFullMariaDBAPI(t)
		})
	}
}

func verifyFullPostgresAPI(t *testing.T, admin *sql.DB, baseDSN, tunnel string, post func(string, map[string]any, int, string) map[string]any, csrf, workspace string) {
	t.Helper()
	const database, writer, password = "desk_logical_http", "desk_logical_writer", "desk-fixture-only"
	exec := func(db *sql.DB, sql string) {
		t.Helper()
		if _, err := db.Exec(sql); err != nil {
			t.Fatal(err)
		}
	}
	exec(admin, "CREATE ROLE desk_logical_writer LOGIN PASSWORD 'desk-fixture-only'")
	defer exec(admin, "DROP ROLE desk_logical_writer")
	exec(admin, "CREATE DATABASE desk_logical_http")
	defer exec(admin, "DROP DATABASE desk_logical_http")
	exec(admin, "GRANT CREATE ON DATABASE desk_logical_http TO desk_logical_writer")
	exec(admin, "ALTER DATABASE desk_logical_http SET search_path=desk_app")
	u, err := url.Parse(baseDSN)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + database
	external, err := sql.Open("pgx", u.String())
	if err != nil {
		t.Fatal(err)
	}
	defer external.Close()
	external.SetMaxIdleConns(0)
	exec(external, `CREATE SCHEMA desk_app AUTHORIZATION desk_logical_writer;
		SET ROLE desk_logical_writer;
		CREATE TABLE desk_http_recovery(id bigint PRIMARY KEY, price numeric(12,2), note text);
		INSERT INTO desk_http_recovery VALUES(9007199254740993,0.00,NULL);
		CREATE VIEW prices AS SELECT id,price FROM desk_http_recovery;
		CREATE FUNCTION changed() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN NEW.note='updated'; RETURN NEW; END $$;
		CREATE TRIGGER changed BEFORE UPDATE ON desk_http_recovery FOR EACH ROW EXECUTE FUNCTION changed();`)
	target := map[string]any{"engine": "postgres", "db_name": database, "schema_name": "desk_app", "tunnel_id": tunnel,
		"db_user": writer, "db_password": password, "maintenance_user": "postgres", "maintenance_password": password}
	request := map[string]any{"target": target, "sql": "UPDATE desk_http_recovery SET price=2.50; DROP VIEW prices; CREATE TABLE later(id int)"}
	const planWrite, applyWrite = "/api/db/mutate/plan", "/api/db/mutate/apply"
	const planUndo, applyUndo = "/api/db/snapshots/restore/plan", "/api/db/snapshots/restore"
	write := func() (map[string]any, string) {
		t.Helper()
		plan := post(planWrite, request, 200, csrf)
		request["approval_token"] = plan["approval_token"]
		snapshot := plan["snapshot"].(map[string]any)
		post(applyWrite, request, 201, csrf)
		post(applyWrite, request, 409, csrf)
		return map[string]any{"target": target, "snapshot_id": snapshot["snapshot_id"]}, snapshot["before_sha256"].(string)
	}
	undo, digest := write()
	undo["approval_token"] = post(planUndo, undo, 200, csrf)["approval_token"]
	verifyPostgresRestoreAdmission(t, external, u.String(), func() {
		if out := post(applyUndo, undo, 201, csrf); out["restored_sha256"] != digest {
			t.Fatalf("full database SHA-256 mismatch: %v", out)
		}
	})
	undo, digest = write()
	undo["approval_token"] = post(planUndo, undo, 200, csrf)["approval_token"]
	exec(external, "INSERT INTO desk_http_recovery VALUES(3,7.50,'external')")
	post(applyUndo, undo, 409, csrf)
	post(planUndo, undo, 409, csrf)
	undo["force"] = true
	force := post(planUndo, undo, 200, csrf)
	undo["approval_token"], undo["baseline_sha256"] = force["approval_token"], force["baseline_sha256"]
	exec(external, "UPDATE desk_http_recovery SET price=4.25 WHERE id=3")
	post(applyUndo, undo, 409, csrf)
	force = post(planUndo, undo, 200, csrf)
	undo["approval_token"], undo["baseline_sha256"] = force["approval_token"], force["baseline_sha256"]
	if out := post(applyUndo, undo, 201, csrf); out["restored_sha256"] != digest || out["forced"] != true {
		t.Fatalf("forced full restore: %v", out)
	}
	var price string
	if err := external.QueryRow("SELECT price FROM prices WHERE id=9007199254740993").Scan(&price); err != nil || price != "0.00" {
		t.Fatalf("restored price %q: %v", price, err)
	}
	t.Logf("HTTP full logical restore SHA-256 %s", digest)
	for _, format := range []string{"csv", "xlsx", "json", "sql"} {
		page := map[string]any{"target": target, "table": "desk_http_recovery", "path": "recovery." + format}
		post("/api/db/export/page", page, 201, csrf)
		exec(external, "DELETE FROM desk_http_recovery")
		plan := post("/api/db/import/plan", page, 200, csrf)
		page["approval_token"] = plan["approval_token"]
		post("/api/db/import/apply", page, 201, csrf)
		var exact string
		var note sql.NullString
		if err := external.QueryRow("SELECT price,note FROM desk_http_recovery WHERE id=9007199254740993").Scan(&exact, &note); err != nil || exact != "0.00" || note.Valid {
			t.Fatalf("%s import changed exact values: %q %v, %v", format, exact, note, err)
		}
		snapshot := plan["snapshot"].(map[string]any)
		restore := map[string]any{"target": target, "snapshot_id": snapshot["snapshot_id"]}
		restore["approval_token"] = post(planUndo, restore, 200, csrf)["approval_token"]
		if out := post(applyUndo, restore, 201, csrf); out["restored_sha256"] != snapshot["before_sha256"] {
			t.Fatalf("%s import rollback digest %v", format, out)
		}
		exec(external, "INSERT INTO desk_http_recovery VALUES(9007199254740993,0.00,NULL)")
	}
	source, err := sql.Open("sqlite", filepath.Join(workspace, "migration-source.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	exec(source, "CREATE TABLE desk_http_recovery (id BIGINT PRIMARY KEY, price DECIMAL(12,2), note TEXT, extra TEXT)")
	comparison := post("/api/db/schema/compare", map[string]any{"source": map[string]any{"engine": "sqlite", "sqlite_path": "migration-source.db"},
		"destination": target}, 200, csrf)
	var migration string
	for _, item := range comparison["changes"].([]any) {
		change := item.(map[string]any)
		if change["object"] == "extra" {
			migration, _ = change["proposed_sql"].(string)
		}
	}
	if migration == "" {
		t.Fatal("missing reviewed migration for the nullable source column")
	}
	request = map[string]any{"target": target, "sql": migration}
	undo, digest = write()
	if _, err := external.Exec("SELECT extra FROM desk_http_recovery"); err != nil {
		t.Fatal(err)
	}
	undo["approval_token"] = post(planUndo, undo, 200, csrf)["approval_token"]
	if out := post(applyUndo, undo, 201, csrf); out["restored_sha256"] != digest {
		t.Fatalf("schema migration rollback digest %v", out)
	}
	exec(external, "SET ROLE desk_logical_writer; CREATE SEQUENCE desk_app.recovery_seq")
	failed := map[string]any{"target": target, "sql": "INSERT INTO desk_http_recovery(id) VALUES(nextval('recovery_seq')); CREATE TABLE desk_http_recovery(id int)"}
	failedPlan := post(planWrite, failed, 200, csrf)
	failed["approval_token"] = failedPlan["approval_token"]
	post(applyWrite, failed, 422, csrf)
	var called bool
	if err := external.QueryRow("SELECT is_called FROM desk_app.recovery_seq").Scan(&called); err != nil || called {
		t.Fatalf("failed mutation advanced a pre-existing sequence: called=%t %v", called, err)
	}
}

func verifyPostgresRestoreAdmission(t *testing.T, db *sql.DB, dsn string, restore func()) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	writer, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Rollback()
	if _, err := writer.ExecContext(ctx, "INSERT INTO desk_http_recovery VALUES (1,7.00,'uncommitted external writer')"); err != nil {
		t.Fatal(err)
	}

	address, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	maintenance := *address
	maintenance.Path = "/postgres"
	admin, err := sql.Open("pgx", maintenance.String())
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	startup := *address
	params := startup.Query()
	params.Set("post_auth_delay", "3")
	startup.RawQuery = params.Encode()
	starting, err := sql.Open("pgx", startup.String())
	if err != nil {
		t.Fatal(err)
	}
	defer starting.Close()
	startupDone := make(chan struct{})
	go func() {
		defer close(startupDone)
		_ = starting.PingContext(ctx)
	}()
	defer func() { <-startupDone }()
	// This backend can pass admission before PostgreSQL closes the database,
	// but it has not yet finished startup. It must also be evicted.
	if err := waitPostgresCheck(ctx, admin, `SELECT EXISTS (SELECT FROM pg_catalog.pg_locks
		WHERE locktype='object' AND classid='pg_catalog.pg_database'::regclass AND mode='RowExclusiveLock'
		AND objid=(SELECT oid FROM pg_catalog.pg_database WHERE datname=$1))`, strings.TrimPrefix(address.Path, "/")); err != nil {
		t.Fatal(err)
	}
	probed := make(chan error, 1)
	go func() {
		if err := waitPostgresCheck(ctx, admin, "SELECT NOT datallowconn FROM pg_catalog.pg_database WHERE datname=$1", strings.TrimPrefix(address.Path, "/")); err != nil {
			probed <- err
			return
		}
		fresh, err := sql.Open("pgx", dsn)
		if err != nil {
			probed <- err
			return
		}
		defer fresh.Close()
		_, err = fresh.ExecContext(ctx, "INSERT INTO desk_http_recovery VALUES (2,8.00,'new external writer')")
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "55000" {
			probed <- fmt.Errorf("expected PostgreSQL to reject the new writer, got %v", err)
			return
		}
		probed <- nil
	}()
	restore()
	if err := <-probed; err != nil {
		t.Fatal(err)
	}
	if err := writer.Commit(); err == nil {
		t.Fatal("external PostgreSQL transaction committed across recovery")
	}
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("PostgreSQL connections did not reopen: %v", err)
	}
}

func waitPostgresCheck(ctx context.Context, db *sql.DB, query string, args ...any) error {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		var ready bool
		if err := db.QueryRowContext(ctx, query, args...).Scan(&ready); err != nil || ready {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

// An advisory lock plus a serializable transaction and a database-wide
// transaction default still allows an ordinary PostgreSQL client to write.
func probePostgresWriteFence(t *testing.T, db *sql.DB, dsn string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	ctl, err := db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer ctl.Close()
	exec := func(statement string) {
		t.Helper()
		if _, err := ctl.ExecContext(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	cleanup := func(statement string) {
		t.Helper()
		cleanupCtx, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		if _, err := ctl.ExecContext(cleanupCtx, statement); err != nil {
			t.Errorf("fixture cleanup: %v", err)
		}
	}
	exec("CREATE ROLE desk_fence_probe LOGIN PASSWORD 'desk-fixture-only'")
	defer cleanup("DROP ROLE desk_fence_probe")
	exec("GRANT SELECT, INSERT, DELETE ON desk_http_recovery TO desk_fence_probe")
	defer cleanup("REVOKE SELECT, INSERT, DELETE ON desk_http_recovery FROM desk_fence_probe")
	exec("ALTER DATABASE desk_fixture SET default_transaction_read_only = on")
	defer cleanup("ALTER DATABASE desk_fixture RESET default_transaction_read_only")
	var locked bool
	if err := ctl.QueryRowContext(ctx, "SELECT pg_try_advisory_lock(106110)").Scan(&locked); err != nil || !locked {
		t.Fatalf("could not acquire fixture advisory lock: %v", err)
	}
	defer cleanup("SELECT pg_advisory_unlock(106110)")
	// This dedicated fixture connection predates the changed database default.
	defer cleanup("DELETE FROM desk_http_recovery WHERE id=1")
	adminTx, err := ctl.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		t.Fatal(err)
	}
	defer adminTx.Rollback()
	addr, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	addr.User = url.UserPassword("desk_fence_probe", "desk-fixture-only")
	writer, err := sql.Open("pgx", addr.String())
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	var readOnly string
	if err := writer.QueryRowContext(ctx, "SHOW default_transaction_read_only").Scan(&readOnly); err != nil || readOnly != "on" {
		t.Fatalf("fixture writer did not inherit the read-only default: %q, %v", readOnly, err)
	}
	writerTx, err := writer.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer writerTx.Rollback()
	if _, err := writerTx.ExecContext(ctx, "SET TRANSACTION READ WRITE"); err != nil {
		t.Fatalf("fixture did not demonstrate that the default is overridable: %v", err)
	}
	if _, err := writerTx.ExecContext(ctx, "INSERT INTO desk_http_recovery VALUES (1,1.00,'bypassed fence')"); err != nil {
		t.Fatalf("fixture did not demonstrate a write through the advisory lock: %v", err)
	}
	if err := writerTx.Commit(); err != nil {
		t.Fatalf("fixture write did not commit during the advisory lock: %v", err)
	}
	t.Log("an ordinary writer committed after overriding the read-only default while the advisory lock and serializable transaction were held")
}

// MariaDB's global read lock blocks the restore connection itself. The
// alternative global read-only setting allows a second privileged writer.
func probeMariaDBWriteFence(t *testing.T, db *sql.DB) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	ctl, err := db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer ctl.Close()
	writer, err := db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	exec := func(conn *sql.Conn, statement string) {
		t.Helper()
		if _, err := conn.ExecContext(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	defer func() {
		cleanupCtx, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		for _, statement := range []string{"UNLOCK TABLES", "SET GLOBAL read_only = OFF", "DELETE FROM desk_http_recovery WHERE id=1", "ALTER TABLE desk_http_recovery DROP COLUMN IF EXISTS native_lock_probe"} {
			if _, err := ctl.ExecContext(cleanupCtx, statement); err != nil {
				t.Errorf("fixture cleanup: %v", err)
			}
		}
	}()
	exec(writer, "SET SESSION lock_wait_timeout = 1")
	defer exec(writer, "SET SESSION lock_wait_timeout = DEFAULT")
	exec(ctl, "FLUSH TABLES WITH READ LOCK")
	_, err = writer.ExecContext(ctx, "ALTER TABLE desk_http_recovery ADD COLUMN native_lock_probe INT")
	var mariaErr *mysql.MySQLError
	if !errors.As(err, &mariaErr) || mariaErr.Number != 1205 {
		t.Fatalf("expected the held global read lock to time out the restore DDL: %v", err)
	}
	exec(ctl, "UNLOCK TABLES")
	exec(ctl, "SET GLOBAL read_only = ON")
	exec(writer, "INSERT INTO desk_http_recovery VALUES (1,1.00,'bypassed fence')")
	t.Log("the held global read lock blocked restore DDL, and a second privileged session wrote with global read_only enabled")
}

func probeMariaDBSwap(t *testing.T, db *sql.DB) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	ctl, err := db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer ctl.Close()
	exec := func(statement string) {
		t.Helper()
		if _, err := ctl.ExecContext(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	exec("CREATE DATABASE desk_fixture__desk_restore")
	defer exec("DROP DATABASE desk_fixture__desk_restore")
	exec("CREATE TABLE desk_swap_probe (id INT PRIMARY KEY) ENGINE=InnoDB")
	defer exec("DROP TABLE desk_swap_probe")
	exec("CREATE TABLE desk_fixture__desk_restore.desk_swap_probe LIKE desk_swap_probe")
	exec("INSERT INTO desk_fixture__desk_restore.desk_swap_probe VALUES (1)")
	swap := `RENAME TABLE desk_swap_probe TO desk_fixture__desk_restore.desk_swap_retired,
		desk_fixture__desk_restore.desk_swap_probe TO desk_swap_probe`
	exec("LOCK TABLES desk_swap_probe WRITE")
	defer exec("UNLOCK TABLES")
	_, err = ctl.ExecContext(ctx, swap)
	var mariaErr *mysql.MySQLError
	if !errors.As(err, &mariaErr) || mariaErr.Number != 1192 {
		t.Fatalf("expected MariaDB to reject RENAME TABLE while holding LOCK TABLES, got %v", err)
	}
	if _, err := db.ExecContext(ctx, "CREATE TABLE desk_swap_concurrent_ddl (id INT PRIMARY KEY) ENGINE=InnoDB"); err != nil {
		t.Fatalf("expected a different table name to permit concurrent DDL, got %v", err)
	}
	defer exec("DROP TABLE desk_swap_concurrent_ddl")
	exec("UNLOCK TABLES")
	if _, err := db.ExecContext(ctx, "INSERT INTO desk_swap_probe VALUES (2)"); err != nil {
		t.Fatal(err)
	}
	exec(swap)
	var restoredID, retiredID int
	if err := ctl.QueryRowContext(ctx, "SELECT id FROM desk_swap_probe").Scan(&restoredID); err != nil || restoredID != 1 {
		t.Fatalf("staged row was not swapped into target: %d, %v", restoredID, err)
	}
	if err := ctl.QueryRowContext(ctx, "SELECT id FROM desk_fixture__desk_restore.desk_swap_retired").Scan(&retiredID); err != nil || retiredID != 2 {
		t.Fatalf("concurrent row was not displaced by the swap: %d, %v", retiredID, err)
	}
	t.Log("the atomic swap displaced a write committed after the table lock was released, and the table lock allowed concurrent DDL at another name")
}

func startDisposableMariaDB(t *testing.T) (portText string, adminPassword string) {
	t.Helper()
	if port := os.Getenv("GROXY_DESK_TEST_MARIADB_LEAST_PRIV_PORT"); port != "" {
		return port, "desk-fixture-only"
	}
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker or GROXY_DESK_TEST_MARIADB_LEAST_PRIV_PORT is required for the least-privilege MariaDB 11.8 fixture")
	}
	seedPassword, err := security.RandomTokenHex(16)
	if err != nil {
		t.Fatal(err)
	}
	nameBytes, err := security.RandomTokenHex(6)
	if err != nil {
		t.Fatal(err)
	}
	containerName := "groxy-desk-mariadb-lp-" + nameBytes
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	out, err := deskexec.CommandContext(ctx, "docker", "run", "--detach", "--rm",
		"--publish", "127.0.0.1::3306", "--name", containerName,
		"--env", "MARIADB_ROOT_PASSWORD="+seedPassword,
		"mariadb:11.8").CombinedOutput()
	if err != nil {
		t.Fatalf("failed starting disposable MariaDB 11.8 container: %v (%s)", err, strings.TrimSpace(string(out)))
	}
	t.Cleanup(func() {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer stopCancel()
		_ = deskexec.CommandContext(stopCtx, "docker", "stop", "--time", "5", containerName).Run()
	})
	portOut, err := deskexec.CommandContext(ctx, "docker", "port", containerName, "3306/tcp").CombinedOutput()
	if err != nil {
		t.Fatalf("failed reading published MariaDB port: %v (%s)", err, strings.TrimSpace(string(portOut)))
	}
	host, portText, err := net.SplitHostPort(strings.TrimSpace(string(portOut)))
	if err != nil || host != "127.0.0.1" {
		t.Fatalf("unexpected published MariaDB address %q: %v", strings.TrimSpace(string(portOut)), err)
	}
	return portText, seedPassword
}

func openMariaTCP(t *testing.T, address, user, password, database string) *sql.DB {
	t.Helper()
	cfg := mysql.NewConfig()
	cfg.User, cfg.Passwd, cfg.Net, cfg.Addr, cfg.DBName = user, password, "tcp", address, database
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	db.SetMaxIdleConns(0)
	return db
}

func waitMariaTCP(t *testing.T, db *sql.DB) {
	t.Helper()
	deadline := time.Now().Add(45 * time.Second)
	for {
		if err := db.Ping(); err == nil {
			return
		} else if time.Now().After(deadline) {
			t.Fatalf("waiting for disposable MariaDB 11.8 fixture: %v", err)
		}
		time.Sleep(250 * time.Millisecond)
	}
}

func verifyFullMariaDBAPI(t *testing.T) {
	portValue, rootPassword := startDisposableMariaDB(t)
	port, err := strconv.ParseUint(portValue, 10, 16)
	if err != nil || port == 0 {
		t.Fatalf("invalid MariaDB least-privilege port %q", portValue)
	}
	address := net.JoinHostPort("127.0.0.1", portValue)

	rootDB := openMariaTCP(t, address, "root", rootPassword, "")
	waitMariaTCP(t, rootDB)

	const targetDB, unrelatedDB = "deskmariadb", "deskunrelated"
	const admissionUser, targetAppUser, unrelatedUser = "admission_admin", "target_app_user", "unrelated_db_user"
	appPassword, err := security.RandomTokenHex(16)
	if err != nil {
		t.Fatal(err)
	}

	execRoot := func(stmt string) {
		t.Helper()
		if _, err := rootDB.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	execRoot("CREATE DATABASE " + targetDB)
	execRoot("CREATE DATABASE " + unrelatedDB)
	execRoot("CREATE USER '" + admissionUser + "'@'%' IDENTIFIED BY '" + appPassword + "'")
	execRoot("CREATE USER '" + targetAppUser + "'@'%' IDENTIFIED BY '" + appPassword + "'")
	execRoot("CREATE USER '" + unrelatedUser + "'@'%' IDENTIFIED BY '" + appPassword + "'")
	execRoot("GRANT ALL PRIVILEGES ON " + targetDB + ".* TO '" + admissionUser + "'@'%' WITH GRANT OPTION")
	execRoot("GRANT CONNECTION ADMIN, PROCESS ON *.* TO '" + admissionUser + "'@'%'")
	execRoot("GRANT SELECT ON mysql.* TO '" + admissionUser + "'@'%'")
	execRoot("GRANT SELECT, INSERT, UPDATE, DELETE, CREATE, DROP, ALTER, INDEX, TRIGGER, CREATE VIEW, SHOW VIEW ON " + targetDB + ".* TO '" + targetAppUser + "'@'%'")
	execRoot("GRANT ALL PRIVILEGES ON " + unrelatedDB + ".* TO '" + unrelatedUser + "'@'%'")

	adminDB := openMariaTCP(t, address, admissionUser, appPassword, targetDB)
	for _, stmt := range []string{
		"CREATE TABLE desk_http_recovery (id BIGINT PRIMARY KEY, price DECIMAL(12,2) NOT NULL, note VARCHAR(40)) ENGINE=InnoDB",
		"INSERT INTO desk_http_recovery VALUES (9007199254740993,0.00,NULL),(9007199254740994,9.00,'')",
		"CREATE VIEW prices AS SELECT id, price FROM desk_http_recovery",
		"CREATE TRIGGER changed BEFORE UPDATE ON desk_http_recovery FOR EACH ROW SET NEW.note='updated'",
	} {
		if _, err := adminDB.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}

	srv, workspace, _ := setupTestServer(t)
	session := srv.Session()
	post := func(path string, body map[string]any, expected int, csrf string) map[string]any {
		t.Helper()
		res, out, raw := doAPIReq(t, http.MethodPost, session.URL+path, session.BearerToken, csrf, body)
		if res.StatusCode != expected {
			t.Fatalf("%s returned %d, want %d: %s", path, res.StatusCode, expected, raw)
		}
		return out
	}
	sshEndpoint := testfixture.PinnedSSHWithSFTP(t, uint32(port), testfixture.ForwardLoopback(uint32(port)))
	tunnel := post("/api/db/tunnels/open", map[string]any{
		"engine": "mariadb", "ssh_host": "127.0.0.1", "ssh_port": sshEndpoint.Port,
		"known_hosts_path": sshEndpoint.KnownHosts, "pinned_fingerprint": sshEndpoint.Fingerprint,
		"remote_db_host": "127.0.0.1", "remote_db_port": portValue,
	}, http.StatusOK, session.CSRFToken)["tunnel"].(map[string]any)

	target := map[string]any{
		"engine": "mariadb", "tunnel_id": tunnel["tunnel_id"], "db_name": targetDB,
		"db_user": admissionUser, "db_password": appPassword,
	}

	// Fail closed with HTTP 409 write_fence_unavailable while any global-write account exists.
	assertFenceUnavailable := func(path string, req map[string]any) {
		t.Helper()
		out := post(path, req, http.StatusConflict, session.CSRFToken)
		if out["error"].(map[string]any)["code"] != security.CodeWriteFenceUnavailable || out["approval_token"] != nil {
			t.Fatalf("%s did not return write_fence_unavailable: %v", path, out)
		}
	}
	assertFenceUnavailable("/api/db/mutate/plan", map[string]any{"target": target, "sql": "UPDATE desk_http_recovery SET price=2.50"})
	assertFenceUnavailable("/api/db/import/plan", map[string]any{"target": target, "path": "data.sql", "table": "desk_http_recovery"})

	// Remove all accounts holding global *.* write privileges so only target-scoped accounts remain.
	rows, err := rootDB.Query(`SELECT DISTINCT GRANTEE FROM information_schema.USER_PRIVILEGES
		WHERE PRIVILEGE_TYPE IN ('INSERT','UPDATE','DELETE','CREATE','DROP','ALTER','INDEX','TRIGGER','CREATE VIEW',
			'CREATE ROUTINE','ALTER ROUTINE','EVENT','SUPER','FILE','SET USER','DELETE HISTORY','CREATE USER',
			'REFERENCES','BINLOG REPLAY','FEDERATED ADMIN','READ_ONLY ADMIN')
		OR IS_GRANTABLE = 'YES'`)
	if err != nil {
		t.Fatal(err)
	}
	var globalAccounts []string
	for rows.Next() {
		var grantee string
		if err := rows.Scan(&grantee); err != nil {
			rows.Close()
			t.Fatal(err)
		}
		globalAccounts = append(globalAccounts, grantee)
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		t.Fatal(err)
	}
	if len(globalAccounts) > 0 {
		execRoot("DROP USER " + strings.Join(globalAccounts, ", "))
	}
	_ = rootDB.Close()

	// Verify target_app_user cannot act as admission_admin (fails closed with 409).
	unprivTarget := map[string]any{
		"engine": "mariadb", "tunnel_id": tunnel["tunnel_id"], "db_name": targetDB,
		"db_user": targetAppUser, "db_password": appPassword,
	}
	assertFenceUnavailable("/api/db/mutate/plan", map[string]any{"target": unprivTarget, "sql": "UPDATE desk_http_recovery SET price=2.50"})

	appDB := openMariaTCP(t, address, targetAppUser, appPassword, targetDB)
	unrelatedDBConn := openMariaTCP(t, address, unrelatedUser, appPassword, unrelatedDB)
	unrelatedConn, err := unrelatedDBConn.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer unrelatedConn.Close()
	if _, err := unrelatedConn.ExecContext(context.Background(), "CREATE TABLE unrelated_tbl (id INT PRIMARY KEY, label VARCHAR(40)) ENGINE=InnoDB"); err != nil {
		t.Fatal(err)
	}
	if _, err := unrelatedConn.ExecContext(context.Background(), "INSERT INTO unrelated_tbl VALUES (1,'intact')"); err != nil {
		t.Fatal(err)
	}
	var unrelatedIDBefore uint64
	if err := unrelatedConn.QueryRowContext(context.Background(), "SELECT CONNECTION_ID()").Scan(&unrelatedIDBefore); err != nil {
		t.Fatal(err)
	}
	assertUnrelatedIntact := func() {
		t.Helper()
		var idAfter uint64
		var dbName, label string
		if err := unrelatedConn.QueryRowContext(context.Background(), "SELECT CONNECTION_ID(), DATABASE()").Scan(&idAfter, &dbName); err != nil || idAfter != unrelatedIDBefore || dbName != unrelatedDB {
			t.Fatalf("unrelated connection was disturbed: id %d->%d db=%q err=%v", unrelatedIDBefore, idAfter, dbName, err)
		}
		if err := unrelatedConn.QueryRowContext(context.Background(), "SELECT label FROM unrelated_tbl WHERE id=1").Scan(&label); err != nil || label != "intact" {
			t.Fatalf("unrelated table changed: %q, %v", label, err)
		}
	}

	const planWrite, applyWrite = "/api/db/mutate/plan", "/api/db/mutate/apply"
	const planUndo, applyUndo = "/api/db/snapshots/restore/plan", "/api/db/snapshots/restore"
	request := map[string]any{"target": target, "sql": "UPDATE desk_http_recovery SET price=2.50; DROP VIEW prices; CREATE TABLE later(id int) ENGINE=InnoDB"}
	write := func() (map[string]any, string) {
		t.Helper()
		plan := post(planWrite, request, 200, session.CSRFToken)
		request["approval_token"] = plan["approval_token"]
		snapshot := plan["snapshot"].(map[string]any)
		post(applyWrite, request, 201, session.CSRFToken)
		post(applyWrite, request, 409, session.CSRFToken)
		return map[string]any{"target": target, "snapshot_id": snapshot["snapshot_id"]}, snapshot["before_sha256"].(string)
	}

	// Open active target_app_user sessions (an uncommitted transaction, an idle target connection,
	// and a session without a default database holding an open target transaction) and verify
	// all are evicted during recovery while unrelated_db_user stays connected.
	noDB := openMariaTCP(t, address, targetAppUser, appPassword, "")
	verifyWriterExcluded := func(fn func()) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		appTx, err := appDB.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer appTx.Rollback()
		if _, err := appTx.ExecContext(ctx, "INSERT INTO desk_http_recovery (id, price, note) VALUES (1,7.00,'uncommitted external writer')"); err != nil {
			t.Fatal(err)
		}
		noDBTx, err := noDB.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer noDBTx.Rollback()
		if _, err := noDBTx.ExecContext(ctx, "INSERT INTO "+targetDB+".desk_http_recovery (id, price, note) VALUES (2,8.00,'qualified external writer')"); err != nil {
			t.Fatal(err)
		}
		idleTargetConn, err := appDB.Conn(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer idleTargetConn.Close()
		var idlePID uint64
		if err := idleTargetConn.QueryRowContext(ctx, "SELECT CONNECTION_ID()").Scan(&idlePID); err != nil {
			t.Fatal(err)
		}
		fn()
		if err := appTx.Commit(); err == nil {
			t.Fatal("target_app_user transaction committed across MariaDB recovery")
		}
		if err := noDBTx.Commit(); err == nil {
			t.Fatal("qualified target_app_user transaction without default DB committed across MariaDB recovery")
		}
		if err := idleTargetConn.PingContext(ctx); err == nil {
			t.Fatalf("idle target_app_user connection %d on target database was not evicted", idlePID)
		}
		var count int
		if err := appDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM desk_http_recovery WHERE id IN (1,2)").Scan(&count); err != nil || count != 0 {
			t.Fatalf("evicted uncommitted row appeared or grants not restored: count=%d err=%v", count, err)
		}
		assertUnrelatedIntact()
	}

	undo, digest := write()
	undo["approval_token"] = post(planUndo, undo, 200, session.CSRFToken)["approval_token"]
	verifyWriterExcluded(func() {
		if out := post(applyUndo, undo, 201, session.CSRFToken); out["restored_sha256"] != digest {
			t.Fatalf("MariaDB full logical restore SHA-256 mismatch: %v", out)
		}
	})

	// Newer changes prior to restore trigger 409 conflict unless explicit force approval is granted.
	undo, digest = write()
	undo["approval_token"] = post(planUndo, undo, 200, session.CSRFToken)["approval_token"]
	if _, err := appDB.Exec("INSERT INTO desk_http_recovery VALUES(3,7.50,'external')"); err != nil {
		t.Fatal(err)
	}
	post(applyUndo, undo, 409, session.CSRFToken)
	post(planUndo, undo, 409, session.CSRFToken)
	undo["force"] = true
	force := post(planUndo, undo, 200, session.CSRFToken)
	undo["approval_token"], undo["baseline_sha256"] = force["approval_token"], force["baseline_sha256"]
	if _, err := appDB.Exec("UPDATE desk_http_recovery SET price=4.25 WHERE id=3"); err != nil {
		t.Fatal(err)
	}
	post(applyUndo, undo, 409, session.CSRFToken)
	force = post(planUndo, undo, 200, session.CSRFToken)
	undo["approval_token"], undo["baseline_sha256"] = force["approval_token"], force["baseline_sha256"]
	verifyWriterExcluded(func() {
		if out := post(applyUndo, undo, 201, session.CSRFToken); out["restored_sha256"] != digest || out["forced"] != true {
			t.Fatalf("forced MariaDB full restore failed: %v", out)
		}
	})
	var price string
	if err := appDB.QueryRow("SELECT price FROM prices WHERE id=9007199254740993").Scan(&price); err != nil || price != "0.00" {
		t.Fatalf("restored view price %q: %v", price, err)
	}

	// CSV, XLSX, JSON, SQL imports + rollback through Desk HTTP endpoints.
	for _, format := range []string{"csv", "xlsx", "json", "sql"} {
		page := map[string]any{"target": target, "table": "desk_http_recovery", "path": "maria-recovery." + format}
		post("/api/db/export/page", page, 201, session.CSRFToken)
		if _, err := appDB.Exec("DELETE FROM desk_http_recovery"); err != nil {
			t.Fatal(err)
		}
		plan := post("/api/db/import/plan", page, 200, session.CSRFToken)
		page["approval_token"] = plan["approval_token"]
		post("/api/db/import/apply", page, 201, session.CSRFToken)
		var exact string
		var note sql.NullString
		if err := appDB.QueryRow("SELECT price,note FROM desk_http_recovery WHERE id=9007199254740993").Scan(&exact, &note); err != nil || exact != "0.00" || note.Valid {
			t.Fatalf("MariaDB %s import changed exact values: %q %v, %v", format, exact, note, err)
		}
		snapshot := plan["snapshot"].(map[string]any)
		restore := map[string]any{"target": target, "snapshot_id": snapshot["snapshot_id"]}
		restore["approval_token"] = post(planUndo, restore, 200, session.CSRFToken)["approval_token"]
		if out := post(applyUndo, restore, 201, session.CSRFToken); out["restored_sha256"] != snapshot["before_sha256"] {
			t.Fatalf("MariaDB %s import rollback digest %v", format, out)
		}
		if _, err := appDB.Exec("INSERT INTO desk_http_recovery VALUES(9007199254740993,0.00,NULL),(9007199254740994,9.00,'')"); err != nil {
			t.Fatal(err)
		}
	}

	// Cross-dialect schema-sync migration + rollback through Desk HTTP endpoints.
	source, err := sql.Open("sqlite", filepath.Join(workspace, "maria-migration-source.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	if _, err := source.Exec("CREATE TABLE desk_http_recovery (id BIGINT PRIMARY KEY, price DECIMAL(12,2), note TEXT, extra TEXT)"); err != nil {
		t.Fatal(err)
	}
	comparison := post("/api/db/schema/compare", map[string]any{
		"source":      map[string]any{"engine": "sqlite", "sqlite_path": "maria-migration-source.db"},
		"destination": target,
	}, 200, session.CSRFToken)
	var migration string
	for _, item := range comparison["changes"].([]any) {
		change := item.(map[string]any)
		if change["object"] == "extra" {
			migration, _ = change["proposed_sql"].(string)
		}
	}
	if migration == "" {
		t.Fatal("missing reviewed MariaDB migration for nullable source column")
	}
	request = map[string]any{"target": target, "sql": migration}
	undo, digest = write()
	if _, err := appDB.Exec("SELECT extra FROM desk_http_recovery"); err != nil {
		t.Fatal(err)
	}
	undo["approval_token"] = post(planUndo, undo, 200, session.CSRFToken)["approval_token"]
	if out := post(applyUndo, undo, 201, session.CSRFToken); out["restored_sha256"] != digest {
		t.Fatalf("MariaDB schema migration rollback digest %v", out)
	}

	// Partial DDL failure through Desk's approved SQL/DDL path -> HTTP 422 with retained snapshot ->
	// rollback through Desk's rollback endpoint to the exact pre-mutation SHA-256 (tables, views, triggers intact).
	partialReq := map[string]any{
		"target": target,
		"sql":    "ALTER TABLE desk_http_recovery ADD COLUMN partial_change INT; UPDATE desk_http_recovery SET price=1.25 WHERE id=9007199254740993; ALTER TABLE desk_http_recovery ADD COLUMN partial_change INT",
	}
	partialPlan := post(planWrite, partialReq, 200, session.CSRFToken)
	partialSnapID := partialPlan["snapshot"].(map[string]any)["snapshot_id"].(string)
	preFailureSHA := partialPlan["snapshot"].(map[string]any)["before_sha256"].(string)
	partialReq["approval_token"] = partialPlan["approval_token"]
	partialOut := post(applyWrite, partialReq, 422, session.CSRFToken)
	if !strings.Contains(fmt.Sprint(partialOut["error"]), partialSnapID) {
		t.Fatalf("partial DDL failure did not surface retained snapshot %s: %v", partialSnapID, partialOut)
	}
	var partialCol string
	if err := appDB.QueryRow("SHOW COLUMNS FROM desk_http_recovery LIKE 'partial_change'").Scan(&partialCol, new(string), new(string), new(string), new(sql.NullString), new(string)); err != nil || partialCol != "partial_change" {
		t.Fatalf("first DDL did not persist before second DDL failure: %q, %v", partialCol, err)
	}
	var partialNote sql.NullString
	if err := appDB.QueryRow("SELECT price, note FROM desk_http_recovery WHERE id=9007199254740993").Scan(&price, &partialNote); err != nil || price != "1.25" || !partialNote.Valid || partialNote.String != "updated" {
		t.Fatalf("intervening DML or trigger did not persist on partial DDL failure: price=%q note=%v err=%v", price, partialNote, err)
	}
	assertUnrelatedIntact()

	partialUndo := map[string]any{"target": target, "snapshot_id": partialSnapID}
	partialUndo["approval_token"] = post(planUndo, partialUndo, 200, session.CSRFToken)["approval_token"]
	verifyWriterExcluded(func() {
		restored := post(applyUndo, partialUndo, 201, session.CSRFToken)
		if restored["restored_sha256"] != preFailureSHA {
			t.Fatalf("partial DDL rollback SHA-256 %v, want %s", restored["restored_sha256"], preFailureSHA)
		}
	})
	var colCount int
	if err := appDB.QueryRow("SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=? AND TABLE_NAME='desk_http_recovery' AND COLUMN_NAME='partial_change'", targetDB).Scan(&colCount); err != nil || colCount != 0 {
		t.Fatalf("partial_change column survived rollback: count=%d err=%v", colCount, err)
	}
	var noteAfter sql.NullString
	if err := appDB.QueryRow("SELECT p.price, t.note FROM prices p JOIN desk_http_recovery t ON t.id=p.id WHERE p.id=9007199254740993").Scan(&price, &noteAfter); err != nil || price != "0.00" || noteAfter.Valid {
		t.Fatalf("restored table/view mismatch after partial DDL rollback: price=%q note=%v err=%v", price, noteAfter, err)
	}
	// Verify trigger survived partial DDL rollback intact.
	tx, err := appDB.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec("UPDATE desk_http_recovery SET price=3.00 WHERE id=9007199254740993"); err != nil {
		_ = tx.Rollback()
		t.Fatal(err)
	}
	if err := tx.QueryRow("SELECT note FROM desk_http_recovery WHERE id=9007199254740993").Scan(&noteAfter); err != nil || !noteAfter.Valid || noteAfter.String != "updated" {
		_ = tx.Rollback()
		t.Fatalf("trigger did not fire after partial DDL rollback: note=%v err=%v", noteAfter, err)
	}
	_ = tx.Rollback()
	assertUnrelatedIntact()
	t.Logf("HTTP MariaDB least-privilege recovery and partial-DDL rollback verified (%s)", preFailureSHA)
}
