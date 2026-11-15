package mcpbridge

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"groxy-desk/internal/auditor"
	"groxy-desk/internal/groxybridge"
	"groxy-desk/internal/security"
	"groxy-desk/internal/workspace"
)

func setupTestMCPBridge(t *testing.T) (*Bridge, *groxybridge.ApprovalManager, string) {
	t.Helper()
	wsDir := t.TempDir()
	stateDir := t.TempDir()

	if err := os.MkdirAll(filepath.Join(wsDir, "src"), 0o755); err != nil {
		t.Fatalf("mkdir src: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(wsDir, "views"), 0o755); err != nil {
		t.Fatalf("mkdir views: %v", err)
	}
	grabSrc := `# Desk Test
route "GET" "/" using req
  give back "ok"
end
`
	if err := os.WriteFile(filepath.Join(wsDir, "src", "app.grab"), []byte(grabSrc), 0o644); err != nil {
		t.Fatalf("write app.grab: %v", err)
	}
	if err := os.WriteFile(filepath.Join(wsDir, ".env"), []byte("API_SECRET=supersecretvalue123\n"), 0o600); err != nil {
		t.Fatalf("write .env: %v", err)
	}
	if err := os.WriteFile(filepath.Join(wsDir, "SITE-CONTEXT.md"), []byte("# SITE-CONTEXT.md\n- /pricing\n- /docs\n"), 0o644); err != nil {
		t.Fatalf("write SITE-CONTEXT.md: %v", err)
	}

	redactor := security.NewSecretRedactor()
	apprMgr, err := groxybridge.NewApprovalManager(stateDir, redactor)
	if err != nil {
		t.Fatalf("NewApprovalManager: %v", err)
	}
	grabBin := workspace.ResolveGrabBinary("")
	bridge, err := New(wsDir, grabBin, stateDir, apprMgr, redactor)
	if err != nil {
		t.Fatalf("mcpbridge.New: %v", err)
	}
	return bridge, apprMgr, wsDir
}

func TestListToolsIncludesAll13EcosystemTools(t *testing.T) {
	b, _, _ := setupTestMCPBridge(t)
	tools := b.ListTools()
	if len(tools) != 13 {
		t.Fatalf("expected 13 unified tools, got %d", len(tools))
	}
}

func TestCoreGrabMCPConfinementAndReadOnlyDefault(t *testing.T) {
	b, _, wsDir := setupTestMCPBridge(t)
	ctx := context.Background()

	// Confinement: path escape and .env secret access must fail GRAB-E401 (403).
	for _, badTool := range []CallRequest{
		{Tool: "grab_check", Arguments: map[string]any{"file": "../outside.grab"}},
		{Tool: "grab_check", Arguments: map[string]any{"file": ".env"}},
		{Tool: "grab_inspect_routes", Arguments: map[string]any{"file": "..\\secret.key"}},
		{Tool: "grab_scaffold", Arguments: map[string]any{"kind": "route", "target": "../escape.grab", "dry_run": true}},
	} {
		_, err := b.CallTool(ctx, badTool)
		var dErr *security.DeskError
		if !errors.As(err, &dErr) || dErr.Code != security.CodeGrabE401 || dErr.Status != 403 {
			t.Fatalf("expected 403 GRAB-E401 for %+v, got err=%v", badTool, err)
		}
	}

	// Read-only Core GRAB tool calls (if grab.exe exists locally) run cleanly without --allow-write.
	if b.grabBin != "" {
		resCheck, err := b.CallTool(ctx, CallRequest{
			Tool:      "grab_check",
			Arguments: map[string]any{"file": "src/app.grab"},
		})
		if err != nil {
			t.Fatalf("grab_check failed: %v", err)
		}
		if resCheck.AllowWriteFlag || !resCheck.ReadOnly {
			t.Fatalf("expected read-only without --allow-write: %+v", resCheck)
		}

		resRoutes, err := b.CallTool(ctx, CallRequest{
			Tool:      "grab_inspect_routes",
			Arguments: map[string]any{"file": "src/app.grab"},
		})
		if err != nil {
			t.Fatalf("grab_inspect_routes failed: %v", err)
		}
		if resRoutes.Status != "ok" {
			t.Fatalf("unexpected inspect_routes response: %+v", resRoutes)
		}

		// Write call without approval_token MUST fail 403 DESK-E403 before touching disk.
		_, err = b.CallTool(ctx, CallRequest{
			Tool: "grab_scaffold",
			Arguments: map[string]any{
				"kind":   "route",
				"target": "src/new_route.grab",
			},
		})
		var dErr *security.DeskError
		if !errors.As(err, &dErr) || dErr.Code != security.CodeDeskE403 || dErr.Status != 403 {
			t.Fatalf("expected 403 DESK-E403 on unapproved write grab_scaffold, got %v", err)
		}

		// Prepare + Approve + Execute grab_scaffold write cleanly.
		prep, err := b.CallTool(ctx, CallRequest{
			Tool: "grab_scaffold",
			Arguments: map[string]any{
				"kind":   "route",
				"target": "src/new_route.grab",
			},
			Prepare: true,
		})
		if err != nil || !prep.ApprovalRequired || prep.ApprovalToken == "" {
			t.Fatalf("prepare grab_scaffold failed: prep=%+v err=%v", prep, err)
		}

		execRes, err := b.CallTool(ctx, CallRequest{
			Tool: "grab_scaffold",
			Arguments: map[string]any{
				"kind":   "route",
				"target": "src/new_route.grab",
			},
			ApprovalToken: prep.ApprovalToken,
		})
		if err != nil || !execRes.AllowWriteFlag {
			t.Fatalf("approved grab_scaffold failed: res=%+v err=%v", execRes, err)
		}
		if _, statErr := os.Stat(filepath.Join(wsDir, "src", "new_route.grab")); statErr != nil {
			t.Fatalf("expected scaffolded file on disk: %v", statErr)
		}

		// Replay approval token -> 409 DESK-E409.
		_, err = b.CallTool(ctx, CallRequest{
			Tool: "grab_scaffold",
			Arguments: map[string]any{
				"kind":   "route",
				"target": "src/new_route.grab",
			},
			ApprovalToken: prep.ApprovalToken,
		})
		if !errors.As(err, &dErr) || dErr.Code != security.CodeDeskE409 || dErr.Status != 409 {
			t.Fatalf("expected 409 DESK-E409 on replay, got %v", err)
		}
	}
}

func TestLoopbackCMSToolsEnforceDraft422AndRejectExternalHosts(t *testing.T) {
	b, _, _ := setupTestMCPBridge(t)
	ctx := context.Background()

	// Non-loopback host rejected DESK-E501
	_, err := b.CallTool(ctx, CallRequest{
		Tool:   "list_content_types",
		CMSURL: "http://evil.example.com:8080",
	})
	var dErr *security.DeskError
	if !errors.As(err, &dErr) || dErr.Code != security.CodeDeskE501 {
		t.Fatalf("expected DESK-E501 for non-loopback CMS URL, got %v", err)
	}

	// Mock local GrabLab CMS MCP server
	var seenStatus string
	mockCMS := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/mcp/tools/create_entry_draft" {
			var body struct {
				Arguments map[string]any `json:"arguments"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			seenStatus, _ = body.Arguments["status"].(string)
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status_enforced": "draft",
				"entry": map[string]any{
					"id":     "42",
					"slug":   body.Arguments["slug"],
					"status": "draft",
				},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer mockCMS.Close()

	// Prepare + approve draft creation even if caller requests status="published" -> forced "draft"!
	prep, err := b.CallTool(ctx, CallRequest{
		Tool:   "create_entry_draft",
		CMSURL: mockCMS.URL,
		Arguments: map[string]any{
			"content_type": "post",
			"slug":         "hello-semantic",
			"title":        "Semantic Draft",
			"status":       "published",
		},
		Prepare: true,
	})
	if err != nil {
		t.Fatalf("prepare draft failed: %v", err)
	}
	res, err := b.CallTool(ctx, CallRequest{
		Tool:   "create_entry_draft",
		CMSURL: mockCMS.URL,
		Arguments: map[string]any{
			"content_type": "post",
			"slug":         "hello-semantic",
			"title":        "Semantic Draft",
			"status":       "published",
		},
		ApprovalToken: prep.ApprovalToken,
	})
	if err != nil {
		t.Fatalf("approved create_entry_draft failed: %v", err)
	}
	if seenStatus != "draft" || res.StatusEnforced != "draft" {
		t.Fatalf("expected forced draft status, seen=%q enforced=%q", seenStatus, res.StatusEnforced)
	}

	// Wrapper <div> on update_entry_layout returns 422 / -32602 BEFORE touching server!
	_, err = b.CallTool(ctx, CallRequest{
		Tool:   "update_entry_layout",
		CMSURL: mockCMS.URL,
		Arguments: map[string]any{
			"entry_id": "42",
			"layout": map[string]any{
				"element": "Section",
				"tag":     "div",
			},
		},
		ApprovalToken: "any-token",
	})
	if !errors.As(err, &dErr) || dErr.Status != 422 || !strings.Contains(dErr.Message, "-32602") {
		t.Fatalf("expected HTTP 422 (-32602) on div layout, got %v", err)
	}
}

func TestStudioApplyLayoutReturnsApplyAiLayoutUndo(t *testing.T) {
	b, _, _ := setupTestMCPBridge(t)
	ctx := context.Background()

	synth, err := auditor.SynthesizeLayout(auditor.SynthesizeRequest{
		StylePack: "obsidian-tech",
		Blueprint: "hero_section",
		Title:     "Studio Layout Bridge",
	})
	if err != nil {
		t.Fatalf("SynthesizeLayout: %v", err)
	}

	prep, err := b.ApplyStudioLayout(ctx, StudioApplyRequest{
		Slot:       "hero",
		Summary:    "Apply Obsidian Hero",
		GrexLayout: synth.GrexLayout,
		Prepare:    true,
	})
	if err != nil || prep.ApprovalToken == "" {
		t.Fatalf("ApplyStudioLayout prepare failed: prep=%+v err=%v", prep, err)
	}

	applied, err := b.ApplyStudioLayout(ctx, StudioApplyRequest{
		Slot:          "hero",
		Summary:       "Apply Obsidian Hero",
		GrexLayout:    synth.GrexLayout,
		ApprovalToken: prep.ApprovalToken,
	})
	if err != nil {
		t.Fatalf("ApplyStudioLayout apply failed: %v", err)
	}
	if applied.Command["type"] != "applyAiLayout" || applied.Command["actor"] != "groxy-ai" {
		t.Fatalf("expected applyAiLayout command for groxy-ai, got %+v", applied.Command)
	}
}
