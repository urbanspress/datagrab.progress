import assert from "node:assert/strict";
import crypto from "node:crypto";
import fs from "node:fs";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

globalThis.__GROXY_DESK_NO_AUTO_MOUNT__ = true;

import {
  auditDom,
  isExactGrabWireString,
} from "../public/grex/grex.js";
import { verifyPaletteContrast } from "../public/desk/theme.js";
import { mountGroxyDeskWorkbench } from "../public/desk/workbench.js";
import { createDomShim } from "./helpers/dom-shim.js";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(__dirname, "..");

const ALLOWED_SEMANTIC_TAGS = new Set([
  "section",
  "header",
  "footer",
  "main",
  "aside",
  "article",
  "nav",
  "h1",
  "h2",
  "h3",
  "h4",
  "h5",
  "h6",
  "p",
  "span",
  "small",
  "strong",
  "em",
  "blockquote",
  "time",
  "code",
  "a",
  "button",
  "img",
  "svg",
  "path",
  "circle",
  "rect",
  "line",
  "polyline",
  "polygon",
  "form",
  "label",
  "input",
  "textarea",
  "select",
  "option",
  "ul",
  "ol",
  "li",
  "table",
  "thead",
  "tbody",
  "tr",
  "th",
  "td",
  "caption",
  "dialog",
  "details",
  "summary",
]);

function sha256File(filePath) {
  const bytes = fs.readFileSync(filePath);
  return crypto.createHash("sha256").update(bytes).digest("hex").toUpperCase();
}

test("Vendored Grex JS v1.0.0 runtime and element catalog match C:\\Development\\grex-js byte-for-byte", () => {
  const localGrexJs = path.join(repoRoot, "public", "grex", "grex.js");
  const localElementsJson = path.join(repoRoot, "public", "grex", "grex.elements.json");

  assert.equal(
    sha256File(localGrexJs),
    "C1E28D74A7ECF19182E225644FA244AA8600658DD1FF8895C133BFC64976B6D6"
  );
  assert.equal(
    sha256File(localElementsJson),
    "C54C4BCF15EC8FEA2DEC4EF41975BA85A4B555DD400604E59C78883A8BBDEBC9"
  );

  const upstreamGrexJs = "C:\\Development\\grex-js\\dist\\grex-js-v1.0.0.js";
  const upstreamElementsJson = "C:\\Development\\grex-js\\grex.elements.json";
  if (fs.existsSync(upstreamGrexJs)) {
    assert.equal(sha256File(localGrexJs), sha256File(upstreamGrexJs));
  }
  if (fs.existsSync(upstreamElementsJson)) {
    assert.equal(sha256File(localElementsJson), sha256File(upstreamElementsJson));
  }
});

test("Static view and all public/desk modules forbid <div>, innerHTML, eval, and unsafe URLs", () => {
  const targetFiles = [
    path.join(repoRoot, "views", "workbench.grab.html"),
    path.join(repoRoot, "public", "desk", "desk.css"),
    path.join(repoRoot, "public", "desk", "theme.js"),
    path.join(repoRoot, "public", "desk", "api-client.js"),
    path.join(repoRoot, "public", "desk", "workspace-panel.js"),
    path.join(repoRoot, "public", "desk", "supervisor-panel.js"),
    path.join(repoRoot, "public", "desk", "groxy-agent.js"),
    path.join(repoRoot, "public", "desk", "studio-panel.js"),
    path.join(repoRoot, "public", "desk", "scorecard-panel.js"),
    path.join(repoRoot, "public", "desk", "fleet-panel.js"),
    path.join(repoRoot, "public", "desk", "db-studio-panel.js"),
    path.join(repoRoot, "public", "desk", "radar-panel.js"),
    path.join(repoRoot, "public", "desk", "dns-panel.js"),
    path.join(repoRoot, "public", "desk", "author-panel.js"),
    path.join(repoRoot, "public", "desk", "workbench.js"),
  ];

  for (const file of targetFiles) {
    const content = fs.readFileSync(file, "utf8");
    const rel = path.relative(repoRoot, file);
    assert.equal(/<div\b/i.test(content), false, `${rel} contains forbidden <div`);
    assert.equal(/\binnerHTML\b/i.test(content), false, `${rel} mentions forbidden innerHTML`);
    assert.equal(/\bouterHTML\b/i.test(content), false, `${rel} mentions forbidden outerHTML`);
    assert.equal(/insertAdjacentHTML/i.test(content), false, `${rel} mentions forbidden insertAdjacentHTML`);
    assert.equal(/\beval\s*\(/i.test(content), false, `${rel} contains forbidden eval()`);
    assert.equal(/new\s+Function\s*\(/i.test(content), false, `${rel} contains forbidden new Function()`);
    assert.equal(/\bon(?:click|load|error|mouseover)\s*=/i.test(content), false, `${rel} contains inline event attr`);
    assert.equal(/\b(?:javascript|vbscript|data:text\/html):/i.test(content), false, `${rel} contains unsafe URL scheme`);
  }
});

test("Light and Dark theme palettes achieve 100% WCAG AA (>= 4.5:1) contrast", () => {
  const report = verifyPaletteContrast();
  for (const r of report.results) {
    assert.equal(
      r.passAA,
      true,
      `Contrast failed for ${r.mode}.${r.pair}: ${r.ratio.toFixed(2)}:1 (< 4.5)`
    );
  }
  assert.equal(report.ok, true);
});

test("Mounted Pure Grex JS workbench satisfies zero-<div>, zero-innerHTML, WCAG DOM audit, ADR-025c, and zero model-dropdowns", async () => {
  const shim = createDomShim("light");
  const root = shim.document.createElement("section");
  root.setAttribute("id", "desk-root");
  shim.document.body.appendChild(root);

  let savedTheme = "light";
  const mockApi = {
    hasSession: () => true,
    setSession: () => {},
    getTheme: async () => ({ theme: savedTheme }),
    setTheme: async (next) => {
      savedTheme = next;
      return { ok: true, theme: next };
    },
    workspaceTree: async () => ({
      entries: [
        { name: "desk.grab", path: "src/desk.grab", is_dir: false, size_bytes: "312" },
        { name: "workbench.grab.html", path: "views/workbench.grab.html", is_dir: false, size_bytes: "942" },
      ],
    }),
    workspaceRead: async (p) => ({
      path: p,
      content: "route \"GET\" \"/_health\"",
      sha256: "abc123",
      size_bytes: "24",
    }),
    workspaceCheck: async () => ({
      ok: true,
      grab_check_ok: true,
      format_clean: true,
      diagnostics: "grab check clean",
    }),
    grabmanInitPlan: async () => ({ approval_token: "tok-init", target_dir: "demo" }),
    grabmanInit: async () => ({ ok: true, plain_summary: "Initialized demo" }),
    grabmanDoctor: async () => ({ ok: true, exit_code: "0", plain_summary: "Doctor clean" }),
    grabmanSolveVerify: async () => ({ ok: true, exit_code: "0", plain_summary: "Lockfile intact" }),
    listWorkers: async () => ({
      workers: [
        {
          id: "worker-1",
          url: "http://127.0.0.1:8765",
          port: "8765",
          pid: "9007199254740993",
          status: "running",
          health_status: "200",
          ready_status: "200",
          reload_count: "2",
          log_count: "4",
        },
      ],
    }),
    groxySnapshots: async () => ({
      snapshots: [
        {
          snapshot_id: "snap-0123456789abcdef0123456789abcdef",
          target_path: "views/workbench.grab.html",
          before_sha256: "deadbeef",
          byte_length: "9007199254740995",
          restored: false,
        },
      ],
    }),
    groxyAsk: async ({ prompt }) => {
      if (prompt.includes("Fix")) {
        return {
          agent: "groxy-ai",
          agent_name: "Groxy AI",
          model_switching_allowed: false,
          reasoning_mode: "deep",
          summary: "Deep plan prepared with snapshot.",
          approval_required: true,
          approval_token: "groxy-appr-123",
          target_path: "views/workbench.grab.html",
          proposed_content: "<section><p>Fixed</p></section>",
          snapshot: { snapshot_id: "snap-0123456789abcdef0123456789abcdef" },
        };
      }
      return {
        agent: "groxy-ai",
        agent_name: "Groxy AI",
        model_switching_allowed: false,
        reasoning_mode: "fast",
        summary: "Audit passed cleanly.",
        approval_required: false,
        audit: {
          score: "100",
          grade: "A",
          publish_safe: true,
          div_count: "0",
        },
      };
    },
    groxyApply: async (payload) => {
      mockApiCalls.appliedToken = payload.approval_token;
      return {
        ok: true,
        target_path: "views/workbench.grab.html",
        snapshot_id: "snap-0123456789abcdef0123456789abcdef",
      };
    },
    groxyRestore: async (snapshotId) => {
      mockApiCalls.restoredSnapshot = snapshotId;
      return {
        ok: true,
        target_path: "views/workbench.grab.html",
        snapshot_id: snapshotId,
      };
    },
  };
  const mockApiCalls = { appliedToken: "", restoredSnapshot: "" };

  const mounted = mountGroxyDeskWorkbench(root, {
    document: shim.document,
    apiClient: mockApi,
    autoHydrate: false,
  });

  await mounted.hydrateWorkspace();

  // 1. Strict Zero-<div> and Zero-innerHTML checks
  const allElements = root.querySelectorAll("*");
  const divCount = allElements.filter((el) => el.localName === "div").length;
  assert.equal(divCount, 0, "Rendered workbench DOM must have divCount === 0");
  assert.equal(shim.innerHTMLWriteCount, 0, "innerHTMLWriteCount must be 0");

  // Every rendered tag must belong cleanly to Pure Grex JS semantic elements
  for (const el of allElements) {
    assert.equal(
      ALLOWED_SEMANTIC_TAGS.has(el.localName),
      true,
      `Disallowed HTML tag <${el.localName}> rendered in Grex workbench`
    );
  }

  // 2. Full Grex auditDom WCAG AA & heading hierarchy verification
  const domAudit = auditDom(root);
  assert.equal(domAudit.ok, true, `auditDom failed: ${JSON.stringify(domAudit.issues)}`);
  assert.equal(domAudit.issues.length, 0, `Unexpected DOM warnings: ${JSON.stringify(domAudit.issues)}`);

  // 3. Strict Unified Groxy AI invariant: zero <select> model-switching dropdowns inside Groxy AI sidebar
  const groxySidebar = root.querySelector("#desk-groxy-sidebar");
  assert.ok(groxySidebar, "Groxy AI sidebar missing");
  assert.equal(
    groxySidebar.querySelectorAll("select").length,
    0,
    "Groxy AI sidebar must contain ZERO model-switching <select> dropdowns"
  );
  assert.equal(
    /select\s+model|gpt-|claude-|gemini-/i.test(groxySidebar.textContent),
    false,
    "Groxy AI sidebar must not advertise external model-switching pickers"
  );
  const authorPanel = root.querySelector("#desk-author-panel");
  assert.ok(authorPanel, "Author Mode panel missing");
  assert.equal(
    authorPanel.querySelectorAll("select").length,
    0,
    "Author Mode / Ollama panel must contain ZERO model-switching <select> dropdowns"
  );

  // 4. ADR-025c base-10 numeric wire string preservation (no float truncation on >2^53 IDs)
  const workerSection = root.querySelector("#desk-grisp-supervisor");
  assert.ok(workerSection.textContent.includes("8765"));
  assert.ok(workerSection.textContent.includes("9007199254740993"));
  assert.equal(isExactGrabWireString("9007199254740993"), true);
  assert.equal(
    root.querySelector("#desk-snapshots-card").textContent.includes("9007199254740995"),
    true
  );

  // 5. Light <-> Dark toggle works without errors or innerHTML writes
  const themeBtn = root.querySelector("#desk-theme-toggle");
  assert.equal(shim.document.documentElement.getAttribute("data-grex-theme"), "light");
  await themeBtn.click();
  assert.equal(shim.document.documentElement.getAttribute("data-grex-theme"), "dark");
  assert.equal(savedTheme, "dark");

  // 6. Fast & Deep Groxy AI UI interactions
  await root.querySelector("#desk-groxy-audit-btn").click();
  assert.ok(root.querySelector("#desk-groxy-mode-badge").textContent.includes("fast"));
  assert.ok(root.querySelector("#desk-groxy-audit-summary").textContent.includes("Score 100/100"));

  const promptInput = root.querySelector("#groxy-prompt-input");
  const textarea = promptInput.localName === "textarea" ? promptInput : promptInput.querySelector("textarea");
  textarea.value = "Fix wrapper divs";
  await textarea.dispatchEvent({ type: "input", target: textarea });
  await root.querySelector("#desk-groxy-form").dispatchEvent({ type: "submit" });

  assert.ok(root.querySelector("#desk-groxy-mode-badge").textContent.includes("deep"));
  assert.equal(root.querySelector("#desk-groxy-approve-btn").getAttribute("disabled"), null);

  await root.querySelector("#desk-groxy-approve-btn").click();
  assert.equal(mockApiCalls.appliedToken, "groxy-appr-123");
  await root.querySelector("#desk-groxy-rollback-btn").click();
  assert.equal(mockApiCalls.restoredSnapshot, "snap-0123456789abcdef0123456789abcdef");

  mounted.dispose();
  assert.equal(shim.innerHTMLWriteCount, 0);
});
