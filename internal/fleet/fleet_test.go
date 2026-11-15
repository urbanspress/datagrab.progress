package fleet

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"groxy-desk/internal/groxybridge"
	"groxy-desk/internal/security"

	"golang.org/x/crypto/ssh"
)

func testFingerprintPair(t *testing.T) (string, string) {
	t.Helper()
	_, privA, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	pubA, _ := ssh.NewPublicKey(privA.Public())
	_, privB, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	pubB, _ := ssh.NewPublicKey(privB.Public())
	return security.SSHFingerprintSHA256(pubA), security.SSHFingerprintSHA256(pubB)
}

func TestFleetSSHPinningAndEncryptedVault(t *testing.T) {
	wsDir := t.TempDir()
	stateDir := t.TempDir()
	redactor := security.NewSecretRedactor()
	approvals, err := groxybridge.NewApprovalManager(stateDir, redactor)
	if err != nil {
		t.Fatal(err)
	}
	mgr, err := NewManager(stateDir, wsDir, nil, approvals, redactor)
	if err != nil {
		t.Fatal(err)
	}

	fpA, fpB := testFingerprintPair(t)
	_, mismatchErr := mgr.VerifyAndSaveNode(context.Background(), UpsertNodeRequest{
		NodeID:               "vps-prod-1",
		Host:                 "127.0.0.1",
		Port:                 "22",
		PinnedFingerprint:    fpA,
		PresentedFingerprint: fpB,
	})
	if mismatchErr == nil || security.ErrorCode(mismatchErr, "") != security.CodeDeskE405 || security.ErrorStatus(mismatchErr, 0) != 403 {
		t.Fatalf("expected 403 DESK-E405 on host key mismatch, got %v", mismatchErr)
	}

	// Non-loopback control_url must fail closed DESK-E501
	_, ssrfErr := mgr.VerifyAndSaveNode(context.Background(), UpsertNodeRequest{
		NodeID:            "vps-prod-1",
		Host:              "127.0.0.1",
		Port:              "22",
		PinnedFingerprint: fpA,
		ControlURL:        "http://203.0.113.10:9440",
	})
	if ssrfErr == nil || security.ErrorCode(ssrfErr, "") != security.CodeDeskE501 {
		t.Fatalf("expected DESK-E501 on non-loopback control_url, got %v", ssrfErr)
	}

	saved, err := mgr.VerifyAndSaveNode(context.Background(), UpsertNodeRequest{
		NodeID:               "vps-prod-1",
		Host:                 "127.0.0.1",
		Port:                 "22",
		PinnedFingerprint:    fpA,
		PresentedFingerprint: fpA,
		ControlURL:           "http://127.0.0.1:9440",
		ControlToken:         "super-secret-daemon-token-99",
	})
	if err != nil {
		t.Fatalf("expected verified node save, got %v", err)
	}
	rawJSON, _ := json.Marshal(saved)
	if strings.Contains(string(rawJSON), "super-secret-daemon-token-99") {
		t.Fatalf("plaintext control token leaked in VerifyAndSaveNode response: %s", string(rawJSON))
	}
}

func TestSiteDNAValidationRejectsDriftAndDiv(t *testing.T) {
	wsDir := t.TempDir()
	stateDir := t.TempDir()
	approvals, _ := groxybridge.NewApprovalManager(stateDir, nil)
	mgr, _ := NewManager(stateDir, wsDir, nil, approvals, nil)

	validDNA := `{
		"schema_version": "groxy-site-dna-v0.4.0",
		"site_name": "Author Sanctuary",
		"products": [{"title": "Novel", "price": "19.99"}],
		"tokens": {"fg": "#111827", "bg": "#ffffff"}
	}`
	res, err := mgr.ValidateSiteDNA(context.Background(), DNAValidateRequest{RawJSON: validDNA})
	if err != nil || !res.OK {
		t.Fatalf("expected valid Site DNA, got res=%+v err=%v", res, err)
	}

	for _, bad := range []string{
		`{"schema_version":"groxy-site-dna-v0.4.0","products":[{"price":"19.990000000000002"}]}`,
		`{"schema_version":"groxy-site-dna-v0.4.0","html":"<div>wrapper</div>"}`,
		`{"schema_version":"groxy-site-dna-v0.4.0","tokens":{"fg":"#bbbbbb","bg":"#ffffff"}}`,
	} {
		_, badErr := mgr.ValidateSiteDNA(context.Background(), DNAValidateRequest{RawJSON: bad})
		if badErr == nil || security.ErrorCode(badErr, "") != security.CodeDeskE302 {
			t.Fatalf("expected DESK-E302 for invalid DNA %s, got %v", bad, badErr)
		}
	}
}

func TestFleetPlanApplySingleUseApprovalAndRollback(t *testing.T) {
	wsDir := t.TempDir()
	stateDir := t.TempDir()
	redactor := security.NewSecretRedactor()
	approvals, _ := groxybridge.NewApprovalManager(stateDir, redactor)
	mgr, _ := NewManager(stateDir, wsDir, nil, approvals, redactor)

	fpA, _ := testFingerprintPair(t)
	daemon := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/_health", "/_ready":
			_, _ = w.Write([]byte(`{"ok":true}`))
		case "/v1/deploy", "/v1/rollout", "/v1/rollback":
			if r.Header.Get("Authorization") != "Bearer gt-tok-secret" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			_, _ = w.Write([]byte(`{"ok":true,"status":"complete"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer daemon.Close()

	_, err := mgr.VerifyAndSaveNode(context.Background(), UpsertNodeRequest{
		NodeID:               "edge-1",
		Host:                 "127.0.0.1",
		Port:                 "22",
		PinnedFingerprint:    fpA,
		PresentedFingerprint: fpA,
		ControlURL:           daemon.URL,
		ControlToken:         "gt-tok-secret",
		RequireDaemonHealth:  true,
	})
	if err != nil {
		t.Fatalf("VerifyAndSaveNode failed: %v", err)
	}

	// Mutable :latest image must be rejected
	_, latestErr := mgr.PlanDeploy(context.Background(), DeployPlanRequest{
		NodeID:         "edge-1",
		AppName:        "cms-blog",
		ContainerImage: "ghcr.io/urbanspress/grablab-cms:latest",
	})
	if latestErr == nil || security.ErrorCode(latestErr, "") != security.CodeDeskE301 {
		t.Fatalf("expected DESK-E301 for :latest image, got %v", latestErr)
	}

	plan, err := mgr.PlanDeploy(context.Background(), DeployPlanRequest{
		NodeID:      "edge-1",
		AppName:     "cms-blog",
		BlueprintID: "grablab-cms-sqlite",
	})
	if err != nil || !strings.HasPrefix(plan.Snapshot.SnapshotID, "snap_deploy_") {
		t.Fatalf("PlanDeploy failed: %+v err=%v", plan, err)
	}

	applyRes, err := mgr.ApplyDeploy(context.Background(), DeployApplyRequest{
		NodeID:        "edge-1",
		AppName:       "cms-blog",
		ApprovalToken: plan.ApprovalToken,
		RequireDaemon: true,
	})
	if err != nil || applyRes["daemon_dispatched"] != true {
		t.Fatalf("ApplyDeploy failed: %+v err=%v", applyRes, err)
	}

	// Replay same token -> 409 DESK-E409
	_, replayErr := mgr.ApplyDeploy(context.Background(), DeployApplyRequest{
		NodeID:        "edge-1",
		AppName:       "cms-blog",
		ApprovalToken: plan.ApprovalToken,
	})
	if replayErr == nil || security.ErrorCode(replayErr, "") != security.CodeDeskE409 {
		t.Fatalf("expected DESK-E409 replay rejection, got %v", replayErr)
	}

	rbRes, err := mgr.RestoreDeploySnapshot(context.Background(), plan.Snapshot.SnapshotID, daemon.URL, "gt-tok-secret")
	if err != nil || rbRes["daemon_rolled_back"] != true {
		t.Fatalf("RestoreDeploySnapshot failed: %+v err=%v", rbRes, err)
	}
}
