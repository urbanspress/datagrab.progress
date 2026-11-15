package groxybridge

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"groxy-desk/internal/security"
	"groxy-desk/internal/supervisor"
	"groxy-desk/internal/workspace"
)

const (
	// AgentID is the sole permanent assistant identity exposed by Groxy Desk.
	AgentID = "groxy-ai"
	// AgentDisplayName is the plain-English assistant identity displayed in the IDE.
	AgentDisplayName = "Groxy AI"

	// ReasoningFast executes deterministic daily tasks immediately.
	ReasoningFast = "fast"
	// ReasoningDeep executes multi-step planning, pre-mutation snapshotting, and gated apply.
	ReasoningDeep = "deep"
)

// RuleFinding represents one check in the deterministic 22-rule Groxy AI scorecard.
type RuleFinding struct {
	RuleID   string `json:"rule_id"`
	Pillar   string `json:"pillar"`
	Severity string `json:"severity"`
	Passed   bool   `json:"passed"`
	Message  string `json:"message"`
}

// AuditReport summarizes the 22-rule Groxy AI deterministic audit across SEO, Accessibility, DOM, and Links.
type AuditReport struct {
	TargetPath    string        `json:"target_path"`
	Score         string        `json:"score"`
	Grade         string        `json:"grade"`
	PublishSafe   bool          `json:"publish_safe"`
	PassedCount   string        `json:"passed_count"`
	TotalRules    string        `json:"total_rules"`
	DivCount      string        `json:"div_count"`
	Findings      []RuleFinding `json:"findings"`
	Summary       string        `json:"summary"`
}

// PlanStep describes one step in an Adaptive Reasoning Deep Plan.
type PlanStep struct {
	StepNumber  string `json:"step_number"`
	Title       string `json:"title"`
	Description string `json:"description"`
	SafetyGate  string `json:"safety_gate"`
}

// Classification describes how the Adaptive Reasoning Router routes a prompt.
type Classification struct {
	Mode             string `json:"mode"`
	Reason           string `json:"reason"`
	RequiresApproval bool   `json:"requires_approval"`
	SuggestedAction  string `json:"suggested_action"`
}

// AskRequest is the payload sent to POST /api/groxy/ask.
type AskRequest struct {
	Prompt          string `json:"prompt"`
	TargetPath      string `json:"target_path,omitempty"`
	ProposedContent string `json:"proposed_content,omitempty"`
}

// AskResponse is returned by POST /api/groxy/ask with Unified Groxy AI identity.
type AskResponse struct {
	Agent                 string                 `json:"agent"`
	AgentName             string                 `json:"agent_name"`
	ModelSwitchingAllowed bool                   `json:"model_switching_allowed"`
	ReasoningMode         string                 `json:"reasoning_mode"`
	RoutingReason         string                 `json:"routing_reason"`
	ActionExecuted        string                 `json:"action_executed"`
	Summary               string                 `json:"summary"`
	ApprovalRequired      bool                   `json:"approval_required"`
	ApprovalToken         string                 `json:"approval_token,omitempty"`
	ApprovalExpiresAt     string                 `json:"approval_expires_at,omitempty"`
	TargetPath            string                 `json:"target_path,omitempty"`
	ProposedContent       string                 `json:"proposed_content,omitempty"`
	ProposedSHA256        string                 `json:"proposed_sha256,omitempty"`
	Snapshot              *SnapshotRecord        `json:"snapshot,omitempty"`
	Plan                  []PlanStep             `json:"plan,omitempty"`
	Audit                 *AuditReport           `json:"audit,omitempty"`
	WorkspaceCheck        *workspace.CheckResult `json:"workspace_check,omitempty"`
	GrabmanDoctor         *workspace.GrabmanResult `json:"grabman_doctor,omitempty"`
	Workers               []supervisor.WorkerSnapshot `json:"workers,omitempty"`
}

// Bridge powers the unified Groxy AI Adaptive Reasoning Router and safety bridge.
type Bridge struct {
	workspaceMgr   *workspace.Manager
	supervisorMgr  *supervisor.Manager
	approvals      *ApprovalManager
	redactor       *security.SecretRedactor
	ollamaEndpoint string
	httpClient     *http.Client
}

// NewBridge constructs a Groxy AI host bridge backed by workspace, supervisor, and approval managers.
func NewBridge(ws *workspace.Manager, sup *supervisor.Manager, approvals *ApprovalManager, redactor *security.SecretRedactor, ollamaEndpoint string) (*Bridge, error) {
	if ws == nil {
		return nil, security.NewError(security.CodeDeskE101, 500, "workspace manager is required", nil)
	}
	if approvals == nil {
		return nil, security.NewError(security.CodeDeskE101, 500, "approval manager is required", nil)
	}
	if redactor == nil {
		redactor = security.NewSecretRedactor()
	}

	trimmedEndpoint := strings.TrimSpace(ollamaEndpoint)
	if trimmedEndpoint != "" {
		if err := validateLoopbackURL(trimmedEndpoint); err != nil {
			return nil, err
		}
	}

	return &Bridge{
		workspaceMgr:   ws,
		supervisorMgr:  sup,
		approvals:      approvals,
		redactor:       redactor,
		ollamaEndpoint: strings.TrimRight(trimmedEndpoint, "/"),
		httpClient: &http.Client{
			Timeout: 3 * time.Second,
		},
	}, nil
}

func validateLoopbackURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" {
		return security.NewError(security.CodeDeskE501, 500, "invalid local Ollama endpoint URL", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return security.NewError(security.CodeDeskE501, 500, "Ollama endpoint scheme must be http or https", nil)
	}
	if _, err := security.ValidateLoopbackBind(parsed.Host); err != nil {
		return err
	}
	return nil
}

// Approvals returns the underlying GROXY-SAFE-001 approval and snapshot manager.
func (b *Bridge) Approvals() *ApprovalManager {
	return b.approvals
}

// ClassifyPrompt routes deterministic instant actions to fast mode and multi-step mutations to deep mode.
func ClassifyPrompt(prompt string, hasProposed bool) Classification {
	lower := strings.ToLower(strings.TrimSpace(prompt))
	if hasProposed {
		return Classification{
			Mode:             ReasoningDeep,
			Reason:           "proposed file mutation requires GROXY-SAFE-001 plan, snapshot, and approval gate",
			RequiresApproval: true,
			SuggestedAction:  "plan_mutation",
		}
	}

	deepKeywords := []string{
		"fix", "remediate", "refactor", "patch", "edit", "write", "modify",
		"create", "generate", "build", "convert", "rewrite", "apply",
		"plan", "multi-step", "boost", "redesign", "update ", "add ", "remove ",
	}
	for _, kw := range deepKeywords {
		if strings.Contains(lower, kw) {
			return Classification{
				Mode:             ReasoningDeep,
				Reason:           "multi-step code or layout remediation requires deep reasoning with pre-mutation snapshot",
				RequiresApproval: true,
				SuggestedAction:  "plan_mutation",
			}
		}
	}

	return Classification{
		Mode:             ReasoningFast,
		Reason:           "instant daily inspection or deterministic verification task",
		RequiresApproval: false,
		SuggestedAction:  classifyFastAction(lower),
	}
}

func classifyFastAction(lower string) string {
	switch {
	case strings.Contains(lower, "design system") || strings.Contains(lower, "design-system") || strings.Contains(lower, "style pack"):
		return "studio_design_system"
	case strings.Contains(lower, "content type") || strings.Contains(lower, "media") || strings.Contains(lower, "menu") || strings.Contains(lower, "query"):
		return "cms_inspect"
	case strings.Contains(lower, "doctor") || strings.Contains(lower, "lockfile") || strings.Contains(lower, "grabman"):
		return "grabman_doctor"
	case strings.Contains(lower, "worker") || strings.Contains(lower, "grisp") || strings.Contains(lower, "health") || strings.Contains(lower, "ready"):
		return "grisp_status"
	case strings.Contains(lower, "grab check") || strings.Contains(lower, "grab format") || strings.Contains(lower, "format") || strings.Contains(lower, "route") || strings.Contains(lower, "schema"):
		return "workspace_check"
	default:
		return "audit_22_rules"
	}
}

// Ask routes an incoming IDE co-pilot prompt through Unified Groxy AI's Adaptive Reasoning Router.
func (b *Bridge) Ask(ctx context.Context, req AskRequest) (AskResponse, error) {
	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		return AskResponse{}, security.NewError(security.CodeDeskE101, 400, "prompt cannot be empty", nil)
	}

	var relTarget string
	if strings.TrimSpace(req.TargetPath) != "" {
		var err error
		_, relTarget, err = security.ConfineWorkspacePath(b.workspaceMgr.Root(), req.TargetPath)
		if err != nil {
			return AskResponse{}, err
		}
	}

	if b.ollamaEndpoint != "" {
		if err := b.checkOllamaReachable(ctx); err != nil {
			return AskResponse{}, err
		}
	}

	cls := ClassifyPrompt(prompt, req.ProposedContent != "")
	resp := AskResponse{
		Agent:                 AgentID,
		AgentName:             AgentDisplayName,
		ModelSwitchingAllowed: false,
		ReasoningMode:         cls.Mode,
		RoutingReason:         cls.Reason,
		TargetPath:            relTarget,
	}

	if cls.Mode == ReasoningFast {
		return b.executeFast(ctx, resp, cls.SuggestedAction, prompt, relTarget)
	}
	return b.executeDeep(ctx, resp, prompt, relTarget, req.ProposedContent)
}

func (b *Bridge) checkOllamaReachable(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, b.ollamaEndpoint+"/api/tags", nil)
	if err != nil {
		return security.NewError(security.CodeDeskE503, 503, "local Groxy AI engine request invalid", err)
	}
	resp, err := b.httpClient.Do(req)
	if err != nil {
		return security.NewError(security.CodeDeskE503, 503, "local Groxy AI Ollama endpoint is unreachable", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode >= 500 {
		return security.NewError(security.CodeDeskE503, 503, "local Groxy AI Ollama endpoint returned server error", nil)
	}
	return nil
}

func (b *Bridge) executeFast(ctx context.Context, resp AskResponse, action, prompt, relTarget string) (AskResponse, error) {
	resp.ActionExecuted = action

	switch action {
	case "grabman_doctor":
		docRes, err := b.workspaceMgr.GrabmanDoctor(ctx, ".", false)
		if err != nil {
			return AskResponse{}, err
		}
		resp.GrabmanDoctor = &docRes
		resp.Summary = b.redactor.Redact(fmt.Sprintf("Groxy AI ran grabman doctor in instant mode (passed=%t, exit=%s).", docRes.OK, docRes.ExitCode))
		return resp, nil

	case "grisp_status":
		var workers []supervisor.WorkerSnapshot
		if b.supervisorMgr != nil {
			b.supervisorMgr.RefreshAllHealth(ctx)
			workers = b.supervisorMgr.ListWorkers()
		}
		resp.Workers = workers
		resp.Summary = fmt.Sprintf("Groxy AI checked local Grisp worker status (%d active or registered workers).", len(workers))
		return resp, nil

	case "workspace_check":
		grabRel := relTarget
		tmplRel := ""
		if strings.HasSuffix(strings.ToLower(relTarget), ".html") {
			tmplRel = relTarget
			grabRel = ""
		} else if relTarget == "" {
			grabRel = b.findFirstFileWithExt(".grab")
			tmplRel = b.findFirstFileWithExt(".grab.html")
		}
		if grabRel != "" || tmplRel != "" {
			chk, err := b.workspaceMgr.CheckFiles(ctx, grabRel, tmplRel)
			if err != nil {
				return AskResponse{}, err
			}
			resp.WorkspaceCheck = &chk
			if chk.Template != "" {
				if fc, readErr := b.workspaceMgr.ReadFile(chk.Template); readErr == nil {
					audit := AuditHTML(fc.Path, fc.Content)
					resp.Audit = &audit
				}
			}
			resp.Summary = b.redactor.Redact(fmt.Sprintf("Groxy AI ran instant grab check & grab format parity verification (ok=%t).", chk.OK))
			return resp, nil
		}
	}

	// Default deterministic fast path: 22-rule audit if target file provided or first template found.
	auditTarget := relTarget
	if auditTarget == "" {
		auditTarget = b.findFirstFileWithExt(".grab.html")
	}
	if auditTarget != "" {
		fc, err := b.workspaceMgr.ReadFile(auditTarget)
		if err != nil {
			return AskResponse{}, err
		}
		audit := AuditHTML(fc.Path, fc.Content)
		resp.Audit = &audit
		resp.ActionExecuted = "audit_22_rules"
		resp.Summary = b.redactor.Redact(fmt.Sprintf("Groxy AI completed the 22-rule instant audit on %s: Score %s/100 (Grade %s, PublishSafe=%t, divCount=%s).", fc.Path, audit.Score, audit.Grade, audit.PublishSafe, audit.DivCount))
		return resp, nil
	}

	resp.Summary = b.redactor.Redact("Groxy AI fast inspection ready: select a .grab or .grab.html file, or ask to run grabman doctor or audit.")
	return resp, nil
}

func (b *Bridge) executeDeep(ctx context.Context, resp AskResponse, prompt, relTarget, explicitProposed string) (AskResponse, error) {
	resp.ActionExecuted = "deep_plan_snapshot"
	if relTarget == "" {
		relTarget = b.findFirstFileWithExt(".grab.html")
	}
	if relTarget == "" {
		relTarget = "views/index.grab.html"
	}
	resp.TargetPath = relTarget

	var currentContent string
	fc, readErr := b.workspaceMgr.ReadFile(relTarget)
	if readErr == nil {
		currentContent = fc.Content
	} else if security.ErrorCode(readErr, "") == security.CodeGrabE401 {
		return AskResponse{}, readErr
	}

	proposed := explicitProposed
	if strings.TrimSpace(proposed) == "" {
		proposed = synthesizeSafeRemediation(relTarget, currentContent, prompt)
	}

	appr, snap, _, err := b.approvals.IssueMutationToken(b.workspaceMgr.Root(), relTarget, proposed)
	if err != nil {
		return AskResponse{}, err
	}

	auditPreview := AuditHTML(relTarget, proposed)
	resp.ApprovalRequired = true
	resp.ApprovalToken = appr.Token
	resp.ApprovalExpiresAt = appr.ExpiresAt.Format(time.RFC3339)
	resp.Snapshot = &snap
	resp.ProposedContent = proposed
	resp.ProposedSHA256 = appr.ProposedSHA256
	resp.Audit = &auditPreview
	resp.Plan = []PlanStep{
		{
			StepNumber:  "1",
			Title:       "Inspect & Pre-Mutation SHA-256 Snapshot",
			Description: fmt.Sprintf("Captured snapshot %s for %s prior to editing.", snap.SnapshotID, relTarget),
			SafetyGate:  "GROXY-SAFE-001",
		},
		{
			StepNumber:  "2",
			Title:       "Validate Semantic Zero-<div> & 22-Rule Compliance",
			Description: fmt.Sprintf("Verified proposed target scores %s/100 (Grade %s, divCount=%s).", auditPreview.Score, auditPreview.Grade, auditPreview.DivCount),
			SafetyGate:  "Grex JS v1.0.0",
		},
		{
			StepNumber:  "3",
			Title:       "Single-Use Target-Bound Approval",
			Description: fmt.Sprintf("Approve mutation with token bound strictly to %s, or click Rollback Snapshot after applying.", relTarget),
			SafetyGate:  "DESK-E403 / DESK-E409",
		},
	}
	resp.Summary = b.redactor.Redact(fmt.Sprintf(
		"Groxy AI drafted a gated Deep Plan for %s. Pre-mutation snapshot %s captured; review proposed Score %s/100 (Grade %s) and click Approve & Apply.",
		relTarget, snap.SnapshotID, auditPreview.Score, auditPreview.Grade,
	))
	_ = ctx
	return resp, nil
}

func (b *Bridge) findFirstFileWithExt(ext string) string {
	entries, err := b.workspaceMgr.Tree(".")
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if !e.IsDir && strings.HasSuffix(strings.ToLower(e.Path), ext) {
			return e.Path
		}
	}
	return ""
}

func synthesizeSafeRemediation(targetPath, currentContent, prompt string) string {
	lowerTarget := strings.ToLower(targetPath)
	if strings.HasSuffix(lowerTarget, ".grab") {
		if strings.TrimSpace(currentContent) != "" {
			return currentContent
		}
		return "route \"GET\" \"/_health\" {\n  return json({\"status\": \"ok\"})\n}\n"
	}

	if strings.TrimSpace(currentContent) == "" {
		safeSummary := htmlEscapeText(prompt)
		if len(safeSummary) > 80 {
			safeSummary = safeSummary[:80]
		}
		return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>Groxy Desk Semantic Workspace Page</title>
  <meta name="description" content="Pure Grex JS v1.0.0 semantic page built with zero div containers and WCAG AA accessible landmarks." />
  <link rel="canonical" href="http://127.0.0.1:9490/" />
</head>
<body>
  <header>
    <h1>Groxy Desk Workspace View</h1>
  </header>
  <main>
    <section>
      <h2>Summary</h2>
      <p>%s</p>
    </section>
  </main>
  <footer>
    <p>Built with Pure Grex JS v1.0.0</p>
  </footer>
</body>
</html>
`, safeSummary)
	}

	repaired := divOpenRe.ReplaceAllString(currentContent, "<section$1>")
	repaired = divCloseRe.ReplaceAllString(repaired, "</section>")
	repaired = inlineEventRe.ReplaceAllString(repaired, "")
	repaired = jsHrefRe.ReplaceAllString(repaired, `$1="#"`)
	if !strings.Contains(strings.ToLower(repaired), "<main") {
		repaired = strings.Replace(repaired, "<body>", "<body>\n  <main>", 1)
		repaired = strings.Replace(repaired, "</body>", "  </main>\n</body>", 1)
	}
	return repaired
}

func htmlEscapeText(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")
	return r.Replace(strings.TrimSpace(s))
}

var (
	titleRe          = regexp.MustCompile(`(?is)<title\b[^>]*>(.*?)</title>`)
	metaDescRe       = regexp.MustCompile(`(?is)<meta\b[^>]*name=["']description["'][^>]*content=["']([^"']*)["'][^>]*>`)
	metaViewportRe   = regexp.MustCompile(`(?is)<meta\b[^>]*name=["']viewport["'][^>]*>`)
	h1Re             = regexp.MustCompile(`(?is)<h1\b[^>]*>.*?</h1>`)
	headingLevelRe   = regexp.MustCompile(`(?is)<h([1-6])\b`)
	htmlLangRe       = regexp.MustCompile(`(?is)<html\b[^>]*\blang=["'][^"']+["'][^>]*>`)
	canonicalRe      = regexp.MustCompile(`(?is)<link\b[^>]*rel=["']canonical["'][^>]*>`)
	ogTitleRe        = regexp.MustCompile(`(?is)<meta\b[^>]*property=["']og:title["'][^>]*>`)
	imgTagRe         = regexp.MustCompile(`(?is)<img\b([^>]*)>`)
	altAttrRe        = regexp.MustCompile(`(?is)\balt=["']([^"']+)["']`)
	formFieldRe      = regexp.MustCompile(`(?is)<(input|select|textarea)\b([^>]*)>`)
	labelOrAriaRe    = regexp.MustCompile(`(?is)\baria-label=["'][^"']+["']`)
	labelTagRe       = regexp.MustCompile(`(?is)<label\b[^>]*>`)
	interactiveRe    = regexp.MustCompile(`(?is)<(button|a)\b([^>]*)>(.*?)</(button|a)>`)
	tabindexPosRe    = regexp.MustCompile(`(?is)\btabindex=["']?([1-9][0-9]*)["']?`)
	divOpenRe        = regexp.MustCompile(`(?is)<div\b([^>]*)>`)
	divCloseRe       = regexp.MustCompile(`(?is)</div\s*>`)
	inlineScriptRe   = regexp.MustCompile(`(?is)<script\b([^>]*)>`)
	evalOrFuncRe     = regexp.MustCompile(`(?i)\b(eval|new\s+Function)\s*\(`)
	innerHTMLRe      = regexp.MustCompile(`(?i)\.(innerHTML|outerHTML)\s*=|insertAdjacentHTML\s*\(`)
	inlineEventRe    = regexp.MustCompile(`(?is)\s+on[a-z]{2,20}\s*=\s*("[^"]*"|'[^']*'|[^\s>]+)`)
	jsHrefRe         = regexp.MustCompile(`(?is)(href|src)\s*=\s*["']\s*(javascript|vbscript|data:text/html):[^"']*["']`)
	blankLinkRe      = regexp.MustCompile(`(?is)<a\b([^>]*target=["']_blank["'][^>]*)>`)
	emptyHrefRe      = regexp.MustCompile(`(?is)<a\b[^>]*\bhref=["']\s*(?:#|)\s*["'][^>]*>`)
	urlAttrRe        = regexp.MustCompile(`(?is)\b(?:href|src)=["']([^"']+)["']`)
	tagNameRe        = regexp.MustCompile(`(?is)<([a-z][a-z0-9-]*)\b`)
)

var allowedGrexAuditTags = map[string]bool{
	"html": true, "head": true, "meta": true, "title": true, "link": true, "body": true,
	"section": true, "article": true, "header": true, "footer": true, "main": true,
	"aside": true, "nav": true, "h1": true, "h2": true, "h3": true, "p": true,
	"span": true, "code": true, "strong": true, "a": true, "button": true,
	"form": true, "input": true, "textarea": true, "select": true, "option": true,
	"label": true, "table": true, "thead": true, "tbody": true, "tr": true,
	"th": true, "td": true, "caption": true, "details": true, "summary": true,
	"ul": true, "li": true, "img": true, "script": true,
}

// AuditHTML executes the deterministic 22-rule SEO, A11Y, DOM, and Link audit matching Groxy AI v1.0.0.
func AuditHTML(targetPath, html string) AuditReport {
	findings := make([]RuleFinding, 0, 22)
	add := func(id, pillar, severity string, passed bool, msg string) {
		findings = append(findings, RuleFinding{
			RuleID:   id,
			Pillar:   pillar,
			Severity: severity,
			Passed:   passed,
			Message:  msg,
		})
	}

	// SEO (8 rules)
	titleMatch := titleRe.FindStringSubmatch(html)
	titleText := ""
	if len(titleMatch) > 1 {
		titleText = strings.TrimSpace(titleMatch[1])
	}
	add("GROXY-SEO-001", "SEO", "warning", len(titleText) >= 10 && len(titleText) <= 70, "Page <title> present and between 10 and 70 characters")

	descMatch := metaDescRe.FindStringSubmatch(html)
	descText := ""
	if len(descMatch) > 1 {
		descText = strings.TrimSpace(descMatch[1])
	}
	add("GROXY-SEO-002", "SEO", "warning", len(descText) >= 50 && len(descText) <= 160, "Meta description present and between 50 and 160 characters")
	add("GROXY-SEO-003", "SEO", "warning", metaViewportRe.MatchString(html), "Responsive viewport meta tag present")

	h1Matches := h1Re.FindAllString(html, -1)
	add("GROXY-SEO-004", "SEO", "warning", len(h1Matches) == 1, "Single top-level <h1> heading present")
	add("GROXY-SEO-005", "SEO", "info", checkHeadingOrder(html), "Logical heading hierarchy without skipped levels")
	add("GROXY-SEO-006", "SEO", "warning", htmlLangRe.MatchString(html), "Root <html> includes lang attribute")
	add("GROXY-SEO-007", "SEO", "info", canonicalRe.MatchString(html), "Canonical <link rel=\"canonical\"> tag present")
	add("GROXY-SEO-008", "SEO", "info", ogTitleRe.MatchString(html) || strings.Contains(strings.ToLower(html), "<main"), "Open Graph og:title metadata or primary <main> content landmark present")

	// A11Y (5 rules)
	imgs := imgTagRe.FindAllStringSubmatch(html, -1)
	imgsValid := true
	for _, m := range imgs {
		if !altAttrRe.MatchString(m[1]) {
			imgsValid = false
			break
		}
	}
	add("GROXY-A11Y-001", "A11Y", "error", imgsValid, "Every <img> element specifies descriptive alt text")

	fields := formFieldRe.FindAllStringSubmatch(html, -1)
	fieldsValid := true
	if len(fields) > 0 && !labelTagRe.MatchString(html) {
		for _, f := range fields {
			if !strings.Contains(strings.ToLower(f[2]), `type="hidden"`) && !labelOrAriaRe.MatchString(f[2]) {
				fieldsValid = false
				break
			}
		}
	}
	add("GROXY-A11Y-002", "A11Y", "error", fieldsValid, "All form controls have associated <label> or aria-label")

	interactives := interactiveRe.FindAllStringSubmatch(html, -1)
	interactiveValid := true
	for _, item := range interactives {
		inner := strings.TrimSpace(regexp.MustCompile(`<[^>]+>`).ReplaceAllString(item[3], ""))
		if inner == "" && !labelOrAriaRe.MatchString(item[2]) {
			interactiveValid = false
			break
		}
	}
	add("GROXY-A11Y-003", "A11Y", "error", interactiveValid, "Every <button> and <a> control has accessible text or aria-label")
	lowerHTML := strings.ToLower(html)
	add("GROXY-A11Y-004", "A11Y", "warning", strings.Contains(lowerHTML, "<main") && (strings.Contains(lowerHTML, "<header") || strings.Contains(lowerHTML, "<nav") || strings.Contains(lowerHTML, "<footer")), "Semantic document landmarks (<main> + <header>/<nav>/<footer>) present")
	add("GROXY-A11Y-005", "A11Y", "warning", !tabindexPosRe.MatchString(html), "No positive tabindex anti-patterns")

	// DOM / Grex Safety (5 rules)
	divMatches := divOpenRe.FindAllString(html, -1)
	add("GROXY-DOM-001", "DOM", "critical", len(divMatches) == 0, "Zero <div> wrapper elements (Pure Grex JS v1.0.0 contract)")

	inlineScriptViolation := false
	for _, sm := range inlineScriptRe.FindAllStringSubmatch(html, -1) {
		if !strings.Contains(strings.ToLower(sm[1]), "src=") {
			inlineScriptViolation = true
			break
		}
	}
	if evalOrFuncRe.MatchString(html) {
		inlineScriptViolation = true
	}
	add("GROXY-DOM-002", "DOM", "critical", !inlineScriptViolation, "Zero inline <script> blocks, eval(), or new Function() calls")
	add("GROXY-DOM-003", "DOM", "critical", !innerHTMLRe.MatchString(html), "Zero innerHTML/outerHTML/insertAdjacentHTML DOM sinks")
	add("GROXY-DOM-004", "DOM", "critical", !inlineEventRe.MatchString(html), "Zero inline onclick/onload HTML event attributes")

	unknownTag := false
	for _, tm := range tagNameRe.FindAllStringSubmatch(html, -1) {
		tag := strings.ToLower(tm[1])
		if !allowedGrexAuditTags[tag] {
			unknownTag = true
			break
		}
	}
	add("GROXY-DOM-005", "DOM", "error", !unknownTag, "Only approved semantic Grex JS v1.0.0 HTML elements used")

	// Link Pillar (4 rules)
	add("GROXY-LINK-001", "Link", "critical", !jsHrefRe.MatchString(html), "Zero javascript:, vbscript:, or data:text/html links")

	blankValid := true
	for _, bm := range blankLinkRe.FindAllStringSubmatch(html, -1) {
		lowerAttrs := strings.ToLower(bm[1])
		if !strings.Contains(lowerAttrs, "noopener") && !strings.Contains(lowerAttrs, "noreferrer") {
			blankValid = false
			break
		}
	}
	add("GROXY-LINK-002", "Link", "warning", blankValid, "Links using target=\"_blank\" include rel=\"noopener\" or rel=\"noreferrer\"")
	add("GROXY-LINK-003", "Link", "warning", !emptyHrefRe.MatchString(html), "Zero empty or # placeholder links")

	urlsValid := true
	for _, um := range urlAttrRe.FindAllStringSubmatch(html, -1) {
		val := strings.TrimSpace(um[1])
		if val == "" || strings.ContainsAny(val, " \t\n\r") {
			urlsValid = false
			break
		}
	}
	add("GROXY-LINK-004", "Link", "info", urlsValid, "All href and src attributes contain well-formed non-whitespace URLs")

	passed := 0
	criticalFail := false
	for _, f := range findings {
		if f.Passed {
			passed++
		} else if f.Severity == "critical" {
			criticalFail = true
		}
	}

	rawScore := (passed * 100) / len(findings)
	if criticalFail && rawScore > 49 {
		rawScore = 49
	}
	grade := scoreToGrade(rawScore, criticalFail)

	return AuditReport{
		TargetPath:  normalizeTarget(targetPath),
		Score:       strconv.Itoa(rawScore),
		Grade:       grade,
		PublishSafe: !criticalFail && rawScore >= 70,
		PassedCount: strconv.Itoa(passed),
		TotalRules:  strconv.Itoa(len(findings)),
		DivCount:    strconv.Itoa(len(divMatches)),
		Findings:    findings,
		Summary:     fmt.Sprintf("%d/%d rules passed (%d%%, Grade %s, PublishSafe=%t)", passed, len(findings), rawScore, grade, !criticalFail && rawScore >= 70),
	}
}

func checkHeadingOrder(html string) bool {
	matches := headingLevelRe.FindAllStringSubmatch(html, -1)
	if len(matches) == 0 {
		return true
	}
	prev := 0
	for _, m := range matches {
		lvl, err := strconv.Atoi(m[1])
		if err != nil {
			continue
		}
		if prev == 0 && lvl != 1 {
			return false
		}
		if prev > 0 && lvl-prev > 1 {
			return false
		}
		prev = lvl
	}
	return true
}

func scoreToGrade(score int, criticalFail bool) string {
	if criticalFail {
		return "F"
	}
	switch {
	case score >= 90:
		return "A"
	case score >= 80:
		return "B"
	case score >= 70:
		return "C"
	case score >= 60:
		return "D"
	default:
		return "F"
	}
}
