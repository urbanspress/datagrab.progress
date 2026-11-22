package server

import (
	"crypto/ed25519"
	"crypto/rand"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"groxy-desk/internal/security"

	"golang.org/x/crypto/ssh"
)

func TestV140DBConnectionsEncryptedProfilesAndAuth(t *testing.T) {
	srv, _, _ := setupTestServer(t)
	stateDir := srv.stateDir
	sess := srv.Session()

	// 1. Missing CSRF/Bearer fails closed (403 DESK-E403)
	rUnauth, _, _ := doAPIReq(t, http.MethodGet, sess.URL+"/api/db/connections", sess.BearerToken, "", nil)
	if rUnauth.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 without CSRF on /api/db/connections, got %d", rUnauth.StatusCode)
	}

	// 2. Malformed SSH fingerprint rejects with 403 DESK-E405
	rBadPin, objBadPin, _ := doAPIReq(t, http.MethodPost, sess.URL+"/api/db/connections", sess.BearerToken, sess.CSRFToken, map[string]any{
		"name":               "Bad SSH",
		"engine":             "postgres",
		"ssh_host":           "203.0.113.10",
		"ssh_user":           "deploy",
		"pinned_fingerprint": "not-sha256",
	})
	if rBadPin.StatusCode != http.StatusForbidden || objBadPin["error"].(map[string]any)["code"] != security.CodeDeskE405 {
		t.Fatalf("expected 403 DESK-E405 on malformed SSH pin, got %d %v", rBadPin.StatusCode, objBadPin)
	}

	// 3. Save profile with secrets -> secrets encrypted at rest (v1:...) and never returned in JSON
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	pub, _ := ssh.NewPublicKey(priv.Public())
	fp := security.SSHFingerprintSHA256(pub)

	rSave, objSave, rawSave := doAPIReq(t, http.MethodPost, sess.URL+"/api/db/connections", sess.BearerToken, sess.CSRFToken, map[string]any{
		"id":                 "prod-pg",
		"name":               "Production PostgreSQL",
		"engine":             "postgres",
		"db_host":            "127.0.0.1",
		"db_port":            "5432",
		"db_user":            "app_reader",
		"db_password":        "super-secret-pg-pass-99",
		"db_name":            "grablab",
		"schema_name":        "public",
		"ssh_enabled":        true,
		"ssh_host":           "198.51.100.42",
		"ssh_port":           "22",
		"ssh_user":           "ubuntu",
		"pinned_fingerprint": fp,
		"ssh_password":       "super-secret-ssh-pass-77",
		"remote_db_port":     "5432",
	})
	if rSave.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/db/connections failed: %d %s", rSave.StatusCode, rawSave)
	}
	if strings.Contains(rawSave, "super-secret-pg-pass-99") || strings.Contains(rawSave, "super-secret-ssh-pass-77") {
		t.Fatalf("plaintext secrets leaked in POST response: %s", rawSave)
	}
	connObj := objSave["connection"].(map[string]any)
	if connObj["has_db_password"] != true || connObj["has_ssh_password"] != true || connObj["db_password_masked"] != "********" {
		t.Fatalf("expected masked secret flags, got %v", connObj)
	}

	// Verify stateDir/db-connections.json stores v1:... AEAD ciphertext, never plaintext
	diskBytes, err := os.ReadFile(filepath.Join(stateDir, "db-connections.json"))
	if err != nil {
		t.Fatalf("expected db-connections.json in stateDir: %v", err)
	}
	diskStr := string(diskBytes)
	if strings.Contains(diskStr, "super-secret-pg-pass-99") || strings.Contains(diskStr, "super-secret-ssh-pass-77") {
		t.Fatalf("plaintext secret written to disk: %s", diskStr)
	}
	if !strings.Contains(diskStr, `"v1:`) {
		t.Fatalf("expected v1:... AEAD ciphertext on disk, got %s", diskStr)
	}

	// 4. GET /api/db/connections lists saved profile without secrets
	rList, objList, rawList := doAPIReq(t, http.MethodGet, sess.URL+"/api/db/connections", sess.BearerToken, sess.CSRFToken, nil)
	if rList.StatusCode != http.StatusOK || strings.Contains(rawList, "super-secret") {
		t.Fatalf("GET /api/db/connections failed or leaked secret: %d %s", rList.StatusCode, rawList)
	}
	conns := objList["connections"].([]any)
	if len(conns) != 1 || conns[0].(map[string]any)["id"] != "prod-pg" {
		t.Fatalf("expected prod-pg connection in list, got %v", conns)
	}

	// 5. Updating metadata without resending password preserves existing encrypted secret
	rUpd, objUpd, _ := doAPIReq(t, http.MethodPost, sess.URL+"/api/db/connections", sess.BearerToken, sess.CSRFToken, map[string]any{
		"id":                 "prod-pg",
		"name":               "Production PostgreSQL Primary",
		"engine":             "postgres",
		"db_user":            "app_reader",
		"db_name":            "grablab",
		"pinned_fingerprint": fp,
	})
	if rUpd.StatusCode != http.StatusOK || objUpd["connection"].(map[string]any)["has_db_password"] != true {
		t.Fatalf("expected preserved secret on update, got %d %v", rUpd.StatusCode, objUpd)
	}
	dbPass, sshPass, _, resErr := srv.dbStudio.Connections().ResolveSecrets("prod-pg")
	if resErr != nil || dbPass != "super-secret-pg-pass-99" || sshPass != "super-secret-ssh-pass-77" {
		t.Fatalf("ResolveSecrets mismatch: db=%q ssh=%q err=%v", dbPass, sshPass, resErr)
	}

	// 6. DELETE /api/db/connections?id=prod-pg removes profile cleanly
	rDel, objDel, _ := doAPIReq(t, http.MethodDelete, sess.URL+"/api/db/connections?id=prod-pg", sess.BearerToken, sess.CSRFToken, nil)
	if rDel.StatusCode != http.StatusOK || objDel["deleted"] != true {
		t.Fatalf("DELETE /api/db/connections failed: %d %v", rDel.StatusCode, objDel)
	}
	if len(srv.dbStudio.Connections().List()) != 0 {
		t.Fatalf("expected empty connection list after delete")
	}
}
