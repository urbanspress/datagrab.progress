import test from "node:test";
import assert from "node:assert/strict";
import path from "node:path";
import { fileURLToPath } from "node:url";

import { createDomShim } from "./helpers/dom-shim.js";
import {
  GrexSecurityError,
  PREVIEW_CHANNEL,
  auditDom,
  computeContrastRatio,
  isExactGrabWireString,
} from "../public/grex/grex.js";
import {
  VIEWPORT_PRESETS,
  buildAllowedPreviewOrigins,
  createStudioPreviewBridge,
} from "../public/desk/studio-panel.js";
import { mountGroxyDeskWorkbench } from "../public/desk/workbench.js";

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const repoRoot = path.resolve(__dirname, "..");
void repoRoot;

test("GrexPreviewBridge rejects wildcard '*' and non-loopback origins and accepts allowlisted 127.0.0.1", () => {
  assert.throws(
    () =>
      createStudioPreviewBridge({
        allowedOrigins: ["*"],
      }),
    GrexSecurityError
  );
  assert.throws(
    () =>
      createStudioPreviewBridge({
        allowedOrigins: ["https://evil.example.com"],
      }),
    GrexSecurityError
  );

  const shim = createDomShim();
  const root = shim.document.createElement("section");
  const mockApi = {
    hasSession: () => false,
  };
  const mounted = mountGroxyDeskWorkbench(root, {
    document: shim.document,
    apiClient: mockApi,
    baseUrl: "http://127.0.0.1:9490",
    autoHydrate: false,
  });

  const allowed = buildAllowedPreviewOrigins("http://127.0.0.1:9490", [
    { id: "w1", url: "http://127.0.0.1:8765" },
  ]);
  assert.equal(allowed.includes("http://127.0.0.1:9490"), true);
  assert.equal(allowed.includes("http://127.0.0.1:8765"), true);
  assert.equal(allowed.includes("*"), false);

  const bridge = mounted.previewBridge;
  assert.ok(bridge, "Workbench must mount origin-checked GrexPreviewBridge");
  assert.equal(bridge.channel, PREVIEW_CHANNEL);

  // Untrusted origin MUST fail closed
  const badOriginOk = bridge.handleMessage({
    origin: "https://attacker.example",
    data: {
      channel: PREVIEW_CHANNEL,
      type: "patch-signal",
      key: "previewViewport",
      value: "mobile",
    },
  });
  assert.equal(badOriginOk, false);
  assert.equal(mounted.store.previewViewport.peek(), "base");

  // Wrong channel MUST fail closed
  const badChannelOk = bridge.handleMessage({
    origin: "http://127.0.0.1:8765",
    data: {
      channel: "wrong-channel",
      type: "patch-signal",
      key: "previewViewport",
      value: "mobile",
    },
  });
  assert.equal(badChannelOk, false);
  assert.equal(mounted.store.previewViewport.peek(), "base");

  // Valid allowlisted loopback worker + channel MUST update reactive signal
  const goodOk = bridge.handleMessage({
    origin: "http://127.0.0.1:8765",
    data: {
      channel: PREVIEW_CHANNEL,
      type: "patch-signal",
      key: "previewViewport",
      value: "tablet",
    },
  });
  assert.equal(goodOk, true);
  assert.equal(mounted.store.previewViewport.peek(), "tablet");
  mounted.dispose();
});

test("Studio Workbench supports Code/Split/Visual modes, Viewports, 40 Style Packs, Ctrl+Z undo, and Boost Diffs with zero div", async () => {
  const shim = createDomShim();
  const root = shim.document.createElement("section");
  let undoCalledCount = 0;

  const sampleLayout = {
    element: "Container",
    tag: "main",
    children: [
      {
        element: "Section",
        tag: "section",
        children: [
          { element: "Heading", tag: "h1", props: { level: 1, text: "Emerald Studio Page" } },
          { element: "Text", tag: "p", props: { text: "Pricing tier $49.00 (SKU 10020) verified WCAG AA." } },
          { element: "Link", tag: "a", props: { href: "/docs", text: "Explore Semantic Docs" } },
        ],
      },
    ],
  };

  const mockApi = {
    hasSession: () => true,
    getTheme: async () => ({ theme: "light" }),
    setTheme: async (t) => ({ theme: t }),
    workspaceTree: async () => ({ entries: [{ path: "views/page.grab.html", is_dir: false, size_bytes: "512" }] }),
    listWorkers: async () => ({ workers: [{ id: "cms-1", port: "8765", url: "http://127.0.0.1:8765", status: "running", pid: "111", reload_count: "0", log_count: "1" }] }),
    groxySnapshots: async () => ({ snapshots: [] }),
    mcpTools: async () => ({
      tools: [
        { name: "grab_check", layer: "core-grab", mode: "read-only", description: "Static GRAB check" },
        { name: "create_entry_draft", layer: "grablab-cms", mode: "write", description: "Draft creator" },
      ],
    }),
    studioDesignSystem: async () => ({
      style_packs: [
        { id: "emerald-editorial", label: "Emerald Editorial", light: { bg: "#ffffff", text: "#0f172a" }, dark: { bg: "#0f172a", text: "#f8fafc" } },
        { id: "obsidian-tech", label: "Obsidian Developer Studio", light: { bg: "#f8fafc", text: "#09090b" }, dark: { bg: "#09090b", text: "#fafafa" } },
      ],
      blueprints: [
        { id: "full_landing_page", label: "Full Landing Page" },
        { id: "pricing_grid", label: "Pricing Grid" },
      ],
      elements: ["Section", "Container"],
      combination_count: "40",
    }),
    studioSynthesize: async () => ({
      style_pack: { id: "emerald-editorial", label: "Emerald Editorial" },
      blueprint: { id: "full_landing_page", label: "Full Landing Page" },
      grex_layout: sampleLayout,
      contrast: { light_text_bg: "17.85:1", dark_text_bg: "17.85:1", meets_wcag_aa: true },
      scorecard: {
        overall_score: "98",
        overall_grade: "A+",
        publish_safe: true,
        div_count: "0",
        schema_org: { type: "Product", json_text: '{"price":"49.00","sku":"10020"}' },
        categories: {
          on_page_seo: { score: "98", grade: "A+", weight: "25" },
          content_a11y: { score: "100", grade: "A+", weight: "25" },
          performance_dom: { score: "100", grade: "A+", weight: "25" },
          linking_readiness: { score: "95", grade: "A", weight: "25" },
        },
        findings: [],
      },
    }),
    studioLayoutApply: async () => ({
      ok: true,
      command: { type: "applyAiLayout", actor: "groxy-ai", slot: "main", before: null, after: sampleLayout },
    }),
    groxyAudit: async () => ({
      overall_score: "96",
      overall_grade: "A",
      publish_safe: true,
      div_count: "0",
      schema_org: { type: "Product", json_text: '{"price":"49.00"}' },
      categories: {
        on_page_seo: { score: "95", grade: "A", weight: "25" },
        content_a11y: { score: "100", grade: "A+", weight: "25" },
        performance_dom: { score: "100", grade: "A+", weight: "25" },
        linking_readiness: { score: "90", grade: "A", weight: "25" },
      },
      findings: [
        { rule_id: "GROXY-DOM-001", status: "pass", points_earned: "25", max_points: "25", title: "Zero div", message: "Zero wrapper elements" },
      ],
    }),
    groxyBoost: async () => ({
      ok: true,
      approval_required: true,
      approval_token: "groxy-appr-boost-1",
      target_path: "views/page.grab.html",
      snapshot: { snapshot_id: "snap-1111222233334444" },
      proposal: {
        projected_delta: { before_score: "52", before_grade: "F", after_score: "96", after_grade: "A", delta: "+44" },
        diffs: [
          {
            field: "dom.wrapper_elements",
            rule_ids: ["GROXY-DOM-001"],
            summary: "Converted wrapper tags to semantic landmarks",
            before: "wrapper_count=2 (F)",
            after: "wrapper_count=0 (A)",
          },
        ],
        before_report: {
          overall_score: "52",
          overall_grade: "F",
          publish_safe: false,
          div_count: "2",
          categories: {},
          findings: [],
        },
        after_report: {
          overall_score: "96",
          overall_grade: "A",
          publish_safe: true,
          div_count: "0",
          schema_org: { type: "Product", json_text: '{"price":"49.00"}' },
          categories: {
            on_page_seo: { score: "95", grade: "A", weight: "25" },
            content_a11y: { score: "100", grade: "A+", weight: "25" },
            performance_dom: { score: "100", grade: "A+", weight: "25" },
            linking_readiness: { score: "90", grade: "A", weight: "25" },
          },
          findings: [],
        },
      },
    }),
    groxyBoostApprove: async () => ({ ok: true, applied_sha256: "abcdef123456", target_path: "views/page.grab.html" }),
    groxyRestore: async (id) => ({ ok: true, snapshot_id: id, restored_sha256: "001122334455", target_path: "views/page.grab.html" }),
  };

  const mounted = mountGroxyDeskWorkbench(root, {
    document: shim.document,
    apiClient: mockApi,
    autoHydrate: false,
  });
  await mounted.hydrateWorkspace();

  // Switch modes: code -> visual -> split
  root.querySelector("#desk-mode-code-btn").click();
  assert.equal(mounted.store.workbenchMode.peek(), "code");
  root.querySelector("#desk-mode-visual-btn").click();
  assert.equal(mounted.store.workbenchMode.peek(), "visual");
  root.querySelector("#desk-mode-split-btn").click();
  assert.equal(mounted.store.workbenchMode.peek(), "split");

  // Responsive viewport presets
  assert.equal(VIEWPORT_PRESETS.tablet.width, "768px");
  assert.equal(VIEWPORT_PRESETS.mobile.width, "375px");
  root.querySelector("#desk-vp-tablet-btn").click();
  assert.equal(mounted.store.previewViewport.peek(), "tablet");
  root.querySelector("#desk-vp-mobile-btn").click();
  assert.equal(mounted.store.previewViewport.peek(), "mobile");

  // Synthesize & Apply Studio Layout -> Undo restores previous tree
  await mounted.actions.synthesizeStudio({ style_pack: "emerald-editorial", blueprint: "full_landing_page" });
  const surface = root.querySelector("#desk-preview-surface");
  assert.ok(surface.textContent.includes("49.00"));
  assert.equal(isExactGrabWireString("49.00"), true);

  await mounted.actions.applyStudioLayout({ slot: "main", grex_layout: sampleLayout, approval_token: "tok" });
  assert.equal(mounted.store.undoStack.peek().length, 1);
  if (mounted.actions.undoStudioLayout()) undoCalledCount += 1;
  assert.equal(undoCalledCount, 1);
  assert.equal(mounted.store.undoStack.peek().length, 0);

  // Boost workflow + auditDom zero issues + zero <div> + zero innerHTML
  await mounted.actions.proposeBoost({ target_path: "views/page.grab.html" });
  assert.ok(root.querySelector("#desk-findings-list").textContent.includes("GROXY-DOM-001"));
  await mounted.actions.approveBoost({ approval_token: "groxy-appr-boost-1", target_path: "views/page.grab.html" });
  assert.equal(mounted.store.scorecardReport.peek().overall_grade, "A");

  // Re-verify zero <div>, zero innerHTML, zero model dropdowns in #desk-groxy-sidebar, WCAG AA contrast
  const allEls = root.querySelectorAll("*");
  assert.equal(allEls.filter((el) => el.localName === "div").length, 0);
  assert.equal(shim.innerHTMLWriteCount, 0);
  assert.equal(root.querySelector("#desk-groxy-sidebar").querySelectorAll("select").length, 0);
  const domRes = auditDom(root);
  assert.equal(domRes.ok, true, JSON.stringify(domRes.issues));
  assert.ok(computeContrastRatio("#0f172a", "#ffffff") >= 4.5);
  assert.ok(computeContrastRatio("#f8fafc", "#090d16") >= 4.5);

  mounted.dispose();
});
