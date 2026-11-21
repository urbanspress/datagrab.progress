package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"groxy-desk/internal/security"
)

func TestRadarDeskReceiptAndSeparateCMSDraftApproval(t *testing.T) {
	const plan = "rplan-0123456789abcdef0123456789abcdef"
	hash := strings.Repeat("1", 64)
	upstreamToken := "rtok_res_" + strings.Repeat("2", 64)
	var researchCalls, receiptCalls, draftCalls atomic.Int32
	var corrupt atomic.Bool
	groxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/groxy" {
			http.SetCookie(w, &http.Cookie{Name: "session", Value: "radar-session", Path: "/"})
			_, _ = fmt.Fprint(w, `<meta name="groxy-csrf" content="groxy-csrf-fixture">`)
			return
		}
		cookie, err := r.Cookie("session")
		if r.Method != "POST" || err != nil || cookie.Value != "radar-session" || r.Header.Get("X-CSRF-Token") != "groxy-csrf-fixture" ||
			r.Header.Get("Authorization") != "Bearer radar-test-key" {
			w.WriteHeader(403)
			return
		}
		var request map[string]any
		_ = json.NewDecoder(r.Body).Decode(&request)
		binding := map[string]any{"radar_plan_id": plan, "target_consumer": "groxy_desk", "target_slot": "desk-copilot",
			"genre_slug": "historical-fiction", "payload_sha256": hash}
		if request["target_consumer"] != binding["target_consumer"] || request["target_slot"] != binding["target_slot"] ||
			request["genre_slug"] != binding["genre_slug"] {
			w.WriteHeader(422)
			return
		}
		switch r.URL.Path {
		case "/api/groxy/research/bestseller-radar":
			if request["price_tier_usd"] != "0.00" {
				w.WriteHeader(422)
				return
			}
			researchCalls.Add(1)
			binding["approval_token"] = upstreamToken
			binding["brief"] = map[string]any{"price_tier_usd": "0.00", "recommendation": "Review the source"}
			binding["expires_at"] = time.Now().Add(time.Minute).UTC().Format(time.RFC3339Nano)
		case "/api/groxy/research/bestseller-radar/apply":
			if request["approval_token"] != upstreamToken || request["radar_plan_id"] != plan || request["payload_sha256"] != hash {
				w.WriteHeader(422)
				return
			}
			receiptCalls.Add(1)
			binding["receipt_sha256"] = security.SHA256Hex([]byte(plan + "|" + hash))
			binding["ok"], binding["approved"], binding["human_confirmation_required"] = true, true, true
			binding["status"], binding["status_enforced"], binding["schema_version"] = "applied", "draft", "1.8.0"
			if corrupt.Load() {
				binding["target_slot"] = "wrong"
			}
			w.WriteHeader(201)
		default:
			w.WriteHeader(404)
			return
		}
		_ = json.NewEncoder(w).Encode(binding)
	}))
	defer groxy.Close()
	cms := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer cms-test-key" {
			w.WriteHeader(401)
			return
		}
		switch {
		case r.Method == "GET" && r.URL.Path == "/api/mcp/tools":
			_, _ = fmt.Fprint(w, `{"tools":[{"name":"create_book_draft","inputSchema":{"properties":{"slug":{"type":"string"},"title":{"type":"string"}},"required":["slug","title"]}}]}`)
		case r.Method == "POST" && r.URL.Path == "/api/mcp/tools/create_book_draft":
			var req map[string]string
			if json.NewDecoder(r.Body).Decode(&req) != nil || len(req) != 2 || req["slug"] != "approved-book" || req["title"] != "Approved Book" {
				w.WriteHeader(422)
				return
			}
			draftCalls.Add(1)
			w.WriteHeader(201)
			_, _ = fmt.Fprint(w, `{"book":{"id":"1","slug":"approved-book","title":"Approved Book","status":"draft"},"status_enforced":"draft","chapters":[]}`)
		default:
			w.WriteHeader(404)
		}
	}))
	defer cms.Close()
	t.Setenv("GROXY_DESK_ALLOW_LOCAL_SERVICES", "1")
	t.Setenv("GROXY_DESK_RADAR_ORIGIN", groxy.URL)
	t.Setenv("GROXY_DESK_RADAR_BEARER", "radar-test-key")
	t.Setenv("GROXY_DESK_CMS_ORIGIN", cms.URL)
	t.Setenv("GROXY_DESK_CMS_MCP_BEARER", "cms-test-key")
	srv, _, _ := setupTestServer(t)
	session := srv.Session()
	post := func(path, csrf string, body map[string]any, want int) map[string]any {
		t.Helper()
		resp, obj, raw := doAPIReq(t, "POST", session.URL+"/api/radar/"+path, session.BearerToken, csrf, body)
		if resp.StatusCode != want {
			t.Fatalf("%s returned %d, want %d: %s", path, resp.StatusCode, want, raw)
		}
		if strings.Contains(raw, upstreamToken) || strings.Contains(raw, "cms-test-key") || strings.Contains(raw, "radar-test-key") {
			t.Fatalf("upstream credential reached the workbench")
		}
		return obj
	}
	raw := map[string]any{"genre_slug": "historical-fiction", "price_tier_usd": 0.0}
	post("research", session.CSRFToken, raw, 422)
	raw["price_tier_usd"] = "0.00"
	post("research", "", raw, 403)
	approved := post("research", session.CSRFToken, raw, 200)
	draft := map[string]any{"radar_plan_id": plan, "slug": "approved-book", "title": "Approved Book"}
	post("draft/plan", session.CSRFToken, draft, 403)
	receiptRequest := map[string]any{"radar_plan_id": plan, "approval_token": approved["approval_token"]}
	corrupt.Store(true)
	post("receipt", session.CSRFToken, receiptRequest, 502)
	post("draft/plan", session.CSRFToken, draft, 403)
	corrupt.Store(false)
	approved = post("research", session.CSRFToken, raw, 200)
	receiptRequest["approval_token"] = approved["approval_token"]
	result := post("receipt", session.CSRFToken, receiptRequest, 201)
	if result["status_enforced"] != "draft" {
		t.Fatal("missing verified draft receipt")
	}
	post("receipt", session.CSRFToken, receiptRequest, 409)
	draft["approval_token"] = approved["approval_token"]
	post("draft/apply", session.CSRFToken, draft, 403)
	delete(draft, "approval_token")
	reviewed := post("draft/plan", session.CSRFToken, draft, 200)
	draft["approval_token"] = reviewed["approval_token"]
	draft["title"] = "Unapproved title"
	post("draft/apply", session.CSRFToken, draft, 403)
	draft["title"] = "Approved Book"
	post("draft/apply", session.CSRFToken, draft, 201)
	post("draft/apply", session.CSRFToken, draft, 409)
	post("draft/plan", session.CSRFToken, draft, 409)
	if researchCalls.Load() != 2 || receiptCalls.Load() != 2 || draftCalls.Load() != 1 {
		t.Fatalf("unexpected upstream call counts %d %d %d", researchCalls.Load(), receiptCalls.Load(), draftCalls.Load())
	}
}
