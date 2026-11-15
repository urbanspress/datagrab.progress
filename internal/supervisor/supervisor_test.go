package supervisor

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"groxy-desk/internal/security"
)

func findTestGrabBinary(t *testing.T) string {
	t.Helper()
	candidates := []string{
		os.Getenv("GRAB_BIN"),
		`C:\Development\GRAB\dist\grab-windows-amd64.exe`,
	}
	for _, c := range candidates {
		if c != "" {
			if _, err := os.Stat(c); err == nil {
				return c
			}
		}
	}
	t.Fatalf("GRAB v1.1.0 binary not found for supervisor test")
	return ""
}

func TestSecretRedactionRingBuffer(t *testing.T) {
	red := security.NewSecretRedactor("my-top-secret-key")
	rb := newRingBuffer(4)
	for i := 0; i < 6; i++ {
		rb.add("stdout", "Authorization: Bearer token1234567890 my-top-secret-key v1:SecretCipherText", red)
	}
	items := rb.list()
	if len(items) != 4 {
		t.Fatalf("expected ring capacity 4, got %d", len(items))
	}
	for _, it := range items {
		if strings.Contains(it.Line, "token1234567890") || strings.Contains(it.Line, "my-top-secret-key") || strings.Contains(it.Line, "SecretCipherText") {
			t.Fatalf("unredacted secret in log ring buffer: %q", it.Line)
		}
	}
}

func TestSupervisorLiveWorkerLifecycle(t *testing.T) {
	grabBin := findTestGrabBinary(t)
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	grabSrc := `use server

route "GET" "/" using request
  give back server.text using 200, "hello-v1"
end
`
	if err := os.WriteFile(filepath.Join(root, "src", "app.grab"), []byte(grabSrc), 0o644); err != nil {
		t.Fatal(err)
	}

	red := security.NewSecretRedactor("session-secret-xyz")
	mgr := NewManager(grabBin, root, red)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = mgr.ShutdownAll(ctx)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Reject out-of-range port
	if _, err := mgr.StartWorker(ctx, StartRequest{Name: "badport", GrabFile: "src/app.grab", Port: "9999"}); err == nil || security.ErrorCode(err, "") != security.CodeDeskE101 {
		t.Fatalf("expected DESK-E101 on out-of-range port, got %v", err)
	}

	// Reject traversal in grab_file
	if _, err := mgr.StartWorker(ctx, StartRequest{Name: "escape", GrabFile: "../outside.grab"}); err == nil || security.ErrorCode(err, "") != security.CodeGrabE401 {
		t.Fatalf("expected GRAB-E401 on path traversal, got %v", err)
	}

	snap, err := mgr.StartWorker(ctx, StartRequest{
		ID:       "site-1",
		Name:     "Test Site",
		GrabFile: "src/app.grab",
	})
	if err != nil {
		t.Fatalf("StartWorker failed: %v", err)
	}
	if !snap.HealthOK || !snap.ReadyOK || snap.Status != "ready" {
		t.Fatalf("expected worker ready with /_health and /_ready OK, got %+v", snap)
	}
	if !security.IsExactIntString(snap.Port) || !security.IsExactIntString(snap.PID) {
		t.Fatalf("expected ADR-025c base-10 port and pid strings, got port=%q pid=%q", snap.Port, snap.PID)
	}

	// Inject invalid syntax -> reload must fail with DESK-E301 and preserve running worker
	if err := os.WriteFile(filepath.Join(root, "src", "app.grab"), []byte("bad syntax [\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := mgr.ReloadWorker(ctx, "site-1"); err == nil || security.ErrorCode(err, "") != security.CodeDeskE301 {
		t.Fatalf("expected DESK-E301 when reload preflight fails, got %v", err)
	}

	// Restore valid source -> atomic reload succeeds and increments reload_count
	if err := os.WriteFile(filepath.Join(root, "src", "app.grab"), []byte(strings.ReplaceAll(grabSrc, "hello-v1", "hello-v2")), 0o644); err != nil {
		t.Fatal(err)
	}
	reloaded, err := mgr.ReloadWorker(ctx, "site-1")
	if err != nil {
		t.Fatalf("ReloadWorker failed: %v", err)
	}
	if reloaded.ReloadCount != "1" || !reloaded.ReadyOK {
		t.Fatalf("expected reload_count=1 and ReadyOK, got %+v", reloaded)
	}

	logs, err := mgr.GetLogs("site-1")
	if err != nil || len(logs) == 0 {
		t.Fatalf("expected non-empty redacted logs, got len=%d err=%v", len(logs), err)
	}

	stopped, err := mgr.StopWorker(ctx, "site-1")
	if err != nil {
		t.Fatalf("StopWorker failed: %v", err)
	}
	if stopped.Status != "stopped" || stopped.ReadyOK {
		t.Fatalf("expected stopped status, got %+v", stopped)
	}
}
