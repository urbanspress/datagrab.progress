package dbstudio

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"groxy-desk/internal/groxybridge"
	"groxy-desk/internal/security"
	"groxy-desk/internal/testfixture"
)

// TestPinnedDatabaseCatalogs uses disposable DB fixtures published on loopback.
func TestPinnedDatabaseCatalogs(t *testing.T) {
	for _, fixture := range []struct{ engine, portEnv, user string }{
		{"postgres", "GROXY_DESK_TEST_POSTGRES_PORT", "postgres"},
		{"mariadb", "GROXY_DESK_TEST_MARIADB_PORT", "root"},
		{"mysql", "GROXY_DESK_TEST_MYSQL_PORT", "root"},
	} {
		t.Run(fixture.engine, func(t *testing.T) {
			configuredPort := os.Getenv(fixture.portEnv)
			if configuredPort == "" {
				t.Skipf("%s is required for the disposable database fixture", fixture.portEnv)
			}
			port, err := strconv.ParseUint(configuredPort, 10, 16)
			if err != nil || port == 0 {
				t.Fatalf("invalid fixture port %q", configuredPort)
			}
			sshEndpoint := testfixture.PinnedSSHWithSFTP(t, uint32(port), testfixture.ForwardLoopback(uint32(port)))
			req := OpenTunnelRequest{Engine: fixture.engine, SSHHost: "127.0.0.1", SSHPort: sshEndpoint.Port,
				KnownHostsPath: sshEndpoint.KnownHosts, PinnedFingerprint: sshEndpoint.Fingerprint,
				RemoteDBHost: "127.0.0.1", RemoteDBPort: configuredPort}
			approvalState := t.TempDir()
			approvals, err := groxybridge.NewApprovalManager(approvalState, nil)
			if err != nil {
				t.Fatal(err)
			}
			m := NewManager(t.TempDir(), approvals, nil)
			defer m.CloseAll()
			tunnel, err := m.Tunnels().OpenTunnel(req)
			if err != nil {
				t.Fatal(err)
			}
			target := TargetSpec{Engine: fixture.engine, TunnelID: tunnel.TunnelID, DBName: "desk_fixture", DBUser: fixture.user, DBPassword: "desk-fixture-only"}
			db, _, _, err := m.openTarget(target)
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			db.SetMaxIdleConns(0)
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			for {
				err := db.PingContext(ctx)
				if err == nil {
					break
				}
				if ctx.Err() != nil {
					t.Fatalf("waiting for disposable DB fixture: %v", err)
				}
				time.Sleep(100 * time.Millisecond)
			}
			for _, statement := range []string{
				"CREATE TABLE authors (id BIGINT PRIMARY KEY, name VARCHAR(80) NOT NULL)",
				"CREATE TABLE books (id BIGINT PRIMARY KEY, author BIGINT, price DECIMAL(12,2) NOT NULL, FOREIGN KEY (author) REFERENCES authors(id))",
				"CREATE INDEX books_by_author ON books(author)",
				"INSERT INTO authors VALUES (1,'Ada')",
				"INSERT INTO books VALUES (9007199254740993,1,0.00)",
			} {
				if _, err := db.ExecContext(ctx, statement); err != nil {
					t.Fatal(err)
				}
			}
			catalog, err := m.GetSchema(ctx, target)
			if err != nil {
				t.Fatal(err)
			}
			tables := catalog["tables"].([]TableSchema)
			if len(tables) != 2 || tables[1].Name != "books" || tables[1].RowCount != "1" || !tables[1].Columns[0].PrimaryKey ||
				len(tables[1].Indexes) < 2 || len(tables[1].ForeignKeys) != 1 || tables[1].ForeignKeys[0].RefTable != "authors" {
				t.Fatalf("unexpected %s catalog: %+v", fixture.engine, tables)
			}
			output, err := m.BrowseRows(ctx, RowsRequest{Target: target, Table: "books", Page: "1", Limit: "10"})
			if err != nil {
				t.Fatal(err)
			}
			if got := fmt.Sprint(output["rows"]); got != "[map[author:1 id:9007199254740993 price:0.00]]" {
				t.Fatalf("lost exact %s row: %v", fixture.engine, got)
			}
			verifyNetworkExports(t, ctx, m, target)
			verifyCrossDialectCompare(t, ctx, m, target)
			if _, err := m.PlanMutation(ctx, MutatePlanRequest{Target: target, SQL: "DELETE FROM books"}); security.ErrorCode(err, "") != security.CodeWriteFenceUnavailable {
				t.Fatalf("expected the network write plan to require a write fence: %v", err)
			}
			verifyPinnedCellRecovery(t, ctx, db, m, target, approvalState)
			verifyNetworkSQLPage(t, ctx, db, m, target)
		})
	}
}

func verifyNetworkExports(t *testing.T, ctx context.Context, m *Manager, target TargetSpec) {
	t.Helper()
	diagram, err := m.ExportDiagram(ctx, ExportDiagramRequest{Target: target, Path: "catalog.html"})
	if err != nil || diagram["tables_exported"] != "2" {
		t.Fatalf("export ER diagram: %v, %v", diagram, err)
	}
	raw, err := os.ReadFile(filepath.Join(m.workspaceRoot, "catalog.html"))
	if err != nil || !strings.Contains(string(raw), `href="#table-1"`) ||
		!strings.Contains(string(raw), ".authors.id") || !strings.Contains(string(raw), "books_by_author") {
		t.Fatalf("diagram lost the foreign key or index: %v", err)
	}

	for _, format := range []struct {
		name string
		read func([]byte) (importTable, error)
	}{
		{"csv", readCSVRows}, {"json", readJSONRows}, {"xlsx", readExcelRows}, {"sql", nil},
	} {
		path := "books." + format.name
		if _, err := m.ExportPage(ctx, ExportPageRequest{RowsRequest: RowsRequest{Target: target, Table: "books"}, Path: path}); err != nil {
			t.Fatal(err)
		}
		body, err := os.ReadFile(filepath.Join(m.workspaceRoot, path))
		if err != nil {
			t.Fatal(err)
		}
		if format.read != nil {
			page, err := format.read(body)
			if err != nil || len(page.columns) != 3 || len(page.rows) != 1 {
				t.Fatalf("%s exported table could not be read: %v", format.name, err)
			}
			values := make(map[string]string, len(page.columns))
			for i, column := range page.columns {
				values[column] = string(page.rows[0][i])
			}
			if values["id"] != `"9007199254740993"` || values["price"] != `"0.00"` {
				t.Fatalf("%s export lost exact numeric strings: %v", format.name, values)
			}
		} else if !bytes.Contains(body, []byte("9007199254740993")) || !bytes.Contains(body, []byte(", 0.00);")) {
			t.Fatal("SQL dump lost exact numeric lexemes")
		}
		if _, err := m.PlanImport(ctx, ImportRequest{Target: target, Path: path, Table: "books"}); security.ErrorStatus(err, 0) != 409 || security.ErrorCode(err, "") != security.CodeWriteFenceUnavailable {
			t.Fatalf("%s network import must stay gated until full recovery is verified: %v", format.name, err)
		}
	}
}

func verifyCrossDialectCompare(t *testing.T, ctx context.Context, m *Manager, network TargetSpec) {
	t.Helper()
	local := TargetSpec{Engine: "sqlite", SQLitePath: "compare.db"}
	db, err := sql.Open("sqlite", filepath.Join(m.workspaceRoot, "compare.db"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, "CREATE TABLE books (id INTEGER PRIMARY KEY, price TEXT, edition TEXT)"); err != nil {
		_ = db.Close()
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	before, err := m.CompareSchema(ctx, CompareRequest{Source: local, Destination: network})
	if err != nil {
		t.Fatal(err)
	}
	changes := before["changes"].([]SchemaChange)
	foundEdition := false
	for _, change := range changes {
		foundEdition = foundEdition || change.Table == "books" && change.Object == "edition" && change.Kind == "column_missing_in_destination"
	}
	if !foundEdition {
		t.Fatalf("cross-dialect comparison missed the source-only column: %+v", changes)
	}
	if _, err := m.RunReadOnlyQuery(ctx, network, "DELETE FROM books"); err == nil {
		t.Fatal("a read-only session accepted a write")
	}
	after, err := m.CompareSchema(ctx, CompareRequest{Source: local, Destination: network})
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("schema comparison or rejected write changed a catalog: %v", err)
	}
}

func verifyNetworkSQLPage(t *testing.T, ctx context.Context, db *sql.DB, m *Manager, target TargetSpec) {
	t.Helper()
	const label = "Édition \\ d'Ada"
	placeholder := "?"
	if target.Engine == "postgres" {
		placeholder = "$1"
	}
	if _, err := db.ExecContext(ctx, "UPDATE inventory SET note="+placeholder+" WHERE id=9007199254740994", label); err != nil {
		t.Fatal(err)
	}
	request := ExportPageRequest{RowsRequest: RowsRequest{Target: target, Table: "inventory", SortCol: "id"}, Path: "inventory.sql"}
	if _, err := m.ExportPage(ctx, request); err != nil {
		t.Fatal(err)
	}
	plan, err := os.ReadFile(filepath.Join(m.workspaceRoot, request.Path))
	if err != nil {
		t.Fatal(err)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, "DELETE FROM inventory"); err != nil {
		t.Fatal(err)
	}
	for _, statement := range bytes.Split(bytes.TrimSpace(plan), []byte("\n")) {
		if _, err := tx.ExecContext(ctx, string(statement)); err != nil {
			t.Fatal(err)
		}
	}
	var price string
	var note sql.NullString
	if err := tx.QueryRowContext(ctx, "SELECT price, note FROM inventory WHERE id=9007199254740993").Scan(&price, &note); err != nil || price != "0.00" || note.Valid {
		t.Fatalf("exact decimal and NULL did not round-trip: %q, %v, %v", price, note, err)
	}
	if err := tx.QueryRowContext(ctx, "SELECT price, note FROM inventory WHERE id=9007199254740994").Scan(&price, &note); err != nil || price != "9.00" || !note.Valid || note.String != label {
		t.Fatalf("exact decimal and text did not round-trip: %q, %v, %v", price, note, err)
	}
}

func pinnedLoopbackFixture(t *testing.T, port uint32) OpenTunnelRequest {
	t.Helper()
	request, _ := pinnedSSHFixture(t, port, testfixture.ForwardLoopback(port))
	return request
}

func verifyPinnedCellRecovery(t *testing.T, ctx context.Context, db *sql.DB, m *Manager, target TargetSpec, approvalState string) {
	t.Helper()
	for _, statement := range []string{
		"CREATE TABLE inventory (id BIGINT PRIMARY KEY, price DECIMAL(12,2) NOT NULL, note TEXT)",
		"CREATE UNIQUE INDEX inventory_price ON inventory(price)",
		"INSERT INTO inventory VALUES (9007199254740993,0.00,NULL)",
		"INSERT INTO inventory VALUES (9007199254740994,9.00,'')",
	} {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	edit := CellEdit{Table: "inventory", Column: "price", Key: map[string]json.RawMessage{"id": json.RawMessage(`"9007199254740993"`)},
		Before: json.RawMessage(`"0.00"`), Value: json.RawMessage(`"1.00"`), ValueKind: "decimal"}
	for _, bad := range []string{`0.1`, `"0e0"`, `"1.001"`} {
		invalid := edit
		invalid.Value = json.RawMessage(bad)
		if _, err := m.PlanMutation(ctx, MutatePlanRequest{Target: target, Edit: &invalid}); security.ErrorStatus(err, 0) != 422 {
			t.Fatalf("invalid numeric value %s returned %v", bad, err)
		}
	}
	verifyRejectedNetworkEdits(t, ctx, db, m, target, approvalState, edit)
	plan, err := m.PlanMutation(ctx, MutatePlanRequest{Target: target, Edit: &edit})
	if err != nil {
		t.Fatal(err)
	}
	token := plan["approval_token"].(string)
	wrong := edit
	wrong.Value = json.RawMessage(`"2.00"`)
	if _, err := m.ApplyMutation(ctx, MutateApplyRequest{Target: target, Edit: &wrong, ApprovalToken: token}); security.ErrorStatus(err, 0) != 403 {
		t.Fatalf("wrong approved operation returned %v", err)
	}
	results := make(chan error, 2)
	for range 2 {
		go func() {
			_, err := m.ApplyMutation(ctx, MutateApplyRequest{Target: target, Edit: &edit, ApprovalToken: token})
			results <- err
		}()
	}
	a, b := <-results, <-results
	if a == nil && security.ErrorStatus(b, 0) == 409 || b == nil && security.ErrorStatus(a, 0) == 409 {
		checkNetworkPrice(t, ctx, db, "1.00")
	} else {
		t.Fatalf("expected one approved write and one replay rejection, got %v and %v", a, b)
	}
	request := RestoreRequest{Target: target, SnapshotID: plan["snapshot"].(groxybridge.SnapshotRecord).SnapshotID}
	restorePlan, err := m.PlanRestore(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	request.ApprovalToken = restorePlan["approval_token"].(string)
	if _, err := db.ExecContext(ctx, "UPDATE inventory SET note='later edit' WHERE id=9007199254740994"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.RestoreSnapshot(ctx, request); security.ErrorStatus(err, 0) != 409 {
		t.Fatalf("restore did not reject a newer database change: %v", err)
	}
	checkNetworkPrice(t, ctx, db, "1.00")
	var later string
	if err := db.QueryRowContext(ctx, "SELECT note FROM inventory WHERE id=9007199254740994").Scan(&later); err != nil || later != "later edit" {
		t.Fatalf("newer database change was overwritten: %q, %v", later, err)
	}
	if _, err := db.ExecContext(ctx, "UPDATE inventory SET note='' WHERE id=9007199254740994"); err != nil {
		t.Fatal(err)
	}
	restorePlan, err = m.PlanRestore(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	request.ApprovalToken = restorePlan["approval_token"].(string)
	if _, err := m.RestoreSnapshot(ctx, request); err != nil {
		t.Fatal(err)
	}
	checkNetworkPrice(t, ctx, db, "0.00")
	if _, err := db.ExecContext(ctx, "UPDATE inventory SET price=9.00 WHERE id=9007199254740993"); err == nil {
		t.Fatal("recovered unique index failed to preserve the uniqueness rule")
	}
}

func verifyRejectedNetworkEdits(t *testing.T, ctx context.Context, db *sql.DB, m *Manager, target TargetSpec, approvalState string, edit CellEdit) {
	t.Helper()
	for _, scenario := range []struct {
		name  string
		value json.RawMessage
	}{
		{"corrupt backup", json.RawMessage(`"1.00"`)},
		{"unique constraint violation", json.RawMessage(`"9.00"`)},
	} {
		attempt := edit
		attempt.Value = scenario.value
		plan, err := m.PlanMutation(ctx, MutatePlanRequest{Target: target, Edit: &attempt})
		if err != nil {
			t.Fatal(err)
		}
		snapshotID := plan["snapshot"].(groxybridge.SnapshotRecord).SnapshotID
		if scenario.name == "corrupt backup" {
			path := filepath.Join(approvalState, "snapshots", snapshotID+".bak")
			if err := os.WriteFile(path, []byte("invalid snapshot"), 0600); err != nil {
				t.Fatal(err)
			}
		}
		apply := MutateApplyRequest{Target: target, Edit: &attempt, ApprovalToken: plan["approval_token"].(string)}
		if _, err := m.ApplyMutation(ctx, apply); security.ErrorStatus(err, 0) != 422 {
			t.Fatalf("%s returned %v", scenario.name, err)
		}
		if _, err := m.ApplyMutation(ctx, apply); security.ErrorStatus(err, 0) != 409 {
			t.Fatalf("%s allowed an approval replay: %v", scenario.name, err)
		}
		checkNetworkPrice(t, ctx, db, "0.00")
	}
}

func checkNetworkPrice(t *testing.T, ctx context.Context, db *sql.DB, want string) {
	t.Helper()
	var price string
	var note sql.NullString
	if err := db.QueryRowContext(ctx, "SELECT price,note FROM inventory WHERE id=9007199254740993").Scan(&price, &note); err != nil || price != want || note.Valid {
		t.Fatalf("price=%q note=%v err=%v want %s and null", price, note, err, want)
	}
}
