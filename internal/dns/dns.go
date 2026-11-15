package dns

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"groxy-desk/internal/groxybridge"
	"groxy-desk/internal/security"
)

const (
	NamecheapProdEndpoint    = "https://api.namecheap.com/xml.response"
	NamecheapSandboxEndpoint = "https://api.sandbox.namecheap.com/xml.response"
	vaultFileName            = "namecheap-vault.json"
)

var (
	reDomainLabel = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$`)
	reHostName    = regexp.MustCompile(`^(?:@|\*|[A-Za-z0-9_-]+(?:\.[A-Za-z0-9_-]+)*)$`)
)

// VaultSaveRequest saves Namecheap API credentials encrypted via AES-256-GCM (v1:...).
type VaultSaveRequest struct {
	APIUser          string `json:"api_user"`
	APIKey           string `json:"api_key"`
	ClientIP         string `json:"client_ip"`
	Sandbox          bool   `json:"sandbox"`
	EndpointOverride string `json:"endpoint_override,omitempty"`
}

type storedVault struct {
	APIUserEnc       string `json:"api_user_enc"`
	APIKeyEnc        string `json:"api_key_enc"`
	ClientIPEnc      string `json:"client_ip_enc"`
	Sandbox          bool   `json:"sandbox"`
	EndpointOverride string `json:"endpoint_override,omitempty"`
	UpdatedAt        string `json:"updated_at"`
}

type decryptedCreds struct {
	APIUser          string
	APIKey           string
	ClientIP         string
	Sandbox          bool
	EndpointOverride string
}

// HostRecord models a single Namecheap DNS record.
type HostRecord struct {
	HostID   string `json:"host_id,omitempty"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Address  string `json:"address"`
	MXPref   string `json:"mx_pref,omitempty"`
	TTL      string `json:"ttl"`
	IsActive string `json:"is_active,omitempty"`
}

// DNSPlanRequest configures email-safe A/AAAA/CNAME DNS updates for a domain.
type DNSPlanRequest struct {
	Domain           string       `json:"domain"`
	RootDomain       string       `json:"root_domain,omitempty"`
	Subdomain        string       `json:"subdomain,omitempty"`
	TargetIPv4       string       `json:"target_ip,omitempty"`
	TargetIPv6       string       `json:"target_ipv6,omitempty"`
	CNAMETarget      string       `json:"cname_target,omitempty"`
	ConfigureWWW     bool         `json:"configure_www,omitempty"`
	TTL              string       `json:"ttl,omitempty"`
	ExistingRecords  []HostRecord `json:"existing_records,omitempty"`
	SimulatedDropMX  bool         `json:"simulated_drop_mx,omitempty"` // test/guard rail hook if caller tries dropping MX
}

// DNSApplyRequest applies a verified DNS plan using single-use GROXY-SAFE-001 approval_token.
type DNSApplyRequest struct {
	Domain          string       `json:"domain"`
	RootDomain      string       `json:"root_domain,omitempty"`
	Subdomain       string       `json:"subdomain,omitempty"`
	ApprovalToken   string       `json:"approval_token"`
	ProposedRecords []HostRecord `json:"proposed_records"`
	ExistingRecords []HostRecord `json:"existing_records,omitempty"`
}

// HTTPSPreflightRequest verifies DNS convergence before allowing ACME issuance.
type HTTPSPreflightRequest struct {
	Domain         string   `json:"domain"`
	ExpectedIPv4   string   `json:"expected_ip"`
	ExpectedIPv6   string   `json:"expected_ipv6,omitempty"`
	EdgeProxy      string   `json:"edge_proxy,omitempty"`
	UpstreamPort   string   `json:"upstream_port,omitempty"`
	ObservedIPs    []string `json:"observed_ips,omitempty"` // optional deterministic resolver override
	ResolverHost   string   `json:"resolver_host,omitempty"`
}

// HTTPSApplyRequest configures Caddy, Nginx+Certbot, or Cloudflare Tunnel after propagation preflight.
type HTTPSApplyRequest struct {
	Domain        string   `json:"domain"`
	ExpectedIPv4  string   `json:"expected_ip"`
	EdgeProxy     string   `json:"edge_proxy"` // caddy | nginx | cloudflare-tunnel
	UpstreamPort  string   `json:"upstream_port"`
	Email         string   `json:"email,omitempty"`
	ApprovalToken string   `json:"approval_token"`
	ObservedIPs   []string `json:"observed_ips,omitempty"`
}

// Manager provides encrypted Namecheap vault storage, email-preserving DNS diffing,
// propagation preflight, and Auto-HTTPS configuration + 1-click rollback.
type Manager struct {
	stateDir   string
	approvals  *groxybridge.ApprovalManager
	redactor   *security.SecretRedactor
	httpClient *http.Client
	resolver   func(ctx context.Context, host string) ([]string, error)
	mu         sync.RWMutex
	vault      *storedVault
}

// NewManager initializes a Step 5 Namecheap & Auto-HTTPS manager.
func NewManager(stateDir string, approvals *groxybridge.ApprovalManager, redactor *security.SecretRedactor) (*Manager, error) {
	if strings.TrimSpace(stateDir) == "" {
		return nil, security.NewError(security.CodeDeskE101, 500, "dns state directory is required", nil)
	}
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		return nil, security.NewError(security.CodeDeskE101, 500, "failed creating dns state directory", err)
	}
	if redactor == nil {
		redactor = security.NewSecretRedactor()
	}
	m := &Manager{
		stateDir:   stateDir,
		approvals:  approvals,
		redactor:   redactor,
		httpClient: &http.Client{Timeout: 15 * time.Second},
		resolver: func(ctx context.Context, host string) ([]string, error) {
			return net.DefaultResolver.LookupHost(ctx, host)
		},
	}
	_ = m.loadVault()
	return m, nil
}

// SaveVault encrypts Namecheap API credentials at rest using AES-256-GCM (v1:...) and returns masked metadata.
func (m *Manager) SaveVault(req VaultSaveRequest) (map[string]any, error) {
	user := strings.TrimSpace(req.APIUser)
	apiKey := strings.TrimSpace(req.APIKey)
	clientIP := strings.TrimSpace(req.ClientIP)
	if user == "" || apiKey == "" {
		return nil, security.NewError(security.CodeDeskE101, 400, "Namecheap api_user and api_key are required", nil)
	}
	if net.ParseIP(clientIP) == nil {
		return nil, security.NewError(security.CodeDeskE101, 400, "client_ip must be a valid IPv4 or IPv6 address", nil)
	}
	if override := strings.TrimSpace(req.EndpointOverride); override != "" {
		if _, err := resolveEndpoint(override, req.Sandbox); err != nil {
			return nil, err
		}
	}

	m.redactor.Register(apiKey)
	m.redactor.Register(user)
	m.redactor.Register(clientIP)

	masterKey, err := security.ResolveMasterKey(m.stateDir)
	if err != nil {
		return nil, err
	}
	userEnc, err := security.EncryptAEAD(masterKey, user)
	if err != nil {
		return nil, err
	}
	keyEnc, err := security.EncryptAEAD(masterKey, apiKey)
	if err != nil {
		return nil, err
	}
	ipEnc, err := security.EncryptAEAD(masterKey, clientIP)
	if err != nil {
		return nil, err
	}

	v := &storedVault{
		APIUserEnc:       userEnc,
		APIKeyEnc:        keyEnc,
		ClientIPEnc:      ipEnc,
		Sandbox:          req.Sandbox,
		EndpointOverride: strings.TrimSpace(req.EndpointOverride),
		UpdatedAt:        time.Now().UTC().Format(time.RFC3339),
	}

	m.mu.Lock()
	m.vault = v
	saveErr := m.saveVaultLocked()
	m.mu.Unlock()
	if saveErr != nil {
		return nil, saveErr
	}

	return map[string]any{
		"ok":                true,
		"encrypted":         true,
		"cipher_format":     "AES-256-GCM (v1:...)",
		"api_user_masked":   security.MaskSecret("user_", user),
		"api_key_masked":    security.MaskSecret("nc_", apiKey),
		"client_ip_masked":  security.MaskSecret("ip_", clientIP),
		"sandbox":           req.Sandbox,
		"updated_at":        v.UpdatedAt,
	}, nil
}

// GetVaultMaskedStatus returns non-secret Namecheap vault status.
func (m *Manager) GetVaultMaskedStatus() map[string]any {
	m.mu.RLock()
	v := m.vault
	m.mu.RUnlock()
	if v == nil {
		return map[string]any{"configured": false}
	}
	masterKey, err := security.ResolveMasterKey(m.stateDir)
	if err != nil {
		return map[string]any{"configured": true, "encrypted": true}
	}
	u, _ := security.DecryptAEAD(masterKey, v.APIUserEnc)
	k, _ := security.DecryptAEAD(masterKey, v.APIKeyEnc)
	return map[string]any{
		"configured":      true,
		"encrypted":       true,
		"api_user_masked": security.MaskSecret("user_", u),
		"api_key_masked":  security.MaskSecret("nc_", k),
		"sandbox":         v.Sandbox,
		"updated_at":      v.UpdatedAt,
	}
}

// PlanDNS fetches current Namecheap records (or uses provided snapshot), computes email-safe A/AAAA/CNAME diff,
// asserts zero email record drops (422 DESK-E303), captures Pre-DNS Snapshot, and mints approval_token.
func (m *Manager) PlanDNS(ctx context.Context, req DNSPlanRequest) (map[string]any, error) {
	fqdn, rootDomain, sld, tld, sub, err := parseDomainParts(req.Domain, req.RootDomain, req.Subdomain)
	if err != nil {
		return nil, err
	}

	ttl := strings.TrimSpace(req.TTL)
	if ttl == "" {
		ttl = "300"
	}
	if !security.IsExactIntString(ttl) {
		return nil, security.NewError(security.CodeDeskE101, 400, "DNS ttl must be a base-10 integer string", nil)
	}

	existing := append([]HostRecord(nil), req.ExistingRecords...)
	if len(existing) == 0 {
		creds, cErr := m.decryptVault()
		if cErr != nil {
			return nil, cErr
		}
		fetched, fErr := m.namecheapGetHosts(ctx, creds, sld, tld)
		if fErr != nil {
			return nil, fErr
		}
		existing = fetched
	}

	next, diffs, err := buildEmailSafeDNSPlan(existing, sub, req)
	if err != nil {
		return nil, err
	}
	if req.SimulatedDropMX {
		next = stripEmailRecords(next)
	}
	preservedCount, emailErr := VerifyEmailRecordsPreserved(existing, next)
	if emailErr != nil {
		return nil, emailErr
	}

	targetKey := "dns:" + fqdn
	snapPayload, _ := json.Marshal(map[string]any{
		"domain":           fqdn,
		"root_domain":      rootDomain,
		"sld":              sld,
		"tld":              tld,
		"existing_records": existing,
	})
	snap, err := m.approvals.CreatePrefixedVirtualSnapshot("snap_dns_", targetKey, string(snapPayload))
	if err != nil {
		return nil, err
	}
	appr, err := m.approvals.IssueSnapshotBoundToken(groxybridge.ActionDNSApply, targetKey, snap.SnapshotID)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"ok":                     true,
		"domain":                 fqdn,
		"root_domain":            rootDomain,
		"target_key":             targetKey,
		"preserved_email_count":  strconv.Itoa(preservedCount),
		"existing_records":       existing,
		"proposed_records":       next,
		"diffs":                  diffs,
		"snapshot":               snap,
		"approval_token":         appr.Token,
		"plain_summary":          fmt.Sprintf("DNS plan ready for %s: %d email/service record(s) verified byte-identical; snapshot %s captured.", fqdn, preservedCount, snap.SnapshotID),
	}, nil
}

// ApplyDNS verifies email preservation (422 DESK-E303) before consuming single-use approval_token and calling setHosts.
func (m *Manager) ApplyDNS(ctx context.Context, req DNSApplyRequest) (map[string]any, error) {
	fqdn, _, sld, tld, _, err := parseDomainParts(req.Domain, req.RootDomain, req.Subdomain)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.ApprovalToken) == "" {
		return nil, security.NewError(security.CodeDeskE403, 403, "missing approval_token", nil)
	}

	creds, err := m.decryptVault()
	if err != nil {
		return nil, err
	}

	existing := append([]HostRecord(nil), req.ExistingRecords...)
	if len(existing) == 0 {
		fetched, fErr := m.namecheapGetHosts(ctx, creds, sld, tld)
		if fErr != nil {
			return nil, fErr
		}
		existing = fetched
	}

	preservedCount, emailErr := VerifyEmailRecordsPreserved(existing, req.ProposedRecords)
	if emailErr != nil {
		return nil, emailErr
	}

	targetKey := "dns:" + fqdn
	appr, err := m.approvals.ConsumeToken(req.ApprovalToken, groxybridge.ActionDNSApply, targetKey)
	if err != nil {
		return nil, err
	}
	if err := m.namecheapSetHosts(ctx, creds, sld, tld, req.ProposedRecords); err != nil {
		return nil, err
	}

	return map[string]any{
		"ok":                    true,
		"domain":                fqdn,
		"snapshot_id":           appr.SnapshotID,
		"record_count":          strconv.Itoa(len(req.ProposedRecords)),
		"preserved_email_count": strconv.Itoa(preservedCount),
		"status":                "applied",
	}, nil
}

// PreflightHTTPS verifies DNS resolution matches expected_ip (returning 422 DESK-E304 before ACME)
// and issues an ActionHTTPSApply approval token + Pre-TLS Snapshot when ready.
func (m *Manager) PreflightHTTPS(ctx context.Context, req HTTPSPreflightRequest) (map[string]any, error) {
	fqdn, _, _, _, _, err := parseDomainParts(req.Domain, "", "")
	if err != nil {
		return nil, err
	}
	edgeProxy := normalizeEdgeProxy(req.EdgeProxy)
	if edgeProxy == "" {
		return nil, security.NewError(security.CodeDeskE101, 400, "edge_proxy must be caddy, nginx, or cloudflare-tunnel", nil)
	}

	port := firstNonEmpty(strings.TrimSpace(req.UpstreamPort), "8765")
	if err := validateAppUpstreamPort(port); err != nil {
		return nil, err
	}

	if edgeProxy == "cloudflare-tunnel" {
		return m.buildHTTPSReadyResult(fqdn, edgeProxy, port, req.ExpectedIPv4, nil, true)
	}

	expectedIP := strings.TrimSpace(req.ExpectedIPv4)
	if net.ParseIP(expectedIP) == nil {
		return nil, security.NewError(security.CodeDeskE101, 400, "valid expected_ip is required for ACME DNS preflight", nil)
	}

	var resolved []string
	if len(req.ObservedIPs) > 0 {
		resolved = append(resolved, req.ObservedIPs...)
	} else {
		rCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		ips, lErr := m.resolver(rCtx, fqdn)
		if lErr == nil {
			resolved = ips
		}
	}

	matched := false
	for _, ip := range resolved {
		if strings.TrimSpace(ip) == expectedIP {
			matched = true
			break
		}
	}
	if !matched {
		return nil, security.NewError(
			security.CodeDeskE304,
			422,
			fmt.Sprintf("DNS for %s has not propagated to %s yet (observed: %v) — Let's Encrypt issuance blocked to protect rate limits (DESK-E304)", fqdn, expectedIP, resolved),
			nil,
		)
	}

	return m.buildHTTPSReadyResult(fqdn, edgeProxy, port, expectedIP, resolved, false)
}

func (m *Manager) buildHTTPSReadyResult(fqdn, edgeProxy, port, expectedIP string, resolved []string, tunnelSkip bool) (map[string]any, error) {
	targetKey := "https:" + fqdn
	configText := renderEdgeHTTPSConfig(fqdn, edgeProxy, port, "")
	snap, err := m.approvals.CreatePrefixedVirtualSnapshot("snap_dns_", targetKey, configText)
	if err != nil {
		return nil, err
	}
	appr, err := m.approvals.IssueSnapshotBoundToken(groxybridge.ActionHTTPSApply, targetKey, snap.SnapshotID)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"ok":                 true,
		"domain":             fqdn,
		"edge_proxy":         edgeProxy,
		"upstream":           "127.0.0.1:" + port,
		"expected_ip":        expectedIP,
		"resolved_ips":       resolved,
		"propagated":         true,
		"skipped_for_tunnel": tunnelSkip,
		"snapshot":           snap,
		"approval_token":     appr.Token,
		"rendered_config":    configText,
	}, nil
}

// ApplyHTTPS verifies single-use approval_token + DNS propagation preflight (422 DESK-E304) and emits edge TLS config.
func (m *Manager) ApplyHTTPS(ctx context.Context, req HTTPSApplyRequest) (map[string]any, error) {
	fqdn, _, _, _, _, err := parseDomainParts(req.Domain, "", "")
	if err != nil {
		return nil, err
	}
	edgeProxy := normalizeEdgeProxy(req.EdgeProxy)
	if edgeProxy == "" {
		return nil, security.NewError(security.CodeDeskE101, 400, "edge_proxy must be caddy, nginx, or cloudflare-tunnel", nil)
	}
	port := firstNonEmpty(strings.TrimSpace(req.UpstreamPort), "8765")
	if err := validateAppUpstreamPort(port); err != nil {
		return nil, err
	}

	targetKey := "https:" + fqdn
	appr, err := m.approvals.ConsumeToken(req.ApprovalToken, groxybridge.ActionHTTPSApply, targetKey)
	if err != nil {
		return nil, err
	}

	if edgeProxy != "cloudflare-tunnel" {
		if _, pErr := m.PreflightHTTPS(ctx, HTTPSPreflightRequest{
			Domain:       fqdn,
			ExpectedIPv4: req.ExpectedIPv4,
			EdgeProxy:    edgeProxy,
			UpstreamPort: port,
			ObservedIPs:  req.ObservedIPs,
		}); pErr != nil {
			return nil, pErr
		}
	}

	configText := renderEdgeHTTPSConfig(fqdn, edgeProxy, port, req.Email)
	return map[string]any{
		"ok":              true,
		"domain":          fqdn,
		"edge_proxy":      edgeProxy,
		"upstream":        "127.0.0.1:" + port,
		"snapshot_id":     appr.SnapshotID,
		"rendered_config": configText,
		"status":          "configured",
	}, nil
}

// RestoreDNSSnapshot restores a Pre-DNS / Pre-TLS Snapshot (snap_dns_*) with SHA-256 verification.
func (m *Manager) RestoreDNSSnapshot(ctx context.Context, snapshotID string) (map[string]any, error) {
	restoreRes, err := m.approvals.RestoreSnapshot(m.workspaceRootFallback(), snapshotID)
	if err != nil {
		return nil, err
	}
	var snapDoc struct {
		SLD             string       `json:"sld"`
		TLD             string       `json:"tld"`
		ExistingRecords []HostRecord `json:"existing_records"`
	}
	recordsRestored := false
	if json.Unmarshal([]byte(restoreRes.RestoredContent), &snapDoc) == nil && len(snapDoc.ExistingRecords) > 0 && snapDoc.SLD != "" {
		if creds, cErr := m.decryptVault(); cErr == nil {
			if m.namecheapSetHosts(ctx, creds, snapDoc.SLD, snapDoc.TLD, snapDoc.ExistingRecords) == nil {
				recordsRestored = true
			}
		}
	}
	return map[string]any{
		"ok":               true,
		"snapshot_id":      restoreRes.SnapshotID,
		"restored_sha256":  restoreRes.RestoredSHA256,
		"records_restored": recordsRestored,
		"snapshot":         restoreRes.Snapshot,
	}, nil
}

func (m *Manager) workspaceRootFallback() string {
	return m.stateDir
}

// VerifyEmailRecordsPreserved ensures every existing MX/TXT/SPF/DKIM/DMARC/CAA/SRV record
// remains 100% byte-identical in nextRecords, returning 422 DESK-E303 if any would be dropped or altered.
func VerifyEmailRecordsPreserved(existing, next []HostRecord) (int, error) {
	emailKey := func(r HostRecord) string {
		return strings.ToUpper(strings.TrimSpace(r.Type)) + "|" +
			strings.ToLower(strings.TrimSpace(r.Name)) + "|" +
			strings.TrimSpace(r.Address) + "|" +
			strings.TrimSpace(r.MXPref) + "|" +
			strings.TrimSpace(r.TTL)
	}
	nextCounts := make(map[string]int, len(next))
	for _, r := range next {
		nextCounts[emailKey(r)]++
	}

	preserved := 0
	for _, rec := range existing {
		if !isProtectedEmailOrServiceRecord(rec) {
			continue
		}
		k := emailKey(rec)
		if nextCounts[k] <= 0 {
			return preserved, security.NewError(
				security.CodeDeskE303,
				422,
				fmt.Sprintf("protected email/service record (%s %s -> %s) would be modified or dropped; change blocked (DESK-E303)", rec.Type, rec.Name, rec.Address),
				nil,
			)
		}
		nextCounts[k]--
		preserved++
	}
	return preserved, nil
}

func isProtectedEmailOrServiceRecord(r HostRecord) bool {
	t := strings.ToUpper(strings.TrimSpace(r.Type))
	n := strings.ToLower(strings.TrimSpace(r.Name))
	switch t {
	case "MX", "TXT", "SPF", "DKIM", "DMARC", "CAA", "SRV", "NS":
		return true
	case "CNAME":
		return strings.Contains(n, "_domainkey") || strings.Contains(n, "autodiscover") || strings.Contains(n, "mail") || strings.Contains(n, "_dmarc")
	}
	return false
}

func buildEmailSafeDNSPlan(existing []HostRecord, sub string, req DNSPlanRequest) ([]HostRecord, []map[string]string, error) {
	ttl := firstNonEmpty(strings.TrimSpace(req.TTL), "300")
	next := make([]HostRecord, 0, len(existing)+3)
	diffs := make([]map[string]string, 0)

	targetHosts := map[string]bool{strings.ToLower(sub): true}
	if req.ConfigureWWW && sub == "@" {
		targetHosts["www"] = true
	}

	for _, rec := range existing {
		recType := strings.ToUpper(strings.TrimSpace(rec.Type))
		recName := strings.ToLower(strings.TrimSpace(rec.Name))
		if isProtectedEmailOrServiceRecord(rec) {
			next = append(next, rec)
			diffs = append(diffs, map[string]string{
				"action":  "preserve-byte-identical",
				"name":    rec.Name,
				"type":    rec.Type,
				"address": rec.Address,
			})
			continue
		}
		if targetHosts[recName] && (recType == "A" || recType == "AAAA" || recType == "CNAME") {
			diffs = append(diffs, map[string]string{
				"action":  "replace",
				"name":    rec.Name,
				"type":    rec.Type,
				"address": rec.Address,
			})
			continue
		}
		next = append(next, rec)
	}

	addWebRecord := func(hostName, recType, addr string) error {
		if !reHostName.MatchString(hostName) {
			return security.NewError(security.CodeDeskE101, 400, fmt.Sprintf("invalid DNS host label %q", hostName), nil)
		}
		next = append(next, HostRecord{
			Name:     hostName,
			Type:     recType,
			Address:  addr,
			TTL:      ttl,
			IsActive: "true",
		})
		diffs = append(diffs, map[string]string{
			"action":  "set",
			"name":    hostName,
			"type":    recType,
			"address": addr,
		})
		return nil
	}

	if ip4 := strings.TrimSpace(req.TargetIPv4); ip4 != "" {
		if net.ParseIP(ip4) == nil || net.ParseIP(ip4).To4() == nil {
			return nil, nil, security.NewError(security.CodeDeskE101, 400, "target_ip must be a valid IPv4 address", nil)
		}
		if err := addWebRecord(sub, "A", ip4); err != nil {
			return nil, nil, err
		}
		if req.ConfigureWWW && sub == "@" {
			if err := addWebRecord("www", "A", ip4); err != nil {
				return nil, nil, err
			}
		}
	}
	if ip6 := strings.TrimSpace(req.TargetIPv6); ip6 != "" {
		if net.ParseIP(ip6) == nil || net.ParseIP(ip6).To16() == nil || net.ParseIP(ip6).To4() != nil {
			return nil, nil, security.NewError(security.CodeDeskE101, 400, "target_ipv6 must be a valid IPv6 address", nil)
		}
		if err := addWebRecord(sub, "AAAA", ip6); err != nil {
			return nil, nil, err
		}
	}
	if cname := strings.TrimSpace(req.CNAMETarget); cname != "" && sub != "@" {
		if err := addWebRecord(sub, "CNAME", cname); err != nil {
			return nil, nil, err
		}
	}

	return next, diffs, nil
}

func stripEmailRecords(in []HostRecord) []HostRecord {
	out := make([]HostRecord, 0, len(in))
	for _, r := range in {
		switch strings.ToUpper(strings.TrimSpace(r.Type)) {
		case "MX", "TXT", "SPF", "DKIM", "DMARC", "CAA", "SRV", "NS":
			continue
		case "CNAME":
			n := strings.ToLower(strings.TrimSpace(r.Name))
			if strings.Contains(n, "_domainkey") || strings.Contains(n, "autodiscover") || strings.Contains(n, "mail") || strings.Contains(n, "_dmarc") {
				continue
			}
		}
		out = append(out, r)
	}
	return out
}

func renderEdgeHTTPSConfig(fqdn, edgeProxy, port, email string) string {
	switch edgeProxy {
	case "nginx":
		mail := firstNonEmpty(strings.TrimSpace(email), "admin@"+fqdn)
		return fmt.Sprintf("server {\n  listen 443 ssl http2;\n  server_name %s;\n  ssl_certificate /etc/letsencrypt/live/%s/fullchain.pem;\n  ssl_certificate_key /etc/letsencrypt/live/%s/privkey.pem;\n  location / { proxy_pass http://127.0.0.1:%s; proxy_set_header Host $host; }\n}\n# certbot --nginx -d %s -m %s --agree-tos --non-interactive\n", fqdn, fqdn, fqdn, port, fqdn, mail)
	case "cloudflare-tunnel":
		return fmt.Sprintf("ingress:\n  - hostname: %s\n    service: http://127.0.0.1:%s\n  - service: http_status:404\n", fqdn, port)
	default:
		return fmt.Sprintf("%s {\n  encode zstd gzip\n  reverse_proxy 127.0.0.1:%s\n}\n", fqdn, port)
	}
}

func validateAppUpstreamPort(portStr string) error {
	if !security.IsExactIntString(portStr) {
		return security.NewError(security.CodeDeskE501, 422, "upstream_port must be a base-10 integer string", nil)
	}
	p, err := strconv.Atoi(portStr)
	if err != nil || p < 8765 || p > 8772 {
		return security.NewError(security.CodeDeskE501, 422, "edge Auto-HTTPS upstream_port must target local Grisp port 8765-8772 (DESK-E501)", nil)
	}
	return nil
}

func normalizeEdgeProxy(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "caddy":
		return "caddy"
	case "nginx", "nginx-certbot":
		return "nginx"
	case "cloudflare", "cloudflare-tunnel":
		return "cloudflare-tunnel"
	default:
		return ""
	}
}

type xmlApiResponse struct {
	XMLName xml.Name `xml:"ApiResponse"`
	Status  string   `xml:"Status,attr"`
	Errors  []struct {
		Number  string `xml:"Number,attr"`
		Message string `xml:",chardata"`
	} `xml:"Errors>Error"`
	CommandResponse struct {
		GetHosts *struct {
			Hosts []xmlDNSHost `xml:"host"`
		} `xml:"DomainDNSGetHostsResult"`
		SetHosts *struct {
			IsSuccess string `xml:"IsSuccess,attr"`
		} `xml:"DomainDNSSetHostsResult"`
	} `xml:"CommandResponse"`
}

type xmlDNSHost struct {
	HostID   string `xml:"HostId,attr"`
	Name     string `xml:"Name,attr"`
	Type     string `xml:"Type,attr"`
	Address  string `xml:"Address,attr"`
	MXPref   string `xml:"MXPref,attr"`
	TTL      string `xml:"TTL,attr"`
	IsActive string `xml:"IsActive,attr"`
}

func (m *Manager) namecheapGetHosts(ctx context.Context, creds decryptedCreds, sld, tld string) ([]HostRecord, error) {
	form := url.Values{}
	form.Set("Command", "namecheap.domains.dns.getHosts")
	form.Set("SLD", sld)
	form.Set("TLD", tld)
	parsed, err := m.postNamecheapXML(ctx, creds, form)
	if err != nil {
		return nil, err
	}
	if parsed.CommandResponse.GetHosts == nil {
		return nil, security.NewError(security.CodeDeskE502, 502, "Namecheap getHosts response missing host list", nil)
	}
	out := make([]HostRecord, 0, len(parsed.CommandResponse.GetHosts.Hosts))
	for _, h := range parsed.CommandResponse.GetHosts.Hosts {
		out = append(out, HostRecord{
			HostID:   h.HostID,
			Name:     h.Name,
			Type:     h.Type,
			Address:  h.Address,
			MXPref:   h.MXPref,
			TTL:      h.TTL,
			IsActive: h.IsActive,
		})
	}
	return out, nil
}

func (m *Manager) namecheapSetHosts(ctx context.Context, creds decryptedCreds, sld, tld string, records []HostRecord) error {
	form := url.Values{}
	form.Set("Command", "namecheap.domains.dns.setHosts")
	form.Set("SLD", sld)
	form.Set("TLD", tld)
	for i, r := range records {
		idx := strconv.Itoa(i + 1)
		form.Set("HostName"+idx, r.Name)
		form.Set("RecordType"+idx, r.Type)
		form.Set("Address"+idx, r.Address)
		form.Set("TTL"+idx, firstNonEmpty(r.TTL, "300"))
		if strings.EqualFold(r.Type, "MX") {
			form.Set("MXPref"+idx, firstNonEmpty(r.MXPref, "10"))
		}
	}
	parsed, err := m.postNamecheapXML(ctx, creds, form)
	if err != nil {
		return err
	}
	if parsed.CommandResponse.SetHosts != nil && strings.EqualFold(parsed.CommandResponse.SetHosts.IsSuccess, "false") {
		return security.NewError(security.CodeDeskE502, 502, "Namecheap setHosts returned IsSuccess=false", nil)
	}
	return nil
}

func (m *Manager) postNamecheapXML(ctx context.Context, creds decryptedCreds, form url.Values) (*xmlApiResponse, error) {
	endpoint, err := resolveEndpoint(creds.EndpointOverride, creds.Sandbox)
	if err != nil {
		return nil, err
	}
	form.Set("ApiUser", creds.APIUser)
	form.Set("ApiKey", creds.APIKey)
	form.Set("UserName", creds.APIUser)
	form.Set("ClientIp", creds.ClientIP)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, security.NewError(security.CodeDeskE502, 502, "failed building Namecheap request", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/xml, text/xml")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, security.NewError(security.CodeDeskE502, 502, "Namecheap API gateway unreachable", nil)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, security.NewError(security.CodeDeskE502, 502, "failed reading Namecheap XML response", err)
	}
	var parsed xmlApiResponse
	if err := xml.Unmarshal(raw, &parsed); err != nil {
		return nil, security.NewError(security.CodeDeskE502, 502, "invalid Namecheap XML response", err)
	}
	if !strings.EqualFold(parsed.Status, "OK") || len(parsed.Errors) > 0 {
		msg := "Namecheap API error"
		if len(parsed.Errors) > 0 {
			msg = fmt.Sprintf("Namecheap error %s: %s", parsed.Errors[0].Number, strings.TrimSpace(parsed.Errors[0].Message))
		}
		return nil, security.NewError(security.CodeDeskE502, 502, msg, nil)
	}
	return &parsed, nil
}

func resolveEndpoint(override string, sandbox bool) (string, error) {
	trimmed := strings.TrimSpace(override)
	if trimmed == "" {
		if sandbox {
			return NamecheapSandboxEndpoint, nil
		}
		return NamecheapProdEndpoint, nil
	}
	u, err := url.Parse(trimmed)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return "", security.NewError(security.CodeDeskE501, 422, "invalid Namecheap endpoint URL (DESK-E501)", err)
	}
	host := u.Hostname()
	if trimmed == NamecheapProdEndpoint || trimmed == NamecheapSandboxEndpoint {
		return trimmed, nil
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return "", security.NewError(security.CodeDeskE501, 422, fmt.Sprintf("forbidden Namecheap SSRF host %q; only official Namecheap API or 127.0.0.1 test endpoints allowed (DESK-E501)", host), nil)
	}
	return trimmed, nil
}

func (m *Manager) decryptVault() (decryptedCreds, error) {
	m.mu.RLock()
	v := m.vault
	m.mu.RUnlock()
	if v == nil {
		return decryptedCreds{}, security.NewError(security.CodeDeskE101, 400, "Namecheap credentials are not saved in encrypted vault yet", nil)
	}
	masterKey, err := security.ResolveMasterKey(m.stateDir)
	if err != nil {
		return decryptedCreds{}, err
	}
	u, err := security.DecryptAEAD(masterKey, v.APIUserEnc)
	if err != nil {
		return decryptedCreds{}, err
	}
	k, err := security.DecryptAEAD(masterKey, v.APIKeyEnc)
	if err != nil {
		return decryptedCreds{}, err
	}
	ip, err := security.DecryptAEAD(masterKey, v.ClientIPEnc)
	if err != nil {
		return decryptedCreds{}, err
	}
	return decryptedCreds{
		APIUser:          u,
		APIKey:           k,
		ClientIP:         ip,
		Sandbox:          v.Sandbox,
		EndpointOverride: v.EndpointOverride,
	}, nil
}

func (m *Manager) loadVault() error {
	path := filepath.Join(m.stateDir, vaultFileName)
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var v storedVault
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil
	}
	m.vault = &v
	return nil
}

func (m *Manager) saveVaultLocked() error {
	raw, err := json.MarshalIndent(m.vault, "", "  ")
	if err != nil {
		return err
	}
	tmp := filepath.Join(m.stateDir, vaultFileName+".tmp")
	dest := filepath.Join(m.stateDir, vaultFileName)
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, dest)
}

func parseDomainParts(domainRaw, rootRaw, subRaw string) (fqdn, rootDomain, sld, tld, sub string, err error) {
	dom := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(domainRaw), "."))
	root := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(rootRaw), "."))
	sub = strings.ToLower(strings.TrimSpace(subRaw))
	if dom == "" && root == "" {
		return "", "", "", "", "", security.NewError(security.CodeDeskE101, 400, "domain is required", nil)
	}
	target := dom
	if target == "" {
		target = root
	}
	labels := strings.Split(target, ".")
	if len(labels) < 2 {
		return "", "", "", "", "", security.NewError(security.CodeDeskE101, 400, fmt.Sprintf("invalid domain %q", target), nil)
	}
	for _, lbl := range labels {
		if !reDomainLabel.MatchString(lbl) {
			return "", "", "", "", "", security.NewError(security.CodeDeskE101, 400, fmt.Sprintf("invalid domain label %q", lbl), nil)
		}
	}
	if root == "" {
		if len(labels) >= 3 && sub == "" {
			sub = labels[0]
			root = strings.Join(labels[1:], ".")
		} else {
			root = target
			if sub == "" {
				sub = "@"
			}
		}
	} else if sub == "" {
		if target != root && strings.HasSuffix(target, "."+root) {
			sub = strings.TrimSuffix(target, "."+root)
		} else {
			sub = "@"
		}
	}
	rootLabels := strings.Split(root, ".")
	sld = rootLabels[0]
	tld = strings.Join(rootLabels[1:], ".")
	if sub == "@" {
		fqdn = root
	} else {
		fqdn = sub + "." + root
	}
	return fqdn, root, sld, tld, sub, nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
