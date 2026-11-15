package author

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"groxy-desk/internal/groxybridge"
	"groxy-desk/internal/security"
)

func TestAuthorModePlanApplyDraftEnforcementAndBookJSONLD(t *testing.T) {
	wsDir := t.TempDir()
	stateDir := t.TempDir()
	approvals, err := groxybridge.NewApprovalManager(stateDir, nil)
	if err != nil {
		t.Fatal(err)
	}
	mgr, err := NewManager(wsDir, "", approvals, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Confined check -> GRAB-E401
	_, confErr := mgr.PlanInit(InitPlanRequest{TargetSubdir: "../../escape", BookTitle: "Novel"})
	if confErr == nil || security.ErrorCode(confErr, "") != security.CodeGrabE401 {
		t.Fatalf("expected GRAB-E401 on escape subdir, got %v", confErr)
	}

	plan, err := mgr.PlanInit(InitPlanRequest{
		TargetSubdir: "novel-one",
		BookTitle:    "The Loopback Express</script>",
		AuthorName:   "Ada Lovelace",
		ISBN13:       "9780143127550",
		PriceUSD:     "19.99",
		WordCount:    "9007199254740993",
	})
	if err != nil {
		t.Fatalf("PlanInit failed: %v", err)
	}
	jsonLD := plan["schema_org_jsonld"].(string)
	if !strings.Contains(jsonLD, `"@type":"Book"`) || !strings.Contains(jsonLD, `"19.99"`) || !strings.Contains(jsonLD, `"9780143127550"`) || !strings.Contains(jsonLD, `"9007199254740993"`) {
		t.Fatalf("schema.org/Book JSON-LD missing exact ADR-025c fields: %s", jsonLD)
	}
	if strings.Contains(jsonLD, "</script>") {
		t.Fatalf("schema.org JSON-LD failed XSS script escape: %s", jsonLD)
	}
	landing := plan["preview_landing_html"].(string)
	if strings.Contains(strings.ToLower(landing), "<div") {
		t.Fatalf("author landing HTML contained wrapper <div>: %s", landing)
	}

	applyRes, err := mgr.ApplyInit(InitApplyRequest{
		TargetSubdir:  "novel-one",
		BookTitle:     "The Loopback Express</script>",
		AuthorName:    "Ada Lovelace",
		ISBN13:        "9780143127550",
		PriceUSD:      "19.99",
		WordCount:     "9007199254740993",
		ApprovalToken: plan["approval_token"].(string),
	})
	if err != nil || applyRes["status_enforced"] != "draft" {
		t.Fatalf("ApplyInit failed: %+v err=%v", applyRes, err)
	}

	// Verify schemas file exists
	b, rErr := os.ReadFile(filepath.Join(wsDir, "novel-one", "author-schemas.json"))
	if rErr != nil || !strings.Contains(string(b), "CharacterBible") || !strings.Contains(string(b), "WorldLore") {
		t.Fatalf("missing Author Mode schemas on disk: err=%v body=%s", rErr, string(b))
	}
}

func TestOllamaStatusLoopbackAndZeroModelSwitching(t *testing.T) {
	wsDir := t.TempDir()
	stateDir := t.TempDir()
	approvals, _ := groxybridge.NewApprovalManager(stateDir, nil)

	mockOllama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tags" {
			_, _ = w.Write([]byte(`{"models":[{"name":"llama3.2:latest"}]}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer mockOllama.Close()

	mgr, err := NewManager(wsDir, mockOllama.URL, approvals, nil)
	if err != nil {
		t.Fatal(err)
	}

	status, err := mgr.OllamaStatus(context.Background(), "")
	if err != nil || status["running"] != true || status["detected_count"] != "1" || status["model_switching_allowed"] != false {
		t.Fatalf("unexpected OllamaStatus: %+v err=%v", status, err)
	}

	// Non-loopback URL must fail closed DESK-E501
	_, badErr := mgr.OllamaStatus(context.Background(), "http://198.51.100.77:11434")
	if badErr == nil || security.ErrorCode(badErr, "") != security.CodeDeskE501 {
		t.Fatalf("expected DESK-E501 for non-loopback Ollama URL, got %v", badErr)
	}
}
