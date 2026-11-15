package groxybridge

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"groxy-desk/internal/security"
	"groxy-desk/internal/workspace"
)

func setupTestBridge(t *testing.T, ollamaURL string) (*Bridge, *workspace.Manager, string, string) {
	t.Helper()
	wsDir := t.TempDir()
	stateDir := t.TempDir()

	viewsDir := filepath.Join(wsDir, "views")
	if err := os.MkdirAll(viewsDir, 0o755); err != nil {
		t.Fatalf("mkdir views: %v", err)
	}
	sampleHTML := `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>Groxy Desk Sample Workspace View</title>
  <meta name="description" content="Sample semantic Grex JS v1.0.0 workspace template verified by Groxy AI deterministic audit rules." />
  <link rel="canonical" href="http://127.0.0.1:9490/" />
</head>
<body>
  <header>
    <h1>Groxy Desk Workbench</h1>
    <nav aria-label="Primary"><a href="/docs">Docs</a></nav>
  </header>
  <main>
    <section>
      <h2>Overview</h2>
      <p>Zero wrapper div layout.</p>
    </section>
  </main>
  <footer>
    <p>Local Workspace</p>
  </footer>
</body>
</html>
`
	if err := os.WriteFile(filepath.Join(viewsDir, "index.grab.html"), []byte(sampleHTML), 0o644); err != nil {
		t.Fatalf("write sample html: %v", err)
	}
	if err := os.WriteFile(filepath.Join(wsDir, "notes.txt"), []byte("other target"), 0o644); err != nil {
		t.Fatalf("write notes.txt: %v", err)
	}

	redactor := security.NewSecretRedactor()
	wsMgr, err := workspace.NewManager(wsDir, workspace.ResolveGrabBinary(""), workspace.ResolveGrabmanBinary(""), redactor)
	if err != nil {
		t.Fatalf("workspace.NewManager: %v", err)
	}
	apprMgr, err := NewApprovalManager(stateDir, redactor)
	if err != nil {
		t.Fatalf("NewApprovalManager: %v", err)
	}
	bridge, err := NewBridge(wsMgr, nil, apprMgr, redactor, ollamaURL)
	if err != nil {
		t.Fatalf("NewBridge: %v", err)
	}
	return bridge, wsMgr, wsDir, stateDir
}

func TestAdaptiveRouterClassificationAndFastAudit(t *testing.T) {
	bridge, _, _, _ := setupTestBridge(t, "")
	ctx := context.Background()

	resp, err := bridge.Ask(ctx, AskRequest{
		Prompt:     "Run deterministic 22-rule audit on index",
		TargetPath: "views/index.grab.html",
	})
	if err != nil {
		t.Fatalf("fast audit Ask failed: %v", err)
	}
	if resp.Agent != AgentID || resp.ModelSwitchingAllowed {
		t.Fatalf("unexpected identity: %+v", resp)
	}
	if resp.ReasoningMode != ReasoningFast {
		t.Fatalf("expected fast reasoning_mode, got %q", resp.ReasoningMode)
	}
	if resp.ApprovalRequired {
		t.Fatalf("fast inspection should not require mutation approval")
	}
	if resp.Audit == nil || resp.Audit.Score != "100" || resp.Audit.Grade != "A" || !resp.Audit.PublishSafe || resp.Audit.DivCount != "0" {
		t.Fatalf("expected 100/A PublishSafe div=0 audit, got %+v", resp.Audit)
	}
}

func TestAuditHTMLSafetyCapsOnViolations(t *testing.T) {
	badHTML := `<html><head><title>Bad</title></head><body><div class="box" onclick="doThing()"><a href="javascript:void(0)">Click</a><script>eval("1+1")</script></div></body></html>`
	report := AuditHTML("views/bad.grab.html", badHTML)
	if report.PublishSafe {
		t.Fatalf("expected PublishSafe=false on div/script/onclick/javascript violations")
	}
	if report.Grade != "F" {
		t.Fatalf("expected Grade F on critical safety cap violation, got %s", report.Grade)
	}
	if report.DivCount != "1" {
		t.Fatalf("expected DivCount 1, got %s", report.DivCount)
	}
}

func TestGroxySafe001ApprovalTargetMismatchReplayAndRestore(t *testing.T) {
	bridge, _, wsDir, _ := setupTestBridge(t, "")
	ctx := context.Background()

	targetAbs := filepath.Join(wsDir, "views", "index.grab.html")
	origBytes, err := os.ReadFile(targetAbs)
	if err != nil {
		t.Fatalf("read original: %v", err)
	}
	origSHA := security.SHA256Hex(origBytes)

	newContent := `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>Groxy Desk Updated Semantic Page</title>
  <meta name="description" content="Updated clean Pure Grex JS v1.0.0 semantic page verified after approval token authorization." />
  <link rel="canonical" href="http://127.0.0.1:9490/" />
</head>
<body>
  <header><h1>Updated Heading</h1></header>
  <main><section><h2>Changed</h2><p>Applied safely.</p></section></main>
  <footer><p>Verified</p></footer>
</body>
</html>
`
	planResp, err := bridge.Ask(ctx, AskRequest{
		Prompt:          "Fix and update heading safely",
		TargetPath:      "views/index.grab.html",
		ProposedContent: newContent,
	})
	if err != nil {
		t.Fatalf("deep Ask failed: %v", err)
	}
	if planResp.ReasoningMode != ReasoningDeep || !planResp.ApprovalRequired {
		t.Fatalf("expected deep mode with approval required, got %+v", planResp)
	}
	if planResp.ApprovalToken == "" || planResp.Snapshot == nil || planResp.Snapshot.BeforeSHA256 != origSHA {
		t.Fatalf("expected pre-mutation SHA-256 snapshot and token, got %+v", planResp)
	}

	// Target mismatch MUST return 403 DESK-E403 and NOT consume valid behavior or mutate other file.
	_, err = bridge.Approvals().ApplyMutation(wsDir, ApplyRequest{
		ApprovalToken: planResp.ApprovalToken,
		TargetPath:    "notes.txt",
		Content:       &newContent,
	})
	if err == nil || security.ErrorStatus(err, 0) != 403 || security.ErrorCode(err, "") != security.CodeDeskE403 {
		t.Fatalf("expected 403 DESK-E403 on target mismatch, got %v", err)
	}

	// Apply to rightful target succeeds.
	applyRes, err := bridge.Approvals().ApplyMutation(wsDir, ApplyRequest{
		ApprovalToken: planResp.ApprovalToken,
		TargetPath:    "views/index.grab.html",
	})
	if err != nil {
		t.Fatalf("ApplyMutation failed: %v", err)
	}
	if !applyRes.OK || applyRes.SnapshotID != planResp.Snapshot.SnapshotID {
		t.Fatalf("unexpected apply result: %+v", applyRes)
	}
	afterBytes, err := os.ReadFile(targetAbs)
	if err != nil || string(afterBytes) != newContent {
		t.Fatalf("file was not mutated as expected")
	}

	// Replay with identical token MUST return 409 DESK-E409.
	_, err = bridge.Approvals().ApplyMutation(wsDir, ApplyRequest{
		ApprovalToken: planResp.ApprovalToken,
		TargetPath:    "views/index.grab.html",
	})
	if err == nil || security.ErrorStatus(err, 0) != 409 || security.ErrorCode(err, "") != security.CodeDeskE409 {
		t.Fatalf("expected 409 DESK-E409 on replay, got %v", err)
	}

	// 1-click Snapshot restore returns exact original SHA-256 bytes.
	restoreRes, err := bridge.Approvals().RestoreSnapshot(wsDir, planResp.Snapshot.SnapshotID)
	if err != nil {
		t.Fatalf("RestoreSnapshot failed: %v", err)
	}
	if restoreRes.RestoredSHA256 != origSHA {
		t.Fatalf("expected restored sha %s, got %s", origSHA, restoreRes.RestoredSHA256)
	}
	restoredDisk, err := os.ReadFile(targetAbs)
	if err != nil || string(restoredDisk) != string(origBytes) {
		t.Fatalf("disk content did not match original bytes after restore")
	}
}

func TestConfinementAndNewFileSnapshotRollback(t *testing.T) {
	bridge, _, wsDir, _ := setupTestBridge(t, "")
	ctx := context.Background()

	// Sensitive & traversal path probes MUST fail with 403 GRAB-E401.
	for _, bad := range []string{"../escape.txt", ".env", ".git/config", "certs/dev.key", "id_rsa"} {
		_, err := bridge.Ask(ctx, AskRequest{Prompt: "Audit file", TargetPath: bad})
		if err == nil || security.ErrorCode(err, "") != security.CodeGrabE401 {
			t.Fatalf("expected GRAB-E401 for %q, got %v", bad, err)
		}
	}

	// Creating a brand-new file captures Existed=false snapshot; restore deletes the file.
	planResp, err := bridge.Ask(ctx, AskRequest{
		Prompt:     "Create clean status view",
		TargetPath: "views/created.grab.html",
	})
	if err != nil {
		t.Fatalf("Ask new file failed: %v", err)
	}
	if planResp.Snapshot == nil || planResp.Snapshot.Existed {
		t.Fatalf("expected Existed=false snapshot for brand new file")
	}
	if _, err := bridge.Approvals().ApplyMutation(wsDir, ApplyRequest{
		ApprovalToken: planResp.ApprovalToken,
		TargetPath:    "views/created.grab.html",
	}); err != nil {
		t.Fatalf("ApplyMutation new file failed: %v", err)
	}
	createdAbs := filepath.Join(wsDir, "views", "created.grab.html")
	if _, err := os.Stat(createdAbs); err != nil {
		t.Fatalf("expected created file on disk: %v", err)
	}
	if _, err := bridge.Approvals().RestoreSnapshot(wsDir, planResp.Snapshot.SnapshotID); err != nil {
		t.Fatalf("RestoreSnapshot new file failed: %v", err)
	}
	if _, err := os.Stat(createdAbs); !os.IsNotExist(err) {
		t.Fatalf("expected newly created file removed on snapshot restore, stat err=%v", err)
	}
}

func TestLoopbackOllamaValidationAndFailClosed(t *testing.T) {
	wsDir := t.TempDir()
	stateDir := t.TempDir()
	redactor := security.NewSecretRedactor()
	wsMgr, err := workspace.NewManager(wsDir, "grab", "grabman", redactor)
	if err != nil {
		t.Fatalf("workspace.NewManager: %v", err)
	}
	apprMgr, err := NewApprovalManager(stateDir, redactor)
	if err != nil {
		t.Fatalf("NewApprovalManager: %v", err)
	}

	if _, err := NewBridge(wsMgr, nil, apprMgr, redactor, "http://0.0.0.0:11434"); err == nil || !strings.Contains(err.Error(), security.CodeDeskE501) {
		t.Fatalf("expected DESK-E501 on non-loopback Ollama endpoint, got %v", err)
	}

	// Bind and immediately close a loopback listener so the port refuses connection cleanly.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	closedAddr := ln.Addr().String()
	_ = ln.Close()

	bridge, err := NewBridge(wsMgr, nil, apprMgr, redactor, "http://"+closedAddr)
	if err != nil {
		t.Fatalf("NewBridge on loopback closed addr: %v", err)
	}
	_, err = bridge.Ask(context.Background(), AskRequest{Prompt: "Check workspace"})
	if err == nil || security.ErrorCode(err, "") != security.CodeDeskE503 || security.ErrorStatus(err, 0) != 503 {
		t.Fatalf("expected 503 DESK-E503 on unreachable Ollama endpoint, got %v", err)
	}
}
