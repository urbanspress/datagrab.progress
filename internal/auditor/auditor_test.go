package auditor

import (
	"strings"
	"testing"
)

func TestAll40StylePackBlueprintCombinations(t *testing.T) {
	cat := StudioCatalog("", "")
	if len(cat.StylePacks) != 8 {
		t.Fatalf("expected 8 style packs, got %d", len(cat.StylePacks))
	}
	if len(cat.Blueprints) != 5 {
		t.Fatalf("expected 5 blueprints, got %d", len(cat.Blueprints))
	}
	if len(cat.Elements) != 20 {
		t.Fatalf("expected 20 grex elements, got %d", len(cat.Elements))
	}
	if cat.CombinationCount != "40" {
		t.Fatalf("expected combination_count=40, got %q", cat.CombinationCount)
	}

	for _, sp := range cat.StylePacks {
		for _, bp := range cat.Blueprints {
			res, err := SynthesizeLayout(SynthesizeRequest{
				StylePack:    sp.ID,
				Blueprint:    bp.ID,
				Title:        "Semantic " + sp.Label + " Showcase",
				FocusKeyword: "semantic layout",
				CustomFields: map[string]string{
					"price": "19.95",
					"sku":   "10042",
				},
			})
			if err != nil {
				t.Fatalf("SynthesizeLayout(%s, %s) failed: %v", sp.ID, bp.ID, err)
			}
			if !res.Contrast.MeetsWCAGAA {
				t.Fatalf("contrast failed WCAG AA for (%s, %s): %+v", sp.ID, bp.ID, res.Contrast)
			}
			if violations := ValidateGrexTree(res.GrexLayout); len(violations) > 0 {
				t.Fatalf("ValidateGrexTree(%s, %s) violations: %+v", sp.ID, bp.ID, violations)
			}
			if !res.Scorecard.PublishSafe || res.Scorecard.DivCount != "0" {
				t.Fatalf("unsafe scorecard for (%s, %s): %+v", sp.ID, bp.ID, res.Scorecard)
			}
			if res.Scorecard.OverallGrade != "A" && res.Scorecard.OverallGrade != "A+" {
				t.Fatalf("expected Grade A/A+ for (%s, %s), got %s (%s)", sp.ID, bp.ID, res.Scorecard.OverallGrade, res.Scorecard.OverallScore)
			}
			if res.CustomFields["price"] != "19.95" || res.CustomFields["sku"] != "10042" {
				t.Fatalf("ADR-025c price/sku coerced: %+v", res.CustomFields)
			}
			if len(res.Scorecard.Findings) != 22 {
				t.Fatalf("expected 22 deterministic findings, got %d", len(res.Scorecard.Findings))
			}
			if !strings.Contains(res.Scorecard.SchemaOrg.JSONText, `"19.95"`) {
				t.Fatalf("schema.org JSON-LD missing exact 19.95 string: %s", res.Scorecard.SchemaOrg.JSONText)
			}
		}
	}
}

func TestBoostRemediatesBadHTMLFromFToAAndPreservesADR025c(t *testing.T) {
	badHTML := `<html><head><title>Hi</title></head><body><div onclick="bad()"><a href="javascript:void(0)">click here</a><h1>One</h1><h1>Two</h1><h4>Skip</h4><img src="/a.png" /><script>eval("1+1")</script></div></body></html>`

	prop, err := Boost(BoostRequest{
		TargetPath: "views/broken.grab.html",
		HTML:       badHTML,
		StylePack:  "emerald-editorial",
		CustomFields: map[string]string{
			"price": "29.90",
			"sku":   "10099",
		},
	})
	if err != nil {
		t.Fatalf("Boost failed: %v", err)
	}
	if prop.BeforeReport.OverallGrade != "F" || prop.BeforeReport.PublishSafe {
		t.Fatalf("expected Grade F unsafe before boost, got %+v", prop.BeforeReport)
	}
	if !prop.AfterReport.PublishSafe || prop.AfterReport.DivCount != "0" {
		t.Fatalf("expected publish_safe div=0 after boost, got %+v", prop.AfterReport)
	}
	if prop.AfterReport.OverallGrade != "A" && prop.AfterReport.OverallGrade != "A+" {
		t.Fatalf("expected Grade A/A+ after boost, got %s (%s)", prop.AfterReport.OverallGrade, prop.AfterReport.OverallScore)
	}
	if len(prop.Diffs) == 0 || !strings.Contains(prop.UnifiedDiff, "--- a/views/broken.grab.html") {
		t.Fatalf("expected structured side-by-side diffs + unified diff, got %+v", prop)
	}
	if prop.CustomFields["price"] != "29.90" || prop.CustomFields["sku"] != "10099" {
		t.Fatalf("ADR-025c mutation detected: %+v", prop.CustomFields)
	}
}

func TestValidateGrexTreeRejectsDivContrastAndMissingAlt(t *testing.T) {
	badTree := &GrexNode{
		Element: "Section",
		Tag:     "div",
		Styles: map[string]any{
			"base": map[string]any{
				"color":            "#94a3b8",
				"background-color": "#cbd5e1",
			},
		},
		Children: []*GrexNode{
			{Element: "Heading", Tag: "h1", Props: map[string]any{"level": 1, "text": "Title"}},
			{Element: "Heading", Tag: "h4", Props: map[string]any{"level": 4, "text": "Skipped"}},
			{Element: "Image", Tag: "img", Props: map[string]any{"src": "/pic.png"}},
			{Element: "Link", Tag: "a", Props: map[string]any{"href": "javascript:alert(1)", "text": "Bad"}},
		},
	}
	vs := ValidateGrexTree(badTree)
	if len(vs) < 5 {
		t.Fatalf("expected >= 5 violations (div, contrast, heading skip, image alt, unsafe url), got %d: %+v", len(vs), vs)
	}
}
