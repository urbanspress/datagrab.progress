package dbstudio

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"groxy-desk/internal/security"

	"golang.org/x/crypto/ssh"
)

func TestRemoteSQLiteBridgeUsesReadCapabilityOverPinnedSSH(t *testing.T) {
	const bearer = "fixture-gashboard-bearer"
	const schema = `{"ok":true,"engine":"sqlite","tables":[{"name":"books","schema":"main","row_count":"1","indexes":[{"name":"books_price"}],"columns":[{"name":"price","type":"TEXT"}]}]}`
	const rows = `{"ok":true,"engine":"sqlite","table":"books","page":"1","limit":"10","total_rows":"1","total_pages":"1","rows":[{"price":"0.00","id":"9007199254740993","note":null}]}`
	requests := make(chan string, 4)
	sshConfig, _ := pinnedSSHFixture(t, 9760, func(ch ssh.Channel) {
		defer ch.Close()
		request, err := http.ReadRequest(bufio.NewReader(ch))
		if err != nil {
			return
		}
		defer request.Body.Close()
		var body map[string]json.RawMessage
		if request.Method != http.MethodPost || request.Header.Get("Authorization") != "Bearer "+bearer ||
			json.NewDecoder(request.Body).Decode(&body) != nil {
			return
		}
		response := schema
		target := body
		if request.URL.Path == "/v1/db/rows" {
			response = rows
			target = nil
			if err := json.Unmarshal(body["target"], &target); err != nil {
				return
			}
		}
		if len(target) != 2 || string(target["engine"]) != `"sqlite"` || string(target["sqlite_path"]) != `"demo_store.db"` {
			return
		}
		requests <- request.URL.Path
		_, _ = fmt.Fprintf(ch, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: %d\r\nConnection: close\r\n\r\n%s", len(response), response)
	})
	sshConfig.Engine = "sqlite-bridge"
	m := NewManager(t.TempDir(), nil, nil)
	t.Cleanup(m.CloseAll)
	tunnel, err := m.Tunnels().OpenTunnel(sshConfig)
	if err != nil {
		t.Fatal(err)
	}
	target := TargetSpec{Engine: "sqlite-bridge", TunnelID: tunnel.TunnelID, SQLitePath: "demo_store.db", BridgeBearer: bearer}
	ctx := context.Background()
	result, err := m.GetSchema(ctx, target)
	if err != nil {
		t.Fatal(err)
	}
	if tables := result["tables"].([]TableSchema); len(tables) != 1 || strings.Join(tables[0].Indexes, "") != "books_price" {
		t.Fatalf("unexpected schema: %+v", tables)
	}
	result, err = m.BrowseRows(ctx, RowsRequest{Target: target, Table: "books", Page: "1", Limit: "10"})
	if err != nil {
		t.Fatal(err)
	}
	got := result["rows"].([]map[string]any)
	if len(got) != 1 || got[0]["price"] != "0.00" || got[0]["id"] != "9007199254740993" || got[0]["note"] != nil {
		t.Fatalf("unexpected remote SQLite row: %#v", got)
	}
	if <-requests != "/v1/db/schema" || <-requests != "/v1/db/rows" {
		t.Fatal("Desk called an unexpected bridge route")
	}
	if _, err := m.PlanMutation(ctx, MutatePlanRequest{Target: target, SQL: "DELETE FROM books"}); security.ErrorStatus(err, 0) != 403 {
		t.Fatalf("remote SQLite mutation returned %v", err)
	}
	target.SQLitePath = "../demo_store.db"
	if _, err := m.GetSchema(ctx, target); security.ErrorCode(err, "") != security.CodeGrabE401 {
		t.Fatalf("remote SQLite traversal returned %v", err)
	}
}

func TestRemoteSQLiteWithFrozenGashboard(t *testing.T) {
	bin := os.Getenv("GROXY_DESK_TEST_GASHBOARD_BIN")
	if bin == "" {
		t.Skip("set GROXY_DESK_TEST_GASHBOARD_BIN to the frozen v1.0.0 bridge executable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	version, err := exec.CommandContext(ctx, bin, "--version").Output()
	if err != nil || strings.TrimSpace(string(version)) != "gashboard-bridge v1.0.0" {
		t.Fatalf("expected frozen Gashboard v1.0.0, got %q: %v", version, err)
	}
	root := t.TempDir()
	db, err := sql.Open("sqlite", filepath.Join(root, "fixture.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.ExecContext(ctx, `CREATE TABLE books (id INTEGER PRIMARY KEY, price TEXT DEFAULT '0.00', note TEXT);
		INSERT INTO books (id) VALUES (9007199254740993);`); err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().(*net.TCPAddr)
	_ = listener.Close()
	const bearer = "gashboard-read-only-fixture"
	command := exec.CommandContext(ctx, bin, "--listen", addr.String(), "--sqlite-root", root)
	command.Env = append(os.Environ(), "GASHBOARD_BRIDGE_TOKEN="+bearer)
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = command.Process.Kill(); _ = command.Wait() }()
	for {
		conn, err := net.DialTimeout("tcp4", addr.String(), 100*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			break
		}
		if ctx.Err() != nil {
			t.Fatalf("Gashboard bridge startup: %v", err)
		}
		time.Sleep(100 * time.Millisecond)
	}
	request := pinnedLoopbackFixture(t, uint32(addr.Port))
	request.Engine = "sqlite-bridge"
	m := NewManager(t.TempDir(), nil, nil)
	t.Cleanup(m.CloseAll)
	tunnel, err := m.Tunnels().OpenTunnel(request)
	if err != nil {
		t.Fatal(err)
	}
	target := TargetSpec{Engine: "sqlite-bridge", TunnelID: tunnel.TunnelID, SQLitePath: "fixture.db", BridgeBearer: bearer}
	schema, err := m.GetSchema(ctx, target)
	if err != nil {
		t.Fatal(err)
	}
	if tables := schema["tables"].([]TableSchema); len(tables) != 1 || len(tables[0].Columns) != 3 || tables[0].Columns[1].DefaultVal != "'0.00'" {
		t.Fatalf("unexpected frozen bridge schema: %#v", schema)
	}
	result, err := m.BrowseRows(ctx, RowsRequest{Target: target, Table: "books", Page: "1", Limit: "10"})
	if err != nil {
		t.Fatal(err)
	}
	if rows := result["rows"].([]map[string]any); len(rows) != 1 || rows[0]["id"] != "9007199254740993" || rows[0]["price"] != "0.00" || rows[0]["note"] != nil {
		t.Fatalf("unexpected frozen bridge rows: %#v", result)
	}
	target.BridgeBearer = "incorrect"
	if _, err := m.GetSchema(ctx, target); security.ErrorStatus(err, 0) != http.StatusBadGateway {
		t.Fatalf("a mismatched bridge bearer returned %v", err)
	}
}
