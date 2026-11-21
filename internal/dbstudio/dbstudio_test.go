package dbstudio

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"database/sql"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"

	"groxy-desk/internal/groxybridge"
	"groxy-desk/internal/security"

	"golang.org/x/crypto/ssh"
)

func genSSHFingerprints(t *testing.T) (string, string) {
	t.Helper()
	_, privA, _ := ed25519.GenerateKey(rand.Reader)
	pubA, _ := ssh.NewPublicKey(privA.Public())
	_, privB, _ := ed25519.GenerateKey(rand.Reader)
	pubB, _ := ssh.NewPublicKey(privB.Public())
	return security.SSHFingerprintSHA256(pubA), security.SSHFingerprintSHA256(pubB)
}

func TestTunnelPoolLimitsPinningAndTCPForwarding(t *testing.T) {
	redactor := security.NewSecretRedactor()
	tm := NewTunnelManager(redactor)
	defer tm.CloseAll()

	fpA, fpB := genSSHFingerprints(t)

	// Non-loopback local bind -> DESK-E501
	_, bindErr := tm.OpenTunnel(OpenTunnelRequest{
		Engine:            "postgres",
		PinnedFingerprint: fpA,
		LocalBindHost:     "0.0.0.0",
		Transport:         "loopback_test",
	})
	if bindErr == nil || security.ErrorCode(bindErr, "") != security.CodeDeskE501 {
		t.Fatalf("expected DESK-E501 for 0.0.0.0 local bind, got %v", bindErr)
	}

	// Mismatched fingerprint -> 403 DESK-E405
	_, pinErr := tm.OpenTunnel(OpenTunnelRequest{
		Engine:               "postgres",
		PinnedFingerprint:    fpA,
		PresentedFingerprint: fpB,
		Transport:            "loopback_test",
	})
	if pinErr == nil || security.ErrorCode(pinErr, "") != security.CodeDeskE405 {
		t.Fatalf("expected 403 DESK-E405 on tunnel key mismatch, got %v", pinErr)
	}

	// Real TCP echo server on 127.0.0.1 forwarded over tunnel
	echoLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer echoLn.Close()
	go func() {
		for {
			c, aErr := echoLn.Accept()
			if aErr != nil {
				return
			}
			go func(conn net.Conn) {
				defer conn.Close()
				_, _ = io.Copy(conn, conn)
			}(c)
		}
	}()
	_, echoPort, _ := net.SplitHostPort(echoLn.Addr().String())

	// Open 8 concurrent tunnels (9th -> 409 DESK-E409)
	for i := 0; i < 8; i++ {
		view, oErr := tm.OpenTunnel(OpenTunnelRequest{
			Engine:               "postgres",
			SSHHost:              "127.0.0.1",
			PinnedFingerprint:    fpA,
			PresentedFingerprint: fpA,
			RemoteDBHost:         "127.0.0.1",
			RemoteDBPort:         echoPort,
			Transport:            "loopback_test",
		})
		if oErr != nil {
			t.Fatalf("tunnel %d open failed: %v", i, oErr)
		}
		if i == 0 {
			conn, dErr := net.Dial("tcp", view.LocalBindAddr)
			if dErr != nil {
				t.Fatalf("failed dialing tunnel %s: %v", view.LocalBindAddr, dErr)
			}
			_, _ = conn.Write([]byte("PING-TUNNEL"))
			buf := make([]byte, 11)
			_, rErr := io.ReadFull(conn, buf)
			_ = conn.Close()
			if rErr != nil || string(buf) != "PING-TUNNEL" {
				t.Fatalf("tunnel TCP forwarding mismatch: got %q err=%v", string(buf), rErr)
			}
		}
	}

	_, capErr := tm.OpenTunnel(OpenTunnelRequest{
		Engine:               "postgres",
		PinnedFingerprint:    fpA,
		PresentedFingerprint: fpA,
		RemoteDBPort:         echoPort,
		Transport:            "loopback_test",
	})
	if capErr == nil || security.ErrorCode(capErr, "") != security.CodeDeskE409 || security.ErrorStatus(capErr, 0) != 409 {
		t.Fatalf("expected 409 DESK-E409 at 9th tunnel, got %v", capErr)
	}
}

func TestDBStudioSQLiteConfinementReadOnlyMutationAndRollback(t *testing.T) {
	wsDir := t.TempDir()
	stateDir := t.TempDir()
	redactor := security.NewSecretRedactor()
	approvals, err := groxybridge.NewApprovalManager(stateDir, redactor)
	if err != nil {
		t.Fatal(err)
	}

	// Seed SQLite DB inside workspace with exact ADR-025c INT64 (> 2^53) and decimal price strings
	dbPath := filepath.Join(wsDir, "data", "books.db")
	_ = os.MkdirAll(filepath.Dir(dbPath), 0o755)
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`
		CREATE TABLE books (
			id INTEGER PRIMARY KEY,
			isbn TEXT NOT NULL,
			price TEXT NOT NULL,
			word_count INTEGER NOT NULL
		);
		INSERT INTO books (id, isbn, price, word_count) VALUES (1, '9780143127550', '19.99', 9007199254740993);
	`)
	_ = db.Close()
	if err != nil {
		t.Fatal(err)
	}

	mgr := NewManager(wsDir, approvals, redactor)
	defer mgr.CloseAll()
	target := TargetSpec{Engine: "sqlite", SQLitePath: "data/books.db"}

	// Traversal -> GRAB-E401
	_, confErr := mgr.GetSchema(context.Background(), TargetSpec{Engine: "sqlite", SQLitePath: "../outside.db"})
	if confErr == nil || security.ErrorCode(confErr, "") != security.CodeGrabE401 {
		t.Fatalf("expected GRAB-E401 on traversal, got %v", confErr)
	}

	// Schema inspection
	schema, err := mgr.GetSchema(context.Background(), target)
	if err != nil {
		t.Fatalf("GetSchema failed: %v", err)
	}
	tables := schema["tables"].([]TableSchema)
	if len(tables) != 1 || tables[0].RowCount != "1" {
		t.Fatalf("unexpected schema: %+v", tables)
	}

	// Rows pagination cap > 100 -> 400 DESK-E101
	_, pageErr := mgr.BrowseRows(context.Background(), RowsRequest{Target: target, Table: "books", Page: "1", Limit: "101"})
	if pageErr == nil || security.ErrorCode(pageErr, "") != security.CodeDeskE101 {
		t.Fatalf("expected DESK-E101 for limit=101, got %v", pageErr)
	}

	rowsRes, err := mgr.BrowseRows(context.Background(), RowsRequest{Target: target, Table: "books", Page: "1", Limit: "50"})
	if err != nil {
		t.Fatal(err)
	}
	rowsList := rowsRes["rows"].([]map[string]any)
	if rowsList[0]["word_count"] != "9007199254740993" || rowsList[0]["price"] != "19.99" {
		t.Fatalf("ADR-025c wire fidelity violated: %+v", rowsList[0])
	}

	// Write SQL in read-only runner -> 403 DESK-E406
	_, roErr := mgr.RunReadOnlyQuery(context.Background(), target, "DELETE FROM books WHERE id = 1")
	if roErr == nil || security.ErrorCode(roErr, "") != security.CodeDeskE406 || security.ErrorStatus(roErr, 0) != 403 {
		t.Fatalf("expected 403 DESK-E406 on DELETE in read-only query, got %v", roErr)
	}

	// Guarded Plan -> Snapshot -> Apply -> Rollback
	plan, err := mgr.PlanMutation(context.Background(), MutatePlanRequest{
		Target: target,
		SQL:    "UPDATE books SET price = '24.50' WHERE id = 1;",
	})
	if err != nil {
		t.Fatal(err)
	}
	snap := plan["snapshot"].(groxybridge.SnapshotRecord)
	token := plan["approval_token"].(string)

	_, applyErr := mgr.ApplyMutation(context.Background(), MutateApplyRequest{
		Target:        target,
		SQL:           "UPDATE books SET price = '24.50' WHERE id = 1;",
		ApprovalToken: token,
	})
	if applyErr != nil {
		t.Fatalf("ApplyMutation failed: %v", applyErr)
	}

	afterMut, _ := mgr.BrowseRows(context.Background(), RowsRequest{Target: target, Table: "books", Page: "1", Limit: "10"})
	if afterMut["rows"].([]map[string]any)[0]["price"] != "24.50" {
		t.Fatalf("expected updated price 24.50, got %+v", afterMut)
	}

	// Restore Pre-Mutation Snapshot -> back to 19.99
	restore := RestoreRequest{Target: target, SnapshotID: snap.SnapshotID}
	restorePlan, err := mgr.PlanRestore(context.Background(), restore)
	if err != nil {
		t.Fatal(err)
	}
	restore.ApprovalToken = restorePlan["approval_token"].(string)
	if _, err := mgr.RestoreSnapshot(context.Background(), restore); err != nil {
		t.Fatalf("RestoreSnapshot failed: %v", err)
	}
	afterRestore, _ := mgr.BrowseRows(context.Background(), RowsRequest{Target: target, Table: "books", Page: "1", Limit: "10"})
	if afterRestore["rows"].([]map[string]any)[0]["price"] != "19.99" {
		t.Fatalf("expected restored price 19.99, got %+v", afterRestore)
	}
}
