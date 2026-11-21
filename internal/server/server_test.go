package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"groxy-desk/internal/security"
)

func setupTestServer(t *testing.T) (*Server, string, string) {
	t.Helper()
	wsDir := t.TempDir()
	stateDir := t.TempDir()

	if err := os.MkdirAll(filepath.Join(wsDir, "views"), 0o755); err != nil {
		t.Fatalf("mkdir views: %v", err)
	}
	origHTML := `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>Workspace Initial View Page</title>
  <meta name="description" content="Initial workspace HTML file verified by server tests for Groxy AI snapshot safety." />
  <link rel="canonical" href="http://127.0.0.1:9490/" />
</head>
<body>
  <header><h1>Initial View</h1></header>
  <main><section><h2>Body</h2><p>Original bytes.</p></section></main>
  <footer><p>Footer</p></footer>
</body>
</html>
`
	if err := os.WriteFile(filepath.Join(wsDir, "views", "page.grab.html"), []byte(origHTML), 0o644); err != nil {
		t.Fatalf("write page.grab.html: %v", err)
	}
	if err := os.WriteFile(filepath.Join(wsDir, ".env"), []byte("SECRET_KEY=topsecret\n"), 0o644); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	viewsFS := fstest.MapFS{
		"workbench.grab.html": &fstest.MapFile{
			Data: []byte(`<!DOCTYPE html><html lang="en" data-grex-theme="{initial_theme}"><head><title>Groxy Desk</title></head><body><main><h1>Desk</h1></main><script type="module" src="/public/desk/workbench.js"></script></body></html>`),
		},
	}
	publicFS := fstest.MapFS{
		"desk/workbench.js": &fstest.MapFile{
			Data: []byte(`export const ready = true;`),
		},
	}

	srv, err := New(Config{
		ListenAddr:    "127.0.0.1:0",
		WorkspaceRoot: wsDir,
		StateDir:      stateDir,
		ViewsFS:       viewsFS,
		PublicFS:      publicFS,
	})
	if err != nil {
		t.Fatalf("server.New failed: %v", err)
	}
	if err := srv.Start(); err != nil {
		t.Fatalf("srv.Start failed: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	})
	return srv, wsDir, origHTML
}

func doAPIReq(t *testing.T, method, url, bearer, csrf string, body any) (*http.Response, map[string]any, string) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	if csrf != "" {
		req.Header.Set("X-CSRF-Token", csrf)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("HTTP %s %s failed: %v", method, url, err)
	}
	rawBytes, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	var parsed map[string]any
	_ = json.Unmarshal(rawBytes, &parsed)
	return resp, parsed, string(rawBytes)
}

func TestLoopbackBindRejectsNonLoopback(t *testing.T) {
	for _, bad := range []string{"0.0.0.0:9490", "[::]:9490", "192.168.1.25:9490"} {
		_, err := New(Config{ListenAddr: bad, WorkspaceRoot: t.TempDir(), StateDir: t.TempDir()})
		if err == nil || security.ErrorCode(err, "") != security.CodeDeskE501 {
			t.Fatalf("expected DESK-E501 for bind %q, got %v", bad, err)
		}
	}
}

func TestAuthCSRFHeadersAndHealth(t *testing.T) {
	srv, _, _ := setupTestServer(t)
	sess := srv.Session()

	// Public endpoints + security headers check
	resp, healthMap, _ := doAPIReq(t, http.MethodGet, sess.URL+"/_health", "", "", nil)
	if resp.StatusCode != 200 || healthMap["status"] != "ok" {
		t.Fatalf("expected 200 ok health, got %d %v", resp.StatusCode, healthMap)
	}
	if resp.Header.Get("X-Frame-Options") != "DENY" || resp.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("missing security headers on /_health: %v", resp.Header)
	}
	if !strings.Contains(resp.Header.Get("Content-Security-Policy"), "script-src 'self'") {
		t.Fatalf("missing CSP script-src 'self': %q", resp.Header.Get("Content-Security-Policy"))
	}

	// Ensure HTML never leaks bearer or CSRF token
	_, _, indexHTML := doAPIReq(t, http.MethodGet, sess.URL+"/", "", "", nil)
	if strings.Contains(indexHTML, sess.BearerToken) || strings.Contains(indexHTML, sess.CSRFToken) {
		t.Fatalf("root HTML leaked bearer or CSRF token")
	}

	// Missing bearer / wrong bearer / missing CSRF MUST return 403 DESK-E403
	for _, tc := range []struct {
		name, bearer, csrf string
	}{
		{"missing_both", "", ""},
		{"missing_csrf", sess.BearerToken, ""},
		{"wrong_bearer", "deadbeef", sess.CSRFToken},
		{"wrong_csrf", sess.BearerToken, "badcsrf"},
	} {
		r, body, _ := doAPIReq(t, http.MethodGet, sess.URL+"/api/workspace/tree?dir=.", tc.bearer, tc.csrf, nil)
		errObj, _ := body["error"].(map[string]any)
		if r.StatusCode != 403 || errObj["code"] != security.CodeDeskE403 {
			t.Fatalf("case %s expected 403 DESK-E403, got %d %v", tc.name, r.StatusCode, body)
		}
	}
}

func TestHTTPConfinementAndThemePersistence(t *testing.T) {
	srv, _, _ := setupTestServer(t)
	sess := srv.Session()

	for _, badPath := range []string{"../etc/passwd", ".env", ".git/config", "certs/server.key", "id_rsa", "id_ed25519"} {
		r, body, _ := doAPIReq(t, http.MethodGet, sess.URL+"/api/workspace/read?path="+badPath, sess.BearerToken, sess.CSRFToken, nil)
		errObj, _ := body["error"].(map[string]any)
		if r.StatusCode != 403 || errObj["code"] != security.CodeGrabE401 {
			t.Fatalf("expected 403 GRAB-E401 for %q, got %d %v", badPath, r.StatusCode, body)
		}
	}

	// Persist dark theme and verify root HTML injects data-grex-theme="dark"
	r, _, _ := doAPIReq(t, http.MethodPost, sess.URL+"/api/preferences/theme", sess.BearerToken, sess.CSRFToken, map[string]string{"theme": "dark"})
	if r.StatusCode != 200 {
		t.Fatalf("expected 200 saving dark theme, got %d", r.StatusCode)
	}
	_, _, indexHTML := doAPIReq(t, http.MethodGet, sess.URL+"/", "", "", nil)
	if !strings.Contains(indexHTML, `data-grex-theme="dark"`) {
		t.Fatalf("expected SSR dark theme attribute in index HTML, got %s", indexHTML)
	}
}

func TestHTTPGroxyPlanApplyReplayRestoreAndGrabmanPlanGate(t *testing.T) {
	srv, wsDir, origHTML := setupTestServer(t)
	sess := srv.Session()
	origSHA := security.SHA256Hex([]byte(origHTML))

	// Deep Ask -> Plan + Pre-mutation snapshot + approval_token
	newHTML := strings.ReplaceAll(origHTML, "Original bytes.", "Updated bytes cleanly.")
	r, askBody, _ := doAPIReq(t, http.MethodPost, sess.URL+"/api/groxy/ask", sess.BearerToken, sess.CSRFToken, map[string]any{
		"prompt":           "Update body copy safely",
		"target_path":      "views/page.grab.html",
		"proposed_content": newHTML,
	})
	if r.StatusCode != 200 {
		t.Fatalf("groxy ask failed: %d %v", r.StatusCode, askBody)
	}
	token, _ := askBody["approval_token"].(string)
	snapObj, _ := askBody["snapshot"].(map[string]any)
	snapID, _ := snapObj["snapshot_id"].(string)
	if token == "" || snapID == "" {
		t.Fatalf("missing token or snapshot id in deep ask response: %v", askBody)
	}

	// Target mismatch -> 403 DESK-E403
	r, mismatchBody, _ := doAPIReq(t, http.MethodPost, sess.URL+"/api/groxy/approvals/apply", sess.BearerToken, sess.CSRFToken, map[string]any{
		"approval_token": token,
		"target_path":    "views/other.grab.html",
	})
	if r.StatusCode != 403 {
		t.Fatalf("expected 403 mismatch, got %d %v", r.StatusCode, mismatchBody)
	}

	// Valid apply -> 200
	r, applyBody, _ := doAPIReq(t, http.MethodPost, sess.URL+"/api/groxy/approvals/apply", sess.BearerToken, sess.CSRFToken, map[string]any{
		"approval_token": token,
		"target_path":    "views/page.grab.html",
	})
	if r.StatusCode != 200 || applyBody["ok"] != true {
		t.Fatalf("apply failed: %d %v", r.StatusCode, applyBody)
	}

	// Replay -> 409 DESK-E409
	r, replayBody, _ := doAPIReq(t, http.MethodPost, sess.URL+"/api/groxy/approvals/apply", sess.BearerToken, sess.CSRFToken, map[string]any{
		"approval_token": token,
		"target_path":    "views/page.grab.html",
	})
	errObj, _ := replayBody["error"].(map[string]any)
	if r.StatusCode != 409 || errObj["code"] != security.CodeDeskE409 {
		t.Fatalf("expected 409 DESK-E409 replay rejection, got %d %v", r.StatusCode, replayBody)
	}

	// Snapshot restore -> 200 and exact original file bytes restored
	r, restBody, _ := doAPIReq(t, http.MethodPost, sess.URL+"/api/groxy/snapshots/restore", sess.BearerToken, sess.CSRFToken, map[string]any{
		"snapshot_id": snapID,
	})
	if r.StatusCode != 200 || restBody["restored_sha256"] != origSHA {
		t.Fatalf("expected restored_sha256=%s, got %d %v", origSHA, r.StatusCode, restBody)
	}

	// Verify grabman init cannot run without valid approval_token
	r, initFailBody, _ := doAPIReq(t, http.MethodPost, sess.URL+"/api/grabman/init", sess.BearerToken, sess.CSRFToken, map[string]any{
		"target_dir":     "subproj",
		"profile":        "app",
		"edge_proxy":     "caddy",
		"approval_token": "",
	})
	if r.StatusCode != 403 {
		t.Fatalf("expected 403 when grabman/init called without token, got %d %v", r.StatusCode, initFailBody)
	}

	// Confirm plan endpoint works cleanly
	if err := os.MkdirAll(filepath.Join(wsDir, "subproj"), 0o755); err != nil {
		t.Fatalf("mkdir subproj: %v", err)
	}
	r, planBody, _ := doAPIReq(t, http.MethodPost, sess.URL+"/api/grabman/init/plan", sess.BearerToken, sess.CSRFToken, map[string]any{
		"target_dir": "subproj",
		"profile":    "app",
		"edge_proxy": "caddy",
	})
	if r.StatusCode != 200 || planBody["approval_token"] == "" {
		t.Fatalf("grabman/init/plan failed: %d %v", r.StatusCode, planBody)
	}
}

func TestStaticPublicServesWoff2WithCorrectMIME(t *testing.T) {
	srv, _, _ := setupTestServer(t)
	srv.publicFS = os.DirFS(filepath.Join("..", "..", "public"))
	sess := srv.Session()

	for _, rel := range []string{
		"/public/desk/fonts/urbanist-latin-400-normal.woff2",
		"/public/desk/fonts/urbanist-latin-500-normal.woff2",
		"/public/desk/fonts/urbanist-latin-600-normal.woff2",
		"/public/desk/fonts/urbanist-latin-700-normal.woff2",
	} {
		resp, err := http.Get(sess.URL + rel)
		if err != nil {
			t.Fatalf("GET %s failed: %v", rel, err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET %s status=%d, want 200", rel, resp.StatusCode)
		}
		if got := resp.Header.Get("Content-Type"); got != "font/woff2" {
			t.Fatalf("GET %s Content-Type=%q, want font/woff2", rel, got)
		}
		if got := resp.Header.Get("X-Content-Type-Options"); got != "nosniff" {
			t.Fatalf("GET %s X-Content-Type-Options=%q, want nosniff", rel, got)
		}
	}
}
