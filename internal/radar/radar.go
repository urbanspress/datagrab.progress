package radar

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"groxy-desk/internal/groxybridge"
	"groxy-desk/internal/security"
)

const (
	researchPath  = "/api/groxy/research/bestseller-radar"
	cmsPath       = "/api/mcp/tools"
	consumer      = "groxy_desk"
	slot          = "desk-copilot"
	receiptAction = "radar_receipt"
	draftAction   = "radar_book_draft"
)

var (
	planPattern  = regexp.MustCompile("^rplan-[0-9a-f]{32}$")
	hashPattern  = regexp.MustCompile("^[0-9a-f]{64}$")
	tokenPattern = regexp.MustCompile("^rtok_res_[0-9a-f]{64}$")
	slugPattern  = regexp.MustCompile("^[a-z0-9]+(?:-[a-z0-9]+)*$")
	csrfMeta     = regexp.MustCompile(`<meta\s+name="groxy-csrf"\s+content="([a-zA-Z0-9_=-]+)"\s*/?>`)
)

type service struct {
	origin security.ServiceOrigin
	bearer string
	client *http.Client
}

func configuredService(raw, bearer string, allowLocal bool) *service {
	origin, err := security.ParseServiceOrigin(raw, allowLocal)
	if err != nil {
		return nil
	}
	jar, _ := cookiejar.New(nil)
	return &service{origin, bearer, &http.Client{
		Transport: origin.Transport(), Jar: jar, Timeout: 20 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}
}

func (s *service) request(ctx context.Context, method, path, csrf string, args any, expected int) ([]byte, error) {
	if s == nil {
		return nil, security.NewError(security.CodeDeskE503, 503, "configure the service origin in the Desk host environment", nil)
	}
	var body io.Reader
	if args != nil {
		raw, err := json.Marshal(args)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, s.origin.URL(path), body)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if csrf != "" {
		req.Header.Set("X-CSRF-Token", csrf)
	}
	if s.bearer != "" {
		req.Header.Set("Authorization", "Bearer "+s.bearer)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, security.NewError(security.ErrorCode(err, security.CodeDeskE502), security.ErrorStatus(err, 502), "configured service request failed", nil)
	}
	defer resp.Body.Close()
	if resp.StatusCode != expected {
		if resp.StatusCode >= 300 && resp.StatusCode < 400 {
			return nil, security.NewError(security.CodeGrabE402, 403, "configured service returned a redirect", nil)
		}
		status := resp.StatusCode
		if status < 400 || status >= 500 {
			status = 502
		}
		return nil, security.NewError(security.CodeDeskE502, status, fmt.Sprintf("configured service returned HTTP %d", resp.StatusCode), nil)
	}
	const limit = 2 << 20
	raw, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil || len(raw) > limit {
		return nil, security.NewError(security.CodeDeskE502, 502, "invalid or oversized service response", nil)
	}
	return raw, nil
}

type binding struct {
	PlanID   string `json:"radar_plan_id"`
	Genre    string `json:"genre_slug"`
	Consumer string `json:"target_consumer"`
	Slot     string `json:"target_slot"`
	Hash     string `json:"payload_sha256"`
}

type researchPlan struct {
	binding
	Token   string          `json:"approval_token"`
	Brief   json.RawMessage `json:"brief"`
	Expires time.Time       `json:"expires_at"`
}

type receipt struct {
	binding
	OK           bool   `json:"ok"`
	Approved     bool   `json:"approved"`
	Version      string `json:"schema_version"`
	Status       string `json:"status"`
	Draft        string `json:"status_enforced"`
	Confirmation bool   `json:"human_confirmation_required"`
	Checksum     string `json:"receipt_sha256"`
}

type Manager struct {
	approvals *groxybridge.ApprovalManager
	research  *service
	cms       *service
	ledgerDir string
	mu        sync.Mutex
	plans     map[string]researchPlan
	receipts  map[string]receipt
}

func NewManager(approvals *groxybridge.ApprovalManager, state string) *Manager {
	local := os.Getenv("GROXY_DESK_ALLOW_LOCAL_SERVICES") == "1"
	return &Manager{
		approvals: approvals,
		research:  configuredService(os.Getenv("GROXY_DESK_RADAR_ORIGIN"), os.Getenv("GROXY_DESK_RADAR_BEARER"), local),
		cms:       configuredService(os.Getenv("GROXY_DESK_CMS_ORIGIN"), os.Getenv("GROXY_DESK_CMS_MCP_BEARER"), local),
		ledgerDir: filepath.Join(state, "radar-drafts"),
		plans:     make(map[string]researchPlan), receipts: make(map[string]receipt),
	}
}

func (m *Manager) Capabilities() map[string]any {
	res := map[string]any{"research_configured": m.research != nil, "cms_configured": m.cms != nil && m.cms.bearer != ""}
	if m.cms != nil && strings.HasPrefix(m.cms.origin.Identity(), "https://") {
		res["cms_radar_url"] = m.cms.origin.URL("/admin/author/radar")
	}
	return res
}

type ResearchRequest struct {
	Genre string          `json:"genre_slug"`
	Price json.RawMessage `json:"price_tier_usd,omitempty"`
	Live  bool            `json:"include_live_feeds"`
}

func (m *Manager) Research(ctx context.Context, r ResearchRequest) (map[string]any, error) {
	if len(r.Genre) > 64 || !slugPattern.MatchString(r.Genre) {
		return nil, security.NewError(security.CodeDeskE101, 422, "enter a valid Radar genre slug", nil)
	}
	args := map[string]any{"genre_slug": r.Genre, "target_consumer": consumer, "target_slot": slot, "include_live_feeds": r.Live}
	if len(r.Price) != 0 {
		var price string
		if err := json.Unmarshal(r.Price, &price); err != nil || !security.IsExactDecimalString(price) {
			return nil, security.NewError(security.CodeDeskE101, 422, "price tier must be an exact base-10 string", nil)
		}
		args["price_tier_usd"] = price
	}
	body, err := m.researchRequest(ctx, researchPath, args, http.StatusOK)
	if err != nil {
		return nil, err
	}
	var p researchPlan
	if json.Unmarshal(body, &p) != nil || p.Genre != r.Genre || p.Consumer != consumer || p.Slot != slot ||
		!planPattern.MatchString(p.PlanID) || !hashPattern.MatchString(p.Hash) || !tokenPattern.MatchString(p.Token) ||
		!p.Expires.After(time.Now()) || len(p.Brief) == 0 || bytes.Equal(p.Brief, []byte("null")) {
		return nil, security.NewError(security.CodeDeskE502, 502, "Radar returned an invalid Desk plan", nil)
	}
	approval, err := m.approvals.IssueActionToken(receiptAction, p.PlanID+":"+p.Hash)
	if err != nil {
		return nil, err
	}
	m.mu.Lock()
	for id, existing := range m.plans {
		if !existing.Expires.After(time.Now()) {
			delete(m.plans, id)
			delete(m.receipts, id)
		}
	}
	m.plans[p.PlanID] = p
	m.mu.Unlock()
	return map[string]any{"radar_plan_id": p.PlanID, "brief": p.Brief, "approval_token": approval.Token}, nil
}

func (m *Manager) researchRequest(ctx context.Context, path string, args any, expected int) ([]byte, error) {
	body, err := m.research.request(ctx, http.MethodGet, "/groxy", "", nil, http.StatusOK)
	if err != nil {
		return nil, err
	}
	match := csrfMeta.FindSubmatch(body)
	if len(match) != 2 {
		return nil, security.NewError(security.CodeDeskE502, 502, "Groxy AI did not expose the Radar CSRF contract", nil)
	}
	return m.research.request(ctx, http.MethodPost, path, string(match[1]), args, expected)
}

type ReceiptRequest struct {
	PlanID   string `json:"radar_plan_id"`
	Approval string `json:"approval_token"`
}

func (m *Manager) Apply(ctx context.Context, r ReceiptRequest) (map[string]any, error) {
	m.mu.Lock()
	p, ok := m.plans[r.PlanID]
	m.mu.Unlock()
	if !ok || !p.Expires.After(time.Now()) {
		return nil, security.NewError(security.CodeDeskE403, 403, "Radar plan is unknown or expired", nil)
	}
	if _, err := m.approvals.ConsumeToken(r.Approval, receiptAction, p.PlanID+":"+p.Hash); err != nil {
		return nil, err
	}
	body, err := m.researchRequest(ctx, researchPath+"/apply", struct {
		binding
		Token string `json:"approval_token"`
	}{p.binding, p.Token}, http.StatusCreated)
	if err != nil {
		return nil, err
	}
	var rec receipt
	if json.Unmarshal(body, &rec) != nil || !rec.matches(p) {
		return nil, security.NewError(security.CodeDeskE502, 502, "Radar receipt failed its Desk draft binding check", nil)
	}
	m.mu.Lock()
	m.receipts[p.PlanID] = rec
	m.mu.Unlock()
	return map[string]any{"radar_plan_id": p.PlanID, "status_enforced": rec.Draft, "human_confirmation_required": true}, nil
}

func (r receipt) matches(p researchPlan) bool {
	return r.binding == p.binding && r.OK && r.Approved && r.Version == "1.8.0" && r.Status == "applied" &&
		r.Draft == "draft" && r.Confirmation &&
		security.ConstantTimeEqual(r.Checksum, security.SHA256Hex([]byte(p.PlanID+"|"+p.Hash)))
}

type DraftRequest struct {
	PlanID   string `json:"radar_plan_id"`
	Slug     string `json:"slug"`
	Title    string `json:"title"`
	Approval string `json:"approval_token,omitempty"`
}

func (m *Manager) draftBinding(r DraftRequest) (string, error) {
	if m.cms == nil || m.cms.bearer == "" {
		return "", security.NewError(security.CodeDeskE503, 503, "configure a GrabLab CMS origin and Author Mode MCP bearer", nil)
	}
	m.mu.Lock()
	rec, ok := m.receipts[r.PlanID]
	p := m.plans[r.PlanID]
	m.mu.Unlock()
	if !ok || !p.Expires.After(time.Now()) {
		return "", security.NewError(security.CodeDeskE403, 403, "a validated current Desk Radar receipt is required", nil)
	}
	if !slugPattern.MatchString(r.Slug) || len(r.Slug) > 100 || strings.TrimSpace(r.Title) == "" || len(r.Title) > 400 {
		return "", security.NewError(security.CodeDeskE101, 422, "enter a valid book slug and title", nil)
	}
	r.Approval = ""
	canonical, _ := json.Marshal(r)
	return "cms:radar:" + security.SHA256Hex(append([]byte(m.cms.origin.Identity()+"|"+rec.Hash+"|"), canonical...)), nil
}

func (m *Manager) draftLedgerPath(r DraftRequest) string {
	return filepath.Join(m.ledgerDir, security.SHA256Hex([]byte(m.cms.origin.Identity()+"|"+r.PlanID))+".jsonl")
}

func (m *Manager) PlanDraft(ctx context.Context, r DraftRequest) (map[string]any, error) {
	target, err := m.draftBinding(r)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(m.draftLedgerPath(r)); !os.IsNotExist(err) {
		return nil, duplicateDraft()
	}
	body, err := m.cms.request(ctx, http.MethodGet, cmsPath, "", nil, http.StatusOK)
	if err != nil {
		return nil, err
	}
	var tools struct {
		Tools []struct {
			Name        string
			InputSchema struct {
				Properties map[string]struct{ Type string }
				Required   []string
			}
		}
	}
	if json.Unmarshal(body, &tools) == nil {
		for _, tool := range tools.Tools {
			if tool.Name != "create_book_draft" || tool.InputSchema.Properties["slug"].Type != "string" ||
				tool.InputSchema.Properties["title"].Type != "string" {
				continue
			}
			for _, field := range tool.InputSchema.Required {
				if field != "slug" && field != "title" {
					return nil, security.NewError(security.CodeDeskE503, 503, "configured CMS requires unsupported book fields", nil)
				}
			}
			approval, err := m.approvals.IssueActionToken(draftAction, target)
			if err != nil {
				return nil, err
			}
			return map[string]any{"approval_token": approval.Token, "target": m.cms.origin.Identity(),
				"slug": r.Slug, "title": r.Title, "status_enforced": "draft"}, nil
		}
	}
	return nil, security.NewError(security.CodeDeskE503, 503, "enable migrated Author Mode with the book draft capability", nil)
}

func duplicateDraft() error {
	return security.NewError(security.CodeDeskE409, 409, "a draft was already attempted for this plan and CMS; review its slug in GrabLab", nil)
}

func (m *Manager) CreateDraft(ctx context.Context, r DraftRequest) (map[string]any, error) {
	target, err := m.draftBinding(r)
	if err != nil {
		return nil, err
	}
	if _, err := m.approvals.ConsumeToken(r.Approval, draftAction, target); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(m.ledgerDir, 0o700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(m.draftLedgerPath(r), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if os.IsExist(err) {
		return nil, duplicateDraft()
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	ledger := json.NewEncoder(f)
	if err := ledger.Encode(map[string]string{"plan": r.PlanID, "target": m.cms.origin.Identity(), "slug": r.Slug, "outcome": "pending"}); err != nil {
		return nil, err
	}
	if err := f.Sync(); err != nil {
		return nil, err
	}
	body, err := m.cms.request(ctx, http.MethodPost, cmsPath+"/create_book_draft", "",
		map[string]string{"slug": r.Slug, "title": r.Title}, http.StatusCreated)
	if err != nil {
		return nil, security.NewError(security.CodeDeskE502, 502, "CMS draft outcome is unconfirmed; check the approved slug in GrabLab", nil)
	}
	var res struct {
		Draft string `json:"status_enforced"`
		Book  struct {
			ID     json.RawMessage
			Slug   string
			Title  string
			Status string
		}
	}
	if json.Unmarshal(body, &res) != nil || res.Draft != "draft" || res.Book.Status != "draft" ||
		res.Book.Slug != r.Slug || res.Book.Title != r.Title || len(res.Book.ID) == 0 || string(res.Book.ID) == "null" {
		return nil, security.NewError(security.CodeDeskE502, 502, "CMS draft response could not be verified; check the approved slug in GrabLab", nil)
	}
	if err := ledger.Encode(map[string]string{"outcome": "draft_created"}); err != nil {
		return nil, err
	}
	if err := f.Sync(); err != nil {
		return nil, err
	}
	return map[string]any{"slug": res.Book.Slug, "title": res.Book.Title, "status_enforced": res.Draft}, nil
}
