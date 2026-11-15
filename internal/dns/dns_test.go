package dns

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"groxy-desk/internal/groxybridge"
	"groxy-desk/internal/security"
)

func TestVaultEncryptionAndSSRFSafety(t *testing.T) {
	stateDir := t.TempDir()
	approvals, _ := groxybridge.NewApprovalManager(stateDir, nil)
	mgr, err := NewManager(stateDir, approvals, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Non-loopback custom override -> DESK-E501
	_, ssrfErr := mgr.SaveVault(VaultSaveRequest{
		APIUser:          "author",
		APIKey:           "nc_super_secret_1234",
		ClientIP:         "198.51.100.1",
		EndpointOverride: "https://evil.attacker.example/xml",
	})
	if ssrfErr == nil || security.ErrorCode(ssrfErr, "") != security.CodeDeskE501 {
		t.Fatalf("expected DESK-E501 for non-Namecheap host, got %v", ssrfErr)
	}

	res, err := mgr.SaveVault(VaultSaveRequest{
		APIUser:  "author",
		APIKey:   "nc_super_secret_1234",
		ClientIP: "198.51.100.1",
	})
	if err != nil || res["api_key_masked"] != "nc_****...1234" {
		t.Fatalf("unexpected SaveVault result: %+v err=%v", res, err)
	}
	rawDisk, _ := os.ReadFile(filepath.Join(stateDir, vaultFileName))
	if strings.Contains(string(rawDisk), "nc_super_secret_1234") || !strings.Contains(string(rawDisk), `"v1:`) {
		t.Fatalf("vault file not encrypted with AES-256-GCM v1: format: %s", string(rawDisk))
	}
}

func TestDNSPlanEmailPreservationPreflightAndHTTPSApply(t *testing.T) {
	stateDir := t.TempDir()
	redactor := security.NewSecretRedactor()
	approvals, _ := groxybridge.NewApprovalManager(stateDir, redactor)
	mgr, _ := NewManager(stateDir, approvals, redactor)

	var lastCommand string
	var setHostsBody string
	mockNC := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		lastCommand = r.Form.Get("Command")
		switch lastCommand {
		case "namecheap.domains.dns.getHosts":
			_, _ = w.Write([]byte(`<?xml version="1.0" encoding="utf-8"?>
<ApiResponse Status="OK">
  <Errors/>
  <CommandResponse Type="namecheap.domains.dns.getHosts">
    <DomainDNSGetHostsResult Domain="example.org">
      <host HostId="1" Name="@" Type="A" Address="192.0.2.10" TTL="300"/>
      <host HostId="2" Name="@" Type="MX" Address="mx1.fastmail.com." MXPref="10" TTL="1800"/>
      <host HostId="3" Name="@" Type="TXT" Address="v=spf1 include:spf.messagingengine.com ~all" TTL="1800"/>
      <host HostId="4" Name="fm1._domainkey" Type="CNAME" Address="fm1.example.org.dkim.fmhosted.com." TTL="1800"/>
    </DomainDNSGetHostsResult>
  </CommandResponse>
</ApiResponse>`))
		case "namecheap.domains.dns.setHosts":
			setHostsBody = r.Form.Encode()
			_, _ = w.Write([]byte(`<?xml version="1.0" encoding="utf-8"?>
<ApiResponse Status="OK"><Errors/><CommandResponse Type="namecheap.domains.dns.setHosts"><DomainDNSSetHostsResult Domain="example.org" IsSuccess="true"/></CommandResponse></ApiResponse>`))
		}
	}))
	defer mockNC.Close()

	_, err := mgr.SaveVault(VaultSaveRequest{
		APIUser:          "author",
		APIKey:           "secret-key-9999",
		ClientIP:         "198.51.100.20",
		EndpointOverride: mockNC.URL,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Simulated email record drop -> 422 DESK-E303
	_, dropErr := mgr.PlanDNS(context.Background(), DNSPlanRequest{
		Domain:          "example.org",
		TargetIPv4:      "203.0.113.42",
		SimulatedDropMX: true,
	})
	if dropErr == nil || security.ErrorCode(dropErr, "") != security.CodeDeskE303 || security.ErrorStatus(dropErr, 0) != 422 {
		t.Fatalf("expected 422 DESK-E303 on MX drop, got %v", dropErr)
	}

	plan, err := mgr.PlanDNS(context.Background(), DNSPlanRequest{
		Domain:       "example.org",
		TargetIPv4:   "203.0.113.42",
		ConfigureWWW: true,
	})
	if err != nil {
		t.Fatalf("PlanDNS failed: %v", err)
	}
	if plan["preserved_email_count"] != "3" {
		t.Fatalf("expected 3 preserved email records (MX, TXT, DKIM), got %v", plan["preserved_email_count"])
	}

	proposed := plan["proposed_records"].([]HostRecord)

	// Verify explicit SPF/DKIM/DMARC types are protected and stripped cleanly in simulation mode
	for _, recType := range []string{"SPF", "DKIM", "DMARC"} {
		rec := HostRecord{Name: "@", Type: recType, Address: "v=test", TTL: "1800"}
		if !isProtectedEmailOrServiceRecord(rec) {
			t.Fatalf("expected explicit %s record type to be protected", recType)
		}
		if len(stripEmailRecords([]HostRecord{rec})) != 0 {
			t.Fatalf("expected stripEmailRecords to drop %s record during simulation", recType)
		}
	}

	// Tampered ApplyDNS payload omitting an email record MUST fail 422 DESK-E303 prior to consuming approval_token
	tampered := make([]HostRecord, 0, len(proposed))
	for _, rec := range proposed {
		if !strings.EqualFold(rec.Type, "MX") {
			tampered = append(tampered, rec)
		}
	}
	_, tamperedErr := mgr.ApplyDNS(context.Background(), DNSApplyRequest{
		Domain:          "example.org",
		ApprovalToken:   plan["approval_token"].(string),
		ProposedRecords: tampered,
	})
	if tamperedErr == nil || security.ErrorCode(tamperedErr, "") != security.CodeDeskE303 || security.ErrorStatus(tamperedErr, 0) != 422 {
		t.Fatalf("expected 422 DESK-E303 on tampered ApplyDNS payload, got %v", tamperedErr)
	}
	if setHostsBody != "" {
		t.Fatalf("setHosts must not be called when tampered payload fails 422 DESK-E303")
	}

	applyRes, err := mgr.ApplyDNS(context.Background(), DNSApplyRequest{
		Domain:          "example.org",
		ApprovalToken:   plan["approval_token"].(string),
		ProposedRecords: proposed,
	})
	if err != nil || applyRes["status"] != "applied" {
		t.Fatalf("ApplyDNS failed: %+v err=%v", applyRes, err)
	}
	if !strings.Contains(setHostsBody, "mx1.fastmail.com.") || !strings.Contains(setHostsBody, "v%3Dspf1") {
		t.Fatalf("setHosts dropped email records: %s", setHostsBody)
	}

	// DNS not propagated -> 422 DESK-E304 before ACME
	_, propErr := mgr.PreflightHTTPS(context.Background(), HTTPSPreflightRequest{
		Domain:       "example.org",
		ExpectedIPv4: "203.0.113.42",
		EdgeProxy:    "caddy",
		UpstreamPort: "8765",
		ObservedIPs:  []string{"192.0.2.10"},
	})
	if propErr == nil || security.ErrorCode(propErr, "") != security.CodeDeskE304 || security.ErrorStatus(propErr, 0) != 422 {
		t.Fatalf("expected 422 DESK-E304 when unpropagated, got %v", propErr)
	}

	// Converged DNS -> Preflight issues token -> ApplyHTTPS succeeds
	preOK, err := mgr.PreflightHTTPS(context.Background(), HTTPSPreflightRequest{
		Domain:       "example.org",
		ExpectedIPv4: "203.0.113.42",
		EdgeProxy:    "caddy",
		UpstreamPort: "8765",
		ObservedIPs:  []string{"203.0.113.42"},
	})
	if err != nil {
		t.Fatalf("PreflightHTTPS failed: %v", err)
	}
	httpsRes, err := mgr.ApplyHTTPS(context.Background(), HTTPSApplyRequest{
		Domain:        "example.org",
		ExpectedIPv4:  "203.0.113.42",
		EdgeProxy:     "caddy",
		UpstreamPort:  "8765",
		ApprovalToken: preOK["approval_token"].(string),
		ObservedIPs:   []string{"203.0.113.42"},
	})
	if err != nil || !strings.Contains(httpsRes["rendered_config"].(string), "reverse_proxy 127.0.0.1:8765") {
		t.Fatalf("ApplyHTTPS failed: %+v err=%v", httpsRes, err)
	}

	snapMap := plan["snapshot"].(groxybridge.SnapshotRecord)
	rb, err := mgr.RestoreDNSSnapshot(context.Background(), snapMap.SnapshotID)
	rawRB, _ := json.Marshal(rb)
	if err != nil || rb["records_restored"] != true {
		t.Fatalf("RestoreDNSSnapshot failed: %s err=%v", string(rawRB), err)
	}
}
