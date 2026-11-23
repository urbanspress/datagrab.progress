package workspace

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"groxy-desk/internal/deskexec"
	"groxy-desk/internal/security"
)

const MaxReadBytes = 10 * 1024 * 1024 // 10 MiB

// Manager provides workspace-confined file access and structured argv wrappers for grab & grabman.
type Manager struct {
	workspaceRoot string
	grabBin       string
	grabmanBin    string
	redactor      *security.SecretRedactor
}

// TreeEntry represents a single workspace file or directory (ADR-025c base-10 size string).
type TreeEntry struct {
	Path       string `json:"path"`
	Name       string `json:"name"`
	IsDir      bool   `json:"is_dir"`
	SizeBytes  string `json:"size_bytes"`
	ModifiedAt string `json:"modified_at"`
}

// FileContent represents a safely read confined file.
type FileContent struct {
	Path      string `json:"path"`
	Content   string `json:"content"`
	SHA256    string `json:"sha256"`
	SizeBytes string `json:"size_bytes"`
}

// CheckResult reports `grab check` and `grab format` parity results.
type CheckResult struct {
	OK          bool   `json:"ok"`
	Code        string `json:"code,omitempty"`
	Path        string `json:"path"`
	Template    string `json:"template,omitempty"`
	GrabCheckOK bool   `json:"grab_check_ok"`
	FormatClean bool   `json:"format_clean"`
	Diagnostics string `json:"diagnostics"`
	Formatted   string `json:"formatted_preview,omitempty"`
}

// GrabmanResult wraps structured JSON output from `grabman` CLI commands.
type GrabmanResult struct {
	OK           bool           `json:"ok"`
	Command      string         `json:"command"`
	TargetDir    string         `json:"target_dir"`
	ExitCode     string         `json:"exit_code"`
	PlainSummary string         `json:"plain_summary"`
	Report       map[string]any `json:"report"`
	RawOutput    string         `json:"raw_output,omitempty"`
}

// NewManager creates a workspace manager bound to workspaceRoot.
func NewManager(workspaceRoot, grabBin, grabmanBin string, redactor *security.SecretRedactor) (*Manager, error) {
	absRoot, err := filepath.Abs(strings.TrimSpace(workspaceRoot))
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(absRoot, 0o755); err != nil {
		return nil, err
	}
	realRoot, err := filepath.EvalSymlinks(absRoot)
	if err != nil {
		return nil, err
	}
	return &Manager{
		workspaceRoot: realRoot,
		grabBin:       strings.TrimSpace(grabBin),
		grabmanBin:    strings.TrimSpace(grabmanBin),
		redactor:      redactor,
	}, nil
}

// Root returns the canonical confined workspace root directory.
func (m *Manager) Root() string {
	return m.workspaceRoot
}

// GrabBinary returns the resolved GRAB CLI path.
func (m *Manager) GrabBinary() string {
	return m.grabBin
}

// GrabmanBinary returns the resolved grabman CLI path.
func (m *Manager) GrabmanBinary() string {
	return m.grabmanBin
}

// ResolveGrabBinary locates GRAB v1.1.0 executable via explicit flag, env, PATH, side-by-side bundle, or standard paths.
func ResolveGrabBinary(explicit string) string {
	return resolveToolBinary(explicit, "GROXY_DESK_GRAB_BIN", "GRAB_BIN", []string{"grab", "grab.exe"}, append(bundledBinFallbacks("grab"),
		`C:\Development\GRAB\dist\grab-windows-amd64.exe`,
		`C:\Program Files\GRAB\grab.exe`,
		filepath.Join(userHomeDir(), "go", "bin", exeName("grab")),
	))
}

// ResolveGrabmanBinary locates grabman v1.0.0 executable via explicit flag, env, PATH, side-by-side bundle, or standard paths.
func ResolveGrabmanBinary(explicit string) string {
	return resolveToolBinary(explicit, "GROXY_DESK_GRABMAN_BIN", "GRABMAN_BIN", []string{"grabman", "grabman.exe"}, append(bundledBinFallbacks("grabman"),
		`C:\Development\grabman\dist\grabman-windows-amd64.exe`,
		`C:\Program Files\GRAB\grabman.exe`,
		filepath.Join(userHomeDir(), "go", "bin", exeName("grabman")),
	))
}

func bundledBinFallbacks(base string) []string {
	name := exeName(base)
	var out []string
	if selfExe, err := os.Executable(); err == nil && selfExe != "" {
		dir := filepath.Dir(selfExe)
		out = append(out,
			filepath.Join(dir, "bin", name),
			filepath.Join(dir, name),
		)
	}
	if localAppData := strings.TrimSpace(os.Getenv("LOCALAPPDATA")); localAppData != "" {
		out = append(out, filepath.Join(localAppData, "Groxy Desk", "bin", name))
	}
	return out
}

func resolveToolBinary(explicit, envPrimary, envSecondary string, names, fallbacks []string) string {
	candidates := []string{
		strings.TrimSpace(explicit),
		strings.TrimSpace(os.Getenv(envPrimary)),
		strings.TrimSpace(os.Getenv(envSecondary)),
	}
	for _, c := range candidates {
		if c != "" {
			if st, err := os.Stat(c); err == nil && !st.IsDir() {
				return c
			}
		}
	}
	for _, name := range names {
		if found, err := exec.LookPath(name); err == nil {
			return found
		}
	}
	for _, fb := range fallbacks {
		if fb != "" {
			if st, err := os.Stat(fb); err == nil && !st.IsDir() {
				return fb
			}
		}
	}
	return ""
}

func userHomeDir() string {
	h, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return h
}

func exeName(base string) string {
	if runtime.GOOS == "windows" {
		return base + ".exe"
	}
	return base
}

// Tree lists confined files and directories under relDir (default ".").
func (m *Manager) Tree(relDir string) ([]TreeEntry, error) {
	dirAbs, _, err := security.ConfineWorkspaceDir(m.workspaceRoot, relDir)
	if err != nil {
		return nil, err
	}
	st, err := os.Stat(dirAbs)
	if err != nil || !st.IsDir() {
		return nil, security.NewError(security.CodeDeskE101, 400, "target workspace directory not found", err)
	}

	var entries []TreeEntry
	err = filepath.WalkDir(dirAbs, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if p == dirAbs {
			return nil
		}
		name := d.Name()
		if security.IsSensitiveSegment(name) || name == "node_modules" || name == "dist" || name == "EBWebView" {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			if _, _, confErr := security.ConfineWorkspacePath(m.workspaceRoot, p); confErr != nil {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}
		rel, err := filepath.Rel(m.workspaceRoot, p)
		if err != nil {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		entries = append(entries, TreeEntry{
			Path:       filepath.ToSlash(rel),
			Name:       name,
			IsDir:      d.IsDir(),
			SizeBytes:  strconv.FormatInt(info.Size(), 10),
			ModifiedAt: info.ModTime().UTC().Format(time.RFC3339),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir && !entries[j].IsDir
		}
		return entries[i].Path < entries[j].Path
	})
	return entries, nil
}

// ReadFile safely reads a confined workspace file up to MaxReadBytes (10 MiB).
func (m *Manager) ReadFile(relPath string) (FileContent, error) {
	absPath, cleanSlash, err := security.ConfineWorkspacePath(m.workspaceRoot, relPath)
	if err != nil {
		return FileContent{}, err
	}
	info, err := os.Stat(absPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return FileContent{}, security.NewError(security.CodeDeskE101, 404, fmt.Sprintf("file %q not found", cleanSlash), err)
		}
		return FileContent{}, security.NewError(security.CodeDeskE101, 400, fmt.Sprintf("cannot inspect %q", cleanSlash), err)
	}
	if !info.Mode().IsRegular() {
		return FileContent{}, security.NewError(security.CodeDeskE101, 400, fmt.Sprintf("%q is not a regular file", cleanSlash), nil)
	}
	if info.Size() > MaxReadBytes {
		return FileContent{}, security.NewError(security.CodeDeskE101, 400, fmt.Sprintf("file %q exceeds 10 MiB read limit", cleanSlash), nil)
	}
	raw, err := os.ReadFile(absPath)
	if err != nil {
		return FileContent{}, security.NewError(security.CodeDeskE101, 400, fmt.Sprintf("failed reading %q", cleanSlash), err)
	}
	return FileContent{
		Path:      cleanSlash,
		Content:   string(raw),
		SHA256:    security.SHA256Hex(raw),
		SizeBytes: strconv.FormatInt(int64(len(raw)), 10),
	}, nil
}

// CheckFiles runs `grab check` and `grab format` parity verification via structured argv slices.
func (m *Manager) CheckFiles(ctx context.Context, grabRel, tmplRel string) (CheckResult, error) {
	if m.grabBin == "" {
		return CheckResult{}, security.NewError(security.CodeDeskE301, 422, "GRAB v1.1.0 binary (grab.exe) not found", nil)
	}
	absGrab, cleanGrab, err := security.ConfineWorkspacePath(m.workspaceRoot, grabRel)
	if err != nil {
		return CheckResult{}, err
	}
	if !strings.HasSuffix(strings.ToLower(absGrab), ".grab") {
		return CheckResult{}, security.NewError(security.CodeDeskE101, 400, "target file for grab check must end with .grab", nil)
	}

	var absTmpl, cleanTmpl string
	if strings.TrimSpace(tmplRel) != "" {
		absTmpl, cleanTmpl, err = security.ConfineWorkspacePath(m.workspaceRoot, tmplRel)
		if err != nil {
			return CheckResult{}, err
		}
		if !strings.HasSuffix(strings.ToLower(absTmpl), ".grab.html") {
			return CheckResult{}, security.NewError(security.CodeDeskE101, 400, "template file must end with .grab.html", nil)
		}
	}

	projectDir := findProjectDirForFile(m.workspaceRoot, filepath.Dir(absGrab))
	checkArgs := []string{"check"}
	if _, err := os.Stat(filepath.Join(projectDir, "grab.platform.json")); err == nil {
		checkArgs = append(checkArgs, "--platform-config", "grab.platform.json")
	}
	if _, err := os.Stat(filepath.Join(projectDir, "grab.data.json")); err == nil {
		checkArgs = append(checkArgs, "--db-config", "grab.data.json")
	}

	relGrabFromProj, err := filepath.Rel(projectDir, absGrab)
	if err != nil {
		relGrabFromProj = absGrab
	}
	checkArgs = append(checkArgs, relGrabFromProj)
	if absTmpl != "" {
		relTmplFromProj, err := filepath.Rel(projectDir, absTmpl)
		if err != nil {
			relTmplFromProj = absTmpl
		}
		checkArgs = append(checkArgs, relTmplFromProj)
	}

	if err := security.ValidateSafeCommandBinary(m.grabBin, checkArgs); err != nil {
		return CheckResult{}, err
	}

	runCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	cmd := deskexec.CommandContext(runCtx, m.grabBin, checkArgs...)
	cmd.Dir = projectDir
	var checkOut bytes.Buffer
	cmd.Stdout = &checkOut
	cmd.Stderr = &checkOut
	checkErr := cmd.Run()

	res := CheckResult{
		Path:        cleanGrab,
		Template:    cleanTmpl,
		GrabCheckOK: checkErr == nil,
		Diagnostics: strings.TrimSpace(m.redact(checkOut.String())),
	}

	// Verify canonical `grab format` byte-for-byte parity on the .grab file.
	fmtArgs := []string{"format", relGrabFromProj}
	if err := security.ValidateSafeCommandBinary(m.grabBin, fmtArgs); err != nil {
		return CheckResult{}, err
	}
	fmtCmd := deskexec.CommandContext(runCtx, m.grabBin, fmtArgs...)
	fmtCmd.Dir = projectDir
	var fmtOut, fmtErrBuf bytes.Buffer
	fmtCmd.Stdout = &fmtOut
	fmtCmd.Stderr = &fmtErrBuf
	if err := fmtCmd.Run(); err == nil {
		origBytes, readErr := os.ReadFile(absGrab)
		if readErr == nil {
			normOrig := strings.ReplaceAll(string(origBytes), "\r\n", "\n")
			normFmt := strings.ReplaceAll(fmtOut.String(), "\r\n", "\n")
			res.FormatClean = normOrig == normFmt
			res.Formatted = fmtOut.String()
		}
	} else if res.Diagnostics == "" {
		res.Diagnostics = strings.TrimSpace(m.redact(fmtErrBuf.String()))
	}

	res.OK = res.GrabCheckOK && res.FormatClean
	if !res.GrabCheckOK {
		res.Code = security.CodeDeskE301
		if res.Diagnostics == "" {
			res.Diagnostics = "grab check failed"
		}
	} else if res.Diagnostics == "" {
		if res.FormatClean {
			res.Diagnostics = "grab check and canonical grab format passed cleanly."
		} else {
			res.Diagnostics = "grab check passed; canonical formatting adjustments available."
		}
	}
	return res, nil
}

// GrabmanInit runs `grabman init` inside a confined subdirectory via structured argv.
func (m *Manager) GrabmanInit(ctx context.Context, targetSubdir, profile, projectName, edgeProxy string, force bool) (GrabmanResult, error) {
	if m.grabmanBin == "" {
		return GrabmanResult{}, security.NewError(security.CodeDeskE301, 422, "grabman v1.0.0 binary not found", nil)
	}
	absDir, relSlash, err := security.ConfineWorkspaceDir(m.workspaceRoot, targetSubdir)
	if err != nil {
		return GrabmanResult{}, err
	}

	cleanProfile := strings.TrimSpace(profile)
	if cleanProfile == "" {
		cleanProfile = "app"
	}
	switch cleanProfile {
	case "app", "cms-stack", "ai-stack":
	default:
		return GrabmanResult{}, security.NewError(security.CodeDeskE101, 400, fmt.Sprintf("unsupported grabman profile %q (expected app, cms-stack, or ai-stack)", cleanProfile), nil)
	}

	cleanEdge := strings.TrimSpace(edgeProxy)
	if cleanEdge == "" {
		cleanEdge = "caddy"
	}
	switch cleanEdge {
	case "caddy", "nginx", "cloudflare-tunnel":
	default:
		return GrabmanResult{}, security.NewError(security.CodeDeskE101, 400, fmt.Sprintf("unsupported edge_proxy %q", cleanEdge), nil)
	}

	cleanName := strings.TrimSpace(projectName)
	if cleanName == "" {
		cleanName = filepath.Base(absDir)
	}
	if len(cleanName) > 120 || strings.ContainsAny(cleanName, "\x00\r\n") {
		return GrabmanResult{}, security.NewError(security.CodeDeskE101, 400, "invalid project name", nil)
	}

	args := []string{"init", "--dir", absDir, "--profile", cleanProfile, "--name", cleanName, "--edge-proxy", cleanEdge, "--json"}
	if force {
		args = append(args, "--force")
	}
	return m.runGrabman(ctx, "init", relSlash, args)
}

// GrabmanDoctor runs read-only `grabman doctor` on a confined workspace directory.
func (m *Manager) GrabmanDoctor(ctx context.Context, targetSubdir string, strict bool) (GrabmanResult, error) {
	if m.grabmanBin == "" {
		return GrabmanResult{}, security.NewError(security.CodeDeskE301, 422, "grabman v1.0.0 binary not found", nil)
	}
	absDir, relSlash, err := security.ConfineWorkspaceDir(m.workspaceRoot, targetSubdir)
	if err != nil {
		return GrabmanResult{}, err
	}
	args := []string{"doctor", "--dir", absDir, "--json"}
	if m.grabBin != "" {
		args = append(args, "--grab-bin", m.grabBin)
	}
	if strict {
		args = append(args, "--strict")
	}
	return m.runGrabman(ctx, "doctor", relSlash, args)
}

// GrabmanSolveVerify runs read-only `grabman solve --verify` on a confined workspace directory.
func (m *Manager) GrabmanSolveVerify(ctx context.Context, targetSubdir string) (GrabmanResult, error) {
	if m.grabmanBin == "" {
		return GrabmanResult{}, security.NewError(security.CodeDeskE301, 422, "grabman v1.0.0 binary not found", nil)
	}
	absDir, relSlash, err := security.ConfineWorkspaceDir(m.workspaceRoot, targetSubdir)
	if err != nil {
		return GrabmanResult{}, err
	}
	args := []string{"solve", "--verify", "--dir", absDir, "--json"}
	return m.runGrabman(ctx, "solve --verify", relSlash, args)
}

func (m *Manager) runGrabman(ctx context.Context, command, relSlash string, args []string) (GrabmanResult, error) {
	if err := security.ValidateSafeCommandBinary(m.grabmanBin, args); err != nil {
		return GrabmanResult{}, err
	}
	runCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	cmd := deskexec.CommandContext(runCtx, m.grabmanBin, args...)
	cmd.Dir = m.workspaceRoot
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	exitCode := 0
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			exitCode = ee.ExitCode()
		} else {
			return GrabmanResult{}, security.NewError(security.CodeDeskE301, 422, fmt.Sprintf("failed executing grabman %s", command), err)
		}
	}

	rawOut := strings.TrimSpace(stdout.String())
	if rawOut == "" {
		rawOut = strings.TrimSpace(stderr.String())
	}
	rawOut = m.redact(rawOut)

	var report map[string]any
	if err := json.Unmarshal([]byte(stdout.String()), &report); err != nil {
		report = map[string]any{}
	}

	summary := buildGrabmanSummary(command, relSlash, exitCode == 0, report, rawOut)
	res := GrabmanResult{
		OK:           exitCode == 0,
		Command:      command,
		TargetDir:    relSlash,
		ExitCode:     strconv.Itoa(exitCode),
		PlainSummary: summary,
		Report:       report,
		RawOutput:    rawOut,
	}
	if exitCode != 0 && command == "init" {
		return res, security.NewError(security.CodeDeskE301, 422, summary, nil)
	}
	return res, nil
}

func buildGrabmanSummary(command, relDir string, ok bool, report map[string]any, raw string) string {
	switch command {
	case "init":
		if ok {
			return fmt.Sprintf("Scaffolded GRAB workspace at %q cleanly with grabman.lock.", relDir)
		}
		if eMsg, has := report["error"].(string); has && eMsg != "" {
			return eMsg
		}
		return fmt.Sprintf("grabman init failed for %q: %s", relDir, raw)
	case "doctor":
		if status, has := report["status"].(string); has && status != "" {
			return fmt.Sprintf("grabman doctor status for %q: %s.", relDir, status)
		}
		if ok {
			return fmt.Sprintf("grabman doctor passed cleanly for %q.", relDir)
		}
		return fmt.Sprintf("grabman doctor reported issues for %q.", relDir)
	case "solve --verify":
		if ok {
			return fmt.Sprintf("grabman.lock verified across all ecosystem layers for %q.", relDir)
		}
		if eMsg, has := report["error"].(string); has && eMsg != "" {
			return eMsg
		}
		return fmt.Sprintf("grabman.lock verification failed for %q.", relDir)
	default:
		return raw
	}
}

func findProjectDirForFile(workspaceRoot, fileDir string) string {
	curr := fileDir
	for {
		if _, err := os.Stat(filepath.Join(curr, "grabman.json")); err == nil {
			return curr
		}
		if _, err := os.Stat(filepath.Join(curr, "grab.platform.json")); err == nil {
			return curr
		}
		if curr == workspaceRoot || filepath.Dir(curr) == curr {
			return workspaceRoot
		}
		parent := filepath.Dir(curr)
		if rel, err := filepath.Rel(workspaceRoot, parent); err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return workspaceRoot
		}
		curr = parent
	}
}

func (m *Manager) redact(s string) string {
	if m.redactor != nil {
		return m.redactor.Redact(s)
	}
	return s
}
