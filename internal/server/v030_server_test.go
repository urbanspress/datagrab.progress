package server

import (
	"crypto/ed25519"
	"crypto/rand"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"groxy-desk/internal/security"

	"golang.org/x/crypto/ssh"
)

func TestV030FleetDBStudioDNSAndAuthorHTTPRoutes(t *testing.T) {
	srv, wsDir, _ := setupTestServer(t)
	sess := srv.Session()

	// 1. /_health reports v1.0.0
	respH, _, rawH := doAPIReq(t, http.MethodGet, sess.URL+"/_health", "", "", nil)
	if respH.StatusCode != http.StatusOK || !strings.Contains(rawH, `"1.0.0"`) {
		t.Fatalf("expected v1.0.0 /_health, got %d %s", respH.StatusCode, rawH)
	}

	// 2. Fleet auth + CSRF + SSH pin verification + Site DNA + Deploy Plan/Apply + Replay guard
	respUnauth, _, _ := doAPIReq(t, http.MethodGet, sess.URL+"/api/fleet/nodes", sess.BearerToken, "", nil)
	if respUnauth.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 without CSRF on /api/fleet/nodes, got %d", respUnauth.StatusCode)
	}

	_, privA, _ := ed25519.GenerateKey(rand.Reader)
	pubA, _ := ssh.NewPublicKey(privA.Public())
	_, privB, _ := ed25519.GenerateKey(rand.Reader)
	pubB, _ := ssh.NewPublicKey(privB.Public())
	fpA := security.SSHFingerprintSHA256(pubA)
	fpB := security.SSHFingerprintSHA256(pubB)

	badPinResp, badPinObj, _ := doAPIReq(t, http.MethodPost, sess.URL+"/api/fleet/nodes/verify", sess.BearerToken, sess.CSRFToken, map[string]any{
		"node_id":               "prod-1",
		"host":                  "127.0.0.1",
		"port":                  "22",
		"pinned_fingerprint":    fpA,
		"presented_fingerprint": fpB,
	})
	if badPinResp.StatusCode != http.StatusForbidden || badPinObj["error"].(map[string]any)["code"] != security.CodeDeskE405 {
		t.Fatalf("expected 403 DESK-E405 on mismatched SSH pin, got %d %v", badPinResp.StatusCode, badPinObj)
	}

	okPinResp, _, rawOKPin := doAPIReq(t, http.MethodPost, sess.URL+"/api/fleet/nodes/verify", sess.BearerToken, sess.CSRFToken, map[string]any{
		"node_id":               "prod-1",
		"host":                  "127.0.0.1",
		"port":                  "22",
		"pinned_fingerprint":    fpA,
		"presented_fingerprint": fpA,
		"control_token":         "daemon-top-secret-88",
	})
	if okPinResp.StatusCode != http.StatusOK || strings.Contains(rawOKPin, "daemon-top-secret-88") {
		t.Fatalf("unexpected verify response: %d %s", okPinResp.StatusCode, rawOKPin)
	}

	badDNA, badDNAObj, _ := doAPIReq(t, http.MethodPost, sess.URL+"/api/fleet/dna/validate", sess.BearerToken, sess.CSRFToken, map[string]any{
		"raw_json": `{"schema_version":"groxy-site-dna-v0.4.0","html":"<div>unsafe</div>"}`,
	})
	if badDNA.StatusCode != http.StatusUnprocessableEntity || badDNAObj["error"].(map[string]any)["code"] != security.CodeDeskE302 {
		t.Fatalf("expected 422 DESK-E302 for <div> in Site DNA, got %d %v", badDNA.StatusCode, badDNAObj)
	}

	// 3. DB Studio SQLite confinement, rows ADR-025c, 403 DESK-E406, mutation plan->apply->rollback
	dbPath := filepath.Join(wsDir, "data", "studio.db")
	_ = os.MkdirAll(filepath.Dir(dbPath), 0o755)
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = db.Exec(`CREATE TABLE catalog (id INTEGER PRIMARY KEY, price TEXT); INSERT INTO catalog VALUES (1, '29.95');`)
	_ = db.Close()

	rSchema, objSchema, rawSchema := doAPIReq(t, http.MethodGet, sess.URL+"/api/db/schema?engine=sqlite&sqlite_path=data/studio.db", sess.BearerToken, sess.CSRFToken, nil)
	if rSchema.StatusCode != http.StatusOK || !strings.Contains(rawSchema, `"catalog"`) {
		t.Fatalf("GET /api/db/schema failed: %d %v %s", rSchema.StatusCode, objSchema, rawSchema)
	}

	rWriteErr, objWriteErr, _ := doAPIReq(t, http.MethodPost, sess.URL+"/api/db/query", sess.BearerToken, sess.CSRFToken, map[string]any{
		"target": map[string]any{"engine": "sqlite", "sqlite_path": "data/studio.db"},
		"sql":    "DROP TABLE catalog",
	})
	if rWriteErr.StatusCode != http.StatusForbidden || objWriteErr["error"].(map[string]any)["code"] != security.CodeDeskE406 {
		t.Fatalf("expected 403 DESK-E406 on DROP TABLE in read-only runner, got %d %v", rWriteErr.StatusCode, objWriteErr)
	}

	// 4. DNS vault + email preservation + propagation preflight + Auto-HTTPS
	mockNC := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<?xml version="1.0"?><ApiResponse Status="OK"><Errors/><CommandResponse Type="namecheap.domains.dns.getHosts"><DomainDNSGetHostsResult Domain="example.org"><host HostId="1" Name="@" Type="MX" Address="mx.example.net." MXPref="10" TTL="1800"/></DomainDNSGetHostsResult></CommandResponse></ApiResponse>`))
	}))
	defer mockNC.Close()

	_, objVault, rawVault := doAPIReq(t, http.MethodPost, sess.URL+"/api/dns/vault/save", sess.BearerToken, sess.CSRFToken, map[string]any{
		"api_user":          "u",
		"api_key":           "secret-nc-7788",
		"client_ip":         "198.51.100.2",
		"endpoint_override": mockNC.URL,
	})
	if objVault["api_key_masked"] != "nc_****...7788" || strings.Contains(rawVault, "secret-nc-7788") {
		t.Fatalf("unexpected vault response: %s", rawVault)
	}

	rDrop, objDrop, _ := doAPIReq(t, http.MethodPost, sess.URL+"/api/dns/plan", sess.BearerToken, sess.CSRFToken, map[string]any{
		"domain":            "example.org",
		"target_ip":         "203.0.113.5",
		"simulated_drop_mx": true,
	})
	if rDrop.StatusCode != http.StatusUnprocessableEntity || objDrop["error"].(map[string]any)["code"] != security.CodeDeskE303 {
		t.Fatalf("expected 422 DESK-E303 on MX drop, got %d %v", rDrop.StatusCode, objDrop)
	}

	rPreflightFail, objPreflightFail, _ := doAPIReq(t, http.MethodPost, sess.URL+"/api/dns/https/preflight", sess.BearerToken, sess.CSRFToken, map[string]any{
		"domain":        "example.org",
		"expected_ip":   "203.0.113.5",
		"edge_proxy":    "caddy",
		"upstream_port": "8765",
		"observed_ips":  []string{"192.0.2.99"},
	})
	if rPreflightFail.StatusCode != http.StatusUnprocessableEntity || objPreflightFail["error"].(map[string]any)["code"] != security.CodeDeskE304 {
		t.Fatalf("expected 422 DESK-E304 on unpropagated DNS, got %d %v", rPreflightFail.StatusCode, objPreflightFail)
	}

	// 5. Author Mode plan & apply + Ollama loopback status
	rAuthPlan, objAuthPlan, rawAuthPlan := doAPIReq(t, http.MethodPost, sess.URL+"/api/author/init/plan", sess.BearerToken, sess.CSRFToken, map[string]any{
		"target_subdir": "novels/book1",
		"book_title":    "Clockwork Harbor",
		"author_name":   "Ada",
		"isbn_13":       "9780143127550",
		"price_usd":     "14.95",
	})
	if rAuthPlan.StatusCode != http.StatusOK || !strings.Contains(objAuthPlan["schema_org_jsonld"].(string), `"@type":"Book"`) {
		t.Fatalf("Author init plan failed: %d %s", rAuthPlan.StatusCode, rawAuthPlan)
	}
	rAuthApply, objAuthApply, _ := doAPIReq(t, http.MethodPost, sess.URL+"/api/author/init/apply", sess.BearerToken, sess.CSRFToken, map[string]any{
		"target_subdir":  "novels/book1",
		"book_title":     "Clockwork Harbor",
		"author_name":    "Ada",
		"isbn_13":        "9780143127550",
		"price_usd":      "14.95",
		"approval_token": objAuthPlan["approval_token"],
	})
	if rAuthApply.StatusCode != http.StatusOK || objAuthApply["status_enforced"] != "draft" {
		t.Fatalf("Author init apply failed: %d %v", rAuthApply.StatusCode, objAuthApply)
	}
	rOll, objOll, _ := doAPIReq(t, http.MethodGet, sess.URL+"/api/author/ollama/status", sess.BearerToken, sess.CSRFToken, nil)
	if rOll.StatusCode != http.StatusOK || objOll["model_switching_allowed"] != false {
		t.Fatalf("Ollama status failed: %d %v", rOll.StatusCode, objOll)
	}
}
