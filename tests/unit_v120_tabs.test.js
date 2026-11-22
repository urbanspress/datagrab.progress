import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

globalThis.__GROXY_DESK_NO_AUTO_MOUNT__ = true;

import { auditDom, computeContrastRatio } from "../public/grex/grex.js";
import { DESK_THEME_PALETTES } from "../public/desk/theme.js";
import { mountGroxyDeskWorkbench, parseStudioHash } from "../public/desk/workbench.js";
import { createDomShim } from "./helpers/dom-shim.js";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(__dirname, "..");

function createWindowMock(initialHash = "", initialStored = "") {
  const storage = new Map();
  if (initialStored) {
    storage.set("groxy_desk_active_studio", initialStored);
  }
  const listeners = new Map();
  return {
    location: { origin: "http://127.0.0.1:9490", hash: initialHash },
    sessionStorage: {
      getItem: (k) => (storage.has(k) ? storage.get(k) : null),
      setItem: (k, v) => storage.set(String(k), String(v)),
    },
    addEventListener: (type, fn) => {
      if (!listeners.has(type)) listeners.set(type, new Set());
      listeners.get(type).add(fn);
    },
    removeEventListener: (type, fn) => {
      listeners.get(type)?.delete(fn);
    },
    storage,
  };
}

test("Offline-safe Urbanist fonts are bundled locally and desk.css has zero CDN URLs or undefined --desk-border", () => {
  for (const weight of ["400", "500", "600", "700"]) {
    const fontPath = path.join(repoRoot, "public", "desk", "fonts", `urbanist-latin-${weight}-normal.woff2`);
    assert.equal(fs.existsSync(fontPath), true, `Missing bundled Urbanist font: ${fontPath}`);
  }
  const css = fs.readFileSync(path.join(repoRoot, "public", "desk", "desk.css"), "utf8");
  assert.equal(css.includes('font-family: "Urbanist"'), true);
  assert.equal(css.includes("./fonts/urbanist-latin-400-normal.woff2"), true);
  assert.equal(/https?:\/\//i.test(css), false, "desk.css must not reference external CDN URLs");
  assert.equal(css.includes("--desk-border"), false, "desk.css must use var(--grex-border), not --desk-border");
  assert.equal(/\.desk-tabpanel\[hidden\]\s*\{\s*display:\s*none;\s*\}/.test(css), true);
});

test("v1.2.0 3-tab switcher supports WAI-ARIA tablist keyboard nav, allowlisted hash/session persistence, and mounted panels", async () => {
  const shim = createDomShim("light");
  const root = shim.document.createElement("section");
  shim.document.body.appendChild(root);
  const win = createWindowMock("#studio=evil-injection", "admin");

  const mounted = mountGroxyDeskWorkbench(root, {
    document: shim.document,
    window: win,
    apiClient: { hasSession: () => false },
    autoHydrate: false,
  });

  // Invalid hash and invalid sessionStorage must fall back safely to "code"
  assert.equal(parseStudioHash("#studio=evil"), "");
  assert.equal(parseStudioHash("#studio=database"), "database");
  assert.equal(mounted.store.activeStudio.peek(), "code");

  // Single <main id="desk-workspace-stage"> landmark, #desk-main-stage inside Tab 3 as <section>
  assert.equal(root.querySelectorAll("main").length, 1);
  assert.equal(root.querySelector("main").id, "desk-workspace-stage");
  const mainStage = root.querySelector("#desk-main-stage");
  assert.ok(mainStage);
  assert.equal(mainStage.localName, "section");

  // All 3 tabpanels stay mounted in DOM with correct hidden & aria-hidden states
  const tabDb = root.querySelector("#desk-tab-database");
  const tabBrowser = root.querySelector("#desk-tab-browser");
  const tabCode = root.querySelector("#desk-tab-code");
  const panelDb = root.querySelector("#desk-panel-database");
  const panelBrowser = root.querySelector("#desk-panel-browser");
  const panelCode = root.querySelector("#desk-panel-code");

  assert.equal(tabDb.getAttribute("aria-selected"), "false");
  assert.equal(tabBrowser.getAttribute("aria-selected"), "false");
  assert.equal(tabCode.getAttribute("aria-selected"), "true");
  assert.equal(panelDb.hasAttribute("hidden"), true);
  assert.equal(panelDb.getAttribute("aria-hidden"), "true");
  assert.equal(panelBrowser.hasAttribute("hidden"), true);
  assert.equal(panelCode.hasAttribute("hidden"), false);
  assert.equal(panelCode.getAttribute("aria-hidden"), "false");

  // Each tabpanel opens with <h2> before <h3>
  for (const panel of [panelDb, panelBrowser, panelCode]) {
    const firstHeading = panel.querySelector("h2, h3, h4");
    assert.ok(firstHeading);
    assert.equal(firstHeading.localName, "h2");
  }

  // Click tab switching updates state, URL hash, and sessionStorage
  await tabDb.click();
  assert.equal(mounted.store.activeStudio.peek(), "database");
  assert.equal(win.location.hash, "#studio=database");
  assert.equal(win.storage.get("groxy_desk_active_studio"), "database");
  assert.equal(panelDb.hasAttribute("hidden"), false);
  assert.equal(panelCode.hasAttribute("hidden"), true);

  // Keyboard navigation: ArrowRight wraps database -> browser -> code -> database; ArrowLeft wraps back; Home/End jump
  const tablist = root.querySelector('nav[role="tablist"]');
  await tablist.dispatchEvent({ type: "keydown", key: "ArrowRight" });
  assert.equal(mounted.store.activeStudio.peek(), "browser");
  assert.equal(shim.document.activeElement, tabBrowser);

  await tablist.dispatchEvent({ type: "keydown", key: "ArrowRight" });
  assert.equal(mounted.store.activeStudio.peek(), "code");
  assert.equal(shim.document.activeElement, tabCode);

  await tablist.dispatchEvent({ type: "keydown", key: "ArrowRight" });
  assert.equal(mounted.store.activeStudio.peek(), "database");
  assert.equal(shim.document.activeElement, tabDb);

  await tablist.dispatchEvent({ type: "keydown", key: "ArrowLeft" });
  assert.equal(mounted.store.activeStudio.peek(), "code");
  assert.equal(shim.document.activeElement, tabCode);

  await tablist.dispatchEvent({ type: "keydown", key: "Home" });
  assert.equal(mounted.store.activeStudio.peek(), "database");
  assert.equal(shim.document.activeElement, tabDb);

  await tablist.dispatchEvent({ type: "keydown", key: "End" });
  assert.equal(mounted.store.activeStudio.peek(), "code");
  assert.equal(shim.document.activeElement, tabCode);

  // Reload persistence from sessionStorage when hash is empty
  mounted.dispose();
  const shim2 = createDomShim("dark");
  const root2 = shim2.document.createElement("section");
  shim2.document.body.appendChild(root2);
  const win2 = createWindowMock("", "browser");
  const mounted2 = mountGroxyDeskWorkbench(root2, {
    document: shim2.document,
    window: win2,
    apiClient: { hasSession: () => false },
    autoHydrate: false,
  });
  assert.equal(mounted2.store.activeStudio.peek(), "browser");
  assert.equal(root2.querySelector("#desk-panel-browser").hasAttribute("hidden"), false);

  // Light and Dark active-tab WCAG AA contrast check (>= 4.5:1)
  for (const mode of ["light", "dark"]) {
    const p = DESK_THEME_PALETTES[mode];
    assert.ok(computeContrastRatio(p["--grex-accent-text"], p["--grex-accent"]) >= 4.5);
    assert.ok(computeContrastRatio(p["--grex-text"], p["--grex-surface"]) >= 4.5);
  }

  const domAudit = auditDom(root2);
  assert.equal(domAudit.ok, true, JSON.stringify(domAudit.issues));
  assert.equal(root2.querySelectorAll("*").filter((e) => e.localName === "div").length, 0);
  assert.equal(shim2.innerHTMLWriteCount, 0);
  mounted2.dispose();
});

test("Database Studio ER diagram, Views/Triggers schema tree, and 3-pane layout satisfy zero heading skips", async () => {
  const shim = createDomShim("light");
  const root = shim.document.createElement("section");
  shim.document.body.appendChild(root);

  const mockApi = {
    hasSession: () => true,
    getTheme: async () => ({ theme: "light" }),
    workspaceTree: async () => ({ entries: [] }),
    listWorkers: async () => ({ workers: [] }),
    groxySnapshots: async () => ({ snapshots: [] }),
    mcpTools: async () => ({ tools: [] }),
    studioDesignSystem: async () => ({ style_packs: [], blueprints: [] }),
    fleetNodes: async () => ({ nodes: [] }),
    dbTunnels: async () => ({ tunnels: [] }),
    authorOllamaStatus: async () => ({ ok: true, running: false, loopback_url: "http://127.0.0.1:11434" }),
    dbSchema: async () => ({
      ok: true,
      engine: "sqlite",
      tables: [
        {
          schema: "main",
          name: "authors",
          columns: [{ name: "id", type: "INTEGER", primary_key: true, nullable: false }],
          foreign_keys: [],
          indexes: ["idx_authors_pk"],
        },
        {
          schema: "main",
          name: "books",
          columns: [
            { name: "id", type: "INTEGER", primary_key: true, nullable: false },
            { name: "author_id", type: "INTEGER", primary_key: false, nullable: false },
            { name: "price", type: "DECIMAL(10,2)", primary_key: false, nullable: false },
          ],
          foreign_keys: [{ column: "author_id", ref_schema: "main", ref_table: "authors", ref_column: "id" }],
          indexes: ["idx_books_author"],
        },
      ],
      views: [{ name: "priced_books" }],
      triggers: [{ name: "books_audit_trig" }],
    }),
  };

  const mounted = mountGroxyDeskWorkbench(root, {
    document: shim.document,
    window: createWindowMock("#studio=database"),
    apiClient: mockApi,
    autoHydrate: false,
  });
  await mounted.hydrateWorkspace();

  await root.querySelector("#db-schema-btn").click();
  const dbText = root.querySelector("#desk-db-studio-panel").textContent;
  assert.equal(dbText.includes("Tables (2)"), true);
  assert.equal(dbText.includes("Views (1)"), true);
  assert.equal(dbText.includes("Triggers (1)"), true);
  assert.equal(dbText.includes("priced_books"), true);
  assert.equal(dbText.includes("books_audit_trig"), true);
  assert.equal(dbText.includes("main.books"), true);

  const domAudit = auditDom(root);
  assert.equal(domAudit.ok, true, `Expected zero heading skips or a11y issues, got: ${JSON.stringify(domAudit.issues)}`);
  assert.equal(root.querySelectorAll("*").filter((e) => e.localName === "div").length, 0);
  assert.equal(shim.innerHTMLWriteCount, 0);
  mounted.dispose();
});

test("v1.3.0 High-Density 100vh Desktop IDE Shell, 5 coordinated chrome layers, object filter, and 21 frozen v1.2.0 IDs", async () => {
  const css = fs.readFileSync(path.join(repoRoot, "public", "desk", "desk.css"), "utf8");
  for (const token of [
    "--desk-header-h: 32px",
    "--desk-statusbar-h: 24px",
    "--desk-rail-w: 48px",
    "--desk-sidebar-w: 240px",
    "--desk-inspector-w: 280px",
    "--desk-row-h: 28px",
    "--grex-base: #0f1115",
    "--grex-surface: #181b20",
    "--grex-base: #f8fafc",
    "--grex-surface: #ffffff",
    "height: 100vh;",
    "max-height: 100vh;",
    "overflow: hidden;",
    "font-variant-numeric: tabular-nums;",
  ]) {
    assert.equal(css.includes(token), true, `desk.css missing v1.3.0 rule/token: ${token}`);
  }

  const shim = createDomShim("dark");
  const root = shim.document.createElement("section");
  root.setAttribute("id", "desk-root");
  shim.document.body.appendChild(root);

  const mockApi = {
    hasSession: () => true,
    getTheme: async () => ({ theme: "dark" }),
    setTheme: async (next) => ({ ok: true, theme: next }),
    workspaceTree: async () => ({ entries: [] }),
    listWorkers: async () => ({
      workers: [{ id: "w1", port: "8765", pid: "1234", status: "running", url: "http://127.0.0.1:8765" }],
    }),
    groxySnapshots: async () => ({
      snapshots: [{ snapshot_id: "snap-1", target_path: "src/desk.grab", byte_length: "312", restored: false }],
    }),
    mcpTools: async () => ({ tools: [] }),
    studioDesignSystem: async () => ({ style_packs: [], blueprints: [] }),
    fleetNodes: async () => ({ nodes: [] }),
    dbTunnels: async () => ({
      tunnels: [
        {
          tunnel_id: "tun-pg",
          engine: "postgres",
          local_bind_addr: "127.0.0.1:15432",
          pinned_fingerprint: "SHA256:verifiedPin123",
        },
      ],
    }),
    authorOllamaStatus: async () => ({ ok: true, running: false, loopback_url: "http://127.0.0.1:11434" }),
    dbSchema: async () => ({
      ok: true,
      engine: "sqlite",
      tables: [
        { schema: "main", name: "authors", columns: [{ name: "id", type: "INTEGER", primary_key: true }] },
        { schema: "main", name: "books", columns: [{ name: "id", type: "INTEGER", primary_key: true }] },
      ],
      views: [{ name: "priced_books" }],
      triggers: [{ name: "books_audit_trig" }],
    }),
  };

  const win = createWindowMock("#studio=code");
  const mounted = mountGroxyDeskWorkbench(root, {
    document: shim.document,
    window: win,
    apiClient: mockApi,
    autoHydrate: false,
  });
  await mounted.hydrateWorkspace();

  // Verify 100% of the 21 frozen v1.2.0 IDs + v1.3.0 additive shell IDs exist
  const requiredIds = [
    "desk-root",
    "desk-header",
    "desk-workspace-stage",
    "desk-theme-toggle",
    "desk-main-stage",
    "desk-tab-database",
    "desk-tab-browser",
    "desk-tab-code",
    "desk-panel-database",
    "desk-panel-browser",
    "desk-panel-code",
    "desk-db-studio-panel",
    "desk-editor-card",
    "desk-active-file-meta",
    "desk-run-grab-check-btn",
    "desk-editor-code",
    "desk-check-diagnostics",
    "desk-preview-surface",
    "desk-snapshots-card",
    "desk-groxy-sidebar",
    "desk-author-panel",
    "desk-connection-badge",
    "desk-host-status-badge",
    "desk-command-search",
    "desk-activity-rail",
    "desk-tab-grex",
    "desk-tab-fleet",
    "desk-action-settings",
    "desk-action-shortcuts",
    "desk-db-sidebar",
    "desk-db-object-filter",
    "desk-db-center",
    "desk-db-inspector",
    "desk-status-bar",
    "desk-status-db",
    "desk-status-ssh",
    "desk-status-safety",
    "desk-status-grisp",
    "desk-status-adr025c",
  ];
  for (const id of requiredIds) {
    assert.ok(shim.document.querySelector(`#${id}`), `Missing required DOM ID #${id}`);
  }

  // Verify 32px Top Menu Bar & Command Search
  const menubar = root.querySelector('nav[role="menubar"]');
  assert.ok(menubar, "Missing nav[role=menubar]");
  assert.equal(menubar.querySelectorAll('button[role="menuitem"]').length, 7);

  // Verify 48px Activity Rail renders SVG icons via createElementNS
  const rail = root.querySelector("#desk-activity-rail");
  assert.ok(rail);
  assert.equal(rail.querySelectorAll("svg").length, 7);

  // Verify 24px Status Bar live indicators and 1-click DB studio navigation
  assert.equal(root.querySelector("#desk-status-db").textContent.includes("postgres (127.0.0.1:15432)"), true);
  assert.equal(root.querySelector("#desk-status-ssh").textContent.includes("SHA256:verifiedPin123"), true);
  assert.equal(root.querySelector("#desk-status-safety").textContent.includes("1 Snapshots"), true);
  assert.equal(root.querySelector("#desk-status-grisp").textContent.includes("1 Active (:8765)"), true);
  assert.equal(root.querySelector("#desk-status-adr025c").textContent.includes("v1.3.1"), true);

  await root.querySelector("#desk-status-db").click();
  assert.equal(mounted.store.activeStudio.peek(), "database");

  // Verify Database Studio object filter (#desk-db-object-filter) filters tables live
  await root.querySelector("#db-schema-btn").click();
  const filterInput = root.querySelector("#desk-db-object-filter");
  filterInput.value = "authors";
  await filterInput.dispatchEvent({ type: "input", target: filterInput });
  const sidebarText = root.querySelector("#desk-db-sidebar").textContent;
  assert.equal(sidebarText.includes("authors (1 cols)"), true);
  assert.equal(sidebarText.includes("books (1 cols)"), false);

  const domAudit = auditDom(root);
  assert.equal(domAudit.ok, true, JSON.stringify(domAudit.issues));
  assert.equal(root.querySelectorAll("*").filter((e) => e.localName === "div").length, 0);
  assert.equal(shim.innerHTMLWriteCount, 0);
  mounted.dispose();
});
