package security

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
)

// Error codes defined in GROXY-DESK-SPEC-001.md and GROXY-DESK-EXPERT.md.
const (
	CodeGrabE401 = "GRAB-E401"
	CodeDeskE101 = "DESK-E101"
	CodeDeskE201 = "DESK-E201"
	CodeDeskE301 = "DESK-E301"
	CodeDeskE302 = "DESK-E302"
	CodeDeskE303 = "DESK-E303"
	CodeDeskE304 = "DESK-E304"
	CodeDeskE403 = "DESK-E403"
	CodeDeskE405 = "DESK-E405"
	CodeDeskE406 = "DESK-E406"
	CodeDeskE409 = "DESK-E409"
	CodeDeskE501 = "DESK-E501"
	CodeDeskE502 = "DESK-E502"
	CodeDeskE503 = "DESK-E503"
)

const CodeWriteFenceUnavailable = "write_fence_unavailable"

// DeskError carries a stable diagnostic code, HTTP status, and plain-English message.
type DeskError struct {
	Code    string
	Status  int
	Message string
	Cause   error
}

func (e *DeskError) Error() string {
	if e == nil {
		return ""
	}
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s (%v)", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *DeskError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

// NewError creates a structured DeskError.
func NewError(code string, status int, msg string, cause error) *DeskError {
	return &DeskError{
		Code:    code,
		Status:  status,
		Message: msg,
		Cause:   cause,
	}
}

// ErrorCode extracts the DeskError code or returns fallback.
func ErrorCode(err error, fallback string) string {
	var de *DeskError
	if errors.As(err, &de) && de.Code != "" {
		return de.Code
	}
	return fallback
}

// ErrorStatus extracts the HTTP status from a DeskError or returns fallback.
func ErrorStatus(err error, fallback int) int {
	var de *DeskError
	if errors.As(err, &de) && de.Status > 0 {
		return de.Status
	}
	return fallback
}

// ValidateLoopbackBind fails closed (DESK-E501) on 0.0.0.0, ::, wildcards, hostnames, or non-loopback IPs.
func ValidateLoopbackBind(addr string) (string, error) {
	trimmed := strings.TrimSpace(addr)
	if trimmed == "" {
		return "", NewError(CodeDeskE501, 500, "bind address must not be empty; use 127.0.0.1:9490", nil)
	}
	host, portStr, err := net.SplitHostPort(trimmed)
	if err != nil {
		return "", NewError(CodeDeskE501, 500, fmt.Sprintf("invalid loopback bind address %q", trimmed), err)
	}
	if host == "" || host == "*" || host == "0.0.0.0" || host == "::" {
		return "", NewError(CodeDeskE501, 500, fmt.Sprintf("non-loopback wildcard bind %q is strictly forbidden (ADR-005)", trimmed), nil)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port < 0 || port > 65535 {
		return "", NewError(CodeDeskE501, 500, fmt.Sprintf("invalid TCP port in bind address %q", trimmed), err)
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return "", NewError(CodeDeskE501, 500, fmt.Sprintf("non-loopback bind address %q is forbidden; bind strictly to 127.0.0.1", trimmed), nil)
	}
	return net.JoinHostPort(ip.String(), strconv.Itoa(port)), nil
}

// ConstantTimeEqual compares two non-empty secret strings in constant time.
func ConstantTimeEqual(a, b string) bool {
	if len(a) == 0 || len(a) != len(b) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// RandomTokenHex generates nBytes of cryptographically random hex token.
func RandomTokenHex(nBytes int) (string, error) {
	if nBytes < 16 {
		nBytes = 16
	}
	buf := make([]byte, nBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("crypto/rand failed: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

// SHA256Hex returns the lowercase hex SHA-256 digest of data.
func SHA256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// IsSensitiveSegment reports whether a path segment or filename must be blocked by GRAB-E401.
func IsSensitiveSegment(seg string) bool {
	s := strings.ToLower(strings.TrimSpace(seg))
	if s == "" || s == "." {
		return false
	}
	if s == ".env.example" {
		return false
	}
	switch s {
	case ".git", ".ssh", ".groxy-desk", ".env", "id_rsa", "id_ed25519", "id_ecdsa", "id_dsa":
		return true
	}
	if strings.HasPrefix(s, ".env.") {
		return true
	}
	for _, prefix := range []string{"id_rsa.", "id_ed25519.", "id_ecdsa.", "id_dsa."} {
		if strings.HasPrefix(s, prefix) {
			return true
		}
	}
	for _, ext := range []string{".pem", ".key", ".p12", ".pfx"} {
		if strings.HasSuffix(s, ext) {
			return true
		}
	}
	return false
}

// ConfineWorkspacePath resolves target strictly within workspaceRoot, enforcing GRAB-E401.
func ConfineWorkspacePath(workspaceRoot, target string) (string, string, error) {
	return confinePath(workspaceRoot, target, false)
}

// ConfineWorkspaceDir resolves target directory (allowing "." for workspace root) strictly within workspaceRoot.
func ConfineWorkspaceDir(workspaceRoot, target string) (string, string, error) {
	if strings.TrimSpace(target) == "" {
		target = "."
	}
	return confinePath(workspaceRoot, target, true)
}

func confinePath(workspaceRoot, target string, allowRoot bool) (string, string, error) {
	rootTrim := strings.TrimSpace(workspaceRoot)
	if rootTrim == "" {
		return "", "", NewError(CodeGrabE401, 403, "workspace root must not be empty", nil)
	}
	if strings.ContainsRune(rootTrim, '\x00') || strings.ContainsRune(target, '\x00') {
		return "", "", NewError(CodeGrabE401, 403, "NUL byte in path is forbidden", nil)
	}
	targetTrim := strings.TrimSpace(target)
	if targetTrim == "" {
		return "", "", NewError(CodeGrabE401, 403, "target path must not be empty", nil)
	}
	if strings.HasPrefix(targetTrim, `\\`) || strings.HasPrefix(targetTrim, `//`) {
		return "", "", NewError(CodeGrabE401, 403, fmt.Sprintf("UNC/network path %q is forbidden", target), nil)
	}

	rawParts := strings.FieldsFunc(targetTrim, func(r rune) bool {
		return r == '/' || r == '\\'
	})
	for _, part := range rawParts {
		if part == ".." {
			return "", "", NewError(CodeGrabE401, 403, fmt.Sprintf("path traversal '..' in %q is forbidden", target), nil)
		}
		if IsSensitiveSegment(part) {
			return "", "", NewError(CodeGrabE401, 403, fmt.Sprintf("access to sensitive path segment %q is blocked by GRAB-E401", part), nil)
		}
	}

	absRoot, err := filepath.Abs(rootTrim)
	if err != nil {
		return "", "", NewError(CodeGrabE401, 403, "unable to resolve workspace root", err)
	}
	realRoot, err := filepath.EvalSymlinks(absRoot)
	if err != nil {
		return "", "", NewError(CodeGrabE401, 403, "unable to verify workspace root real path", err)
	}

	var candidate string
	if filepath.IsAbs(targetTrim) {
		candidate = filepath.Clean(targetTrim)
	} else {
		candidate = filepath.Join(absRoot, filepath.FromSlash(strings.ReplaceAll(targetTrim, `\`, "/")))
	}

	relRaw, err := filepath.Rel(absRoot, candidate)
	if err != nil || !isCleanRelative(relRaw, allowRoot) {
		return "", "", NewError(CodeGrabE401, 403, fmt.Sprintf("path %q escapes workspace root", target), err)
	}

	for _, seg := range splitRelParts(relRaw) {
		if IsSensitiveSegment(seg) {
			return "", "", NewError(CodeGrabE401, 403, fmt.Sprintf("access to sensitive path %q is blocked by GRAB-E401", seg), nil)
		}
	}

	// Verify no existing parent or target symlink escapes realRoot or points to a sensitive path.
	if err := checkSymlinkChain(realRoot, absRoot, relRaw); err != nil {
		return "", "", err
	}

	resolvedTarget, err := resolveExistingOrParent(candidate)
	if err != nil {
		return "", "", NewError(CodeGrabE401, 403, fmt.Sprintf("unable to resolve path %q", target), err)
	}
	relReal, err := filepath.Rel(realRoot, resolvedTarget)
	if err != nil || !isCleanRelative(relReal, true) {
		return "", "", NewError(CodeGrabE401, 403, fmt.Sprintf("resolved path for %q escapes workspace root", target), err)
	}
	for _, seg := range splitRelParts(relReal) {
		if IsSensitiveSegment(seg) {
			return "", "", NewError(CodeGrabE401, 403, fmt.Sprintf("symlink resolves to sensitive segment %q", seg), nil)
		}
	}

	cleanSlash := filepath.ToSlash(relRaw)
	if cleanSlash == "" {
		cleanSlash = "."
	}
	if !allowRoot && cleanSlash == "." {
		return "", "", NewError(CodeGrabE401, 403, "target file path cannot be workspace root '.'", nil)
	}

	// Return path anchored at realRoot when target exists or via clean joined realRoot + relRaw.
	finalAbs := filepath.Join(realRoot, relRaw)
	return finalAbs, cleanSlash, nil
}

func checkSymlinkChain(realRoot, absRoot, rel string) error {
	parts := splitRelParts(rel)
	curr := absRoot
	for _, p := range parts {
		curr = filepath.Join(curr, p)
		info, err := os.Lstat(curr)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				break
			}
			return NewError(CodeGrabE401, 403, fmt.Sprintf("stat failed for %q", curr), err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			realStep, err := filepath.EvalSymlinks(curr)
			if err != nil {
				return NewError(CodeGrabE401, 403, "dangling or escaping symlink blocked", err)
			}
			relStep, err := filepath.Rel(realRoot, realStep)
			if err != nil || !isCleanRelative(relStep, true) {
				return NewError(CodeGrabE401, 403, "symlink escapes workspace confinement", err)
			}
			for _, seg := range splitRelParts(relStep) {
				if IsSensitiveSegment(seg) {
					return NewError(CodeGrabE401, 403, fmt.Sprintf("symlink targets sensitive segment %q", seg), nil)
				}
			}
		}
	}
	return nil
}

func resolveExistingOrParent(candidate string) (string, error) {
	if _, err := os.Lstat(candidate); err == nil {
		return filepath.EvalSymlinks(candidate)
	}
	curr := candidate
	var tail []string
	for {
		parent := filepath.Dir(curr)
		if parent == curr {
			return "", fmt.Errorf("no existing ancestor for %q", candidate)
		}
		tail = append([]string{filepath.Base(curr)}, tail...)
		if _, err := os.Lstat(parent); err == nil {
			realParent, err := filepath.EvalSymlinks(parent)
			if err != nil {
				return "", err
			}
			out := realParent
			for _, part := range tail {
				out = filepath.Join(out, part)
			}
			return out, nil
		}
		curr = parent
	}
}

func isCleanRelative(rel string, allowRoot bool) bool {
	if rel == "." {
		return allowRoot
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false
	}
	if filepath.IsAbs(rel) {
		return false
	}
	if runtime.GOOS == "windows" && strings.Contains(rel, ":") {
		return false
	}
	return true
}

func splitRelParts(rel string) []string {
	if rel == "" || rel == "." {
		return nil
	}
	return strings.FieldsFunc(rel, func(r rune) bool {
		return r == '/' || r == '\\'
	})
}

// ValidateSafeCommandBinary ensures structured argv execution never invokes a shell interpreter.
func ValidateSafeCommandBinary(bin string, args []string) error {
	clean := strings.TrimSpace(bin)
	if clean == "" {
		return NewError(CodeDeskE301, 422, "command binary path must not be empty", nil)
	}
	base := strings.ToLower(filepath.Base(clean))
	switch base {
	case "sh", "bash", "zsh", "dash", "ash", "cmd", "cmd.exe", "powershell", "powershell.exe", "pwsh", "pwsh.exe":
		return NewError(CodeDeskE301, 422, fmt.Sprintf("shell interpreter %q is strictly forbidden (ADR-017)", base), nil)
	}
	for _, a := range args {
		if strings.ContainsRune(a, '\x00') {
			return NewError(CodeDeskE301, 422, "NUL byte in command argument is forbidden", nil)
		}
	}
	return nil
}

// SecretRedactor scrubs Bearer tokens, AEAD ciphertexts, API keys, and registered secrets from logs.
type SecretRedactor struct {
	mu      sync.RWMutex
	secrets []string
}

var (
	reAuthBearer = regexp.MustCompile(`(?i)(authorization\s*:\s*bearer\s+)([^\s"']+)`)
	reBearer     = regexp.MustCompile(`(?i)(\bbearer\s+)([A-Za-z0-9._\-+/=]{8,})`)
	reAEAD       = regexp.MustCompile(`\bv1:[A-Za-z0-9+/=_-]{8,}(?::[A-Za-z0-9+/=_-]{8,})?`)
	reKeyAssign  = regexp.MustCompile(`(?i)\b(MASTER_KEY|GRAB_MASTER_KEY|GROXY_DESK_MASTER_KEY|GROXY_API_KEY|OPENAI_API_KEY|ANTHROPIC_API_KEY|GEMINI_API_KEY|NAMECHEAP_API_KEY|API_KEY|SECRET_KEY|PASSWORD|DB_PASSWORD|AUTH_TOKEN|CSRF_TOKEN)\s*([:=])\s*([^\s"';]+)`)
	reSKToken    = regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{12,}`)
)

// NewSecretRedactor constructs a SecretRedactor with optional initial secrets.
func NewSecretRedactor(initial ...string) *SecretRedactor {
	sr := &SecretRedactor{}
	for _, s := range initial {
		sr.Register(s)
	}
	return sr
}

// Register adds a non-empty secret string to be scrubbed from logs.
func (r *SecretRedactor) Register(secret string) {
	s := strings.TrimSpace(secret)
	if len(s) < 6 {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, existing := range r.secrets {
		if existing == s {
			return
		}
	}
	r.secrets = append(r.secrets, s)
}

// Redact replaces known secrets and sensitive token patterns with [REDACTED].
func (r *SecretRedactor) Redact(input string) string {
	out := input
	out = reAuthBearer.ReplaceAllString(out, "${1}[REDACTED]")
	out = reBearer.ReplaceAllString(out, "${1}[REDACTED]")
	out = reAEAD.ReplaceAllString(out, "v1:[REDACTED]")
	out = reKeyAssign.ReplaceAllString(out, "${1}${2}[REDACTED]")
	out = reSKToken.ReplaceAllString(out, "sk-[REDACTED]")

	if r != nil {
		r.mu.RLock()
		secrets := append([]string(nil), r.secrets...)
		r.mu.RUnlock()
		for _, s := range secrets {
			if strings.Contains(out, s) {
				out = strings.ReplaceAll(out, s, "[REDACTED]")
			}
		}
	}
	return out
}

var (
	reExactInt     = regexp.MustCompile(`^-?\d+$`)
	reExactDecimal = regexp.MustCompile(`^-?\d+(?:\.\d+)?$`)
	reFloatDrift   = regexp.MustCompile(`\.\d*?(?:000000|999999)\d$`)
)

// IsExactIntString checks ADR-025c base-10 integer string preservation.
func IsExactIntString(s string) bool {
	return reExactInt.MatchString(strings.TrimSpace(s))
}

// IsExactDecimalString checks ADR-025c base-10 decimal string preservation without IEEE-754 drift.
func IsExactDecimalString(s string) bool {
	trimmed := strings.TrimSpace(s)
	if !reExactDecimal.MatchString(trimmed) {
		return false
	}
	return !reFloatDrift.MatchString(trimmed)
}
