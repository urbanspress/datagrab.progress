package auditor

import (
	"fmt"
	"html"
	"regexp"
	"strings"

	"groxy-desk/internal/security"
)

// PalettePair stores WCAG AA verified colors for Light or Dark mode.
type PalettePair struct {
	Bg        string `json:"bg"`
	Surface   string `json:"surface"`
	Text      string `json:"text"`
	Primary   string `json:"primary"`
	OnPrimary string `json:"on_primary"`
	Border    string `json:"border"`
	Muted     string `json:"muted"`
}

// StylePack defines one of the 8 frozen Grex Theme Builder / Groxy AI packs.
type StylePack struct {
	ID          string      `json:"id"`
	Label       string      `json:"label"`
	Description string      `json:"description"`
	ClassHints  []string    `json:"class_hints"`
	Light       PalettePair `json:"light"`
	Dark        PalettePair `json:"dark"`
}

// Blueprint describes one of the 5 layout composition blueprints.
type Blueprint struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	DefaultSlot string `json:"default_slot"`
	Description string `json:"description"`
}

// DesignSystemInfo summarizes tokens and classes extracted from GRAB-DESIGN-SYSTEM.md.
type DesignSystemInfo struct {
	RawMarkdown string            `json:"raw_markdown,omitempty"`
	Tokens      map[string]string `json:"tokens"`
	Classes     []string          `json:"classes"`
}

// SiteContextInfo holds parsed entries, redirects, and routes from SITE-CONTEXT.md.
type SiteContextInfo struct {
	KnownPaths []string          `json:"known_paths"`
	Redirects  map[string]string `json:"redirects"`
}

// StudioCatalogResponse returns design system info, 8 style packs, 5 blueprints, and 20 elements.
type StudioCatalogResponse struct {
	StylePacks       []StylePack      `json:"style_packs"`
	Blueprints       []Blueprint      `json:"blueprints"`
	Elements         []string         `json:"elements"`
	DesignSystem     DesignSystemInfo `json:"design_system"`
	SiteContext      SiteContextInfo  `json:"site_context"`
	CombinationCount string           `json:"combination_count"`
}

// SynthesizeRequest configures a Style Pack x Blueprint synthesis run.
type SynthesizeRequest struct {
	StylePack      string            `json:"style_pack"`
	Blueprint      string            `json:"blueprint"`
	Topic          string            `json:"topic,omitempty"`
	Title          string            `json:"title,omitempty"`
	Slot           string            `json:"slot,omitempty"`
	FocusKeyword   string            `json:"focus_keyword,omitempty"`
	DesignSystemMD string            `json:"design_system_md,omitempty"`
	SiteContextMD  string            `json:"site_context_md,omitempty"`
	CustomFields   map[string]string `json:"custom_fields,omitempty"`
}

// ContrastSummary reports measured WCAG AA ratios in Light and Dark mode.
type ContrastSummary struct {
	LightTextBg      string `json:"light_text_bg"`
	LightPrimaryBtn  string `json:"light_primary_btn"`
	DarkTextBg       string `json:"dark_text_bg"`
	DarkPrimaryBtn   string `json:"dark_primary_btn"`
	MeetsWCAGAA      bool   `json:"meets_wcag_aa"`
	ContrastThreshold string `json:"contrast_threshold"`
}

// SynthesizeResponse contains the validated zero-<div> Grex tree, rendered HTML preview, and Scorecard.
type SynthesizeResponse struct {
	StylePack    StylePack       `json:"style_pack"`
	Blueprint    Blueprint       `json:"blueprint"`
	Slot         string          `json:"slot"`
	Actor        string          `json:"actor"`
	GrexLayout   *GrexNode       `json:"grex_layout"`
	PreviewHTML  string          `json:"preview_html"`
	Contrast     ContrastSummary `json:"contrast"`
	Scorecard    ScorecardReport `json:"scorecard"`
	CustomFields map[string]string `json:"custom_fields,omitempty"`
}

// FrozenStylePacks holds all 8 curated WCAG AA verified Design Style Packs.
var FrozenStylePacks = []StylePack{
	{
		ID:          "emerald-editorial",
		Label:       "Emerald Editorial",
		Description: "Editorial hierarchy with emerald accents (--grex-primary) and calm reading rhythm.",
		ClassHints:  []string{"hero", "hero-card", "card", "btn-primary", "badge-pill", "grid-3"},
		Light:       PalettePair{Bg: "#ffffff", Surface: "#f8fafc", Text: "#0f172a", Primary: "#047857", OnPrimary: "#ffffff", Border: "#cbd5e1", Muted: "#334155"},
		Dark:        PalettePair{Bg: "#0f172a", Surface: "#1e293b", Text: "#f8fafc", Primary: "#34d399", OnPrimary: "#064e3b", Border: "#334155", Muted: "#cbd5e1"},
	},
	{
		ID:          "obsidian-tech",
		Label:       "Obsidian Developer Studio",
		Description: "High-contrast technical hierarchy with crisp cobalt/blue elevation cards and Grids.",
		ClassHints:  []string{"hero", "hero-card", "card", "btn-primary", "badge-pill", "grid-3"},
		Light:       PalettePair{Bg: "#f8fafc", Surface: "#ffffff", Text: "#09090b", Primary: "#1d4ed8", OnPrimary: "#ffffff", Border: "#cbd5e1", Muted: "#334155"},
		Dark:        PalettePair{Bg: "#09090b", Surface: "#18181b", Text: "#fafafa", Primary: "#60a5fa", OnPrimary: "#09090b", Border: "#27272a", Muted: "#d4d4d8"},
	},
	{
		ID:          "warm-commerce",
		Label:       "Warm Precision Commerce",
		Description: "Conversion-focused product hierarchy preserving exact ADR-025c base-10 GrabDecimal pricing.",
		ClassHints:  []string{"hero", "card", "btn-primary", "badge-pill", "grid-3", "cta-button"},
		Light:       PalettePair{Bg: "#fffbeb", Surface: "#ffffff", Text: "#1c1917", Primary: "#92400e", OnPrimary: "#ffffff", Border: "#fde68a", Muted: "#44403c"},
		Dark:        PalettePair{Bg: "#1c1917", Surface: "#292524", Text: "#fafaf9", Primary: "#fbbf24", OnPrimary: "#1c1917", Border: "#44403c", Muted: "#d6d3d1"},
	},
	{
		ID:          "swiss-minimal",
		Label:       "Swiss Grid Minimal",
		Description: "Geometric grid proportions with strong H1-H3 scale and crisp high-contrast borders.",
		ClassHints:  []string{"hero", "card", "btn-primary", "grid-3"},
		Light:       PalettePair{Bg: "#ffffff", Surface: "#f4f4f5", Text: "#18181b", Primary: "#b91c1c", OnPrimary: "#ffffff", Border: "#d4d4d8", Muted: "#3f3f46"},
		Dark:        PalettePair{Bg: "#18181b", Surface: "#27272a", Text: "#fafafa", Primary: "#f87171", OnPrimary: "#18181b", Border: "#3f3f46", Muted: "#d4d4d8"},
	},
	{
		ID:          "local-service-trust",
		Label:       "Local Service Trust",
		Description: "Authority-driven trust sections with clear booking CTAs and slate/cobalt AA contrast.",
		ClassHints:  []string{"hero", "hero-card", "card", "btn-primary", "badge-pill", "cta-button"},
		Light:       PalettePair{Bg: "#f8fafc", Surface: "#ffffff", Text: "#0f172a", Primary: "#0369a1", OnPrimary: "#ffffff", Border: "#cbd5e1", Muted: "#334155"},
		Dark:        PalettePair{Bg: "#0f172a", Surface: "#1e293b", Text: "#f8fafc", Primary: "#38bdf8", OnPrimary: "#0f172a", Border: "#334155", Muted: "#cbd5e1"},
	},
	{
		ID:          "creative-studio-bold",
		Label:       "Creative Studio Bold",
		Description: "Expressive portfolio Grid rhythm with crisp violet primary highlights.",
		ClassHints:  []string{"hero", "hero-card", "card", "btn-primary", "badge-pill", "grid-3"},
		Light:       PalettePair{Bg: "#fafafa", Surface: "#ffffff", Text: "#18181b", Primary: "#6d28d9", OnPrimary: "#ffffff", Border: "#d4d4d8", Muted: "#3f3f46"},
		Dark:        PalettePair{Bg: "#121216", Surface: "#1e1e26", Text: "#fafafa", Primary: "#c084fc", OnPrimary: "#121216", Border: "#2e2e38", Muted: "#d4d4d8"},
	},
	{
		ID:          "creator-academy",
		Label:       "Creator Academy",
		Description: "Structured syllabus & cohort card flow with teal highlights and Accordion Q&A.",
		ClassHints:  []string{"hero", "hero-card", "card", "btn-primary", "grid-3"},
		Light:       PalettePair{Bg: "#ffffff", Surface: "#f8fafc", Text: "#1e293b", Primary: "#0f766e", OnPrimary: "#ffffff", Border: "#cbd5e1", Muted: "#334155"},
		Dark:        PalettePair{Bg: "#0f172a", Surface: "#1e293b", Text: "#f1f5f9", Primary: "#2dd4bf", OnPrimary: "#0f172a", Border: "#334155", Muted: "#cbd5e1"},
	},
	{
		ID:          "wellness-organic",
		Label:       "Wellness Organic",
		Description: "Calm botanical palette with forest/mint contrast pairs and spacious rhythm.",
		ClassHints:  []string{"hero", "hero-card", "card", "btn-primary", "badge-pill", "grid-3"},
		Light:       PalettePair{Bg: "#f7faf7", Surface: "#ffffff", Text: "#14261c", Primary: "#15803d", OnPrimary: "#ffffff", Border: "#bbf7d0", Muted: "#1f3a29"},
		Dark:        PalettePair{Bg: "#0e1a14", Surface: "#15261d", Text: "#ecfdf5", Primary: "#4ade80", OnPrimary: "#0e1a14", Border: "#224432", Muted: "#bbf7d0"},
	},
}

// FrozenBlueprints holds all 5 Grex layout composition blueprints.
var FrozenBlueprints = []Blueprint{
	{
		ID:          "full_landing_page",
		Label:       "Full Landing Page",
		DefaultSlot: "main",
		Description: "Nav header, Hero H1, value Grid, GrabDecimal pricing Grid, ForEach showcase, and FAQ Accordion.",
	},
	{
		ID:          "hero_section",
		Label:       "Hero Section",
		DefaultSlot: "hero",
		Description: "Semantic Hero Section with H1, lead copy, primary action link/button, and accessible feature Image.",
	},
	{
		ID:          "pricing_grid",
		Label:       "Pricing Grid (GrabDecimal)",
		DefaultSlot: "main",
		Description: "Accessible 3-tier pricing Grid preserving exact base-10 GrabDecimal wire strings and SKU values.",
	},
	{
		ID:          "faq_accordion",
		Label:       "FAQ Accordion",
		DefaultSlot: "main",
		Description: "Sequential H1 -> H2 -> H3 semantic Accordion populated with topic Q&A and internal links.",
	},
	{
		ID:          "foreach_showcase",
		Label:       "ForEach Content Showcase",
		DefaultSlot: "main",
		Description: "Dynamic ForEach collection showcase bound to GrabLab CMS entries with semantic Cards.",
	},
}

// DefaultDesignSystemMarkdown provides baseline tokens when the workspace lacks GRAB-DESIGN-SYSTEM.md.
const DefaultDesignSystemMarkdown = `# GRAB-DESIGN-SYSTEM.md
- --grex-bg: #ffffff
- --grex-surface: #f8fafc
- --grex-text: #0f172a
- --grex-primary: #047857
- --grex-on-primary: #ffffff
- --grex-space-lg: 2rem
- --grex-radius-md: 0.75rem
- Classes: .hero, .hero-card, .card, .btn-primary, .badge-pill, .grid-3
`

var (
	reClassToken = regexp.MustCompile(`\.(grex-c-[a-z0-9-]+|hero(?:-card)?|card|btn-primary|badge-pill|grid-3|cta-button)\b`)
	reTokenAssign = regexp.MustCompile(`(--grex-[a-z0-9-]+)\s*:\s*([^;\n]+)`)
	reMarkdownLink = regexp.MustCompile(`(/[a-z0-9][a-z0-9/_-]*)`)
	reRedirectRule = regexp.MustCompile(`(/[a-z0-9/_-]+)\s*->\s*(/[a-z0-9/_-]+)`)
)

// ParseDesignSystemMarkdown parses CSS custom properties and registered class names.
func ParseDesignSystemMarkdown(md string) DesignSystemInfo {
	if strings.TrimSpace(md) == "" {
		md = DefaultDesignSystemMarkdown
	}
	tokens := map[string]string{
		"--grex-bg":        "#ffffff",
		"--grex-surface":   "#f8fafc",
		"--grex-text":      "#0f172a",
		"--grex-primary":   "#047857",
		"--grex-space-lg":  "2rem",
		"--grex-radius-md": "0.75rem",
	}
	for _, m := range reTokenAssign.FindAllStringSubmatch(md, -1) {
		tokens[strings.TrimSpace(m[1])] = strings.TrimSpace(m[2])
	}
	if len(tokens) == 0 {
		for _, m := range reCSSVar.FindAllString(md, -1) {
			tokens[m] = "active"
		}
	}
	classSet := map[string]bool{}
	var classes []string
	for _, m := range reClassToken.FindAllStringSubmatch(md, -1) {
		c := "." + m[1]
		if !classSet[c] {
			classSet[c] = true
			classes = append(classes, c)
		}
	}
	return DesignSystemInfo{
		RawMarkdown: md,
		Tokens:      tokens,
		Classes:     classes,
	}
}

// ParseSiteContextMarkdown extracts internal paths and redirects from SITE-CONTEXT.md.
func ParseSiteContextMarkdown(md string) SiteContextInfo {
	pathSet := map[string]bool{"/": true, "/blog": true, "/pricing": true, "/about": true, "/docs": true}
	redirs := map[string]string{}
	for _, m := range reRedirectRule.FindAllStringSubmatch(md, -1) {
		from := normalizeRoutePath(m[1])
		to := normalizeRoutePath(m[2])
		if from != "" && to != "" && from != to {
			redirs[from] = to
		}
	}
	for _, m := range reMarkdownLink.FindAllStringSubmatch(md, -1) {
		p := normalizeRoutePath(m[1])
		if p != "" {
			pathSet[p] = true
		}
	}
	paths := make([]string, 0, len(pathSet))
	for p := range pathSet {
		paths = append(paths, p)
	}
	return SiteContextInfo{
		KnownPaths: paths,
		Redirects:  redirs,
	}
}

// StudioCatalog builds the metadata payload for GET /api/studio/design-system.
func StudioCatalog(designSystemMD, siteContextMD string) StudioCatalogResponse {
	return StudioCatalogResponse{
		StylePacks:       FrozenStylePacks,
		Blueprints:       FrozenBlueprints,
		Elements:         GrexElementsCatalog,
		DesignSystem:     ParseDesignSystemMarkdown(designSystemMD),
		SiteContext:      ParseSiteContextMarkdown(siteContextMD),
		CombinationCount: fmt.Sprintf("%d", len(FrozenStylePacks)*len(FrozenBlueprints)),
	}
}

// FindStylePack returns a frozen pack by ID (defaulting cleanly to emerald-editorial).
func FindStylePack(id string) (StylePack, bool) {
	id = strings.ToLower(strings.TrimSpace(id))
	for _, p := range FrozenStylePacks {
		if p.ID == id {
			return p, true
		}
	}
	return FrozenStylePacks[0], false
}

// FindBlueprint returns a frozen blueprint by ID (defaulting to full_landing_page).
func FindBlueprint(id string) (Blueprint, bool) {
	id = strings.ToLower(strings.TrimSpace(id))
	for _, b := range FrozenBlueprints {
		if b.ID == id {
			return b, true
		}
	}
	return FrozenBlueprints[0], false
}

// SynthesizeLayout produces a Grade A/A+ zero-<div> GrexNode tree and preview HTML
// for any of the 8 Style Packs x 5 Blueprints (40 combinations), preserving ADR-025c strings.
func SynthesizeLayout(req SynthesizeRequest) (SynthesizeResponse, error) {
	pack, _ := FindStylePack(req.StylePack)
	bp, _ := FindBlueprint(req.Blueprint)
	slot := strings.TrimSpace(req.Slot)
	if slot == "" {
		slot = bp.DefaultSlot
	}
	topic := firstNonEmpty(req.Title, req.Topic, "Fast Semantic Web Architecture with GRAB & Grex JS")
	if len(topic) < 28 {
		topic = topic + " — Zero-Div Accessible Studio"
	}
	if len(topic) > 66 {
		topic = truncateClean(topic, 64)
	}
	focusKW := firstNonEmpty(req.FocusKeyword, deriveKeywordFromTitle(topic), "semantic grab layout")

	// Preserve exact ADR-025c base-10 wire strings when supplied.
	cf := map[string]string{
		"price":    "49.00",
		"sku":      "10020",
		"currency": "USD",
	}
	for k, v := range req.CustomFields {
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			continue
		}
		if (k == "price" || k == "amount") && !security.IsExactDecimalString(trimmed) && !security.IsExactIntString(trimmed) {
			return SynthesizeResponse{}, fmt.Errorf("custom field %s must be an exact ADR-025c base-10 string", k)
		}
		cf[k] = trimmed
	}

	lightRatio := ComputeContrastRatio(pack.Light.Text, pack.Light.Bg)
	lightBtnRatio := ComputeContrastRatio(pack.Light.OnPrimary, pack.Light.Primary)
	darkRatio := ComputeContrastRatio(pack.Dark.Text, pack.Dark.Bg)
	darkBtnRatio := ComputeContrastRatio(pack.Dark.OnPrimary, pack.Dark.Primary)

	rootStyles := map[string]any{
		"base": map[string]any{
			"color":            fmt.Sprintf("var(--grex-text, %s)", pack.Light.Text),
			"background-color": fmt.Sprintf("var(--grex-bg, %s)", pack.Light.Bg),
			"border-color":     fmt.Sprintf("var(--grex-border, %s)", pack.Light.Border),
		},
		"tablet": map[string]any{
			"padding": "var(--grex-space-md, 1.25rem)",
		},
		"mobile": map[string]any{
			"padding": "var(--grex-space-sm, 0.875rem)",
		},
	}
	cardStyles := map[string]any{
		"base": map[string]any{
			"color":            fmt.Sprintf("var(--grex-text, %s)", pack.Light.Text),
			"background-color": fmt.Sprintf("var(--grex-surface, %s)", pack.Light.Surface),
			"border-color":     fmt.Sprintf("var(--grex-border, %s)", pack.Light.Border),
		},
	}
	btnStyles := map[string]any{
		"base": map[string]any{
			"color":            fmt.Sprintf("var(--grex-on-primary, %s)", pack.Light.OnPrimary),
			"background-color": fmt.Sprintf("var(--grex-primary, %s)", pack.Light.Primary),
		},
	}

	navNode := &GrexNode{
		Element: "Nav",
		Tag:     "nav",
		Props:   map[string]any{"ariaLabel": "Primary studio navigation", "class": "studio-nav"},
		Children: []*GrexNode{
			{Element: "Link", Tag: "a", Props: map[string]any{"href": "/", "text": "Home Overview"}},
			{Element: "Link", Tag: "a", Props: map[string]any{"href": "/docs", "text": "Documentation Guides"}},
			{Element: "Link", Tag: "a", Props: map[string]any{"href": "/pricing", "text": "Transparent Plans"}},
		},
	}

	heroSection := &GrexNode{
		Element: "Section",
		Tag:     "section",
		Props:   map[string]any{"id": "studio-hero", "class": "hero"},
		Styles:  rootStyles,
		Children: []*GrexNode{
			{
				Element: "Heading",
				Tag:     "h1",
				Props:   map[string]any{"level": 1, "text": topic},
			},
			{
				Element: "Text",
				Tag:     "p",
				Props: map[string]any{
					"text": fmt.Sprintf("Crafted in the %s design system with verified Light and Dark WCAG AA contrast, zero wrapper div containers, and semantic %s landmarks.", pack.Label, bp.Label),
				},
			},
			{
				Element: "Stack",
				Tag:     "section",
				Props:   map[string]any{"direction": "row", "gap": "md"},
				Children: []*GrexNode{
					{
						Element: "Button",
						Tag:     "button",
						Props:   map[string]any{"label": "Launch Interactive Draft", "variant": "primary", "class": "btn-primary"},
						Styles:  btnStyles,
					},
					{
						Element: "Link",
						Tag:     "a",
						Props:   map[string]any{"href": "/blog", "text": "Explore Semantic Architecture Guides"},
					},
				},
			},
			{
				Element: "Image",
				Tag:     "img",
				Props: map[string]any{
					"src": fmt.Sprintf("/public/desk/%s-preview.svg", pack.ID),
					"alt": fmt.Sprintf("%s layout preview illustrating accessible %s typography and tokens", pack.Label, bp.Label),
				},
			},
		},
	}

	pricingSection := &GrexNode{
		Element: "Section",
		Tag:     "section",
		Props:   map[string]any{"id": "studio-pricing", "class": "pricing-section"},
		Children: []*GrexNode{
			{
				Element: "Heading",
				Tag:     "h2",
				Props:   map[string]any{"level": 2, "text": "Predictable GrabDecimal Licensing Tiers"},
			},
			{
				Element: "Grid",
				Tag:     "section",
				Props:   map[string]any{"columns": 3, "class": "grid-3"},
				Children: []*GrexNode{
					buildPricingCard("Starter Workspace", "0.00", "10010", "Ideal for local semantic authoring and preview checks.", cardStyles),
					buildPricingCard("Studio Pro", cf["price"], cf["sku"], "Full GrabLab CMS draft sync and 22-rule Boost automation.", cardStyles),
					buildPricingCard("Team Scale", "149.00", "10030", "Multi-worker fleet workflows with encrypted secret handling.", cardStyles),
				},
			},
		},
	}

	faqSection := &GrexNode{
		Element: "Section",
		Tag:     "section",
		Props:   map[string]any{"id": "studio-faq", "class": "faq-section"},
		Children: []*GrexNode{
			{
				Element: "Heading",
				Tag:     "h2",
				Props:   map[string]any{"level": 2, "text": "Frequently Asked Accessibility and DOM Questions"},
			},
			{
				Element: "Accordion",
				Tag:     "section",
				Props:   map[string]any{"id": "faq-accordion"},
				Children: []*GrexNode{
					{
						Element: "Card",
						Tag:     "article",
						Styles:  cardStyles,
						Children: []*GrexNode{
							{Element: "Heading", Tag: "h3", Props: map[string]any{"level": 3, "text": "Why does Grex enforce zero wrapper div tags?"}},
							{Element: "Text", Tag: "p", Props: map[string]any{"text": "Semantic HTML5 landmarks give assistive technologies clean navigation structure while minimizing DOM complexity."}},
						},
					},
					{
						Element: "Card",
						Tag:     "article",
						Styles:  cardStyles,
						Children: []*GrexNode{
							{Element: "Heading", Tag: "h3", Props: map[string]any{"level": 3, "text": "How are GrabDecimal prices protected from floating point drift?"}},
							{Element: "Text", Tag: "p", Props: map[string]any{"text": "All prices stay exact base-10 ADR-025c strings such as $" + cf["price"] + " across schema.org JSON-LD and MCP calls."}},
						},
					},
				},
			},
		},
	}

	foreachSection := &GrexNode{
		Element: "Section",
		Tag:     "section",
		Props:   map[string]any{"id": "studio-showcase", "class": "foreach-showcase"},
		Children: []*GrexNode{
			{
				Element: "Heading",
				Tag:     "h2",
				Props:   map[string]any{"level": 2, "text": "Featured Semantic CMS Entries"},
			},
			{
				Element: "ForEach",
				Tag:     "section",
				Props:   map[string]any{"source": "entries", "content_type": "post"},
				Children: []*GrexNode{
					{
						Element: "Card",
						Tag:     "article",
						Styles:  cardStyles,
						Children: []*GrexNode{
							{Element: "Heading", Tag: "h3", Props: map[string]any{"level": 3, "text": "Semantic Component Architecture Guide"}},
							{Element: "RichText", Tag: "p", Props: map[string]any{"text": "Build instant SSR pages powered by GRAB bytecode workers and deterministic accessibility verification."}},
							{Element: "Link", Tag: "a", Props: map[string]any{"href": "/docs", "text": "Read Complete Architecture Documentation"}},
						},
					},
				},
			},
		},
	}

	rootChildren := []*GrexNode{navNode, heroSection}
	switch bp.ID {
	case "hero_section":
		// Add a single H2 summary card so hero_section preserves 0 skipped headings & depth compliance.
		rootChildren = append(rootChildren, &GrexNode{
			Element: "Section",
			Tag:     "section",
			Children: []*GrexNode{
				{Element: "Heading", Tag: "h2", Props: map[string]any{"level": 2, "text": "Key Semantic Highlights"}},
				{Element: "Text", Tag: "p", Props: map[string]any{"text": "Guaranteed Light/Dark >= 4.5:1 contrast, clean typography, and direct internal links."}},
			},
		})
	case "pricing_grid":
		rootChildren = append(rootChildren, pricingSection)
	case "faq_accordion":
		rootChildren = append(rootChildren, faqSection)
	case "foreach_showcase":
		rootChildren = append(rootChildren, foreachSection)
	default: // full_landing_page
		rootChildren = append(rootChildren, pricingSection, foreachSection, faqSection)
	}

	root := &GrexNode{
		Element:  "Container",
		Tag:      "main",
		Props:    map[string]any{"id": "studio-canvas-root", "slot": slot, "style_pack": pack.ID, "blueprint": bp.ID},
		Styles:   rootStyles,
		Children: rootChildren,
	}

	previewHTML := RenderGrexTreeHTML(root, topic, fmt.Sprintf("Semantic %s synthesized with %s achieving verified WCAG AA Light and Dark contrast.", bp.Label, pack.Label))
	scorecard := Audit(AuditInput{
		TargetPath:     fmt.Sprintf("studio/%s-%s.grex.json", pack.ID, bp.ID),
		ContentType:    mapBlueprintContentType(bp.ID),
		Title:          topic,
		MetaDesc:       fmt.Sprintf("Semantic %s layout built in the %s design pack with accessible Light/Dark contrast and zero wrapper divs.", bp.Label, pack.Label),
		Slug:           normalizeSlug(pack.ID + "-" + bp.ID),
		FocusKeyword:   focusKW,
		CanonicalURL:   "/" + normalizeSlug(bp.ID),
		HTML:           previewHTML,
		GrexLayout:     root,
		DesignSystemMD: req.DesignSystemMD,
		SiteContextMD:  req.SiteContextMD,
		CustomFields:   cf,
	})

	return SynthesizeResponse{
		StylePack:   pack,
		Blueprint:   bp,
		Slot:        slot,
		Actor:       "groxy-ai",
		GrexLayout:  root,
		PreviewHTML: previewHTML,
		Contrast: ContrastSummary{
			LightTextBg:       fmt.Sprintf("%.2f:1", lightRatio),
			LightPrimaryBtn:   fmt.Sprintf("%.2f:1", lightBtnRatio),
			DarkTextBg:        fmt.Sprintf("%.2f:1", darkRatio),
			DarkPrimaryBtn:    fmt.Sprintf("%.2f:1", darkBtnRatio),
			MeetsWCAGAA:       lightRatio >= 4.5 && lightBtnRatio >= 4.5 && darkRatio >= 4.5 && darkBtnRatio >= 4.5,
			ContrastThreshold: "4.50:1",
		},
		Scorecard:    scorecard,
		CustomFields: cf,
	}, nil
}

// RenderGrexTreeHTML converts a validated GrexNode tree into semantic zero-<div> HTML5 markup.
func RenderGrexTreeHTML(root *GrexNode, title, metaDesc string) string {
	var body strings.Builder
	renderNodeHTML(&body, root)
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>%s</title>
  <meta name="description" content="%s" />
  <link rel="canonical" href="/" />
</head>
<body>
%s
</body>
</html>
`, html.EscapeString(title), html.EscapeString(metaDesc), body.String())
}

func renderNodeHTML(b *strings.Builder, n *GrexNode) {
	if n == nil {
		return
	}
	tag := resolveSemanticTag(n)
	if !allowedSemanticHTMLTags[tag] {
		tag = "section"
	}
	switch tag {
	case "img":
		src := html.EscapeString(firstNonEmpty(fmt.Sprint(nonNilValue(n.Props["src"])), "/public/desk/preview.svg"))
		alt := html.EscapeString(fmt.Sprint(nonNilValue(n.Props["alt"])))
		b.WriteString(fmt.Sprintf(`<img src="%s" alt="%s" />`, src, alt))
		return
	case "a":
		href := fmt.Sprint(nonNilValue(n.Props["href"]))
		if !IsSafeURL(href) {
			href = "#"
		}
		txt := html.EscapeString(firstNonEmpty(fmt.Sprint(nonNilValue(n.Props["text"])), fmt.Sprint(nonNilValue(n.Props["label"])), "Explore link"))
		b.WriteString(fmt.Sprintf(`<a href="%s">%s</a>`, html.EscapeString(href), txt))
		return
	case "button":
		lbl := html.EscapeString(firstNonEmpty(fmt.Sprint(nonNilValue(n.Props["label"])), fmt.Sprint(nonNilValue(n.Props["text"])), "Action"))
		b.WriteString(fmt.Sprintf(`<button type="button">%s</button>`, lbl))
		return
	}

	b.WriteByte('<')
	b.WriteString(tag)
	if id := strings.TrimSpace(fmt.Sprint(nonNilValue(n.Props["id"]))); id != "" {
		b.WriteString(fmt.Sprintf(` id="%s"`, html.EscapeString(id)))
	}
	if aria := strings.TrimSpace(fmt.Sprint(nonNilValue(n.Props["ariaLabel"]))); aria != "" {
		b.WriteString(fmt.Sprintf(` aria-label="%s"`, html.EscapeString(aria)))
	}
	if cls := strings.TrimSpace(fmt.Sprint(nonNilValue(n.Props["class"]))); cls != "" {
		b.WriteString(fmt.Sprintf(` class="%s"`, html.EscapeString(cls)))
	}
	b.WriteByte('>')
	if txt := strings.TrimSpace(fmt.Sprint(nonNilValue(n.Props["text"]))); txt != "" && len(n.Children) == 0 {
		b.WriteString(html.EscapeString(txt))
	}
	for _, ch := range n.Children {
		renderNodeHTML(b, ch)
	}
	b.WriteString("</")
	b.WriteString(tag)
	b.WriteByte('>')
}

func resolveSemanticTag(n *GrexNode) string {
	if tag := strings.ToLower(strings.TrimSpace(n.Tag)); tag != "" {
		return tag
	}
	switch n.Element {
	case "Container":
		return "main"
	case "Nav":
		return "nav"
	case "Card":
		return "article"
	case "Heading":
		return fmt.Sprintf("h%d", headingLevelFromNode(n))
	case "Text", "RichText":
		return "p"
	case "Link":
		return "a"
	case "Button":
		return "button"
	case "Image":
		return "img"
	case "Form":
		return "form"
	case "Input":
		return "input"
	case "Select":
		return "select"
	default:
		return "section"
	}
}

func buildPricingCard(name, price, sku, summary string, styles map[string]any) *GrexNode {
	return &GrexNode{
		Element: "Card",
		Tag:     "article",
		Props:   map[string]any{"class": "card pricing-card", "sku": sku, "price": price},
		Styles:  styles,
		Children: []*GrexNode{
			{Element: "Heading", Tag: "h3", Props: map[string]any{"level": 3, "text": name}},
			{Element: "Text", Tag: "p", Props: map[string]any{"text": fmt.Sprintf("$%s / month — SKU %s", price, sku)}},
			{Element: "Text", Tag: "p", Props: map[string]any{"text": summary}},
			{Element: "Link", Tag: "a", Props: map[string]any{"href": "/pricing", "text": "Select " + name + " Plan"}},
		},
	}
}

func mapBlueprintContentType(bp string) string {
	switch bp {
	case "pricing_grid", "full_landing_page":
		return "product"
	case "faq_accordion":
		return "faq"
	default:
		return "post"
	}
}
