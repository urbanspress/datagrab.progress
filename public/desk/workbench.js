/**
 * Pure Grex JS v1.0.0 Main IDE Workbench Entrypoint for Groxy Desk v1.2.0.
 * Zero wrapper elements, zero raw DOM writes, zero unsafe script calls, sole Groxy AI identity.
 */
import { ForEach, effect, renderNode, signal } from "../grex/grex.js";
import { createDeskApiClient } from "./api-client.js";
import { createAuthorPanelNode } from "./author-panel.js";
import { createDBStudioPanelNode } from "./db-studio-panel.js";
import { createRadarPanelNode } from "./radar-panel.js";
import { createDNSPanelNode } from "./dns-panel.js";
import { createFleetPanelNode } from "./fleet-panel.js";
import { createGroxySidebarNode } from "./groxy-agent.js";
import { createScorecardPanelNode } from "./scorecard-panel.js";
import {
  buildAllowedPreviewOrigins,
  createStudioPanelNode,
  createStudioPreviewBridge,
  renderPreviewTree,
} from "./studio-panel.js";
import { createSupervisorPanelNode } from "./supervisor-panel.js";
import { applyDeskTheme, resolveInitialTheme } from "./theme.js";
import { createWorkspacePanelNode } from "./workspace-panel.js";

const STUDIO_MODES = ["database", "browser", "code"];
const STUDIO_SET = new Set(STUDIO_MODES);
const STUDIO_STORAGE_KEY = "groxy_desk_active_studio";

export function parseStudioHash(rawHash) {
  const m = /^#studio=(database|browser|code)$/.exec(String(rawHash || "").trim());
  return m ? m[1] : "";
}

function resolveInitialStudio(options, win) {
  if (options && STUDIO_SET.has(options.initialStudio)) {
    return options.initialStudio;
  }
  const fromHash = win && win.location ? parseStudioHash(win.location.hash) : "";
  if (fromHash) {
    return fromHash;
  }
  if (win && win.sessionStorage && typeof win.sessionStorage.getItem === "function") {
    try {
      const saved = win.sessionStorage.getItem(STUDIO_STORAGE_KEY);
      if (STUDIO_SET.has(saved)) {
        return saved;
      }
    } catch {
      // Ignore storage failures in restricted contexts.
    }
  }
  return "code";
}

const DEFAULT_STYLE_PACKS = [
  { id: "emerald-editorial", label: "Emerald Editorial" },
  { id: "obsidian-tech", label: "Obsidian Developer Studio" },
  { id: "warm-commerce", label: "Warm Precision Commerce" },
  { id: "swiss-minimal", label: "Swiss Grid Minimal" },
  { id: "local-service-trust", label: "Local Service Trust" },
  { id: "creative-studio-bold", label: "Creative Studio Bold" },
  { id: "creator-academy", label: "Creator Academy" },
  { id: "wellness-organic", label: "Wellness Organic" },
];

const DEFAULT_BLUEPRINTS = [
  { id: "full_landing_page", label: "Full Landing Page" },
  { id: "hero_section", label: "Hero Section" },
  { id: "pricing_grid", label: "Pricing Grid (GrabDecimal)" },
  { id: "faq_accordion", label: "FAQ Accordion" },
  { id: "foreach_showcase", label: "ForEach Content Showcase" },
];

export function mountGroxyDeskWorkbench(rootEl, options = {}) {
  const doc = options.document || globalThis.document;
  const win = options.window || globalThis.window;
  if (!rootEl) {
    throw new Error("mountGroxyDeskWorkbench requires a root DOM element.");
  }

  const apiClient =
    options.apiClient ||
    createDeskApiClient({
      baseUrl: options.baseUrl || "",
      bearerToken: options.session ? options.session.bearer_token : "",
      csrfToken: options.session ? options.session.csrf_token : "",
      fetchImpl: options.fetchImpl,
    });

  const store = {
    themeMode: signal(resolveInitialTheme(doc)),
    activeStudio: signal(resolveInitialStudio(options, win)),
    sessionReady: signal(apiClient.hasSession()),
    treeEntries: signal(options.initialEntries || []),
    activePath: signal(options.initialActivePath || "src/desk.grab"),
    activeSHA256: signal(""),
    activeSizeBytes: signal("0"),
    activeFileContent: signal(
      options.initialFileContent || "Select any workspace file to inspect cleanly."
    ),
    checkDiagnostics: signal("Click Run grab check to validate syntax & formatting."),
    workers: signal(options.initialWorkers || []),
    activeLogsText: signal("Select a worker and click View Logs."),
    snapshots: signal(options.initialSnapshots || []),
    groxyResponse: signal(options.initialGroxyResponse || null),
    workbenchMode: signal(options.initialMode || "split"),
    previewViewport: signal("base"),
    stylePacks: signal(DEFAULT_STYLE_PACKS),
    blueprints: signal(DEFAULT_BLUEPRINTS),
    selectedStylePack: signal("emerald-editorial"),
    selectedBlueprint: signal("full_landing_page"),
    synthesizedLayout: signal(null),
    undoStack: signal([]),
    scorecardReport: signal(null),
    boostProposal: signal(null),
    mcpTools: signal([]),
    mcpLastOutput: signal("Unified grab mcp, GrabLab CMS, and Grex Builder outputs appear here."),
    fleetNodes: signal([]),
    fleetDeployPlan: signal(null),
    dbTunnels: signal([]),
    browserStatus: signal({ origin: "", message: "" }),
    browserSelection: signal(null),
    browserBrand: signal(null),
    groxyPrompt: signal(""),
    dnsPlan: signal(null),
    httpsPlan: signal(null),
    authorPlan: signal(null),
    ollamaStatus: signal(null),
  };

  function setActiveStudio(nextStudio, persist = true) {
    if (!STUDIO_SET.has(nextStudio)) {
      return false;
    }
    store.activeStudio.set(nextStudio);
    if (persist) {
      if (win && win.sessionStorage && typeof win.sessionStorage.setItem === "function") {
        try {
          win.sessionStorage.setItem(STUDIO_STORAGE_KEY, nextStudio);
        } catch {
          // Ignore storage failures in restricted contexts.
        }
      }
      if (win && win.location) {
        try {
          const targetHash = `#studio=${nextStudio}`;
          if (win.location.hash !== targetHash) {
            win.location.hash = targetHash;
          }
        } catch {
          // Ignore location hash failures in restricted contexts.
        }
      }
    }
    return true;
  }

  function handleTablistKeydown(ev) {
    const key = ev && ev.key;
    if (!key) return;
    const current = store.activeStudio.peek();
    const idx = Math.max(0, STUDIO_MODES.indexOf(current));
    let nextIdx = null;
    if (key === "ArrowRight") {
      nextIdx = (idx + 1) % STUDIO_MODES.length;
    } else if (key === "ArrowLeft") {
      nextIdx = (idx - 1 + STUDIO_MODES.length) % STUDIO_MODES.length;
    } else if (key === "Home") {
      nextIdx = 0;
    } else if (key === "End") {
      nextIdx = STUDIO_MODES.length - 1;
    }
    if (nextIdx === null) return;
    if (typeof ev.preventDefault === "function") {
      ev.preventDefault();
    }
    const targetStudio = STUDIO_MODES[nextIdx];
    setActiveStudio(targetStudio);
    if (doc && typeof doc.querySelector === "function") {
      const targetBtn = doc.querySelector(`#desk-tab-${targetStudio}`);
      if (targetBtn && typeof targetBtn.focus === "function") {
        targetBtn.focus();
      }
    }
  }

  async function refreshTree(dir = ".") {
    if (!apiClient.hasSession()) return [];
    const data = await apiClient.workspaceTree(dir);
    const list = Array.isArray(data.entries) ? data.entries : [];
    store.treeEntries.set(list);
    return list;
  }

  async function openFile(relPath) {
    store.activePath.set(relPath);
    if (!apiClient.hasSession()) return null;
    const fc = await apiClient.workspaceRead(relPath);
    store.activePath.set(fc.path);
    store.activeSHA256.set(fc.sha256 || "");
    store.activeSizeBytes.set(fc.size_bytes || "0");
    store.activeFileContent.set(fc.content || "");
    return fc;
  }

  async function runActiveFileCheck() {
    const active = String(store.activePath.peek() || "");
    const grabFile = active.endsWith(".grab") ? active : "src/desk.grab";
    const tmplFile = active.endsWith(".grab.html") ? active : "";
    store.checkDiagnostics.set("Running grab check & canonical format parity...");
    const res = await apiClient.workspaceCheck(grabFile, tmplFile);
    store.checkDiagnostics.set(
      `${res.diagnostics || "Completed."} (grab_check_ok=${res.grab_check_ok}, format_clean=${res.format_clean})`
    );
    return res;
  }

  async function refreshWorkers() {
    if (!apiClient.hasSession()) return [];
    const data = await apiClient.listWorkers();
    const list = Array.isArray(data.workers) ? data.workers : [];
    store.workers.set(list);
    return list;
  }

  async function refreshSnapshots() {
    if (!apiClient.hasSession()) return [];
    const data = await apiClient.groxySnapshots();
    const list = Array.isArray(data.snapshots) ? data.snapshots : [];
    store.snapshots.set(list);
    return list;
  }

  async function hydrateWorkspace() {
    if (!apiClient.hasSession()) return;
    store.sessionReady.set(true);
    try {
      const pref = await apiClient.getTheme();
      if (pref && pref.theme) {
        const applied = await applyDeskTheme(pref.theme, doc, null);
        store.themeMode.set(applied);
      }
    } catch {
      // Preserve SSR initial theme if preference read fails.
    }
    await Promise.all([
      refreshTree(".").catch(() => []),
      refreshWorkers().catch(() => []),
      refreshSnapshots().catch(() => []),
      actions.loadMcpTools().catch(() => []),
      actions.loadStudioCatalog().catch(() => null),
      actions.loadFleetNodes().catch(() => []),
      actions.loadDBTunnels().catch(() => []),
      actions.authorOllamaStatus().catch(() => null),
    ]);
  }

  const actions = {
    refreshTree,
    openFile,
    grabmanInitPlan: (payload) => apiClient.grabmanInitPlan(payload),
    grabmanInit: (payload) => apiClient.grabmanInit(payload),
    grabmanDoctor: (dir) => apiClient.grabmanDoctor(dir, false),
    grabmanSolveVerify: (dir) => apiClient.grabmanSolveVerify(dir),
    refreshWorkers,
    startWorker: async (payload) => {
      const snap = await apiClient.startWorker(payload);
      await refreshWorkers();
      return snap;
    },
    reloadWorker: async (id) => {
      const snap = await apiClient.reloadWorker(id);
      await refreshWorkers();
      return snap;
    },
    stopWorker: async (id) => {
      const snap = await apiClient.stopWorker(id);
      await refreshWorkers();
      return snap;
    },
    loadWorkerLogs: async (id) => {
      const data = await apiClient.workerLogs(id);
      const lines = Array.isArray(data.logs)
        ? data.logs.map((l) => `[${l.seq} ${l.stream}] ${l.line}`).join("\n")
        : "";
      store.activeLogsText.set(lines || `No buffered log lines for ${id}.`);
    },
    askGroxy: async (payload) => {
      const resp = await apiClient.groxyAsk(payload);
      store.groxyResponse.set(resp);
      await refreshSnapshots().catch(() => []);
      return resp;
    },
    applyGroxyMutation: async (payload) => {
      const res = await apiClient.groxyApply(payload);
      await Promise.all([
        openFile(res.target_path).catch(() => null),
        refreshTree(".").catch(() => []),
        refreshSnapshots().catch(() => []),
      ]);
      return res;
    },
    restoreSnapshot: async (snapshotId) => {
      const res = await apiClient.groxyRestore(snapshotId);
      await Promise.all([
        openFile(res.target_path).catch(() => null),
        refreshTree(".").catch(() => []),
        refreshSnapshots().catch(() => []),
      ]);
      return res;
    },
    loadMcpTools: async () => {
      if (typeof apiClient.mcpTools !== "function" || !apiClient.hasSession()) return [];
      const data = await apiClient.mcpTools();
      const list = Array.isArray(data.tools) ? data.tools : [];
      store.mcpTools.set(list);
      return list;
    },
    callMcpTool: async (payload) => {
      const res = await apiClient.mcpCall(payload);
      await refreshSnapshots().catch(() => []);
      return res;
    },
    loadStudioCatalog: async (studioUrl = "") => {
      if (typeof apiClient.studioDesignSystem !== "function" || !apiClient.hasSession()) return null;
      const cat = await apiClient.studioDesignSystem(studioUrl);
      if (Array.isArray(cat.style_packs) && cat.style_packs.length > 0) {
        store.stylePacks.set(cat.style_packs);
      }
      if (Array.isArray(cat.blueprints) && cat.blueprints.length > 0) {
        store.blueprints.set(cat.blueprints);
      }
      return cat;
    },
    synthesizeStudio: async (payload) => {
      const res = await apiClient.studioSynthesize(payload);
      if (res && res.grex_layout) {
        store.synthesizedLayout.set(res.grex_layout);
      }
      if (res && res.scorecard) {
        store.scorecardReport.set(res.scorecard);
      }
      return res;
    },
    applyStudioLayout: async (payload) => {
      const res = await apiClient.studioLayoutApply(payload);
      if (res && res.command && res.command.type === "applyAiLayout") {
        const cur = store.undoStack.peek() || [];
        store.undoStack.set([...cur, res.command]);
      }
      await refreshSnapshots().catch(() => []);
      return res;
    },
    undoStudioLayout: () => {
      const stack = store.undoStack.peek() || [];
      if (stack.length === 0) return false;
      const top = stack[stack.length - 1];
      store.undoStack.set(stack.slice(0, -1));
      if (top && Object.prototype.hasOwnProperty.call(top, "before")) {
        store.synthesizedLayout.set(top.before || null);
      }
      return true;
    },
    runV2Audit: async (payload) => {
      const rep = await apiClient.groxyAudit(payload);
      store.scorecardReport.set(rep);
      return rep;
    },
    proposeBoost: async (payload) => {
      const res = await apiClient.groxyBoost(payload);
      store.boostProposal.set(res);
      if (res && res.proposal && res.proposal.before_report) {
        store.scorecardReport.set(res.proposal.before_report);
      }
      await refreshSnapshots().catch(() => []);
      return res;
    },
    approveBoost: async (payload) => {
      const res = await apiClient.groxyBoostApprove(payload);
      const boost = store.boostProposal.peek();
      if (boost && boost.proposal) {
        if (boost.proposal.after_report) {
          store.scorecardReport.set(boost.proposal.after_report);
        }
        if (boost.proposal.proposed_layout) {
          store.synthesizedLayout.set(boost.proposal.proposed_layout);
        }
      }
      if (res && res.target_path && !String(res.target_path).includes(":")) {
        await openFile(res.target_path).catch(() => null);
      }
      await refreshSnapshots().catch(() => []);
      return res;
    },
    loadFleetNodes: async () => {
      if (typeof apiClient.fleetNodes !== "function" || !apiClient.hasSession()) return [];
      const data = await apiClient.fleetNodes();
      const list = Array.isArray(data.nodes) ? data.nodes : [];
      store.fleetNodes.set(list);
      return list;
    },
    fleetVerifyNode: async (payload) => {
      const res = await apiClient.fleetVerifyNode(payload);
      await actions.loadFleetNodes().catch(() => []);
      return res;
    },
    fleetValidateDNA: (payload) => apiClient.fleetValidateDNA(payload),
    fleetDeployPlan: async (payload) => {
      const res = await apiClient.fleetDeployPlan(payload);
      await refreshSnapshots().catch(() => []);
      return res;
    },
    fleetDeployApply: async (payload) => {
      const res = await apiClient.fleetDeployApply(payload);
      await refreshSnapshots().catch(() => []);
      return res;
    },
    fleetRestoreSnapshot: async (payload) => {
      const res = await apiClient.fleetRestoreSnapshot(payload);
      await refreshSnapshots().catch(() => []);
      return res;
    },
    loadDBTunnels: async () => {
      if (typeof apiClient.dbTunnels !== "function" || !apiClient.hasSession()) return [];
      const data = await apiClient.dbTunnels();
      const list = Array.isArray(data.tunnels) ? data.tunnels : [];
      store.dbTunnels.set(list);
      return list;
    },
    dbTunnelOpen: async (payload) => {
      const res = await apiClient.dbTunnelOpen(payload);
      await actions.loadDBTunnels().catch(() => []);
      return res;
    },
    dbTunnelClose: async (payload) => {
      const res = await apiClient.dbTunnelClose(payload);
      await actions.loadDBTunnels().catch(() => []);
      return res;
    },
    dbSchema: (target) => apiClient.dbSchema(target),
    dbRows: (payload) => apiClient.dbRows(payload),
    dbQuery: (payload, signal) => apiClient.dbQuery(payload, signal),
    dbCompare: (payload) => apiClient.dbCompare(payload),
    dbExportPage: (payload) => apiClient.dbExportPage(payload),
    dbExportDiagram: (payload) => apiClient.dbExportDiagram(payload),
    dbImportPlan: (payload) => apiClient.dbImportPlan(payload),
    dbImportApply: (payload) => apiClient.dbImportApply(payload),
    dbRestorePlan: (payload) => apiClient.dbRestorePlan(payload),
    dbRestore: (payload) => apiClient.dbRestoreSnapshot(payload),
    dbMutatePlan: async (payload) => {
      const res = await apiClient.dbMutatePlan(payload);
      await refreshSnapshots().catch(() => []);
      return res;
    },
    dbMutateApply: async (payload) => {
      const res = await apiClient.dbMutateApply(payload);
      await refreshSnapshots().catch(() => []);
      return res;
    },
    dnsVaultSave: (payload) => apiClient.dnsVaultSave(payload),
    dnsPlan: async (payload) => {
      const res = await apiClient.dnsPlan(payload);
      await refreshSnapshots().catch(() => []);
      return res;
    },
    dnsApply: async (payload) => {
      const res = await apiClient.dnsApply(payload);
      await refreshSnapshots().catch(() => []);
      return res;
    },
    httpsPreflight: async (payload) => {
      const res = await apiClient.httpsPreflight(payload);
      await refreshSnapshots().catch(() => []);
      return res;
    },
    httpsApply: async (payload) => {
      const res = await apiClient.httpsApply(payload);
      await refreshSnapshots().catch(() => []);
      return res;
    },
    authorInitPlan: async (payload) => {
      const res = await apiClient.authorInitPlan(payload);
      await refreshSnapshots().catch(() => []);
      return res;
    },
    authorInitApply: async (payload) => {
      const res = await apiClient.authorInitApply(payload);
      await Promise.all([refreshTree(".").catch(() => []), refreshSnapshots().catch(() => [])]);
      return res;
    },
    authorOllamaStatus: async (url = "") => {
      if (typeof apiClient.authorOllamaStatus !== "function" || !apiClient.hasSession()) return null;
      const st = await apiClient.authorOllamaStatus(url);
      store.ollamaStatus.set(st);
      return st;
    },
    authorOllamaStart: async (payload = {}) => {
      const res = await apiClient.authorOllamaStart(payload);
      await actions.authorOllamaStatus().catch(() => null);
      return res;
    },
  };

  const toggleTheme = async () => {
    const next = store.themeMode.peek() === "dark" ? "light" : "dark";
    const applied = await applyDeskTheme(next, doc, apiClient);
    store.themeMode.set(applied);
  };

  const studioTab = (id, label) => ({
    type: "Button",
    id: `desk-tab-${id}`,
    attrs: {
      id: `desk-tab-${id}`,
      role: "tab",
      "aria-controls": `desk-panel-${id}`,
      "aria-selected": () => (store.activeStudio.get() === id ? "true" : "false"),
      tabindex: () => (store.activeStudio.get() === id ? "0" : "-1"),
    },
    className: () => (store.activeStudio.get() === id ? "desk-tab desk-tab-active" : "desk-tab"),
    props: {
      type: "button",
      text: label,
      onClick: () => setActiveStudio(id),
    },
  });

  const studioTabpanel = (id, title, children) => ({
    type: "Section",
    tag: "section",
    id: `desk-panel-${id}`,
    attrs: {
      id: `desk-panel-${id}`,
      role: "tabpanel",
      "aria-labelledby": `desk-tab-${id}`,
      tabindex: "0",
      hidden: () => store.activeStudio.get() !== id,
      "aria-hidden": () => (store.activeStudio.get() !== id ? "true" : "false"),
    },
    className: "desk-tabpanel",
    children: [
      {
        type: "Heading",
        className: "desk-tabpanel-heading",
        props: { level: 2, text: title },
      },
      ...children,
    ],
  });

  const snapshotsCard = {
    type: "Card",
    tag: "section",
    id: "desk-snapshots-card",
    attrs: { id: "desk-snapshots-card" },
    className: "desk-card",
    children: [
      {
        type: "Heading",
        props: { level: 3, text: "Pre-Mutation Snapshots" },
      },
      ForEach(
        {
          items: store.snapshots,
          key: (s) => s.snapshot_id,
          parent: {
            type: "Section",
            tag: "section",
            className: "desk-list",
            props: { label: "Recorded Safety Snapshots" },
          },
          empty: () => ({
            type: "Text",
            tag: "p",
            className: "desk-muted",
            props: { text: "No pre-mutation snapshots captured yet." },
          }),
          render: (snapSig) => {
            const snapId = snapSig.peek().snapshot_id;
            return {
              type: "Card",
              tag: "article",
              className: "desk-card",
              children: [
                {
                  type: "Text",
                  tag: "strong",
                  props: {
                    text: () => {
                      const s = snapSig.get();
                      return `${s.snapshot_id} -> ${s.target_path}`;
                    },
                  },
                },
                {
                  type: "Text",
                  tag: "span",
                  className: "desk-muted",
                  props: {
                    text: () => {
                      const s = snapSig.get();
                      return `Bytes: ${s.byte_length} | Restored: ${s.restored}`;
                    },
                  },
                },
                {
                  type: "Button",
                  props: {
                    type: "button",
                    text: "1-Click Rollback Snapshot",
                    onClick: () =>
                      actions
                        .restoreSnapshot(snapId)
                        .catch((e) =>
                          store.checkDiagnostics.set(e.message)
                        ),
                  },
                },
              ],
            };
          },
        },
        { document: doc }
      ),
    ],
  };

  const appTree = {
    type: "Section",
    tag: "section",
    className: "desk-root",
    props: { label: "Groxy Desk Workbench" },
    children: [
      {
        type: "Section",
        tag: "header",
        className: "desk-header",
        children: [
          {
            type: "Stack",
            tag: "section",
            className: "desk-brand-group",
            children: [
              {
                type: "Heading",
                props: { level: 1, text: "Groxy Desk" },
              },
              {
                type: "Text",
                tag: "span",
                className: "desk-version-tag",
                props: { text: "v1.2.0" },
              },
            ],
          },
          {
            type: "Stack",
            tag: "nav",
            className: "desk-studio-tabs",
            attrs: {
              role: "tablist",
              "aria-label": "Workspace Studio Modes",
            },
            on: { keydown: handleTablistKeydown },
            children: [
              studioTab("database", "Database Studio"),
              studioTab("browser", "Browser & Radar"),
              studioTab("code", "Code & Grex Studio"),
            ],
          },
          {
            type: "Stack",
            tag: "section",
            className: "desk-header-actions",
            children: [
              {
                type: "Text",
                tag: "span",
                id: "desk-connection-badge",
                attrs: { id: "desk-connection-badge" },
                className: "desk-badge",
                props: {
                  text: () =>
                    store.sessionReady.get()
                      ? "127.0.0.1:9490 Authenticated"
                      : "Waiting for Host Session",
                },
              },
              {
                type: "Button",
                id: "desk-theme-toggle",
                attrs: { id: "desk-theme-toggle" },
                props: {
                  type: "button",
                  label: "Toggle Light or Dark Theme",
                  text: () =>
                    store.themeMode.get() === "dark"
                      ? "Switch to Light Mode"
                      : "Switch to Dark Mode",
                  onClick: () => toggleTheme(),
                },
              },
            ],
          },
        ],
      },
      {
        type: "Section",
        tag: "main",
        id: "desk-workspace-stage",
        attrs: { id: "desk-workspace-stage" },
        className: "desk-main",
        props: { label: "Groxy Desk Studio Workspace" },
        children: [
          studioTabpanel("database", "Database Studio", [
            createDBStudioPanelNode(store, actions, doc),
          ]),
          studioTabpanel("browser", "Browser & Radar", [
            createRadarPanelNode(store, apiClient, doc),
          ]),
          studioTabpanel("code", "Code & Grex Studio", [
            {
              type: "Section",
              tag: "section",
              className: "desk-workspace-grid",
              props: { label: "Three-Pane Code and Grex Studio" },
              children: [
                {
                  type: "Section",
                  tag: "section",
                  className: "desk-code-left-rail",
                  props: { label: "Workspace Explorer, grabman, Grisp and Step 5 Tools" },
                  children: [
                    createWorkspacePanelNode(store, actions, doc),
                    {
                      type: "Accordion",
                      className: "desk-collapsible-group",
                      props: {
                        label: "Supervisor and Step 5 Tools",
                        items: [
                          {
                            id: "grisp-supervisor",
                            summary: "Local Grisp Multi-Worker Supervisor",
                            open: true,
                            content: createSupervisorPanelNode(store, actions, doc),
                          },
                          {
                            id: "step5-fleet",
                            summary: "Step 5 Fleet & Grabtaculous Bridge",
                            open: false,
                            content: createFleetPanelNode(store, actions, doc),
                          },
                          {
                            id: "step5-dns",
                            summary: "Namecheap DNS Vault & Auto-HTTPS Wizard",
                            open: false,
                            content: createDNSPanelNode(store, actions, doc),
                          },
                          {
                            id: "step5-author",
                            summary: "1-Click Local Author Mode & Open-Weights Runtime",
                            open: false,
                            content: createAuthorPanelNode(store, actions, doc),
                          },
                        ],
                      },
                    },
                  ],
                },
                {
                  type: "Section",
                  tag: "section",
                  id: "desk-main-stage",
                  attrs: { id: "desk-main-stage" },
                  className: "desk-panel",
                  props: { label: "Editor and Grex Preview Canvas" },
                  children: [
                    {
                      type: "Heading",
                      props: { level: 2, text: "Code Workbench & Grex Preview" },
                    },
                    {
                      type: "Card",
                      tag: "section",
                      id: "desk-editor-card",
                      attrs: { id: "desk-editor-card" },
                      className: "desk-card",
                      children: [
                        {
                          type: "Heading",
                          props: { level: 3, text: "Active File Preview" },
                        },
                        {
                          type: "Text",
                          tag: "p",
                          id: "desk-active-file-meta",
                          attrs: { id: "desk-active-file-meta" },
                          className: "desk-muted",
                          props: {
                            text: () =>
                              `${store.activePath.get()} (${store.activeSizeBytes.get()} B)`,
                          },
                        },
                        {
                          type: "Stack",
                          tag: "section",
                          className: "desk-badge-row",
                          children: [
                            {
                              type: "Button",
                              id: "desk-run-grab-check-btn",
                              attrs: { id: "desk-run-grab-check-btn" },
                              className: "desk-btn-primary",
                              props: {
                                type: "button",
                                text: "Run grab check & format",
                                onClick: () =>
                                  runActiveFileCheck().catch((e) =>
                                    store.checkDiagnostics.set(e.message)
                                  ),
                              },
                            },
                          ],
                        },
                        {
                          type: "Text",
                          tag: "code",
                          id: "desk-editor-code",
                          attrs: { id: "desk-editor-code" },
                          className: "desk-code-block",
                          props: { text: store.activeFileContent },
                        },
                        {
                          type: "Text",
                          tag: "p",
                          id: "desk-check-diagnostics",
                          attrs: { id: "desk-check-diagnostics" },
                          className: "desk-muted",
                          props: { text: store.checkDiagnostics },
                        },
                      ],
                    },
                    createStudioPanelNode(store, actions, doc),
                  ],
                },
                {
                  type: "Section",
                  tag: "section",
                  className: "desk-code-right-rail",
                  props: { label: "Unified Groxy AI Co-Pilot, 22-Rule Scorecard and Safety Snapshots" },
                  children: [
                    createGroxySidebarNode(store, actions),
                    createScorecardPanelNode(store, actions, doc),
                    snapshotsCard,
                  ],
                },
              ],
            },
          ]),
        ],
      },
      {
        type: "Section",
        tag: "footer",
        className: "desk-footer",
        children: [
          {
            type: "Text",
            tag: "p",
            props: {
              text: "Groxy Desk v1.2.0 — Pure Grex JS v1.0.0 | Zero Wrapper Elements | Unified Groxy AI",
            },
          },
        ],
      },
    ],
  };

  const rendered = renderNode(appTree, { document: doc });
  while (rootEl.firstChild) {
    rootEl.removeChild(rootEl.firstChild);
  }
  rootEl.appendChild(rendered.element);

  const previewHostEl =
    rendered.element && typeof rendered.element.querySelector === "function"
      ? rendered.element.querySelector("#desk-preview-surface")
      : null;
  const stopPreviewEffect = effect(() => {
    const layoutNode = store.synthesizedLayout.get();
    if (previewHostEl && layoutNode) {
      renderPreviewTree(previewHostEl, layoutNode, doc);
    }
  });

  let previewBridge = null;
  try {
    previewBridge = createStudioPreviewBridge({
      allowedOrigins: buildAllowedPreviewOrigins(options.baseUrl || "", store.workers.peek()),
      targetWindow: win,
      root: doc,
      signals: {
        activeFileContent: store.activeFileContent,
        previewViewport: store.previewViewport,
        checkDiagnostics: store.checkDiagnostics,
      },
    });
    previewBridge.start();
  } catch {
    previewBridge = null;
  }

  let onUndoKeydown = null;
  if (doc && typeof doc.addEventListener === "function") {
    onUndoKeydown = (ev) => {
      if ((ev.ctrlKey || ev.metaKey) && String(ev.key || "").toLowerCase() === "z" && !ev.shiftKey) {
        if (actions.undoStudioLayout()) {
          if (typeof ev.preventDefault === "function") ev.preventDefault();
        }
      }
    };
    doc.addEventListener("keydown", onUndoKeydown);
  }

  let onHashChange = null;
  if (win && typeof win.addEventListener === "function") {
    onHashChange = () => {
      const fromHash = win.location ? parseStudioHash(win.location.hash) : "";
      if (fromHash && fromHash !== store.activeStudio.peek()) {
        setActiveStudio(fromHash, true);
      }
    };
    win.addEventListener("hashchange", onHashChange);
  }

  applyDeskTheme(store.themeMode.peek(), doc, null).catch(() => {});

  const webview =
    globalThis.window &&
    globalThis.window.chrome &&
    globalThis.window.chrome.webview;
  let onWebViewMsg = null;
  if (webview && typeof webview.addEventListener === "function") {
    onWebViewMsg = (event) => {
      let msg = event && event.data ? event.data : null;
      if (typeof msg === "string") {
        try {
          msg = JSON.parse(msg);
        } catch {
          msg = null;
        }
      }
      if (msg?.type === "groxy-browser-status") {
        if (msg.origin !== store.browserStatus.peek().origin) {
          store.browserSelection.set(null);
          store.browserBrand.set(null);
        }
        store.browserStatus.set({ origin: msg.origin, message: msg.message });
      } else if (msg?.type === "groxy-browser-selection" &&
          msg.origin === store.browserStatus.peek().origin && typeof msg.text === "string") {
        store.browserSelection.set({ text: msg.text.slice(0, 4000), origin: msg.origin });
      } else if (msg?.type === "groxy-browser-brand") {
        store.browserBrand.set(msg);
      } else if (msg?.type === "groxy-desk-session") {
        apiClient.setSession({
          baseUrl: msg.url || "",
          bearerToken: msg.bearer_token || "",
          csrfToken: msg.csrf_token || "",
        });
        hydrateWorkspace();
      }
    };
    webview.addEventListener("message", onWebViewMsg);
    if (typeof webview.postMessage === "function") {
      webview.postMessage(JSON.stringify({ type: "groxy-desk-ready" }));
    }
  } else if (apiClient.hasSession() && options.autoHydrate !== false) {
    hydrateWorkspace();
  }

  return {
    store,
    actions,
    apiClient,
    previewBridge,
    hydrateWorkspace,
    setActiveStudio,
    toggleTheme,
    element: rendered.element,
    dispose() {
      stopPreviewEffect();
      if (previewBridge) {
        previewBridge.dispose();
      }
      if (doc && onUndoKeydown && typeof doc.removeEventListener === "function") {
        doc.removeEventListener("keydown", onUndoKeydown);
      }
      if (win && onHashChange && typeof win.removeEventListener === "function") {
        win.removeEventListener("hashchange", onHashChange);
      }
      if (
        webview &&
        onWebViewMsg &&
        typeof webview.removeEventListener === "function"
      ) {
        webview.removeEventListener("message", onWebViewMsg);
      }
      rendered.dispose();
    },
  };
}

if (
  typeof globalThis.document !== "undefined" &&
  !globalThis.__GROXY_DESK_NO_AUTO_MOUNT__
) {
  const boot = () => {
    const root = globalThis.document.getElementById("desk-root");
    if (root) {
      mountGroxyDeskWorkbench(root);
    }
  };
  if (globalThis.document.readyState === "loading") {
    globalThis.document.addEventListener("DOMContentLoaded", boot, {
      once: true,
    });
  } else {
    boot();
  }
}
