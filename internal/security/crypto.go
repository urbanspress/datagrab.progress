package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/ssh"
)

const masterKeyFileName = "desk-master.key"

// ResolveMasterKey resolves a 32-byte AES-256 master key from GROXY_DESK_MASTER_KEY
// (falling back to GRAB_MASTER_KEY), or creates/loads a 0600 key outside the workspace.
func ResolveMasterKey(stateDir string) ([]byte, error) {
	for _, envName := range []string{"GROXY_DESK_MASTER_KEY", "GRAB_MASTER_KEY"} {
		if raw := strings.TrimSpace(os.Getenv(envName)); raw != "" {
			return deriveAES256Key(raw), nil
		}
	}
	cleanState := strings.TrimSpace(stateDir)
	if cleanState == "" {
		sum := sha256.Sum256([]byte("groxy-desk-ephemeral-fallback-key"))
		out := make([]byte, 32)
		copy(out, sum[:])
		return out, nil
	}
	if err := os.MkdirAll(cleanState, 0o700); err != nil {
		return nil, NewError(CodeDeskE101, 500, "failed to prepare master key state directory", err)
	}
	keyPath := filepath.Join(cleanState, masterKeyFileName)
	if rawBytes, err := os.ReadFile(keyPath); err == nil {
		if trimmed := strings.TrimSpace(string(rawBytes)); trimmed != "" {
			return deriveAES256Key(trimmed), nil
		}
	}
	var rnd [32]byte
	if _, err := io.ReadFull(rand.Reader, rnd[:]); err != nil {
		return nil, NewError(CodeDeskE101, 500, "failed to generate AES-256 master key", err)
	}
	encoded := hex.EncodeToString(rnd[:])
	if err := os.WriteFile(keyPath, []byte(encoded+"\n"), 0o600); err != nil {
		return nil, NewError(CodeDeskE101, 500, "failed to persist AES-256 master key", err)
	}
	out := make([]byte, 32)
	copy(out, rnd[:])
	return out, nil
}

func deriveAES256Key(raw string) []byte {
	s := strings.TrimSpace(raw)
	if len(s) == 64 {
		if decoded, err := hex.DecodeString(s); err == nil && len(decoded) == 32 {
			return decoded
		}
	}
	for _, enc := range []*base64.Encoding{
		base64.RawURLEncoding,
		base64.URLEncoding,
		base64.StdEncoding,
		base64.RawStdEncoding,
	} {
		if decoded, err := enc.DecodeString(s); err == nil && len(decoded) == 32 {
			return decoded
		}
	}
	if len(s) == 32 {
		out := make([]byte, 32)
		copy(out, s)
		return out
	}
	sum := sha256.Sum256([]byte(s))
	out := make([]byte, 32)
	copy(out, sum[:])
	return out
}

// EncryptAEAD encrypts UTF-8 plaintext using AES-256-GCM in v1:<nonce_b64>:<ciphertext_b64> format.
func EncryptAEAD(key []byte, plaintext string) (string, error) {
	if len(key) != 32 {
		return "", NewError(CodeDeskE101, 500, "AES-256-GCM requires a 32-byte key", nil)
	}
	if !utf8.ValidString(plaintext) {
		return "", NewError(CodeDeskE101, 400, "plaintext must be valid UTF-8", nil)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", NewError(CodeDeskE101, 500, "failed to create AES-256 cipher", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", NewError(CodeDeskE101, 500, "failed to initialize GCM", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", NewError(CodeDeskE101, 500, "failed to generate AEAD nonce", err)
	}
	sealed := gcm.Seal(nil, nonce, []byte(plaintext), nil)
	return "v1:" + base64.RawStdEncoding.EncodeToString(nonce) + ":" + base64.RawStdEncoding.EncodeToString(sealed), nil
}

// DecryptAEAD decrypts both v1:<nonce_b64>:<ciphertext_b64> and GRAB v1.1.0 use crypto v1:<rawurl> ciphertexts.
func DecryptAEAD(key []byte, encoded string) (string, error) {
	if len(key) != 32 {
		return "", NewError(CodeDeskE101, 500, "AES-256-GCM requires a 32-byte key", nil)
	}
	trimmed := strings.TrimSpace(encoded)
	if !strings.HasPrefix(trimmed, "v1:") {
		return "", NewError(CodeDeskE101, 400, "unsupported ciphertext prefix; expected v1:", nil)
	}
	rest := strings.TrimPrefix(trimmed, "v1:")

	var nonce, ciphertext []byte
	if idx := strings.IndexByte(rest, ':'); idx >= 0 {
		var err error
		nonce, err = decodeAnyBase64(rest[:idx])
		if err != nil {
			return "", NewError(CodeDeskE101, 400, "invalid AEAD nonce encoding", err)
		}
		ciphertext, err = decodeAnyBase64(rest[idx+1:])
		if err != nil {
			return "", NewError(CodeDeskE101, 400, "invalid AEAD ciphertext encoding", err)
		}
	} else {
		combined, err := decodeAnyBase64(rest)
		if err != nil || len(combined) < 12+16 {
			return "", NewError(CodeDeskE101, 400, "invalid compact v1 AEAD payload", err)
		}
		nonce = combined[:12]
		ciphertext = combined[12:]
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", NewError(CodeDeskE101, 500, "failed to create AES-256 cipher", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", NewError(CodeDeskE101, 500, "failed to initialize GCM", err)
	}
	if len(nonce) != gcm.NonceSize() {
		return "", NewError(CodeDeskE101, 400, "invalid AEAD nonce size", nil)
	}
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", NewError(CodeDeskE403, 403, "AEAD credential decryption failed", err)
	}
	return string(plain), nil
}

func decodeAnyBase64(s string) ([]byte, error) {
	for _, enc := range []*base64.Encoding{
		base64.RawStdEncoding,
		base64.StdEncoding,
		base64.RawURLEncoding,
		base64.URLEncoding,
	} {
		if b, err := enc.DecodeString(s); err == nil {
			return b, nil
		}
	}
	return nil, fmt.Errorf("invalid base64 segment")
}

// MaskSecret renders a masked representation (e.g., nc_****...ab12) safe for UI and API responses.
func MaskSecret(prefix, secret string) string {
	trimmed := strings.TrimSpace(secret)
	if trimmed == "" {
		return ""
	}
	if len(trimmed) <= 4 {
		return prefix + "****"
	}
	return prefix + "****..." + trimmed[len(trimmed)-4:]
}

// SSHFingerprintSHA256 computes OpenSSH SHA256:<base64> fingerprint for a public key.
func SSHFingerprintSHA256(pub ssh.PublicKey) string {
	return ssh.FingerprintSHA256(pub)
}

// NormalizeSSHFingerprint validates and canonicalizes a SHA256:... SSH host key fingerprint.
func NormalizeSSHFingerprint(fp string) (string, error) {
	trimmed := strings.TrimSpace(fp)
	if !strings.HasPrefix(trimmed, "SHA256:") {
		return "", NewError(CodeDeskE405, 403, "SSH host key fingerprint must use pinned SHA256:... format (DESK-E405)", nil)
	}
	body := strings.TrimSpace(strings.TrimPrefix(trimmed, "SHA256:"))
	if len(body) < 16 {
		return "", NewError(CodeDeskE405, 403, "SSH host key fingerprint SHA256:... is truncated or empty (DESK-E405)", nil)
	}
	return "SHA256:" + body, nil
}

// VerifySSHHostKey verifies a presented SSH public key against a pinned SHA256:... fingerprint
// and never falls back to InsecureIgnoreHostKey.
func VerifySSHHostKey(expectedFingerprint string, presented ssh.PublicKey) error {
	expected, err := NormalizeSSHFingerprint(expectedFingerprint)
	if err != nil {
		return err
	}
	if presented == nil {
		return NewError(CodeDeskE405, 403, "missing SSH host public key during verification (DESK-E405)", nil)
	}
	actual := SSHFingerprintSHA256(presented)
	if !ConstantTimeEqual(actual, expected) {
		return NewError(CodeDeskE405, 403, fmt.Sprintf("SSH host key fingerprint mismatch (expected %s, got %s) — connection blocked (DESK-E405)", expected, actual), nil)
	}
	return nil
}

// VerifySSHFingerprintString checks two SHA256:... fingerprints without ignoring host keys.
func VerifySSHFingerprintString(expectedFingerprint, presentedFingerprint string) error {
	expected, err := NormalizeSSHFingerprint(expectedFingerprint)
	if err != nil {
		return err
	}
	presented, err := NormalizeSSHFingerprint(presentedFingerprint)
	if err != nil {
		return err
	}
	if !ConstantTimeEqual(expected, presented) {
		return NewError(CodeDeskE405, 403, fmt.Sprintf("SSH known_hosts mismatch: expected %s, got %s (DESK-E405)", expected, presented), nil)
	}
	return nil
}

// NewPinnedHostKeyCallback constructs a fail-closed ssh.HostKeyCallback verifying known_hosts and SHA256 pin.
func NewPinnedHostKeyCallback(expectedFingerprint, knownHostsPath string) (ssh.HostKeyCallback, error) {
	expected, err := NormalizeSSHFingerprint(expectedFingerprint)
	if err != nil {
		return nil, err
	}
	var khEntries []ssh.PublicKey
	if strings.TrimSpace(knownHostsPath) != "" {
		parsed, err := ReadKnownHostKeys(knownHostsPath)
		if err != nil {
			return nil, err
		}
		khEntries = parsed
	}
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		if err := VerifySSHHostKey(expected, key); err != nil {
			return err
		}
		if len(khEntries) > 0 {
			actual := SSHFingerprintSHA256(key)
			for _, allowed := range khEntries {
				if ConstantTimeEqual(actual, SSHFingerprintSHA256(allowed)) {
					return nil
				}
			}
			return NewError(CodeDeskE405, 403, fmt.Sprintf("SSH host %s key %s not present in pinned known_hosts (DESK-E405)", hostname, actual), nil)
		}
		return nil
	}, nil
}

// ReadKnownHostKeys parses OpenSSH known_hosts public keys without requiring hostname hashing.
func ReadKnownHostKeys(knownHostsPath string) ([]ssh.PublicKey, error) {
	raw, err := os.ReadFile(knownHostsPath)
	if err != nil {
		return nil, NewError(CodeDeskE405, 403, "failed to read pinned known_hosts file (DESK-E405)", err)
	}
	var keys []ssh.PublicKey
	rest := raw
	for len(rest) > 0 {
		_, _, pub, _, next, parseErr := ssh.ParseKnownHosts(rest)
		if parseErr != nil {
			break
		}
		if pub != nil {
			keys = append(keys, pub)
		}
		rest = next
	}
	if len(keys) == 0 {
		return nil, NewError(CodeDeskE405, 403, "pinned known_hosts contains no valid SSH public keys (DESK-E405)", nil)
	}
	return keys, nil
}
