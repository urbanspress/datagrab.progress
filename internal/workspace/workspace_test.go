package workspace

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"groxy-desk/internal/security"
)

func TestWorkspaceConfinementTreeReadAndGrabman(t *testing.T) {
	grabBin := ResolveGrabBinary("")
	grabmanBin := ResolveGrabmanBinary("")
	if grabBin == "" || grabmanBin == "" {
		t.Fatalf("expected GRAB and grabman binaries to resolve (grab=%q grabman=%q)", grabBin, grabmanBin)
	}

	root := t.TempDir()
	// Create sensitive files that must NEVER appear in Tree or be readable via ReadFile
	if err := os.WriteFile(filepath.Join(root, ".env"), []byte("SECRET=123\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".git", "config"), []byte("[core]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "private.key"), []byte("KEY\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	mgr, err := NewManager(root, grabBin, grabmanBin, security.NewSecretRedactor())
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Reject traversal & unsupported profile
	if _, err := mgr.GrabmanInit(ctx, "../escape", "app", "Bad", "caddy", false); err == nil || security.ErrorCode(err, "") != security.CodeGrabE401 {
		t.Fatalf("expected GRAB-E401 on init traversal, got %v", err)
	}
	if _, err := mgr.GrabmanInit(ctx, "demo-site", "invalid-profile", "Bad", "caddy", false); err == nil || security.ErrorCode(err, "") != security.CodeDeskE101 {
		t.Fatalf("expected DESK-E101 on invalid profile, got %v", err)
	}

	// Scaffold a project via grabman init
	initRes, err := mgr.GrabmanInit(ctx, "demo-site", "app", "Demo Desk Site", "caddy", false)
	if err != nil || !initRes.OK {
		t.Fatalf("GrabmanInit failed: %v (%+v)", err, initRes)
	}

	// Verify grabman doctor & solve --verify
	docRes, err := mgr.GrabmanDoctor(ctx, "demo-site", false)
	if err != nil {
		t.Fatalf("GrabmanDoctor failed: %v", err)
	}
	if docRes.Report["schema_version"] == nil {
		t.Fatalf("expected parsed JSON doctor report, got %+v", docRes)
	}

	solveRes, err := mgr.GrabmanSolveVerify(ctx, "demo-site")
	if err != nil || !solveRes.OK {
		t.Fatalf("GrabmanSolveVerify failed: %v (%+v)", err, solveRes)
	}

	// Verify Tree excludes .env, .git, and *.key
	entries, err := mgr.Tree(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name == ".env" || e.Name == ".git" || e.Name == "private.key" {
			t.Fatalf("sensitive entry %q leaked in workspace tree", e.Path)
		}
		if !security.IsExactIntString(e.SizeBytes) {
			t.Fatalf("expected ADR-025c base-10 size string, got %q", e.SizeBytes)
		}
	}

	// Verify ReadFile enforces GRAB-E401 on .env & private.key, allows .env.example & src/app.grab
	if _, err := mgr.ReadFile(".env"); err == nil || security.ErrorCode(err, "") != security.CodeGrabE401 {
		t.Fatalf("expected GRAB-E401 on .env read, got %v", err)
	}
	if _, err := mgr.ReadFile("demo-site/deploy/.env.example"); err != nil {
		t.Fatalf("expected .env.example to be readable, got %v", err)
	}
	fc, err := mgr.ReadFile("demo-site/src/app.grab")
	if err != nil || fc.Content == "" {
		t.Fatalf("expected src/app.grab readable, got %v", err)
	}

	// Verify grab check + format parity
	chk, err := mgr.CheckFiles(ctx, "demo-site/src/app.grab", "")
	if err != nil {
		t.Fatalf("CheckFiles failed: %v", err)
	}
	if !chk.GrabCheckOK {
		t.Fatalf("expected grab check to pass on grabman scaffold, got %+v", chk)
	}
}
