package server

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"groxy-desk/internal/security"
)

func TestV020HealthMCPStudioAndBoostSecurityLifecycle(t *testing.T) {
	srv, wsDir, _ := setupTestServer(t)
	sess := srv.Session()

	// 1. /_health reports v1.3.0
	respH, _, rawH := doAPIReq(t, http.MethodGet, sess.URL+"/_health", "", "", nil)
	if respH.StatusCode != http.StatusOK || !strings.Contains(rawH, `"1.3.0"`) {
		t.Fatalf("expected v1.3.0 /_health, got status=%d body=%s", respH.StatusCode, rawH)
	}

	// 2. GET /api/mcp/tools enforces auth+CSRF and exposes 13 tools
	respUnauth, _, _ := doAPIReq(t, http.MethodGet, sess.URL+"/api/mcp/tools", sess.BearerToken, "", nil)
	if respUnauth.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 missing CSRF on /api/mcp/tools, got %d", respUnauth.StatusCode)
	}
	respTools, objTools, rawTools := doAPIReq(t, http.MethodGet, sess.URL+"/api/mcp/tools", sess.BearerToken, sess.CSRFToken, nil)
	if respTools.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/mcp/tools failed: %d %s", respTools.StatusCode, rawTools)
	}
	toolsSlice, _ := objTools["tools"].([]any)
	if len(toolsSlice) != 13 || objTools["model_switching_allowed"] != false {
		t.Fatalf("expected 13 tools & zero model switching, got len=%d body=%s", len(toolsSlice), rawTools)
	}

	// 3. POST /api/mcp/call GRAB-E401 confinement + unapproved write 403 DESK-E403 + 422/-32602
	r401, obj401, _ := doAPIReq(t, http.MethodPost, sess.URL+"/api/mcp/call", sess.BearerToken, sess.CSRFToken, map[string]any{
		"tool":      "grab_check",
		"arguments": map[string]any{"file": ".env"},
	})
	if r401.StatusCode != http.StatusForbidden || obj401["error"].(map[string]any)["code"] != "GRAB-E401" {
		t.Fatalf("expected 403 GRAB-E401 on .env, got %d %v", r401.StatusCode, obj401)
	}

	r422, obj422, _ := doAPIReq(t, http.MethodPost, sess.URL+"/api/mcp/call", sess.BearerToken, sess.CSRFToken, map[string]any{
		"tool": "update_entry_layout",
		"arguments": map[string]any{
			"entry_id": "10",
			"layout":   map[string]any{"element": "Section", "tag": "div"},
		},
	})
	if r422.StatusCode != http.StatusUnprocessableEntity || obj422["mcp_error"].(map[string]any)["code"].(float64) != -32602 {
		t.Fatalf("expected 422 (-32602) on div layout, got %d %v", r422.StatusCode, obj422)
	}

	// 4. Studio design-system + synthesize + applyAiLayout + replay 409
	rDS, objDS, _ := doAPIReq(t, http.MethodGet, sess.URL+"/api/studio/design-system", sess.BearerToken, sess.CSRFToken, nil)
	if rDS.StatusCode != http.StatusOK || objDS["combination_count"] != "40" {
		t.Fatalf("expected 40 combinations in studio catalog, got %d %v", rDS.StatusCode, objDS)
	}

	rSynth, objSynth, rawSynth := doAPIReq(t, http.MethodPost, sess.URL+"/api/studio/synthesize", sess.BearerToken, sess.CSRFToken, map[string]any{
		"style_pack": "warm-commerce",
		"blueprint":  "pricing_grid",
		"title":      "Precision Commerce Grid Layout",
		"custom_fields": map[string]string{
			"price": "39.50",
			"sku":   "10088",
		},
	})
	if rSynth.StatusCode != http.StatusOK {
		t.Fatalf("synthesize failed: %d %s", rSynth.StatusCode, rawSynth)
	}
	layoutPayload := objSynth["grex_layout"]

	rPrep, objPrep, _ := doAPIReq(t, http.MethodPost, sess.URL+"/api/studio/layout/apply", sess.BearerToken, sess.CSRFToken, map[string]any{
		"slot":        "main",
		"grex_layout": layoutPayload,
		"prepare":     true,
	})
	if rPrep.StatusCode != http.StatusOK || objPrep["approval_required"] != true {
		t.Fatalf("studio apply prepare failed: %d %v", rPrep.StatusCode, objPrep)
	}
	studioTok, _ := objPrep["approval_token"].(string)

	rApp, objApp, rawApp := doAPIReq(t, http.MethodPost, sess.URL+"/api/studio/layout/apply", sess.BearerToken, sess.CSRFToken, map[string]any{
		"slot":           "main",
		"summary":        "Apply Warm Commerce Pricing Grid",
		"grex_layout":    layoutPayload,
		"approval_token": studioTok,
	})
	if rApp.StatusCode != http.StatusOK || objApp["command"].(map[string]any)["type"] != "applyAiLayout" {
		t.Fatalf("studio apply failed: %d %s", rApp.StatusCode, rawApp)
	}

	rRep, objRep, _ := doAPIReq(t, http.MethodPost, sess.URL+"/api/studio/layout/apply", sess.BearerToken, sess.CSRFToken, map[string]any{
		"slot":           "main",
		"grex_layout":    layoutPayload,
		"approval_token": studioTok,
	})
	if rRep.StatusCode != http.StatusConflict || objRep["error"].(map[string]any)["code"] != "DESK-E409" {
		t.Fatalf("expected 409 DESK-E409 replay on studio apply, got %d %v", rRep.StatusCode, objRep)
	}

	// 5. Write a broken file in workspace -> Audit (F) -> Boost -> Mismatch 403 -> Approve 200 -> Replay 409 -> Snapshot Restore!
	brokenPath := filepath.Join(wsDir, "views", "needs_boost.grab.html")
	brokenContent := `<html><head><title>Bad</title></head><body><div onclick="x()"><a href="javascript:void(0)">click here</a><img src="/a.png" /><h1>Only</h1><h4>Skip</h4></div></body></html>`
	if err := os.WriteFile(brokenPath, []byte(brokenContent), 0o644); err != nil {
		t.Fatalf("write needs_boost: %v", err)
	}
	origBytes, _ := os.ReadFile(brokenPath)
	origSHA := security.SHA256Hex(origBytes)

	rAudit, objAudit, _ := doAPIReq(t, http.MethodPost, sess.URL+"/api/groxy/audit", sess.BearerToken, sess.CSRFToken, map[string]any{
		"target_path": "views/needs_boost.grab.html",
	})
	if rAudit.StatusCode != http.StatusOK || objAudit["overall_grade"] != "F" || objAudit["publish_safe"] != false {
		t.Fatalf("expected Grade F publish_safe=false before boost, got %d %v", rAudit.StatusCode, objAudit)
	}

	rBoost, objBoost, rawBoost := doAPIReq(t, http.MethodPost, sess.URL+"/api/groxy/boost", sess.BearerToken, sess.CSRFToken, map[string]any{
		"target_path":   "views/needs_boost.grab.html",
		"style_pack":    "emerald-editorial",
		"custom_fields": map[string]string{"price": "79.00"},
	})
	if rBoost.StatusCode != http.StatusOK || objBoost["approval_required"] != true {
		t.Fatalf("POST /api/groxy/boost failed: %d %s", rBoost.StatusCode, rawBoost)
	}
	boostTok, _ := objBoost["approval_token"].(string)
	snapID := objBoost["snapshot"].(map[string]any)["snapshot_id"].(string)
	propMap := objBoost["proposal"].(map[string]any)
	deltaMap := propMap["projected_delta"].(map[string]any)
	if deltaMap["before_grade"] != "F" || (deltaMap["after_grade"] != "A" && deltaMap["after_grade"] != "A+") {
		t.Fatalf("expected F -> A/A+ projected delta, got %v", deltaMap)
	}

	// Target mismatch -> 403 DESK-E403
	rMM, objMM, _ := doAPIReq(t, http.MethodPost, sess.URL+"/api/groxy/boost/approve", sess.BearerToken, sess.CSRFToken, map[string]any{
		"approval_token": boostTok,
		"target_path":    "views/page.grab.html",
	})
	if rMM.StatusCode != http.StatusForbidden || objMM["error"].(map[string]any)["code"] != "DESK-E403" {
		t.Fatalf("expected 403 DESK-E403 on target mismatch, got %d %v", rMM.StatusCode, objMM)
	}

	// Valid approve -> 200 & file updated cleanly without wrapper <div>
	rBA, objBA, _ := doAPIReq(t, http.MethodPost, sess.URL+"/api/groxy/boost/approve", sess.BearerToken, sess.CSRFToken, map[string]any{
		"approval_token": boostTok,
		"target_path":    "views/needs_boost.grab.html",
	})
	if rBA.StatusCode != http.StatusOK || objBA["ok"] != true {
		t.Fatalf("boost approve failed: %d %v", rBA.StatusCode, objBA)
	}
	updatedBytes, _ := os.ReadFile(brokenPath)
	if strings.Contains(strings.ToLower(string(updatedBytes)), "<div") {
		t.Fatalf("expected zero <div> after boost approve, got:\n%s", string(updatedBytes))
	}

	// Replay -> 409 DESK-E409
	rBR, objBR, _ := doAPIReq(t, http.MethodPost, sess.URL+"/api/groxy/boost/approve", sess.BearerToken, sess.CSRFToken, map[string]any{
		"approval_token": boostTok,
		"target_path":    "views/needs_boost.grab.html",
	})
	if rBR.StatusCode != http.StatusConflict || objBR["error"].(map[string]any)["code"] != "DESK-E409" {
		t.Fatalf("expected 409 DESK-E409 on replay, got %d %v", rBR.StatusCode, objBR)
	}

	// 1-Click Snapshot Restore -> exact original SHA-256
	rRst, objRst, _ := doAPIReq(t, http.MethodPost, sess.URL+"/api/groxy/snapshots/restore", sess.BearerToken, sess.CSRFToken, map[string]any{
		"snapshot_id": snapID,
	})
	if rRst.StatusCode != http.StatusOK || objRst["restored_sha256"] != origSHA {
		t.Fatalf("expected restored_sha256=%s, got %d %v", origSHA, rRst.StatusCode, objRst)
	}

	var checkJSON map[string]any
	_ = json.Unmarshal([]byte(rawH), &checkJSON)
}
