package security

import (
	"crypto/ed25519"
	"crypto/rand"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
)

func TestEncryptDecryptAEADAndRedaction(t *testing.T) {
	tmp := t.TempDir()
	key, err := ResolveMasterKey(tmp)
	if err != nil {
		t.Fatalf("ResolveMasterKey failed: %v", err)
	}
	if len(key) != 32 {
		t.Fatalf("expected 32-byte key, got %d", len(key))
	}

	ct, err := EncryptAEAD(key, "nc-secret-api-key-9876")
	if err != nil {
		t.Fatalf("EncryptAEAD failed: %v", err)
	}
	if !strings.HasPrefix(ct, "v1:") || strings.Count(ct, ":") != 2 {
		t.Fatalf("expected v1:<nonce_b64>:<ciphertext_b64>, got %q", ct)
	}
	plain, err := DecryptAEAD(key, ct)
	if err != nil || plain != "nc-secret-api-key-9876" {
		t.Fatalf("DecryptAEAD failed: plain=%q err=%v", plain, err)
	}

	redactor := NewSecretRedactor("nc-secret-api-key-9876")
	scrubbed := redactor.Redact("token=" + ct + " NAMECHEAP_API_KEY=rawval")
	if strings.Contains(scrubbed, "nc-secret") || strings.Contains(scrubbed, "rawval") || strings.Contains(scrubbed, ct) {
		t.Fatalf("redactor failed to scrub v1 AEAD or key: %q", scrubbed)
	}
}

func TestPinnedSSHHostKeyVerification(t *testing.T) {
	_, privA, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	pubA, err := ssh.NewPublicKey(privA.Public())
	if err != nil {
		t.Fatal(err)
	}
	_, privB, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	pubB, err := ssh.NewPublicKey(privB.Public())
	if err != nil {
		t.Fatal(err)
	}

	fpA := SSHFingerprintSHA256(pubA)
	if err := VerifySSHHostKey(fpA, pubA); err != nil {
		t.Fatalf("expected pinned fingerprint match, got %v", err)
	}
	mismatch := VerifySSHHostKey(fpA, pubB)
	if mismatch == nil || ErrorCode(mismatch, "") != CodeDeskE405 || ErrorStatus(mismatch, 0) != 403 {
		t.Fatalf("expected 403 DESK-E405 on SSH mismatch, got %v", mismatch)
	}
	missing := VerifySSHHostKey("invalid-fp", pubA)
	if missing == nil || ErrorCode(missing, "") != CodeDeskE405 {
		t.Fatalf("expected DESK-E405 on missing/bad fingerprint, got %v", missing)
	}
}
