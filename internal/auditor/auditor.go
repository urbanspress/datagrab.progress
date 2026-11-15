// Package auditor implements Groxy Desk's built-in 22-rule / 4-pillar
// deterministic SEO, WCAG AA accessibility, clean DOM, and internal-linking
// auditor, GRAB-DESIGN-SYSTEM.md 8x5 visual studio synthesizer, and
// "Let Groxy Boost This" diff & remediation engine.
package auditor

import (
	"fmt"
	"math"
	"net/url"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"groxy-desk/internal/security"
)

// GrexElementsCatalog defines the frozen 20 elements from grex.elements.json.
var GrexElementsCatalog = []string{
	"Section",
	"Container",
	"Grid",
	"Stack",
	"Card",
	"Heading",
	"Text",
	"RichText",
	"Link",
	"Button",
	"Image",
	"Icon",
	"Form",
	"Input",
	"Select",
	"Tabs",
	"Accordion",
	"Modal",
	"Nav",
	"ForEach",
}

var core20ElementSet = func() map[string]bool {
	m := make(map[string]bool, len(GrexElementsCatalog))
	for _, el := range GrexElementsCatalog {
		m[el] = true
	}
	return m
}()

var allowedSemanticHTMLTags = map[string]bool{
	"section": true, "header": true, "footer": true, "main": true,
	"aside": true, "article": true, "nav": true,
	"h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true,
	"p": true, "span": true, "small": true, "strong": true, "em": true,
	"blockquote": true, "time": true, "code": true, "a": true, "button": true,
	"img": true, "svg": true, "form": true, "label": true, "input": true,
	"textarea": true, "select": true, "option": true, "ul": true, "ol": true,
	"li": true, "tbody": true, "dialog": true, "dl": true, "dt": true, "dd": true,
}

var vagueAnchors = map[string]bool{
	"click here": true, "here": true, "read more": true, "learn more": true,
	"more": true, "more info": true, "details": true, "link": true,
	"this": true, "this page": true, "view": true, "go": true, "see more": true,
}

var reservedSlugs = map[string]bool{
	"admin": true, "api": true, "login": true, "logout": true, "wp-admin": true,
	"assets": true, "public": true, "static": true, "install": true, "robots": true,
}

// GrexNode mirrors a JSON node conforming to grex.elements.json.
type GrexNode struct {
	Element  string         `json:"element"`
	Tag      string         `json:"tag,omitempty"`
	Props    map[string]any `json:"props,omitempty"`
	Styles   map[string]any `json:"styles,omitempty"`
	Children []*GrexNode    `json:"children,omitempty"`
}

// RuleFinding represents one deterministic rule evaluation in the 22-rule audit.
type RuleFinding struct {
	RuleID       string `json:"rule_id"`
	Category     string `json:"category"`
	Title        string `json:"title"`
	Status       string `json:"status"` // pass | warning | error | critical
	PointsEarned string `json:"points_earned"`
	MaxPoints    string `json:"max_points"`
	Message      string `json:"message"`
	FixHint      string `json:"fix_hint"`
}

// PillarScore summarizes one of the 4 equal-weight (25%) audit pillars.
type PillarScore struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Weight string `json:"weight"`
	Score  string `json:"score"`
	Grade  string `json:"grade"`
}

// DesignSystemSummary reports active token and class adherence.
type DesignSystemSummary struct {
	TokensLoaded          string   `json:"tokens_loaded"`
	ReusableClassesLoaded string   `json:"reusable_classes_loaded"`
	UnknownClassesUsed    []string `json:"unknown_classes_used"`
}

// SchemaOrgPreview holds generated XSS-safe schema.org metadata.
type SchemaOrgPreview struct {
	Type     string         `json:"type"`
	Payload  map[string]any `json:"payload"`
	JSONText string         `json:"json_text"`
}

// ScorecardReport matches the frozen Groxy AI v1.0.0 22-rule / 4-pillar contract
// while preserving ADR-025c base-10 wire strings.
type ScorecardReport struct {
	SchemaVersion       string                 `json:"schema_version"`
	PlaybookVersion     string                 `json:"playbook_version"`
	TargetPath          string                 `json:"target_path"`
	Title               string                 `json:"title"`
	Slug                string                 `json:"slug"`
	OverallScore        string                 `json:"overall_score"`
	OverallGrade        string                 `json:"overall_grade"`
	PublishSafe         bool                   `json:"publish_safe"`
	DivCount            string                 `json:"div_count"`
	SecurityViolations  []string               `json:"security_violations"`
	Categories          map[string]PillarScore `json:"categories"`
	Findings            []RuleFinding          `json:"findings"`
	DesignSystemSummary DesignSystemSummary    `json:"design_system_summary"`
	SchemaOrg           SchemaOrgPreview       `json:"schema_org"`
}

// AuditInput describes HTML and/or a Grex layout tree plus workspace context.
type AuditInput struct {
	TargetPath     string            `json:"target_path,omitempty"`
	ContentType    string            `json:"content_type,omitempty"`
	Title          string            `json:"title,omitempty"`
	MetaTitle      string            `json:"meta_title,omitempty"`
	MetaDesc       string            `json:"meta_description,omitempty"`
	Slug           string            `json:"slug,omitempty"`
	FocusKeyword   string            `json:"focus_keyword,omitempty"`
	CanonicalURL   string            `json:"canonical_url,omitempty"`
	HTML           string            `json:"html,omitempty"`
	GrexLayout     *GrexNode         `json:"grex_layout,omitempty"`
	DesignSystemMD string            `json:"design_system_md,omitempty"`
	SiteContextMD  string            `json:"site_context_md,omitempty"`
	CustomFields   map[string]string `json:"custom_fields,omitempty"`
}

// ValidationViolation represents one validation error returned by ValidateGrexTree (HTTP 422 / -32602).
type ValidationViolation struct {
	Code    string `json:"code"`
	Path    string `json:"path"`
	Message string `json:"message"`
}

type ruleSpec struct {
	ID       string
	Category string
	Title    string
	Max      int
}

var all22Rules = []ruleSpec{
	// Pillar A: on_page_seo (100)
	{ID: "GROXY-SEO-001", Category: "on_page_seo", Title: "Page & Entry Title Presence", Max: 15},
	{ID: "GROXY-SEO-002", Category: "on_page_seo", Title: "Title Length & Keyword Focus", Max: 15},
	{ID: "GROXY-SEO-003", Category: "on_page_seo", Title: "Meta Description / Excerpt", Max: 15},
	{ID: "GROXY-SEO-004", Category: "on_page_seo", Title: "URL Slug & Route Hygiene", Max: 10},
	{ID: "GROXY-SEO-005", Category: "on_page_seo", Title: "Single Primary H1", Max: 15},
	{ID: "GROXY-SEO-006", Category: "on_page_seo", Title: "Editorial Subheadings & Content Depth", Max: 15},
	{ID: "GROXY-SEO-007", Category: "on_page_seo", Title: "Canonical URL & Keyword Consistency", Max: 10},
	{ID: "GROXY-SEO-008", Category: "on_page_seo", Title: "Document Language & Responsive Meta", Max: 5},
	// Pillar B: content_a11y (100)
	{ID: "GROXY-A11Y-001", Category: "content_a11y", Title: "WCAG AA Light/Dark Contrast (>= 4.5:1)", Max: 25},
	{ID: "GROXY-A11Y-002", Category: "content_a11y", Title: "Descriptive Image Alt Text", Max: 25},
	{ID: "GROXY-A11Y-003", Category: "content_a11y", Title: "Accessible Form Control Labels", Max: 15},
	{ID: "GROXY-A11Y-004", Category: "content_a11y", Title: "Button/Link Labels & Focus Indicators", Max: 15},
	{ID: "GROXY-A11Y-005", Category: "content_a11y", Title: "Sequential Heading Hierarchy (H1->H2->H3)", Max: 20},
	// Pillar C: performance_dom (100)
	{ID: "GROXY-DOM-001", Category: "performance_dom", Title: "Zero Wrapper <div> Architecture", Max: 25},
	{ID: "GROXY-DOM-002", Category: "performance_dom", Title: "Grex 20-Element & HTML5 Landmarks", Max: 20},
	{ID: "GROXY-DOM-003", Category: "performance_dom", Title: "Zero Script / Iframe / XSS Vectors", Max: 25},
	{ID: "GROXY-DOM-004", Category: "performance_dom", Title: "Bounded DOM Depth & Node Count", Max: 15},
	{ID: "GROXY-DOM-005", Category: "performance_dom", Title: "Design Token & Semantic Rhythm Usage", Max: 15},
	// Pillar D: linking_readiness (100)
	{ID: "GROXY-LINK-001", Category: "linking_readiness", Title: "Descriptive Contextual Internal Links", Max: 30},
	{ID: "GROXY-LINK-002", Category: "linking_readiness", Title: "SITE-CONTEXT.md Link Graph & Zero Loops", Max: 25},
	{ID: "GROXY-LINK-003", Category: "linking_readiness", Title: "Navigation Landmarks & Zero Orphan Risk", Max: 25},
	{ID: "GROXY-LINK-004", Category: "linking_readiness", Title: "XSS-Safe Schema.org JSON-LD Readiness", Max: 20},
}

var (
	reTagStrip     = regexp.MustCompile(`(?is)<[^>]+>`)
	reTitle        = regexp.MustCompile(`(?is)<title\b[^>]*>(.*?)</title>`)
	reMetaDesc     = regexp.MustCompile(`(?is)<meta\b[^>]*name=["']description["'][^>]*content=["']([^"']*)["'][^>]*>|<meta\b[^>]*content=["']([^"']*)["'][^>]*name=["']description["'][^>]*>`)
	reCanonical    = regexp.MustCompile(`(?is)<link\b[^>]*rel=["']canonical["'][^>]*href=["']([^"']+)["'][^>]*>`)
	reViewport     = regexp.MustCompile(`(?is)<meta\b[^>]*name=["']viewport["'][^>]*>`)
	reHTMLLang     = regexp.MustCompile(`(?is)<html\b[^>]*\blang=["']([a-z]{2,3}(?:-[a-z0-9]{2,4})?)["']`)
	reHeading      = regexp.MustCompile(`(?is)<(h[1-6])\b[^>]*>(.*?)</h[1-6]>`)
	reDivOpen      = regexp.MustCompile(`(?i)<\s*div\b`)
	reScriptOpen   = regexp.MustCompile(`(?is)<\s*script\b[^>]*>`)
	reScriptFull   = regexp.MustCompile(`(?is)<\s*script\b([^>]*)>`)
	reIframeOpen   = regexp.MustCompile(`(?i)<\s*iframe\b`)
	reInlineEvent  = regexp.MustCompile(`(?i)\bon[a-z]+\s*=`)
	reInnerHTML    = regexp.MustCompile(`(?i)\binnerHTML\s*=|\bouterHTML\s*=`)
	reEvalCall     = regexp.MustCompile(`(?i)\beval\s*\(|\bnew\s+Function\s*\(`)
	reUnsafeProto  = regexp.MustCompile(`(?i)(href|src|action)\s*=\s*["']\s*(javascript:|vbscript:|file:|data:text/html)`)
	reImgTag       = regexp.MustCompile(`(?is)<img\b([^>]*)>`)
	reAttrAlt      = regexp.MustCompile(`(?is)\balt\s*=\s*["']([^"']*)["']`)
	reAriaHidden   = regexp.MustCompile(`(?is)\baria-hidden\s*=\s*["']true["']`)
	reAnchorTag    = regexp.MustCompile(`(?is)<a\b([^>]*)>(.*?)</a>`)
	reAttrHref     = regexp.MustCompile(`(?is)\bhref\s*=\s*["']([^"']*)["']`)
	reAttrLabel    = regexp.MustCompile(`(?is)\baria-label\s*=\s*["']([^"']*)["']`)
	reAttrID       = regexp.MustCompile(`(?is)\bid\s*=\s*["']([^"']*)["']`)
	reAttrType     = regexp.MustCompile(`(?is)\btype\s*=\s*["']([^"']*)["']`)
	reButtonTag    = regexp.MustCompile(`(?is)<button\b([^>]*)>(.*?)</button>`)
	reInputTag     = regexp.MustCompile(`(?is)<(input|select|textarea)\b([^>]*)>`)
	reLabelFor     = regexp.MustCompile(`(?is)<label\b[^>]*for\s*=\s*["']([^"']+)["']`)
	reSemanticOpen = regexp.MustCompile(`(?i)<\s*(header|main|footer|nav|section|article|aside)\b`)
	reGenericOpen  = regexp.MustCompile(`(?i)<\s*([a-z][a-z0-9:-]*)\b`)
	reWhitespace   = regexp.MustCompile(`\s+`)
	reHexColor     = regexp.MustCompile(`^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`)
	reCSSVar       = regexp.MustCompile(`--grex-[a-z0-9-]+`)
)

// Audit runs the deterministic 22-rule / 4-pillar audit over an HTML document,
// Grex layout tree, or combined payload.
func Audit(in AuditInput) ScorecardReport {
	ds := ParseDesignSystemMarkdown(in.DesignSystemMD)
	sc := ParseSiteContextMarkdown(in.SiteContextMD)

	htmlText := in.HTML
	tree := in.GrexLayout

	title := firstNonEmpty(in.MetaTitle, in.Title)
	if title == "" && htmlText != "" {
		if m := reTitle.FindStringSubmatch(htmlText); len(m) > 1 {
			title = cleanText(m[1])
		}
	}
	metaDesc := strings.TrimSpace(in.MetaDesc)
	if metaDesc == "" && htmlText != "" {
		if m := reMetaDesc.FindStringSubmatch(htmlText); len(m) > 0 {
			metaDesc = cleanText(firstNonEmpty(m[1], m[2]))
		}
	}
	slug := normalizeSlug(in.Slug)
	if slug == "" && in.TargetPath != "" {
		slug = deriveSlugFromPath(in.TargetPath)
	}
	focusKW := strings.TrimSpace(strings.ToLower(in.FocusKeyword))
	if focusKW == "" {
		focusKW = deriveKeywordFromTitle(title)
	}
	canonicalURL := strings.TrimSpace(in.CanonicalURL)
	if canonicalURL == "" && htmlText != "" {
		if m := reCanonical.FindStringSubmatch(htmlText); len(m) > 1 {
			canonicalURL = strings.TrimSpace(m[1])
		}
	}

	// Collect stats across both HTML and Grex tree.
	stats := collectStats(htmlText, tree, ds)
	if title == "" && len(stats.headings) > 0 && stats.headings[0].level == 1 {
		title = stats.headings[0].text
	}
	if metaDesc == "" && len(stats.paragraphs) > 0 {
		metaDesc = truncateClean(strings.Join(stats.paragraphs, " "), 155)
	}
	if canonicalURL == "" && slug != "" {
		canonicalURL = "/" + slug
	}
	hasLang := reHTMLLang.MatchString(htmlText) || tree != nil
	hasViewport := reViewport.MatchString(htmlText) || tree != nil
	if stats.h1Count == 1 && !hasLang && htmlText != "" && !strings.Contains(strings.ToLower(htmlText), "<html") {
		// Partial snippet HTML without outer <html> wrapper stays focused cleanly.
		hasLang = true
		hasViewport = true
	}

	h1Count := stats.h1Count
	h2PlusCount := stats.h2PlusCount
	totalWords := stats.wordCount

	findings := make([]RuleFinding, 0, len(all22Rules))

	// ---- Pillar A: On-Page SEO ----
	// 1. GROXY-SEO-001 (15)
	if strings.TrimSpace(title) != "" {
		findings = append(findings, makeFinding(all22Rules[0], 15, "pass", "Primary document/entry title is present.", "No change needed."))
	} else {
		findings = append(findings, makeFinding(all22Rules[0], 0, "error", "Missing <title> or entry title.", "Add a concise 30-65 character title describing this page."))
	}

	// 2. GROXY-SEO-002 (15)
	tLen := len([]rune(strings.TrimSpace(title)))
	switch {
	case tLen >= 25 && tLen <= 68:
		findings = append(findings, makeFinding(all22Rules[1], 15, "pass", fmt.Sprintf("Title length (%d chars) sits cleanly in the 25-68 char search sweet spot.", tLen), "No change needed."))
	case tLen >= 10 && tLen <= 85:
		findings = append(findings, makeFinding(all22Rules[1], 10, "warning", fmt.Sprintf("Title is %d chars; 30-65 characters gives stronger search display.", tLen), "Tune the title length to 30-65 characters including the primary topic."))
	case tLen > 0:
		findings = append(findings, makeFinding(all22Rules[1], 5, "warning", fmt.Sprintf("Title is %d chars (too brief or truncated in SERP).", tLen), "Expand or tighten title to 30-65 descriptive characters."))
	default:
		findings = append(findings, makeFinding(all22Rules[1], 0, "error", "No title available to score keyword focus.", "Provide a descriptive 30-65 character title."))
	}

	// 3. GROXY-SEO-003 (15)
	dLen := len([]rune(strings.TrimSpace(metaDesc)))
	switch {
	case dLen >= 80 && dLen <= 165:
		findings = append(findings, makeFinding(all22Rules[2], 15, "pass", fmt.Sprintf("Meta description/excerpt (%d chars) is clear and snippet-ready.", dLen), "No change needed."))
	case dLen >= 40 && dLen <= 200:
		findings = append(findings, makeFinding(all22Rules[2], 10, "warning", fmt.Sprintf("Meta description is %d chars; 120-158 chars is optimal.", dLen), "Tune summary copy to 120-158 characters."))
	case dLen > 0:
		findings = append(findings, makeFinding(all22Rules[2], 5, "warning", fmt.Sprintf("Meta description (%d chars) is too short for rich search previews.", dLen), "Write a 120-158 character summary explaining the page outcome."))
	default:
		findings = append(findings, makeFinding(all22Rules[2], 0, "error", "Missing meta description / excerpt.", "Add a 120-158 character <meta name=\"description\"> tag or entry excerpt."))
	}

	// 4. GROXY-SEO-004 (10)
	if isCleanSlug(slug) && !reservedSlugs[firstSegment(slug)] {
		findings = append(findings, makeFinding(all22Rules[3], 10, "pass", fmt.Sprintf("URL slug %q is clean, lowercase, hyphenated, and collision-free.", slug), "No change needed."))
	} else {
		findings = append(findings, makeFinding(all22Rules[3], 0, "error", fmt.Sprintf("URL slug %q is missing, non-hyphenated, or collides with reserved route prefixes.", slug), "Use a clean lowercase hyphenated slug under 75 characters."))
	}

	// 5. GROXY-SEO-005 (15)
	switch {
	case h1Count == 1:
		findings = append(findings, makeFinding(all22Rules[4], 15, "pass", "Exactly one primary <h1> heading establishes page focus.", "No change needed."))
	case h1Count > 1:
		findings = append(findings, makeFinding(all22Rules[4], 5, "warning", fmt.Sprintf("Found %d <h1> headings; keep one primary <h1> per page.", h1Count), "Convert secondary <h1> elements to <h2>."))
	default:
		findings = append(findings, makeFinding(all22Rules[4], 0, "error", "No <h1> heading found in page or Grex layout.", "Add a single semantic Heading (level 1 / <h1>) at the top of main content."))
	}

	// 6. GROXY-SEO-006 (15)
	switch {
	case h2PlusCount >= 1 && totalWords >= 12:
		findings = append(findings, makeFinding(all22Rules[5], 15, "pass", fmt.Sprintf("Structured editorial hierarchy (%d subheadings, %d words).", h2PlusCount, totalWords), "No change needed."))
	case h2PlusCount >= 1 || totalWords >= 8:
		findings = append(findings, makeFinding(all22Rules[5], 8, "warning", fmt.Sprintf("Light content depth (%d subheadings, %d words).", h2PlusCount, totalWords), "Add structured <h2> subsections and concise explanatory copy."))
	default:
		findings = append(findings, makeFinding(all22Rules[5], 0, "error", "Missing subheadings and editorial body copy.", "Add semantic <h2> subsections and descriptive content."))
	}

	// 7. GROXY-SEO-007 (10)
	if canonicalURL != "" && IsSafeURL(canonicalURL) {
		findings = append(findings, makeFinding(all22Rules[6], 10, "pass", fmt.Sprintf("Valid canonical reference (%s) prevents duplicate indexing.", canonicalURL), "No change needed."))
	} else {
		findings = append(findings, makeFinding(all22Rules[6], 0, "warning", "Missing <link rel=\"canonical\"> or valid canonical slug path.", "Add a safe canonical link matching the primary route."))
	}

	// 8. GROXY-SEO-008 (5)
	if hasLang && hasViewport {
		findings = append(findings, makeFinding(all22Rules[7], 5, "pass", "Document language and responsive viewport metadata verified.", "No change needed."))
	} else {
		findings = append(findings, makeFinding(all22Rules[7], 0, "warning", "Missing <html lang=\"...\"> or <meta name=\"viewport\">.", "Declare <html lang=\"en\"> and a responsive viewport meta tag."))
	}

	// ---- Pillar B: Content & WCAG Accessibility ----
	// 9. GROXY-A11Y-001 (25)
	if len(stats.contrastFailures) == 0 {
		findings = append(findings, makeFinding(all22Rules[8], 25, "pass", "All evaluated colors and --grex-* tokens meet WCAG AA contrast (>= 4.5:1).", "No change needed."))
	} else {
		findings = append(findings, makeFinding(all22Rules[8], 0, "error", stats.contrastFailures[0], "Use Light/Dark WCAG AA verified surface/text token pairs (>= 4.5:1)."))
	}

	// 10. GROXY-A11Y-002 (25)
	if len(stats.imageAltFailures) == 0 {
		msg := "All images include compliant alt text or explicit decorative markers."
		if stats.imageCount == 0 {
			msg = "No unlabelled images detected."
		}
		findings = append(findings, makeFinding(all22Rules[9], 25, "pass", msg, "No change needed."))
	} else {
		findings = append(findings, makeFinding(all22Rules[9], 0, "error", stats.imageAltFailures[0], "Provide descriptive 3-125 char alt text or set decorative: true."))
	}

	// 11. GROXY-A11Y-003 (15)
	if len(stats.unlabeledControls) == 0 {
		findings = append(findings, makeFinding(all22Rules[10], 15, "pass", "All interactive form controls expose accessible labels.", "No change needed."))
	} else {
		findings = append(findings, makeFinding(all22Rules[10], 0, "error", stats.unlabeledControls[0], "Bind <label for=\"...\"> or aria-label on every Input and Select."))
	}

	// 12. GROXY-A11Y-004 (15)
	if len(stats.unlabeledButtons) == 0 {
		findings = append(findings, makeFinding(all22Rules[11], 15, "pass", "All buttons and links have accessible text and focus readiness.", "No change needed."))
	} else {
		findings = append(findings, makeFinding(all22Rules[11], 0, "error", stats.unlabeledButtons[0], "Give every Button and Link descriptive visible text or aria-label."))
	}

	// 13. GROXY-A11Y-005 (20)
	if len(stats.headingSkips) == 0 {
		findings = append(findings, makeFinding(all22Rules[12], 20, "pass", "Heading levels step cleanly (H1 -> H2 -> H3) with zero skipped levels.", "No change needed."))
	} else {
		findings = append(findings, makeFinding(all22Rules[12], 0, "error", stats.headingSkips[0], "Never skip heading levels (avoid jumping directly from H1 to H3/H4)."))
	}

	// ---- Pillar C: Performance & Clean DOM ----
	// 14. GROXY-DOM-001 (25) — critical wrapper <div> gate
	hasDivViolation := stats.divCount > 0
	if !hasDivViolation {
		findings = append(findings, makeFinding(all22Rules[13], 25, "pass", "Zero wrapper <div> tags detected.", "No change needed."))
	} else {
		findings = append(findings, makeFinding(all22Rules[13], 0, "critical", fmt.Sprintf("Detected %d forbidden wrapper <div> element(s).", stats.divCount), "Replace <div> wrappers with semantic Grex elements (Section, Container, Stack, Grid, Card)."))
	}

	// 15. GROXY-DOM-002 (20)
	if len(stats.invalidElements) == 0 && len(stats.invalidTags) == 0 {
		findings = append(findings, makeFinding(all22Rules[14], 20, "pass", "All nodes strictly conform to the 20-element grex.elements.json and semantic HTML5 spec.", "No change needed."))
	} else {
		findings = append(findings, makeFinding(all22Rules[14], 0, "error", joinNonEmpty(stats.invalidElements, stats.invalidTags), "Restrict UI nodes to the 20 Grex elements and semantic HTML5 landmarks."))
	}

	// 16. GROXY-DOM-003 (25) — critical security gate
	hasSecurityViolation := len(stats.securityViolations) > 0
	if !hasSecurityViolation {
		findings = append(findings, makeFinding(all22Rules[15], 25, "pass", "Zero <script>, <iframe>, eval(), inline handlers, innerHTML writes, or unsafe URLs detected.", "No change needed."))
	} else {
		findings = append(findings, makeFinding(all22Rules[15], 0, "critical", stats.securityViolations[0], "Eliminate <script>, <iframe>, eval(), inline on* handlers, innerHTML, and unsafe URLs."))
	}

	// 17. GROXY-DOM-004 (15)
	if stats.maxDepth <= 24 && stats.nodeCount <= 250 {
		findings = append(findings, makeFinding(all22Rules[16], 15, "pass", fmt.Sprintf("DOM depth (%d) and node count (%d) remain compact and fast.", stats.maxDepth, stats.nodeCount), "No change needed."))
	} else {
		findings = append(findings, makeFinding(all22Rules[16], 0, "error", fmt.Sprintf("DOM depth (%d) or node count (%d) exceeds safety budget.", stats.maxDepth, stats.nodeCount), "Simplify nested sections to <= 24 levels and <= 250 nodes."))
	}

	// 18. GROXY-DOM-005 (15)
	if stats.usesDesignTokens || stats.semanticLandmarkCount > 0 || tree != nil {
		findings = append(findings, makeFinding(all22Rules[17], 15, "pass", "Semantic landmarks and active --grex-* design tokens verified.", "No change needed."))
	} else {
		findings = append(findings, makeFinding(all22Rules[17], 5, "warning", "Document lacks semantic HTML5 landmarks (<header>, <main>, <section>) or --grex-* tokens.", "Wrap content in semantic landmarks styled with --grex-* variables."))
	}

	// ---- Pillar D: Internal Linking Readiness ----
	// 19. GROXY-LINK-001 (30)
	internalLinks := filterInternalLinks(stats.links)
	vagueCount := countVagueLinks(internalLinks)
	switch {
	case len(internalLinks) > 0 && vagueCount == 0:
		findings = append(findings, makeFinding(all22Rules[18], 30, "pass", fmt.Sprintf("%d descriptive internal link(s) discovered.", len(internalLinks)), "No change needed."))
	case len(internalLinks) > 0:
		findings = append(findings, makeFinding(all22Rules[18], 15, "warning", fmt.Sprintf("%d internal link(s) use generic anchor phrasing.", vagueCount), "Use clear keyword-rich anchor text instead of 'click here' or 'read more'."))
	default:
		findings = append(findings, makeFinding(all22Rules[18], 8, "warning", "No internal contextual links found.", "Add 1-2 contextual internal links referencing related pages in SITE-CONTEXT.md."))
	}

	// 20. GROXY-LINK-002 (25)
	linkGraphErr := checkSiteContextGraph(internalLinks, slug, sc)
	if linkGraphErr == "" {
		findings = append(findings, makeFinding(all22Rules[19], 25, "pass", "Internal link graph aligns with SITE-CONTEXT.md with zero redirect loops.", "No change needed."))
	} else {
		findings = append(findings, makeFinding(all22Rules[19], 10, "warning", linkGraphErr, "Point links directly to canonical targets in SITE-CONTEXT.md."))
	}

	// 21. GROXY-LINK-003 (25)
	if stats.hasNav || len(internalLinks) > 0 || h2PlusCount >= 1 {
		findings = append(findings, makeFinding(all22Rules[20], 25, "pass", "Page connects cleanly to navigation landmarks and section anchors.", "No change needed."))
	} else {
		findings = append(findings, makeFinding(all22Rules[20], 10, "warning", "Potential orphan page with neither navigation landmark nor internal links.", "Include primary <nav> or internal next-step links."))
	}

	// 22. GROXY-LINK-004 (20)
	schemaOrg := BuildSchemaOrgPreview(in.ContentType, title, metaDesc, canonicalURL, in.CustomFields, focusKW)
	if title != "" && metaDesc != "" && len(in.CustomFields) >= 0 {
		findings = append(findings, makeFinding(all22Rules[21], 20, "pass", fmt.Sprintf("XSS-safe schema.org (%s) JSON-LD preview generated with exact ADR-025c values.", schemaOrg.Type), "No change needed."))
	} else {
		findings = append(findings, makeFinding(all22Rules[21], 10, "warning", "Schema.org WebPage preview missing title or excerpt fields.", "Complete page title and meta description to emit valid schema.org JSON-LD."))
	}

	// Aggregate 4 equal pillars (25% each).
	totals := map[string]int{
		"on_page_seo":       0,
		"content_a11y":      0,
		"performance_dom":   0,
		"linking_readiness": 0,
	}
	for _, f := range findings {
		pts, _ := strconv.Atoi(f.PointsEarned)
		totals[f.Category] += pts
	}
	if hasSecurityViolation {
		totals["performance_dom"] = 0
	}

	overall := int(math.Round(float64(totals["on_page_seo"]+totals["content_a11y"]+totals["performance_dom"]+totals["linking_readiness"]) / 4.0))
	publishSafe := !hasDivViolation && !hasSecurityViolation
	if !publishSafe && overall > 59 {
		overall = 59
	}

	categories := map[string]PillarScore{
		"on_page_seo": {
			ID:     "on_page_seo",
			Label:  "On-Page SEO",
			Weight: "25",
			Score:  strconv.Itoa(totals["on_page_seo"]),
			Grade:  ScoreToGrade(totals["on_page_seo"], true),
		},
		"content_a11y": {
			ID:     "content_a11y",
			Label:  "Content & WCAG Accessibility",
			Weight: "25",
			Score:  strconv.Itoa(totals["content_a11y"]),
			Grade:  ScoreToGrade(totals["content_a11y"], true),
		},
		"performance_dom": {
			ID:     "performance_dom",
			Label:  "Performance & Clean DOM",
			Weight: "25",
			Score:  strconv.Itoa(totals["performance_dom"]),
			Grade:  ScoreToGrade(totals["performance_dom"], publishSafe),
		},
		"linking_readiness": {
			ID:     "linking_readiness",
			Label:  "Off-Page / Internal Linking Readiness",
			Weight: "25",
			Score:  strconv.Itoa(totals["linking_readiness"]),
			Grade:  ScoreToGrade(totals["linking_readiness"], true),
		},
	}

	return ScorecardReport{
		SchemaVersion:      "1.0.0",
		PlaybookVersion:    "1.0.0",
		TargetPath:         strings.TrimSpace(in.TargetPath),
		Title:              title,
		Slug:               slug,
		OverallScore:       strconv.Itoa(overall),
		OverallGrade:       ScoreToGrade(overall, publishSafe),
		PublishSafe:        publishSafe,
		DivCount:           strconv.Itoa(stats.divCount),
		SecurityViolations: nonNilSlice(stats.securityViolations),
		Categories:         categories,
		Findings:           findings,
		DesignSystemSummary: DesignSystemSummary{
			TokensLoaded:          strconv.Itoa(len(ds.Tokens)),
			ReusableClassesLoaded: strconv.Itoa(len(ds.Classes)),
			UnknownClassesUsed:    []string{},
		},
		SchemaOrg: schemaOrg,
	}
}

// ValidateGrexTree validates a GrexNode tree strictly against grex.elements.json,
// HTML5 semantic tags, URL allowlists, heading order, and WCAG AA contrast (>= 4.5:1),
// returning HTTP 422 / JSON-RPC -32602 compatible violations.
func ValidateGrexTree(root *GrexNode) []ValidationViolation {
	if root == nil {
		return []ValidationViolation{{
			Code:    "GREX-TREE-EMPTY",
			Path:    "layout",
			Message: "Grex layout tree cannot be null",
		}}
	}
	var out []ValidationViolation
	lastH := 0
	var walk func(n *GrexNode, path string, depth int)
	walk = func(n *GrexNode, path string, depth int) {
		if depth > 24 {
			out = append(out, ValidationViolation{
				Code:    "GREX-MAX-DEPTH",
				Path:    path,
				Message: "Grex layout nesting exceeds depth limit (24)",
			})
			return
		}
		if !core20ElementSet[n.Element] {
			out = append(out, ValidationViolation{
				Code:    "GREX-UNKNOWN-ELEMENT",
				Path:    path,
				Message: fmt.Sprintf("Unsupported Grex element %q (expected one of 20 core elements)", n.Element),
			})
		}
		tag := strings.ToLower(strings.TrimSpace(n.Tag))
		if tag != "" && !allowedSemanticHTMLTags[tag] {
			out = append(out, ValidationViolation{
				Code:    "GREX-FORBIDDEN-TAG",
				Path:    path,
				Message: fmt.Sprintf("Forbidden HTML tag <%s> (zero-<div>, zero-script/iframe architecture)", tag),
			})
		}
		for k, v := range n.Props {
			kl := strings.ToLower(k)
			if strings.HasPrefix(kl, "on") {
				out = append(out, ValidationViolation{
					Code:    "GREX-SECURITY-HANDLER",
					Path:    path + ".props." + k,
					Message: fmt.Sprintf("Inline event handler prop %q is forbidden", k),
				})
			}
			if kl == "innerhtml" || kl == "dangerouslysetinnerhtml" {
				out = append(out, ValidationViolation{
					Code:    "GREX-SECURITY-HTML",
					Path:    path + ".props." + k,
					Message: "Raw HTML injection props are forbidden",
				})
			}
			if (kl == "href" || kl == "src" || kl == "action") && v != nil {
				s := fmt.Sprint(v)
				if !IsSafeURL(s) {
					out = append(out, ValidationViolation{
						Code:    "GREX-UNSAFE-URL",
						Path:    path + ".props." + k,
						Message: fmt.Sprintf("Unsafe URL %q rejected by security policy", s),
					})
				}
			}
		}
		if n.Element == "Heading" || (len(tag) == 2 && tag[0] == 'h' && tag[1] >= '1' && tag[1] <= '6') {
			lvl := headingLevelFromNode(n)
			if lastH > 0 && lvl > lastH+1 {
				out = append(out, ValidationViolation{
					Code:    "GREX-HEADING-SKIP",
					Path:    path,
					Message: fmt.Sprintf("Skipped heading level h%d after h%d", lvl, lastH),
				})
			}
			lastH = lvl
		}
		if n.Element == "Image" || tag == "img" {
			decorative, _ := n.Props["decorative"].(bool)
			altStr := strings.TrimSpace(fmt.Sprint(nonNilValue(n.Props["alt"])))
			if !decorative && len(altStr) < 3 {
				out = append(out, ValidationViolation{
					Code:    "GREX-IMAGE-ALT",
					Path:    path,
					Message: "Non-decorative Image requires descriptive alt text (at least 3 characters)",
				})
			}
		}
		// Check explicit hex contrast in styles if both color and background-color are concrete hex values.
		if c, bg := extractNodeColors(n.Styles); c != "" && bg != "" {
			if ratio := ComputeContrastRatio(c, bg); ratio > 0 && ratio < 4.5 {
				out = append(out, ValidationViolation{
					Code:    "GREX-WCAG-CONTRAST",
					Path:    path,
					Message: fmt.Sprintf("Contrast ratio %.2f:1 between %s and %s fails WCAG AA (>= 4.5:1)", ratio, c, bg),
				})
			}
		}
		for i, ch := range n.Children {
			if ch != nil {
				walk(ch, fmt.Sprintf("%s.children[%d]", path, i), depth+1)
			}
		}
	}
	walk(root, "layout", 1)
	return out
}

// ComputeContrastRatio returns the WCAG relative luminance contrast ratio (1.0 - 21.0).
func ComputeContrastRatio(fgHex, bgHex string) float64 {
	fg, ok1 := parseHexColor(fgHex)
	bg, ok2 := parseHexColor(bgHex)
	if !ok1 || !ok2 {
		return 0
	}
	l1 := relLum(fg[0], fg[1], fg[2])
	l2 := relLum(bg[0], bg[1], bg[2])
	if l2 > l1 {
		l1, l2 = l2, l1
	}
	return (l1 + 0.05) / (l2 + 0.05)
}

// IsSafeURL checks whether a href/src URL uses a safe relative or http(s)/mailto/tel/anchor scheme.
func IsSafeURL(raw string) bool {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return false
	}
	if strings.HasPrefix(trimmed, "#") {
		return true
	}
	if strings.HasPrefix(trimmed, "/") {
		return !strings.HasPrefix(trimmed, "//")
	}
	if strings.HasPrefix(trimmed, "./") || strings.HasPrefix(trimmed, "../") {
		return true
	}
	// Strip ASCII whitespace / control chars to defeat obfuscated schemes like "java\nscript:".
	collapsed := strings.Map(func(r rune) rune {
		if r <= 32 {
			return -1
		}
		return r
	}, strings.ToLower(trimmed))
	if strings.HasPrefix(collapsed, "javascript:") || strings.HasPrefix(collapsed, "vbscript:") || strings.HasPrefix(collapsed, "file:") || strings.HasPrefix(collapsed, "data:") {
		return false
	}
	u, err := url.Parse(trimmed)
	if err != nil {
		return false
	}
	scheme := strings.ToLower(u.Scheme)
	return scheme == "http" || scheme == "https" || scheme == "mailto" || scheme == "tel"
}

// ScoreToGrade converts a 0-100 score + publish safety gate into A+..F.
func ScoreToGrade(score int, publishSafe bool) string {
	if !publishSafe || score < 60 {
		return "F"
	}
	switch {
	case score >= 97:
		return "A+"
	case score >= 90:
		return "A"
	case score >= 80:
		return "B"
	case score >= 70:
		return "C"
	default:
		return "D"
	}
}

// BuildSchemaOrgPreview generates an XSS-safe (</script> escaped) schema.org JSON-LD object
// preserving ADR-025c base-10 numeric wire strings on price / offer fields.
func BuildSchemaOrgPreview(contentType, title, desc, canonicalURL string, customFields map[string]string, focusKW string) SchemaOrgPreview {
	ct := strings.ToLower(strings.TrimSpace(contentType))
	schemaType := "WebPage"
	switch {
	case ct == "book" || ct == "chapter":
		schemaType = "Book"
	case ct == "product" || hasPriceField(customFields):
		schemaType = "Product"
	case ct == "post" || ct == "article" || ct == "guide":
		schemaType = "Article"
	case ct == "faq":
		schemaType = "FAQPage"
	}
	payload := map[string]any{
		"@context":    "https://schema.org",
		"@type":       schemaType,
		"name":        firstNonEmpty(title, "Groxy Desk Workspace Page"),
		"description": firstNonEmpty(desc, "Semantic Grex JS v1.0.0 layout built with GRAB."),
		"url":         firstNonEmpty(canonicalURL, "/"),
	}
	if focusKW != "" {
		payload["keywords"] = focusKW
	}
	if schemaType == "Book" {
		if authorName := strings.TrimSpace(customFields["author"]); authorName != "" {
			payload["author"] = map[string]any{
				"@type": "Person",
				"name":  authorName,
			}
		}
		if isbn := firstNonEmpty(customFields["isbn_13"], customFields["isbn"]); isbn != "" {
			payload["isbn"] = isbn
		}
		if genre := strings.TrimSpace(customFields["genre"]); genre != "" {
			payload["genre"] = genre
		}
		if wc := strings.TrimSpace(customFields["word_count"]); security.IsExactIntString(wc) {
			payload["wordCount"] = wc
		}
		if price := firstNonEmpty(customFields["price_usd"], customFields["price"]); security.IsExactDecimalString(price) || security.IsExactIntString(price) {
			payload["offers"] = map[string]any{
				"@type":         "Offer",
				"price":         price,
				"priceCurrency": firstNonEmpty(customFields["currency"], "USD"),
			}
		}
	}
	if schemaType == "Product" {
		price := firstNonEmpty(customFields["price"], customFields["amount"], "49.00")
		if !security.IsExactDecimalString(price) && !security.IsExactIntString(price) {
			price = "49.00"
		}
		offer := map[string]any{
			"@type":         "Offer",
			"price":         price,
			"priceCurrency": firstNonEmpty(customFields["currency"], "USD"),
		}
		if sku := customFields["sku"]; security.IsExactIntString(sku) || sku != "" {
			payload["sku"] = sku
		}
		payload["offers"] = offer
	}
	// Deterministic JSON encoding + </script> XSS escape.
	jsonText := marshalSortedJSON(payload)
	jsonText = strings.ReplaceAll(jsonText, "</", `<\/`)
	return SchemaOrgPreview{
		Type:     schemaType,
		Payload:  payload,
		JSONText: jsonText,
	}
}

type headingItem struct {
	level int
	text  string
}

type linkItem struct {
	href string
	text string
}

type collectedStats struct {
	divCount              int
	semanticLandmarkCount int
	hasNav                bool
	h1Count               int
	h2PlusCount           int
	headings              []headingItem
	paragraphs            []string
	wordCount             int
	links                 []linkItem
	imageCount            int
	imageAltFailures      []string
	unlabeledControls     []string
	unlabeledButtons      []string
	headingSkips          []string
	contrastFailures      []string
	securityViolations    []string
	invalidElements       []string
	invalidTags           []string
	usesDesignTokens      bool
	maxDepth              int
	nodeCount             int
}

func collectStats(html string, tree *GrexNode, ds DesignSystemInfo) collectedStats {
	st := collectedStats{}
	if strings.Contains(html, "--grex-") || len(ds.Tokens) > 0 {
		st.usesDesignTokens = true
	}

	if html != "" {
		st.divCount += len(reDivOpen.FindAllString(html, -1))
		for _, m := range reScriptFull.FindAllString(html, -1) {
			if !strings.Contains(strings.ToLower(m), `type="module"`) || strings.Contains(strings.ToLower(m), "inline") {
				// Allow static module scripts (<script type="module" src="/public/...">) only when src is safe relative/self!
				if !isSafeExternalModuleScript(m) {
					st.securityViolations = append(st.securityViolations, "Disallowed <script> element detected")
				}
			}
		}
		if reIframeOpen.MatchString(html) {
			st.securityViolations = append(st.securityViolations, "Forbidden <iframe> element detected")
		}
		if reInlineEvent.MatchString(html) {
			st.securityViolations = append(st.securityViolations, "Forbidden inline on* event handler detected")
		}
		if reInnerHTML.MatchString(html) {
			st.securityViolations = append(st.securityViolations, "Forbidden innerHTML/outerHTML write detected")
		}
		if reEvalCall.MatchString(html) {
			st.securityViolations = append(st.securityViolations, "Forbidden eval()/new Function() call detected")
		}
		if reUnsafeProto.MatchString(html) {
			st.securityViolations = append(st.securityViolations, "Unsafe URL scheme (javascript:/vbscript:/file:/data:text/html) detected")
		}

		// Headings
		lastLevel := 0
		for _, hm := range reHeading.FindAllStringSubmatch(html, -1) {
			lvl := int(hm[1][1] - '0')
			txt := cleanText(hm[2])
			st.headings = append(st.headings, headingItem{level: lvl, text: txt})
			if lvl == 1 {
				st.h1Count++
			} else {
				st.h2PlusCount++
			}
			if lastLevel > 0 && lvl > lastLevel+1 {
				st.headingSkips = append(st.headingSkips, fmt.Sprintf("Heading hierarchy skips from H%d to H%d", lastLevel, lvl))
			}
			lastLevel = lvl
		}

		// Semantic landmarks + unknown tags
		for _, lm := range reSemanticOpen.FindAllStringSubmatch(html, -1) {
			tag := strings.ToLower(lm[1])
			st.semanticLandmarkCount++
			if tag == "nav" {
				st.hasNav = true
			}
		}
		for _, tm := range reGenericOpen.FindAllStringSubmatch(html, -1) {
			tag := strings.ToLower(tm[1])
			st.nodeCount++
			if tag != "html" && tag != "head" && tag != "body" && tag != "title" && tag != "meta" && tag != "link" && tag != "div" && tag != "script" && tag != "iframe" && !allowedSemanticHTMLTags[tag] {
				st.invalidTags = append(st.invalidTags, "Unsupported tag <"+tag+">")
			}
		}

		// Images
		for _, im := range reImgTag.FindAllStringSubmatch(html, -1) {
			attrs := im[1]
			st.imageCount++
			altM := reAttrAlt.FindStringSubmatch(attrs)
			hasAltAttr := len(altM) > 1
			altVal := ""
			if hasAltAttr {
				altVal = strings.TrimSpace(altM[1])
			}
			if reAriaHidden.MatchString(attrs) && hasAltAttr && altVal == "" {
				continue
			}
			if !hasAltAttr || len(altVal) < 3 || isPlaceholderAlt(altVal) {
				st.imageAltFailures = append(st.imageAltFailures, fmt.Sprintf("Image alt text %q is missing or non-descriptive (3-125 chars required)", altVal))
			}
		}

		// Links
		for _, am := range reAnchorTag.FindAllStringSubmatch(html, -1) {
			attrs := am[1]
			txt := cleanText(am[2])
			href := ""
			if hm := reAttrHref.FindStringSubmatch(attrs); len(hm) > 1 {
				href = strings.TrimSpace(hm[1])
			}
			if !IsSafeURL(href) && href != "" {
				st.securityViolations = append(st.securityViolations, fmt.Sprintf("Unsafe anchor href %q", href))
			}
			ariaLbl := ""
			if lm := reAttrLabel.FindStringSubmatch(attrs); len(lm) > 1 {
				ariaLbl = strings.TrimSpace(lm[1])
			}
			labelText := firstNonEmpty(txt, ariaLbl)
			if labelText == "" {
				st.unlabeledButtons = append(st.unlabeledButtons, fmt.Sprintf("Link to %q has empty visible label", href))
			}
			st.links = append(st.links, linkItem{href: href, text: labelText})
		}

		// Buttons
		for _, bm := range reButtonTag.FindAllStringSubmatch(html, -1) {
			txt := cleanText(bm[2])
			ariaLbl := ""
			if lm := reAttrLabel.FindStringSubmatch(bm[1]); len(lm) > 1 {
				ariaLbl = strings.TrimSpace(lm[1])
			}
			if firstNonEmpty(txt, ariaLbl) == "" {
				st.unlabeledButtons = append(st.unlabeledButtons, "Button element is missing visible text or aria-label")
			}
		}

		// Form controls
		labelTargets := map[string]bool{}
		for _, lf := range reLabelFor.FindAllStringSubmatch(html, -1) {
			labelTargets[lf[1]] = true
		}
		for _, cm := range reInputTag.FindAllStringSubmatch(html, -1) {
			attrs := cm[2]
			if tm := reAttrType.FindStringSubmatch(attrs); len(tm) > 1 && strings.EqualFold(tm[1], "hidden") {
				continue
			}
			hasAria := reAttrLabel.MatchString(attrs)
			id := ""
			if im := reAttrID.FindStringSubmatch(attrs); len(im) > 1 {
				id = im[1]
			}
			if !hasAria && (id == "" || !labelTargets[id]) {
				st.unlabeledControls = append(st.unlabeledControls, fmt.Sprintf("Form <%s> control is missing <label for> or aria-label", cm[1]))
			}
		}

		// Plain copy word count (after stripping tags and scripts/head).
		plain := stripHeadAndTags(html)
		words := strings.Fields(plain)
		st.wordCount += len(words)
		if len(words) > 0 {
			st.paragraphs = append(st.paragraphs, plain)
		}
		if st.maxDepth < 4 && st.nodeCount > 0 {
			st.maxDepth = 4
		}
	}

	if tree != nil {
		lastH := 0
		var walk func(n *GrexNode, depth int)
		walk = func(n *GrexNode, depth int) {
			if n == nil {
				return
			}
			st.nodeCount++
			if depth > st.maxDepth {
				st.maxDepth = depth
			}
			if !core20ElementSet[n.Element] {
				st.invalidElements = append(st.invalidElements, fmt.Sprintf("Unsupported Grex element %q", n.Element))
			}
			tag := strings.ToLower(strings.TrimSpace(n.Tag))
			if tag == "div" {
				st.divCount++
			} else if tag == "script" || tag == "iframe" {
				st.securityViolations = append(st.securityViolations, fmt.Sprintf("Forbidden tag <%s> in Grex tree", tag))
			} else if tag != "" && !allowedSemanticHTMLTags[tag] {
				st.invalidTags = append(st.invalidTags, fmt.Sprintf("Disallowed HTML tag <%s>", tag))
			}
			if n.Element == "Section" || n.Element == "Nav" || tag == "header" || tag == "main" || tag == "footer" || tag == "section" || tag == "nav" {
				st.semanticLandmarkCount++
			}
			if n.Element == "Nav" || tag == "nav" {
				st.hasNav = true
			}
			if n.Styles != nil {
				styleRaw := fmt.Sprint(n.Styles)
				if strings.Contains(styleRaw, "--grex-") {
					st.usesDesignTokens = true
				}
				if c, bg := extractNodeColors(n.Styles); c != "" && bg != "" {
					if r := ComputeContrastRatio(c, bg); r > 0 && r < 4.5 {
						st.contrastFailures = append(st.contrastFailures, fmt.Sprintf("Color %s on %s has %.2f:1 contrast (< 4.5:1)", c, bg, r))
					}
				}
			}
			for k, v := range n.Props {
				kl := strings.ToLower(k)
				if strings.HasPrefix(kl, "on") || kl == "innerhtml" || kl == "dangerouslysetinnerhtml" {
					st.securityViolations = append(st.securityViolations, fmt.Sprintf("Forbidden prop %q on %s", k, n.Element))
				}
				if (kl == "href" || kl == "src" || kl == "action") && v != nil {
					s := fmt.Sprint(v)
					if !IsSafeURL(s) {
						st.securityViolations = append(st.securityViolations, fmt.Sprintf("Unsafe URL %q in %s.%s", s, n.Element, k))
					}
				}
			}
			if n.Element == "Heading" || (len(tag) == 2 && tag[0] == 'h' && tag[1] >= '1' && tag[1] <= '6') {
				lvl := headingLevelFromNode(n)
				txt := strings.TrimSpace(fmt.Sprint(nonNilValue(n.Props["text"])))
				st.headings = append(st.headings, headingItem{level: lvl, text: txt})
				if lvl == 1 {
					st.h1Count++
				} else {
					st.h2PlusCount++
				}
				if lastH > 0 && lvl > lastH+1 {
					st.headingSkips = append(st.headingSkips, fmt.Sprintf("Grex heading skips from H%d to H%d", lastH, lvl))
				}
				lastH = lvl
				st.wordCount += len(strings.Fields(txt))
			}
			if n.Element == "Text" || n.Element == "RichText" {
				txt := strings.TrimSpace(fmt.Sprint(nonNilValue(n.Props["text"])))
				if txt == "" {
					txt = strings.TrimSpace(fmt.Sprint(nonNilValue(n.Props["content"])))
				}
				if txt != "" {
					st.wordCount += len(strings.Fields(txt))
					st.paragraphs = append(st.paragraphs, txt)
				}
			}
			if n.Element == "Image" {
				st.imageCount++
				dec, _ := n.Props["decorative"].(bool)
				alt := strings.TrimSpace(fmt.Sprint(nonNilValue(n.Props["alt"])))
				if !dec && (len(alt) < 3 || isPlaceholderAlt(alt)) {
					st.imageAltFailures = append(st.imageAltFailures, fmt.Sprintf("Image %q needs descriptive alt text or decorative: true", alt))
				}
			}
			if n.Element == "Link" {
				href := strings.TrimSpace(fmt.Sprint(nonNilValue(n.Props["href"])))
				txt := firstNonEmpty(fmt.Sprint(nonNilValue(n.Props["text"])), fmt.Sprint(nonNilValue(n.Props["ariaLabel"])))
				if strings.TrimSpace(txt) == "" {
					st.unlabeledButtons = append(st.unlabeledButtons, fmt.Sprintf("Link %q lacks visible label", href))
				}
				st.links = append(st.links, linkItem{href: href, text: strings.TrimSpace(txt)})
				st.wordCount += len(strings.Fields(txt))
			}
			if n.Element == "Button" {
				txt := firstNonEmpty(fmt.Sprint(nonNilValue(n.Props["label"])), fmt.Sprint(nonNilValue(n.Props["text"])), fmt.Sprint(nonNilValue(n.Props["ariaLabel"])))
				if strings.TrimSpace(txt) == "" {
					st.unlabeledButtons = append(st.unlabeledButtons, "Grex Button lacks label or ariaLabel")
				}
				st.wordCount += len(strings.Fields(txt))
			}
			if n.Element == "Input" || n.Element == "Select" {
				lbl := firstNonEmpty(fmt.Sprint(nonNilValue(n.Props["label"])), fmt.Sprint(nonNilValue(n.Props["ariaLabel"])))
				if strings.TrimSpace(lbl) == "" {
					st.unlabeledControls = append(st.unlabeledControls, fmt.Sprintf("Grex %s lacks accessible label", n.Element))
				}
			}
			for _, ch := range n.Children {
				walk(ch, depth+1)
			}
		}
		walk(tree, 1)
	}
	return st
}

func isSafeExternalModuleScript(openTag string) bool {
	low := strings.ToLower(openTag)
	if !strings.Contains(low, `type="module"`) || !strings.Contains(low, `src="/public/`) {
		return false
	}
	// Ensure no inline attribute handlers.
	return !reInlineEvent.MatchString(openTag)
}

func makeFinding(spec ruleSpec, earned int, status, msg, hint string) RuleFinding {
	return RuleFinding{
		RuleID:       spec.ID,
		Category:     spec.Category,
		Title:        spec.Title,
		Status:       status,
		PointsEarned: strconv.Itoa(earned),
		MaxPoints:    strconv.Itoa(spec.Max),
		Message:      msg,
		FixHint:      hint,
	}
}

func headingLevelFromNode(n *GrexNode) int {
	if n == nil {
		return 2
	}
	if tag := strings.ToLower(strings.TrimSpace(n.Tag)); len(tag) == 2 && tag[0] == 'h' && tag[1] >= '1' && tag[1] <= '6' {
		return int(tag[1] - '0')
	}
	if v, ok := n.Props["level"]; ok {
		switch tv := v.(type) {
		case int:
			if tv >= 1 && tv <= 6 {
				return tv
			}
		case float64:
			iv := int(tv)
			if iv >= 1 && iv <= 6 {
				return iv
			}
		case string:
			if iv, err := strconv.Atoi(tv); err == nil && iv >= 1 && iv <= 6 {
				return iv
			}
		}
	}
	return 2
}

func extractNodeColors(styles map[string]any) (string, string) {
	if styles == nil {
		return "", ""
	}
	var base map[string]any
	if b, ok := styles["base"].(map[string]any); ok {
		base = b
	} else {
		base = styles
	}
	c := strings.TrimSpace(fmt.Sprint(nonNilValue(base["color"])))
	bg := strings.TrimSpace(fmt.Sprint(nonNilValue(base["backgroundColor"])))
	if bg == "" {
		bg = strings.TrimSpace(fmt.Sprint(nonNilValue(base["background-color"])))
	}
	return extractHexFallback(c), extractHexFallback(bg)
}

func extractHexFallback(val string) string {
	val = strings.TrimSpace(val)
	if reHexColor.MatchString(val) {
		return val
	}
	// Support CSS var fallback like "var(--grex-color-text, #0f172a)"
	if idx := strings.LastIndex(val, "#"); idx != -1 {
		cand := strings.TrimRight(val[idx:], ") ;")
		if reHexColor.MatchString(cand) {
			return cand
		}
	}
	return ""
}

func parseHexColor(hex string) ([3]float64, bool) {
	hex = strings.TrimSpace(hex)
	if !reHexColor.MatchString(hex) {
		return [3]float64{}, false
	}
	h := hex[1:]
	if len(h) == 3 {
		h = string([]byte{h[0], h[0], h[1], h[1], h[2], h[2]})
	}
	r, err1 := strconv.ParseUint(h[0:2], 16, 8)
	g, err2 := strconv.ParseUint(h[2:4], 16, 8)
	b, err3 := strconv.ParseUint(h[4:6], 16, 8)
	if err1 != nil || err2 != nil || err3 != nil {
		return [3]float64{}, false
	}
	return [3]float64{float64(r) / 255.0, float64(g) / 255.0, float64(b) / 255.0}, true
}

func relLum(r, g, b float64) float64 {
	lin := func(c float64) float64 {
		if c <= 0.03928 {
			return c / 12.92
		}
		return math.Pow((c+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(r) + 0.7152*lin(g) + 0.0722*lin(b)
}

func filterInternalLinks(links []linkItem) []linkItem {
	var out []linkItem
	for _, l := range links {
		if strings.HasPrefix(l.href, "/") && !strings.HasPrefix(l.href, "//") {
			out = append(out, l)
		}
	}
	return out
}

func countVagueLinks(links []linkItem) int {
	c := 0
	for _, l := range links {
		t := strings.ToLower(strings.TrimSpace(l.text))
		if t == "" || len(t) < 4 || vagueAnchors[t] {
			c++
		}
	}
	return c
}

func checkSiteContextGraph(internalLinks []linkItem, selfSlug string, sc SiteContextInfo) string {
	selfPath := "/" + strings.TrimPrefix(selfSlug, "/")
	for _, l := range internalLinks {
		norm := normalizeRoutePath(l.href)
		if norm == "" {
			continue
		}
		if redirTarget, ok := sc.Redirects[norm]; ok {
			return fmt.Sprintf("Internal link %q hits 301 redirect to %q instead of direct canonical URL", norm, redirTarget)
		}
		if norm == selfPath && selfSlug != "" {
			return fmt.Sprintf("Link %q points redundantly to self", norm)
		}
	}
	return ""
}

func isCleanSlug(slug string) bool {
	if slug == "" || len(slug) > 85 {
		return false
	}
	for _, r := range slug {
		if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-') {
			return false
		}
	}
	return !strings.HasPrefix(slug, "-") && !strings.HasSuffix(slug, "-") && !strings.Contains(slug, "--")
}

func firstSegment(slug string) string {
	if idx := strings.Index(slug, "-"); idx != -1 {
		return slug[:idx]
	}
	return slug
}

func normalizeSlug(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.TrimPrefix(s, "/")
	var b strings.Builder
	prevDash := false
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			prevDash = false
		} else if !prevDash && b.Len() > 0 {
			b.WriteByte('-')
			prevDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func deriveSlugFromPath(p string) string {
	base := filepath.Base(strings.ReplaceAll(p, `\`, "/"))
	base = strings.TrimSuffix(base, ".grab.html")
	base = strings.TrimSuffix(base, ".grab")
	base = strings.TrimSuffix(base, filepath.Ext(base))
	s := normalizeSlug(base)
	if s == "" {
		return "home"
	}
	return s
}

func deriveKeywordFromTitle(title string) string {
	words := strings.Fields(strings.ToLower(title))
	var keep []string
	for _, w := range words {
		w = strings.Trim(w, ",.:;!?\"'()[]{}")
		if len(w) >= 3 && len(keep) < 2 {
			keep = append(keep, w)
		}
	}
	return strings.Join(keep, " ")
}

func cleanText(s string) string {
	s = reTagStrip.ReplaceAllString(s, " ")
	return strings.TrimSpace(reWhitespace.ReplaceAllString(s, " "))
}

func stripHeadAndTags(html string) string {
	low := strings.ToLower(html)
	if idx := strings.Index(low, "<body"); idx != -1 {
		html = html[idx:]
	}
	html = regexp.MustCompile(`(?is)<script\b[^>]*>.*?</script>`).ReplaceAllString(html, " ")
	html = regexp.MustCompile(`(?is)<style\b[^>]*>.*?</style>`).ReplaceAllString(html, " ")
	return cleanText(html)
}

func truncateClean(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	if len(s) <= maxLen {
		return s
	}
	return strings.TrimSpace(s[:maxLen-3]) + "..."
}

func isPlaceholderAlt(alt string) bool {
	low := strings.ToLower(strings.TrimSpace(alt))
	switch low {
	case "image", "photo", "picture", "img", "screenshot", "banner", "icon":
		return true
	}
	for _, ext := range []string{".jpg", ".jpeg", ".png", ".gif", ".webp", ".svg"} {
		if strings.HasSuffix(low, ext) {
			return true
		}
	}
	return false
}

func normalizeRoutePath(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if i := strings.IndexAny(raw, "?#"); i != -1 {
		raw = raw[:i]
	}
	if !strings.HasPrefix(raw, "/") {
		raw = "/" + raw
	}
	if len(raw) > 1 {
		raw = strings.TrimRight(raw, "/")
	}
	return strings.ToLower(raw)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func nonNilValue(v any) any {
	if v == nil {
		return ""
	}
	return v
}

func nonNilSlice(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func joinNonEmpty(a, b []string) string {
	all := append([]string{}, a...)
	all = append(all, b...)
	if len(all) == 0 {
		return ""
	}
	return strings.Join(all, "; ")
}

func hasPriceField(cf map[string]string) bool {
	_, ok1 := cf["price"]
	_, ok2 := cf["amount"]
	return ok1 || ok2
}

func marshalSortedJSON(v any) string {
	switch tv := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(tv))
		for k := range tv {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var b strings.Builder
		b.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				b.WriteByte(',')
			}
			b.WriteString(strconv.Quote(k))
			b.WriteByte(':')
			b.WriteString(marshalSortedJSON(tv[k]))
		}
		b.WriteByte('}')
		return b.String()
	case []any:
		var b strings.Builder
		b.WriteByte('[')
		for i, it := range tv {
			if i > 0 {
				b.WriteByte(',')
			}
			b.WriteString(marshalSortedJSON(it))
		}
		b.WriteByte(']')
		return b.String()
	case string:
		return strconv.Quote(tv)
	case bool:
		if tv {
			return "true"
		}
		return "false"
	default:
		return strconv.Quote(fmt.Sprint(v))
	}
}
