package security

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateLoopbackBind(t *testing.T) {
	valid := []string{"127.0.0.1:9490", "127.0.0.1:0", "[::1]:9490"}
	for _, addr := range valid {
		got, err := ValidateLoopbackBind(addr)
		if err != nil {
			t.Fatalf("expected %q to succeed, got %v", addr, err)
		}
		if got == "" {
			t.Fatalf("expected normalized bind address for %q", addr)
		}
	}

	invalid := []string{"", "0.0.0.0:9490", ":9490", "[::]:9490", "*:9490", "192.168.1.10:9490", "example.com:9490", "127.0.0.1:99999"}
	for _, addr := range invalid {
		_, err := ValidateLoopbackBind(addr)
		if err == nil {
			t.Fatalf("expected non-loopback/invalid bind %q to fail closed", addr)
		}
		if code := ErrorCode(err, ""); code != CodeDeskE501 {
			t.Fatalf("expected DESK-E501 for %q, got %q (%v)", addr, code, err)
		}
	}
}

func TestConfineWorkspacePath(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "deploy"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "src", "app.grab"), []byte("show \"ok\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "deploy", ".env.example"), []byte("PORT=8080\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, rel, err := ConfineWorkspacePath(root, "src/app.grab"); err != nil || rel != "src/app.grab" {
		t.Fatalf("expected valid path src/app.grab, got rel=%q err=%v", rel, err)
	}
	if _, rel, err := ConfineWorkspacePath(root, "deploy/.env.example"); err != nil || rel != "deploy/.env.example" {
		t.Fatalf("expected .env.example to be allowed, got rel=%q err=%v", rel, err)
	}
	if _, rel, err := ConfineWorkspacePath(root, "views/new-page.grab.html"); err != nil || rel != "views/new-page.grab.html" {
		t.Fatalf("expected non-existent confined file path to pass, got rel=%q err=%v", rel, err)
	}

	blocked := []string{
		"",
		".",
		"../secret.txt",
		`..\secret.txt`,
		"src/../../etc/passwd",
		"bad\x00path.grab",
		`\\fileserver\share\app.grab`,
		".env",
		".env.local",
		".env.production",
		".git/config",
		".ssh/known_hosts",
		"keys/id_rsa",
		"keys/id_ed25519",
		"certs/server.pem",
		"certs/private.key",
		"certs/bundle.p12",
		"certs/cert.pfx",
	}
	for _, p := range blocked {
		_, _, err := ConfineWorkspacePath(root, p)
		if err == nil {
			t.Fatalf("expected path %q to be blocked by GRAB-E401", p)
		}
		if code := ErrorCode(err, ""); code != CodeGrabE401 {
			t.Fatalf("expected GRAB-E401 for %q, got %q (%v)", p, code, err)
		}
	}
}

func TestConfineWorkspaceSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	outsideFile := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(outsideFile, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	relTarget, err := createTestReparseEscape(root, "escape.txt", outside, outsideFile)
	if err != nil {
		t.Fatalf("failed to create reparse escape fixture: %v", err)
	}
	_, _, err = ConfineWorkspacePath(root, relTarget)
	if err == nil || ErrorCode(err, "") != CodeGrabE401 {
		t.Fatalf("expected symlink/junction escape %q to be blocked with GRAB-E401, got err=%v", relTarget, err)
	}
}

func TestSecretRedactorAndConstantTime(t *testing.T) {
	if !ConstantTimeEqual("abc123", "abc123") {
		t.Fatal("expected equal tokens to match")
	}
	if ConstantTimeEqual("", "") || ConstantTimeEqual("abc123", "abc124") {
		t.Fatal("expected mismatched or empty tokens to fail")
	}

	r := NewSecretRedactor("my-runtime-secret-token")
	raw := "Authorization: Bearer supersecret999 MASTER_KEY=deadbeef v1:AbCdEfGhIjKlMnOp my-runtime-secret-token sk-test1234567890"
	cleaned := r.Redact(raw)
	for _, leak := range []string{"supersecret999", "deadbeef", "AbCdEfGhIjKlMnOp", "my-runtime-secret-token", "test1234567890"} {
		if strings.Contains(cleaned, leak) {
			t.Fatalf("secret %q leaked after redaction: %q", leak, cleaned)
		}
	}
}

func TestADR025cExactNumbers(t *testing.T) {
	if !IsExactIntString("8765") || IsExactIntString("8765.5") {
		t.Fatal("unexpected integer validation")
	}
	if !IsExactDecimalString("19.9900") || IsExactDecimalString("0.30000000000000004") || IsExactDecimalString("1.9999999999999998") {
		t.Fatal("expected exact decimal validation and float drift rejection")
	}
}
