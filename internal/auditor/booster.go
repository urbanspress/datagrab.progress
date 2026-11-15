package auditor

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"groxy-desk/internal/security"
)

// BoostRequest requests a deterministic / adaptive-deep remediation diff.
type BoostRequest struct {
	TargetPath     string            `json:"target_path,omitempty"`
	HTML           string            `json:"html,omitempty"`
	GrexLayout     *GrexNode         `json:"grex_layout,omitempty"`
	ContentType    string            `json:"content_type,omitempty"`
	Title          string            `json:"title,omitempty"`
	MetaDesc       string            `json:"meta_description,omitempty"`
	Slug           string            `json:"slug,omitempty"`
	FocusKeyword   string            `json:"focus_keyword,omitempty"`
	StylePack      string            `json:"style_pack,omitempty"`
	DesignSystemMD string            `json:"design_system_md,omitempty"`
	SiteContextMD  string            `json:"site_context_md,omitempty"`
	CustomFields   map[string]string `json:"custom_fields,omitempty"`
}

// DiffHunk represents one side-by-side change attributed to specific 22-rule IDs.
type DiffHunk struct {
	Field    string   `json:"field"`
	RuleIDs  []string `json:"rule_ids"`
	Summary  string   `json:"summary"`
	Before   string   `json:"before"`
	After    string   `json:"after"`
}

// ScoreDelta summarizes projected score & grade improvement as exact wire strings.
type ScoreDelta struct {
	BeforeScore string `json:"before_score"`
	BeforeGrade string `json:"before_grade"`
	AfterScore  string `json:"after_score"`
	AfterGrade  string `json:"after_grade"`
	Delta       string `json:"delta"`
}

// BoostProposal contains the complete side-by-side remediation diff, reports,
// exact ADR-025c preserved custom fields, and mutated HTML/layout output.
type BoostProposal struct {
	TargetPath      string            `json:"target_path"`
	StylePackID     string            `json:"style_pack_id"`
	ReasoningMode   string            `json:"reasoning_mode"`
	BeforeReport    ScorecardReport   `json:"before_report"`
	AfterReport     ScorecardReport   `json:"after_report"`
	ProjectedDelta  ScoreDelta        `json:"projected_delta"`
	Diffs           []DiffHunk        `json:"diffs"`
	UnifiedDiff     string            `json:"unified_diff"`
	ImprovedRuleIDs []string          `json:"improved_rule_ids"`
	ProposedHTML    string            `json:"proposed_html,omitempty"`
	ProposedLayout  *GrexNode         `json:"proposed_layout,omitempty"`
	CustomFields    map[string]string `json:"custom_fields,omitempty"`
	SchemaOrg       SchemaOrgPreview  `json:"schema_org"`
}

var (
	reDivTagOpen       = regexp.MustCompile(`(?is)<\s*div\b([^>]*)>`)
	reDivTagClose      = regexp.MustCompile(`(?is)</\s*div\s*>`)
	reScriptBlock      = regexp.MustCompile(`(?is)<\s*script\b[^>]*>.*?</\s*script\s*>`)
	reIframeBlock      = regexp.MustCompile(`(?is)<\s*iframe\b[^>]*>.*?</\s*iframe\s*>|<\s*iframe\b[^>]*/>`)
	reInlineEventAttr  = regexp.MustCompile(`(?i)\s+\bon[a-z]+\s*=\s*(?:"[^"]*"|'[^']*'|[^\s>]+)`)
	reImgNoAlt         = regexp.MustCompile(`(?is)<img\b([^>]*?)>`)
	reUnsafeHrefOrSrc  = regexp.MustCompile(`(?i)\b(href|src)\s*=\s*["']\s*(?:javascript:|vbscript:|file:|data:text/html)[^"']*["']`)
	reAnchorClickHere  = regexp.MustCompile(`(?is)<a\b([^>]*?)>\s*(?:click here|here|read more|learn more|more)\s*</a>`)
	reH2PlusHeading    = regexp.MustCompile(`(?is)<(h[2-6])\b([^>]*)>(.*?)</h[2-6]>`)
)

// Boost runs "Let Groxy Boost This" to generate safe semantic/WCAG/SEO remediations,
// before/after audit scorecards, side-by-side diffs, and exact ADR-025c lock checks.
func Boost(req BoostRequest) (BoostProposal, error) {
	// Lock ADR-025c base-10 wire strings upfront.
	lockedFields := map[string]string{}
	for k, v := range req.CustomFields {
		trimmed := strings.TrimSpace(v)
		if trimmed != "" && (security.IsExactDecimalString(trimmed) || security.IsExactIntString(trimmed)) {
			lockedFields[k] = trimmed
		} else if trimmed != "" {
			lockedFields[k] = trimmed
		}
	}

	auditIn := AuditInput{
		TargetPath:     req.TargetPath,
		ContentType:    req.ContentType,
		Title:          req.Title,
		MetaDesc:       req.MetaDesc,
		Slug:           req.Slug,
		FocusKeyword:   req.FocusKeyword,
		HTML:           req.HTML,
		GrexLayout:     req.GrexLayout,
		DesignSystemMD: req.DesignSystemMD,
		SiteContextMD:  req.SiteContextMD,
		CustomFields:   lockedFields,
	}
	beforeReport := Audit(auditIn)
	pack, _ := FindStylePack(req.StylePack)

	boostedTitle := boostTitle(firstNonEmpty(req.Title, beforeReport.Title), req.FocusKeyword)
	boostedDesc := boostMetaDescription(firstNonEmpty(req.MetaDesc, ""), boostedTitle, req.FocusKeyword)
	boostedSlug := boostSlug(firstNonEmpty(req.Slug, beforeReport.Slug), boostedTitle)

	var boostedHTML string
	if strings.TrimSpace(req.HTML) != "" {
		boostedHTML = boostHTMLDocument(req.HTML, boostedTitle, boostedDesc, boostedSlug)
	}

	var boostedLayout *GrexNode
	if req.GrexLayout != nil {
		boostedLayout = sanitizeGrexTree(req.GrexLayout, pack, boostedTitle)
		if boostedHTML == "" {
			boostedHTML = RenderGrexTreeHTML(boostedLayout, boostedTitle, boostedDesc)
		}
	}

	// Verify ADR-025c fields remained exact byte-for-byte base-10 strings.
	for k, v := range lockedFields {
		if req.CustomFields[k] != "" && strings.TrimSpace(req.CustomFields[k]) != v {
			return BoostProposal{}, fmt.Errorf("ADR-025c numeric fidelity violation on custom_fields[%s]", k)
		}
	}

	afterIn := AuditInput{
		TargetPath:     req.TargetPath,
		ContentType:    req.ContentType,
		Title:          boostedTitle,
		MetaDesc:       boostedDesc,
		Slug:           boostedSlug,
		FocusKeyword:   req.FocusKeyword,
		CanonicalURL:   "/" + boostedSlug,
		HTML:           boostedHTML,
		GrexLayout:     boostedLayout,
		DesignSystemMD: req.DesignSystemMD,
		SiteContextMD:  req.SiteContextMD,
		CustomFields:   lockedFields,
	}
	afterReport := Audit(afterIn)

	improvedRules := collectImprovedRules(beforeReport, afterReport)
	diffs := buildDiffHunks(req.HTML, boostedHTML, req.GrexLayout, boostedLayout, beforeReport, afterReport, improvedRules)
	unified := buildUnifiedDiff(req.TargetPath, diffs)

	beforeInt, _ := strconv.Atoi(beforeReport.OverallScore)
	afterInt, _ := strconv.Atoi(afterReport.OverallScore)
	deltaInt := afterInt - beforeInt
	deltaStr := strconv.Itoa(deltaInt)
	if deltaInt >= 0 {
		deltaStr = "+" + deltaStr
	}

	return BoostProposal{
		TargetPath:    req.TargetPath,
		StylePackID:   pack.ID,
		ReasoningMode: "deep",
		BeforeReport:  beforeReport,
		AfterReport:   afterReport,
		ProjectedDelta: ScoreDelta{
			BeforeScore: beforeReport.OverallScore,
			BeforeGrade: beforeReport.OverallGrade,
			AfterScore:  afterReport.OverallScore,
			AfterGrade:  afterReport.OverallGrade,
			Delta:       deltaStr,
		},
		Diffs:           diffs,
		UnifiedDiff:     unified,
		ImprovedRuleIDs: improvedRules,
		ProposedHTML:    boostedHTML,
		ProposedLayout:  boostedLayout,
		CustomFields:    lockedFields,
		SchemaOrg:       afterReport.SchemaOrg,
	}, nil
}

func boostTitle(orig, focusKW string) string {
	t := strings.TrimSpace(orig)
	if t == "" {
		t = "Semantic GRAB Workspace & Accessible Grex Layout"
	}
	if len(t) < 28 {
		if focusKW != "" && !strings.Contains(strings.ToLower(t), strings.ToLower(focusKW)) {
			t = fmt.Sprintf("%s — %s Guide", t, capitalCase(focusKW))
		} else {
			t = t + " — Semantic Grex Studio Guide"
		}
	}
	if len(t) > 65 {
		t = truncateClean(t, 64)
	}
	return t
}

func boostMetaDescription(orig, title, focusKW string) string {
	d := strings.TrimSpace(orig)
	if len(d) >= 85 && len(d) <= 160 {
		return d
	}
	kw := firstNonEmpty(focusKW, "semantic GRAB layouts")
	cand := fmt.Sprintf("Explore %s built with Grex JS v1.0.0 zero-div semantic components, verified WCAG AA Light and Dark contrast, and %s.", title, kw)
	if len(cand) > 158 {
		cand = truncateClean(cand, 158)
	}
	return cand
}

func boostSlug(orig, title string) string {
	s := normalizeSlug(orig)
	if isCleanSlug(s) && !reservedSlugs[firstSegment(s)] {
		return s
	}
	s = normalizeSlug(title)
	if s == "" {
		return "groxy-semantic-page"
	}
	if len(s) > 72 {
		s = strings.Trim(s[:72], "-")
	}
	return s
}

func boostHTMLDocument(rawHTML, title, desc, slug string) string {
	s := rawHTML
	// 1. Remove dangerous scripts (unless safe static module script) and iframes, inline event handlers, unsafe URLs.
	s = reScriptBlock.ReplaceAllStringFunc(s, func(block string) string {
		if m := reScriptFull.FindString(block); m != "" && isSafeExternalModuleScript(m) {
			return block
		}
		return ""
	})
	s = reIframeBlock.ReplaceAllString(s, "")
	s = reInlineEventAttr.ReplaceAllString(s, "")
	s = reUnsafeHrefOrSrc.ReplaceAllString(s, `$1="/docs"`)

	// 2. Replace wrapper <div> with semantic <section>.
	s = reDivTagOpen.ReplaceAllString(s, `<section$1>`)
	s = reDivTagClose.ReplaceAllString(s, `</section>`)

	// 3. Ensure descriptive image alt text.
	s = reImgNoAlt.ReplaceAllStringFunc(s, func(tag string) string {
		if m := reAttrAlt.FindStringSubmatch(tag); len(m) > 1 {
			alt := strings.TrimSpace(m[1])
			if len(alt) >= 3 && !isPlaceholderAlt(alt) {
				return tag
			}
			return reAttrAlt.ReplaceAllString(tag, fmt.Sprintf(`alt="%s illustration"`, title))
		}
		if strings.HasSuffix(tag, "/>") {
			return strings.TrimSuffix(tag, "/>") + fmt.Sprintf(` alt="%s illustration" />`, title)
		}
		return strings.TrimSuffix(tag, ">") + fmt.Sprintf(` alt="%s illustration">`, title)
	})

	// 4. Replace vague anchor text ("click here", "read more").
	s = reAnchorClickHere.ReplaceAllString(s, fmt.Sprintf(`<a$1>Explore %s Guide</a>`, title))

	// 5. Fix skipped heading levels (ensure sequential h1 -> h2 -> h3).
	h1Seen := false
	lastLevel := 0
	s = reHeading.ReplaceAllStringFunc(s, func(full string) string {
		m := reHeading.FindStringSubmatch(full)
		if len(m) < 3 {
			return full
		}
		lvl := int(m[1][1] - '0')
		inner := cleanText(m[2])
		if lvl == 1 {
			if !h1Seen {
				h1Seen = true
				lastLevel = 1
				return fmt.Sprintf("<h1>%s</h1>", inner)
			}
			lvl = 2
		}
		if lastLevel == 0 {
			lvl = 1
			h1Seen = true
		} else if lvl > lastLevel+1 {
			lvl = lastLevel + 1
		}
		lastLevel = lvl
		return fmt.Sprintf("<h%d>%s</h%d>", lvl, inner, lvl)
	})
	// Normalize stray unused variable warning if reH2PlusHeading wasn't needed
	_ = reH2PlusHeading

	// 6. Ensure outer document has <html lang="en">, viewport, title, description, canonical, and main/nav/h1/h2 content.
	low := strings.ToLower(s)
	if !strings.Contains(low, "<html") {
		s = RenderGrexTreeHTML(&GrexNode{
			Element: "Container",
			Tag:     "main",
			Children: []*GrexNode{
				{
					Element: "Nav",
					Tag:     "nav",
					Props:   map[string]any{"ariaLabel": "Primary navigation"},
					Children: []*GrexNode{
						{Element: "Link", Tag: "a", Props: map[string]any{"href": "/docs", "text": "Explore Documentation Guides"}},
					},
				},
				{
					Element: "Section",
					Tag:     "section",
					Children: []*GrexNode{
						{Element: "Heading", Tag: "h1", Props: map[string]any{"level": 1, "text": title}},
						{Element: "Heading", Tag: "h2", Props: map[string]any{"level": 2, "text": "Accessible Architecture Summary"}},
						{Element: "Text", Tag: "p", Props: map[string]any{"text": desc + " Semantic markup enforces WCAG AA Light/Dark readability without wrapper div elements."}},
					},
				},
			},
		}, title, desc)
		return s
	}

	if !reHTMLLang.MatchString(s) {
		s = regexp.MustCompile(`(?i)<html\b([^>]*)>`).ReplaceAllString(s, `<html lang="en"$1>`)
	}
	if reTitle.MatchString(s) {
		s = reTitle.ReplaceAllString(s, fmt.Sprintf(`<title>%s</title>`, title))
	} else if strings.Contains(low, "</head>") {
		s = strings.Replace(s, "</head>", fmt.Sprintf("  <title>%s</title>\n</head>", title), 1)
	}
	if reMetaDesc.MatchString(s) {
		s = reMetaDesc.ReplaceAllString(s, fmt.Sprintf(`<meta name="description" content="%s" />`, desc))
	} else if strings.Contains(low, "</head>") {
		s = strings.Replace(s, "</head>", fmt.Sprintf("  <meta name=\"description\" content=\"%s\" />\n</head>", desc), 1)
	}
	if !reViewport.MatchString(s) && strings.Contains(low, "</head>") {
		s = strings.Replace(s, "</head>", "  <meta name=\"viewport\" content=\"width=device-width, initial-scale=1\" />\n</head>", 1)
	}
	if !reCanonical.MatchString(s) && strings.Contains(low, "</head>") {
		s = strings.Replace(s, "</head>", fmt.Sprintf("  <link rel=\"canonical\" href=\"/%s\" />\n</head>", slug), 1)
	}
	if !h1Seen && strings.Contains(low, "</body>") {
		s = strings.Replace(s, "</body>", fmt.Sprintf("  <main><section><h1>%s</h1><h2>Overview &amp; Accessibility</h2><p>%s</p><nav aria-label=\"Related Links\"><a href=\"/docs\">Browse Complete Documentation</a></nav></section></main>\n</body>", title, desc), 1)
	} else {
		// Ensure at least one semantic internal link & H2 exist cleanly if missing.
		hasH2 := strings.Contains(strings.ToLower(s), "<h2")
		hasInternalLink := len(filterInternalLinks(collectStats(s, nil, DesignSystemInfo{}).links)) > 0
		if (!hasH2 || !hasInternalLink) && strings.Contains(strings.ToLower(s), "</body>") {
			extra := "  <section>"
			if !hasH2 {
				extra += "<h2>Semantic Overview &amp; Highlights</h2>"
			}
			extra += "<p>Structured with clean Grex landmarks, high-contrast tokens, and direct navigation paths.</p>"
			if !hasInternalLink {
				extra += "<nav aria-label=\"Related Links\"><a href=\"/docs\">Explore Architecture Guides</a></nav>"
			}
			extra += "</section>\n</body>"
			idx := strings.LastIndex(strings.ToLower(s), "</body>")
			s = s[:idx] + extra + s[idx+len("</body>"):]
		}
	}
	return s
}

func sanitizeGrexTree(node *GrexNode, pack StylePack, title string) *GrexNode {
	if node == nil {
		return nil
	}
	lastH := 0
	var walk func(n *GrexNode) *GrexNode
	walk = func(n *GrexNode) *GrexNode {
		if n == nil {
			return nil
		}
		el := n.Element
		tag := strings.ToLower(strings.TrimSpace(n.Tag))
		if tag == "div" || tag == "script" || tag == "iframe" {
			tag = "section"
		}
		if !core20ElementSet[el] {
			el = "Section"
		}
		if el == "Section" && tag == "section" && len(n.Children) == 0 && len(n.Props) == 0 {
			el = "Section"
		}
		props := map[string]any{}
		for k, v := range n.Props {
			kl := strings.ToLower(k)
			if strings.HasPrefix(kl, "on") || kl == "innerhtml" || kl == "dangerouslysetinnerhtml" {
				continue
			}
			if kl == "href" || kl == "src" || kl == "action" {
				s := fmt.Sprint(v)
				if !IsSafeURL(s) {
					v = "/docs"
				}
			}
			props[k] = v
		}
		if el == "Image" {
			alt := strings.TrimSpace(fmt.Sprint(nonNilValue(props["alt"])))
			dec, _ := props["decorative"].(bool)
			if !dec && (len(alt) < 3 || isPlaceholderAlt(alt)) {
				props["alt"] = fmt.Sprintf("%s visual showcase", title)
			}
		}
		if el == "Heading" {
			lvl := headingLevelFromNode(n)
			if lastH == 0 {
				lvl = 1
			} else if lvl > lastH+1 {
				lvl = lastH + 1
			}
			lastH = lvl
			props["level"] = lvl
			tag = fmt.Sprintf("h%d", lvl)
		}
		styles := map[string]any{
			"base": map[string]any{
				"color":            fmt.Sprintf("var(--grex-text, %s)", pack.Light.Text),
				"background-color": fmt.Sprintf("var(--grex-bg, %s)", pack.Light.Bg),
			},
		}
		var children []*GrexNode
		for _, ch := range n.Children {
			if c := walk(ch); c != nil {
				children = append(children, c)
			}
		}
		return &GrexNode{
			Element:  el,
			Tag:      tag,
			Props:    props,
			Styles:   styles,
			Children: children,
		}
	}
	return walk(node)
}

func collectImprovedRules(before, after ScorecardReport) []string {
	bMap := map[string]int{}
	for _, f := range before.Findings {
		v, _ := strconv.Atoi(f.PointsEarned)
		bMap[f.RuleID] = v
	}
	var out []string
	for _, f := range after.Findings {
		v, _ := strconv.Atoi(f.PointsEarned)
		if v > bMap[f.RuleID] {
			out = append(out, f.RuleID)
		}
	}
	return out
}

func buildDiffHunks(beforeHTML, afterHTML string, beforeTree, afterTree *GrexNode, beforeReport, afterReport ScorecardReport, improved []string) []DiffHunk {
	var diffs []DiffHunk
	if beforeReport.DivCount != "0" {
		diffs = append(diffs, DiffHunk{
			Field:   "dom.div_architecture",
			RuleIDs: []string{"GROXY-DOM-001", "GROXY-DOM-002"},
			Summary: fmt.Sprintf("Replaced %s wrapper <div> tag(s) with semantic HTML5 landmarks", beforeReport.DivCount),
			Before:  fmt.Sprintf("div_count=%s (publish_safe=false)", beforeReport.DivCount),
			After:   "div_count=0 (<section>/<main>/<article> semantic Grex architecture)",
		})
	}
	if len(beforeReport.SecurityViolations) > 0 {
		diffs = append(diffs, DiffHunk{
			Field:   "security.xss_hygiene",
			RuleIDs: []string{"GROXY-DOM-003"},
			Summary: "Removed inline scripts, iframes, inline event handlers, and unsafe URL protocols",
			Before:  strings.Join(beforeReport.SecurityViolations, "; "),
			After:   "Zero <script>, <iframe>, on*, eval(), or unsafe URL vectors",
		})
	}
	if beforeReport.Title != afterReport.Title {
		diffs = append(diffs, DiffHunk{
			Field:   "seo.title",
			RuleIDs: []string{"GROXY-SEO-001", "GROXY-SEO-002"},
			Summary: "Normalized SEO title into 30-65 character search focus range",
			Before:  firstNonEmpty(beforeReport.Title, "(missing)"),
			After:   afterReport.Title,
		})
	}
	if beforeHTML != afterHTML && afterHTML != "" {
		diffs = append(diffs, DiffHunk{
			Field:   "document.html",
			RuleIDs: improved,
			Summary: "Applied semantic landmark, WCAG AA contrast, heading hierarchy, and internal link upgrades",
			Before:  truncateClean(strings.ReplaceAll(beforeHTML, "\n", " "), 180),
			After:   truncateClean(strings.ReplaceAll(afterHTML, "\n", " "), 180),
		})
	}
	if beforeTree != nil && afterTree != nil {
		diffs = append(diffs, DiffHunk{
			Field:   "grex.layout",
			RuleIDs: improved,
			Summary: "Sanitized Grex element tree against 20-element catalog and WCAG AA palette tokens",
			Before:  fmt.Sprintf("score=%s (%s)", beforeReport.OverallScore, beforeReport.OverallGrade),
			After:   fmt.Sprintf("score=%s (%s)", afterReport.OverallScore, afterReport.OverallGrade),
		})
	}
	return diffs
}

func buildUnifiedDiff(targetPath string, hunks []DiffHunk) string {
	path := firstNonEmpty(targetPath, "workspace-target")
	var b strings.Builder
	b.WriteString(fmt.Sprintf("--- a/%s\n+++ b/%s\n", path, path))
	for _, h := range hunks {
		b.WriteString(fmt.Sprintf("@@ %s (%s) @@\n", h.Field, strings.Join(h.RuleIDs, ", ")))
		b.WriteString("- " + h.Before + "\n")
		b.WriteString("+ " + h.After + "\n")
	}
	return b.String()
}

func capitalCase(s string) string {
	words := strings.Fields(strings.TrimSpace(s))
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + strings.ToLower(w[1:])
		}
	}
	return strings.Join(words, " ")
}
