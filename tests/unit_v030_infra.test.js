import assert from "node:assert/strict";
import test from "node:test";

import { auditDom, isExactGrabWireString } from "../public/grex/grex.js";
import { mountGroxyDeskWorkbench } from "../public/desk/workbench.js";
import { createDomShim } from "./helpers/dom-shim.js";

test("v0.3.0 Fleet, DB Studio, Namecheap DNS/HTTPS and Author panels enforce Pure Grex, ADR-025c and single-use safety", async () => {
  const shim = createDomShim("light");
  const root = shim.document.createElement("section");
  shim.document.body.appendChild(root);

  const calls = {
    fleetPlan: 0,
    fleetAppliedToken: "",
    dbRows: [],
    dnsVaultSaved: false,
    dnsApplied: false,
    authorApplied: false,
  };

  const mockApi = {
    hasSession: () => true,
    getTheme: async () => ({ theme: "light" }),
    setTheme: async ({ theme }) => ({ theme }),
    workspaceTree: async () => ({ entries: [] }),
    listWorkers: async () => ({ workers: [] }),
    groxySnapshots: async () => ({ snapshots: [] }),
    mcpTools: async () => ({ tools: [] }),
    studioDesignSystem: async () => ({ style_packs: [], blueprints: [] }),
    fleetNodes: async () => ({ nodes: [] }),
    fleetVerifyNode: async (payload) => ({
      ok: true,
      node: {
        node_id: payload.node_id,
        host: payload.host,
        port: payload.port,
        pinned_fingerprint: payload.pinned_fingerprint,
        control_url: payload.control_url,
        status: "verified",
      },
    }),
    fleetValidateDNA: async () => ({
      ok: true,
      summary: "Site DNA groxy-site-dna-v0.4.0 verified cleanly.",
    }),
    fleetDeployPlan: async () => {
      calls.fleetPlan += 1;
      return {
        ok: true,
        node_id: "vps-prod-1",
        app_name: "grablab-cms",
        approval_token: "groxy-appr-fleet-99",
        snapshot: { snapshot_id: "snap_deploy_0011223344556677" },
        plain_summary: "Deployment plan ready with Pre-Deploy Snapshot snap_deploy_0011223344556677.",
      };
    },
    fleetDeployApply: async (payload) => {
      calls.fleetAppliedToken = payload.approval_token;
      return { ok: true, app_name: payload.app_name, drain_timeout_sec: "30" };
    },
    dbTunnels: async () => ({
      tunnels: [
        {
          tunnel_id: "tun-1",
          engine: "postgres",
          local_bind_addr: "127.0.0.1:15432",
          ttl_seconds: "900",
        },
      ],
    }),
    dbRows: async () => {
      const rows = [
        { id: "1", isbn: "9780143127550", price: "19.99", word_count: "9007199254740993" },
      ];
      calls.dbRows = rows;
      return {
        ok: true,
        table: "books",
        page: "1",
        total_pages: "1",
        total_rows: "1",
        rows,
      };
    },
    dnsVaultSave: async () => {
      calls.dnsVaultSaved = true;
      return { ok: true, api_key_masked: "nc_****...1234" };
    },
    dnsPlan: async () => ({
      ok: true,
      domain: "example.org",
      preserved_email_count: "2",
      approval_token: "groxy-appr-dns-88",
      proposed_records: [],
      diffs: [
        { action: "preserve-byte-identical", type: "MX", name: "@", address: "mx1.example.net." },
        { action: "set", type: "A", name: "@", address: "203.0.113.42" },
      ],
      plain_summary: "DNS plan ready for example.org: 2 email record(s) verified byte-identical.",
    }),
    dnsApply: async () => {
      calls.dnsApplied = true;
      return { ok: true, record_count: "2", preserved_email_count: "2" };
    },
    httpsPreflight: async () => ({
      ok: true,
      domain: "example.org",
      edge_proxy: "caddy",
      upstream: "127.0.0.1:8765",
      approval_token: "groxy-appr-https-77",
    }),
    httpsApply: async () => ({
      ok: true,
      domain: "example.org",
      edge_proxy: "caddy",
      upstream: "127.0.0.1:8765",
    }),
    authorInitPlan: async () => ({
      ok: true,
      target_subdir: "author-studio",
      approval_token: "groxy-appr-author-55",
      schema_org_jsonld:
        '{"@context":"https://schema.org","@type":"Book","isbn":"9780143127550","offers":{"@type":"Offer","price":"19.99","priceCurrency":"USD"}}',
      planned_files: [{ path: "author-studio/author-schemas.json", byte_count: "670" }],
      plain_summary: "Prepared Local Author Mode safely in author-studio.",
    }),
    authorInitApply: async () => {
      calls.authorApplied = true;
      return { ok: true, target_subdir: "author-studio", status_enforced: "draft" };
    },
    authorOllamaStatus: async () => ({
      ok: true,
      running: true,
      loopback_url: "http://127.0.0.1:11434",
      detected_count: "2",
      model_switching_allowed: false,
      plain_summary: "Ollama ready at 127.0.0.1:11434 ($0 fee).",
    }),
  };

  const mounted = mountGroxyDeskWorkbench(root, {
    document: shim.document,
    apiClient: mockApi,
    autoHydrate: false,
  });
  await mounted.hydrateWorkspace();

  // Trigger v0.3.0 UI flows
  await root.querySelector("#fleet-dna-btn").click();
  await root.querySelector("#fleet-plan-btn").click();
  await root.querySelector("#fleet-apply-btn").click();
  assert.equal(calls.fleetPlan, 1);
  assert.equal(calls.fleetAppliedToken, "groxy-appr-fleet-99");

  await root.querySelector("#db-rows-btn").click();
  const dbPanelText = root.querySelector("#desk-db-studio-panel").textContent;
  assert.equal(dbPanelText.includes("9007199254740993"), true);
  assert.equal(isExactGrabWireString("9007199254740993"), true);

  await root.querySelector("#dns-plan-btn").click();
  assert.equal(root.querySelector("#desk-dns-panel").textContent.includes("preserve-byte-identical"), true);
  await root.querySelector("#dns-apply-btn").click();
  await root.querySelector("#dns-https-preflight-btn").click();
  await root.querySelector("#dns-https-apply-btn").click();
  assert.equal(calls.dnsApplied, true);

  await root.querySelector("#author-plan-btn").click();
  await root.querySelector("#author-apply-btn").click();
  assert.equal(calls.authorApplied, true);
  assert.equal(root.querySelector("#author-jsonld-preview").textContent.includes('"19.99"'), true);

  // Strict DOM & WCAG audit after all interactions
  const divCount = root.querySelectorAll("*").filter((el) => el.localName === "div").length;
  assert.equal(divCount, 0);
  assert.equal(shim.innerHTMLWriteCount, 0);
  const domAudit = auditDom(root);
  assert.equal(domAudit.ok, true, `auditDom failed: ${JSON.stringify(domAudit.issues)}`);
});
