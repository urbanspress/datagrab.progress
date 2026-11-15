package author

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"groxy-desk/internal/auditor"
	"groxy-desk/internal/groxybridge"
	"groxy-desk/internal/security"
)

const (
	DefaultOllamaURL = "http://127.0.0.1:11434"
	StatusEnforced   = "draft"
)

var (
	reISBN13 = regexp.MustCompile(`^[0-9]{10,13}$`)
)

// InitPlanRequest configures 1-click Local Author Mode workspace scaffolding + JSON-LD preview.
type InitPlanRequest struct {
	TargetSubdir string `json:"target_subdir"`
	BookTitle    string `json:"book_title"`
	AuthorName   string `json:"author_name"`
	ISBN13       string `json:"isbn_13,omitempty"`
	PriceUSD     string `json:"price_usd,omitempty"`
	WordCount    string `json:"word_count,omitempty"`
	Genre        string `json:"genre,omitempty"`
	ChapterTitle string `json:"chapter_title,omitempty"`
}

// InitApplyRequest executes an approved Author Mode scaffold inside workspace confinement.
type InitApplyRequest struct {
	TargetSubdir  string `json:"target_subdir"`
	BookTitle     string `json:"book_title"`
	AuthorName    string `json:"author_name"`
	ISBN13        string `json:"isbn_13,omitempty"`
	PriceUSD      string `json:"price_usd,omitempty"`
	WordCount     string `json:"word_count,omitempty"`
	Genre         string `json:"genre,omitempty"`
	ChapterTitle  string `json:"chapter_title,omitempty"`
	ApprovalToken string `json:"approval_token"`
}

// OllamaStartRequest configures local loopback-only open-weights runtime launch.
type OllamaStartRequest struct {
	OllamaBin string `json:"ollama_bin,omitempty"`
	URL       string `json:"url,omitempty"`
}

// Manager implements 1-click Local Author Mode scaffolding and zero-fee loopback Ollama control.
type Manager struct {
	workspaceRoot string
	ollamaURL     string
	approvals     *groxybridge.ApprovalManager
	redactor      *security.SecretRedactor
	httpClient    *http.Client
	mu            sync.Mutex
	ollamaCmd     *exec.Cmd
}

// NewManager constructs an Author Mode manager scoped to workspaceRoot.
func NewManager(workspaceRoot, ollamaURL string, approvals *groxybridge.ApprovalManager, redactor *security.SecretRedactor) (*Manager, error) {
	cleanURL := strings.TrimSpace(ollamaURL)
	if cleanURL == "" {
		cleanURL = DefaultOllamaURL
	}
	if err := validateLoopbackURL(cleanURL); err != nil {
		return nil, err
	}
	if redactor == nil {
		redactor = security.NewSecretRedactor()
	}
	return &Manager{
		workspaceRoot: workspaceRoot,
		ollamaURL:     strings.TrimRight(cleanURL, "/"),
		approvals:     approvals,
		redactor:      redactor,
		httpClient:    &http.Client{Timeout: 3 * time.Second},
	}, nil
}

// PlanInit validates confined directory + ADR-025c book metadata, builds preview files + schema.org/Book JSON-LD,
// captures a Pre-Author Snapshot (snap_author_*), and issues a target-bound single-use approval_token.
func (m *Manager) PlanInit(req InitPlanRequest) (map[string]any, error) {
	subdir := firstNonEmpty(strings.TrimSpace(req.TargetSubdir), "author-studio")
	absDir, relDir, err := security.ConfineWorkspaceDir(m.workspaceRoot, subdir)
	if err != nil {
		return nil, err
	}
	spec, files, jsonLD, err := buildAuthorArtifacts(req, relDir)
	if err != nil {
		return nil, err
	}

	targetKey := "author:" + relDir
	existingState := readExistingAuthorSummary(absDir)
	snap, err := m.approvals.CreatePrefixedVirtualSnapshot("snap_author_", targetKey, existingState)
	if err != nil {
		return nil, err
	}
	appr, err := m.approvals.IssueSnapshotBoundToken(groxybridge.ActionAuthorInit, targetKey, snap.SnapshotID)
	if err != nil {
		return nil, err
	}

	fileList := make([]map[string]string, 0, len(files))
	for rel, body := range files {
		fileList = append(fileList, map[string]string{
			"path":       filepath.ToSlash(filepath.Join(relDir, rel)),
			"byte_count": strconv.Itoa(len([]byte(body))),
		})
	}

	return map[string]any{
		"ok":                      true,
		"target_subdir":           relDir,
		"target_key":              targetKey,
		"status_enforced":         StatusEnforced,
		"schemas":                 []string{"Book", "Chapter", "CharacterBible", "WorldLore"},
		"spec":                    spec,
		"schema_org_jsonld":       jsonLD.JSONText,
		"planned_files":           fileList,
		"preview_landing_html":    files["views/author-landing.grab.html"],
		"snapshot":                snap,
		"approval_token":          appr.Token,
		"model_switching_allowed": false,
		"plain_summary":           fmt.Sprintf("Prepared Local Author Mode for %q (%d draft-enforced files, schema.org/Book JSON-LD, zero wrapper <div> elements).", spec["book_title"], len(files)),
	}, nil
}

// ApplyInit consumes a single-use approval_token and writes the Author Mode schemas, draft manuscript,
// SITE-CONTEXT.md, and semantic landing page inside the confined workspace directory.
func (m *Manager) ApplyInit(req InitApplyRequest) (map[string]any, error) {
	subdir := firstNonEmpty(strings.TrimSpace(req.TargetSubdir), "author-studio")
	absDir, relDir, err := security.ConfineWorkspaceDir(m.workspaceRoot, subdir)
	if err != nil {
		return nil, err
	}
	targetKey := "author:" + relDir
	appr, err := m.approvals.ConsumeToken(req.ApprovalToken, groxybridge.ActionAuthorInit, targetKey)
	if err != nil {
		return nil, err
	}

	planReq := InitPlanRequest{
		TargetSubdir: relDir,
		BookTitle:    req.BookTitle,
		AuthorName:   req.AuthorName,
		ISBN13:       req.ISBN13,
		PriceUSD:     req.PriceUSD,
		WordCount:    req.WordCount,
		Genre:        req.Genre,
		ChapterTitle: req.ChapterTitle,
	}
	spec, files, jsonLD, err := buildAuthorArtifacts(planReq, relDir)
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(absDir, 0o755); err != nil {
		return nil, security.NewError(security.CodeDeskE101, 500, "failed creating Author Mode directory", err)
	}
	written := make([]string, 0, len(files))
	for rel, content := range files {
		relFromRoot := filepath.ToSlash(filepath.Join(relDir, rel))
		absFile, confRel, confErr := security.ConfineWorkspacePath(m.workspaceRoot, relFromRoot)
		if confErr != nil {
			return nil, confErr
		}
		if err := os.MkdirAll(filepath.Dir(absFile), 0o755); err != nil {
			return nil, security.NewError(security.CodeDeskE101, 500, "failed creating Author Mode subdirectory", err)
		}
		if err := os.WriteFile(absFile, []byte(content), 0o644); err != nil {
			return nil, security.NewError(security.CodeDeskE101, 500, "failed writing Author Mode file", err)
		}
		written = append(written, confRel)
	}

	return map[string]any{
		"ok":                      true,
		"target_subdir":           relDir,
		"snapshot_id":             appr.SnapshotID,
		"status_enforced":         StatusEnforced,
		"schemas":                 []string{"Book", "Chapter", "CharacterBible", "WorldLore"},
		"spec":                    spec,
		"schema_org_jsonld":       jsonLD.JSONText,
		"written_files":           written,
		"model_switching_allowed": false,
	}, nil
}

// OllamaStatus probes local loopback Ollama /api/tags (read-only status check; no model dropdowns).
func (m *Manager) OllamaStatus(ctx context.Context, overrideURL string) (map[string]any, error) {
	probeBase := firstNonEmpty(strings.TrimSpace(overrideURL), m.ollamaURL)
	if err := validateLoopbackURL(probeBase); err != nil {
		return nil, err
	}
	probeBase = strings.TrimRight(probeBase, "/")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, probeBase+"/api/tags", nil)
	if err != nil {
		return nil, security.NewError(security.CodeDeskE501, 422, "invalid loopback Ollama URL", err)
	}
	resp, err := m.httpClient.Do(req)
	if err != nil {
		return map[string]any{
			"ok":                      true,
			"running":                 false,
			"loopback_url":            probeBase,
			"detected_count":          "0",
			"zero_cloud_fee":          true,
			"model_switching_allowed": false,
			"plain_summary":           "Local Ollama runtime is currently stopped; click Launch Local AI Runtime to run open-weights drafts offline at $0 API cost.",
		}, nil
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	var tags struct {
		Models []map[string]any `json:"models"`
	}
	_ = json.Unmarshal(raw, &tags)
	return map[string]any{
		"ok":                      true,
		"running":                 resp.StatusCode == http.StatusOK,
		"loopback_url":            probeBase,
		"detected_count":          strconv.Itoa(len(tags.Models)),
		"zero_cloud_fee":          true,
		"model_switching_allowed": false,
		"plain_summary":           fmt.Sprintf("Local Ollama runtime is ready on %s with %d open-weights model(s) loaded at $0 API fee.", probeBase, len(tags.Models)),
	}, nil
}

// StartOllama launches a local loopback `ollama serve` daemon using structured argv (ADR-017).
func (m *Manager) StartOllama(ctx context.Context, req OllamaStartRequest) (map[string]any, error) {
	targetURL := firstNonEmpty(strings.TrimSpace(req.URL), m.ollamaURL)
	if err := validateLoopbackURL(targetURL); err != nil {
		return nil, err
	}
	status, _ := m.OllamaStatus(ctx, targetURL)
	if running, _ := status["running"].(bool); running {
		status["already_running"] = true
		return status, nil
	}

	bin := resolveOllamaBinary(req.OllamaBin)
	if bin == "" {
		return nil, security.NewError(security.CodeDeskE301, 422, "ollama binary not found in PATH; install Ollama for optional local open-weights drafting (DESK-E301)", nil)
	}
	args := []string{"serve"}
	if err := security.ValidateSafeCommandBinary(bin, args); err != nil {
		return nil, err
	}

	u, _ := url.Parse(targetURL)
	cmd := exec.Command(bin, args...)
	cmd.Env = append(os.Environ(), "OLLAMA_HOST="+u.Host)
	if err := cmd.Start(); err != nil {
		return nil, security.NewError(security.CodeDeskE503, 503, "failed starting local Ollama runtime", err)
	}

	m.mu.Lock()
	m.ollamaCmd = cmd
	m.mu.Unlock()

	return map[string]any{
		"ok":                      true,
		"started":                 true,
		"pid":                     strconv.Itoa(cmd.Process.Pid),
		"loopback_url":            targetURL,
		"zero_cloud_fee":          true,
		"model_switching_allowed": false,
		"plain_summary":           fmt.Sprintf("Started local loopback Ollama runtime (PID %d on %s) via structured argv.", cmd.Process.Pid, u.Host),
	}, nil
}

// Shutdown terminates any local loopback Ollama subprocess started by this Manager.
func (m *Manager) Shutdown() {
	m.mu.Lock()
	cmd := m.ollamaCmd
	m.ollamaCmd = nil
	m.mu.Unlock()
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}
}

func buildAuthorArtifacts(req InitPlanRequest, relDir string) (map[string]string, map[string]string, auditor.SchemaOrgPreview, error) {
	bookTitle := firstNonEmpty(strings.TrimSpace(req.BookTitle), "Untitled Novel")
	authorName := firstNonEmpty(strings.TrimSpace(req.AuthorName), "Independent Author")
	isbn := firstNonEmpty(strings.TrimSpace(req.ISBN13), "9780143127550")
	cleanISBN := strings.ReplaceAll(isbn, "-", "")
	if !reISBN13.MatchString(cleanISBN) {
		return nil, nil, auditor.SchemaOrgPreview{}, security.NewError(security.CodeDeskE101, 400, "isbn_13 must contain 10 or 13 digits", nil)
	}
	price := firstNonEmpty(strings.TrimSpace(req.PriceUSD), "19.99")
	if !security.IsExactDecimalString(price) {
		return nil, nil, auditor.SchemaOrgPreview{}, security.NewError(security.CodeDeskE101, 400, "price_usd must be an exact ADR-025c base-10 decimal string (e.g. 19.99)", nil)
	}
	wordCount := firstNonEmpty(strings.TrimSpace(req.WordCount), "1450")
	if !security.IsExactIntString(wordCount) {
		return nil, nil, auditor.SchemaOrgPreview{}, security.NewError(security.CodeDeskE101, 400, "word_count must be an exact ADR-025c base-10 integer string", nil)
	}
	genre := firstNonEmpty(strings.TrimSpace(req.Genre), "Speculative Fiction")
	chapterTitle := firstNonEmpty(strings.TrimSpace(req.ChapterTitle), "Chapter 1 — Opening Scene")

	spec := map[string]string{
		"book_title":      bookTitle,
		"author_name":     authorName,
		"isbn_13":         isbn,
		"price_usd":       price,
		"word_count":      wordCount,
		"genre":           genre,
		"chapter_title":   chapterTitle,
		"status_enforced": StatusEnforced,
		"target_subdir":   relDir,
	}

	jsonLD := auditor.BuildSchemaOrgPreview("book", bookTitle, genre+" novel by "+authorName, "/books/opening", map[string]string{
		"author":     authorName,
		"isbn_13":    isbn,
		"price_usd":  price,
		"currency":   "USD",
		"word_count": wordCount,
		"genre":      genre,
	}, genre)

	schemasJSON, _ := json.MarshalIndent(map[string]any{
		"mode":            "author",
		"status_enforced": StatusEnforced,
		"schemas": map[string]any{
			"Book": map[string]any{
				"fields": []string{"title", "author", "isbn_13", "price_usd", "genre", "status"},
			},
			"Chapter": map[string]any{
				"fields": []string{"chapter_number", "title", "body_markdown", "word_count", "status"},
			},
			"CharacterBible": map[string]any{
				"fields": []string{"character_name", "role", "arc_notes", "voice_tags"},
			},
			"WorldLore": map[string]any{
				"fields": []string{"entry_title", "era", "continuity_rules", "canonical_facts"},
			},
		},
	}, "", "  ")

	chapterJSON, _ := json.MarshalIndent(map[string]any{
		"content_type":   "Chapter",
		"book_title":     bookTitle,
		"chapter_number": "1",
		"title":          chapterTitle,
		"word_count":     wordCount,
		"status":         StatusEnforced,
		"continuity":     []string{"CharacterBible", "WorldLore"},
	}, "", "  ")

	siteContext := fmt.Sprintf("# SITE-CONTEXT.md — %s\n\n- **Author:** %s\n- **Genre:** %s\n- **ISBN-13 (`ADR-025c`):** `%s`\n- **Price USD (`ADR-025c`):** `%s`\n- **CMS Status Rule:** `status_enforced: %q`\n- **Linked Collections:** `Book` -> `Chapter` <-> `CharacterBible` <-> `WorldLore`\n- **UI Invariant:** Pure `Grex JS v1.0.0` semantic markup (`divCount === 0`)\n",
		bookTitle, authorName, genre, isbn, price, StatusEnforced)

	landingHTML := fmt.Sprintf(`<main aria-label="Author manuscript home">
  <header>
    <h1>%s</h1>
    <p>By %s — %s</p>
  </header>
  <section aria-label="Featured chapter draft">
    <h2>%s</h2>
    <p>Draft preview ready under status_enforced="%s" (ISBN %s, List USD %s, %s words).</p>
    <ul>
      <li>Character Bible linked via SITE-CONTEXT.md</li>
      <li>World Lore continuity verified</li>
    </ul>
  </section>
</main>
`, htmlEscape(bookTitle), htmlEscape(authorName), htmlEscape(genre), htmlEscape(chapterTitle), StatusEnforced, htmlEscape(isbn), htmlEscape(price), htmlEscape(wordCount))

	files := map[string]string{
		"author-schemas.json":           string(schemasJSON) + "\n",
		"SITE-CONTEXT.md":               siteContext,
		"content/drafts/chapter-01.json": string(chapterJSON) + "\n",
		"views/author-landing.grab.html": landingHTML,
		"book-jsonld.json":              jsonLD.JSONText + "\n",
	}
	return spec, files, jsonLD, nil
}

func htmlEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&#39;")
	return r.Replace(s)
}

func validateLoopbackURL(raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return security.NewError(security.CodeDeskE501, 422, "Ollama URL must be a valid loopback URL (DESK-E501)", err)
	}
	if _, err := security.ValidateLoopbackBind(u.Host); err != nil {
		return err
	}
	return nil
}

func resolveOllamaBinary(explicit string) string {
	if s := strings.TrimSpace(explicit); s != "" {
		if _, err := os.Stat(s); err == nil {
			return s
		}
	}
	if envBin := strings.TrimSpace(os.Getenv("GROXY_DESK_OLLAMA_BIN")); envBin != "" {
		if _, err := os.Stat(envBin); err == nil {
			return envBin
		}
	}
	for _, cand := range []string{"ollama", "ollama.exe"} {
		if found, err := exec.LookPath(cand); err == nil {
			return found
		}
	}
	return ""
}

func readExistingAuthorSummary(absDir string) string {
	b, err := os.ReadFile(filepath.Join(absDir, "author-schemas.json"))
	if err != nil {
		return `{"existed":false}`
	}
	return string(b)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
