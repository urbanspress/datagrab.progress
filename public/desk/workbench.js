/**
 * Pure Grex JS v1.0.0 Main IDE Workbench Entrypoint for Groxy Desk v0.1.0.
 * Zero wrapper div containers, zero raw HTML DOM writes, zero unsafe script calls, sole Groxy AI identity.
 */
import { ForEach, renderNode, signal } from "../grex/grex.js";
import { createDeskApiClient } from "./api-client.js";
import { createGroxySidebarNode } from "./groxy-agent.js";
import { createSupervisorPanelNode } from "./supervisor-panel.js";
import { applyDeskTheme, resolveInitialTheme } from "./theme.js";
import { createWorkspacePanelNode } from "./workspace-panel.js";

export function mountGroxyDeskWorkbench(rootEl, options = {}) {
  const doc = options.document || globalThis.document;
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
  };

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
  };

  const toggleTheme = async () => {
    const next = store.themeMode.peek() === "dark" ? "light" : "dark";
    const applied = await applyDeskTheme(next, doc, apiClient);
    store.themeMode.set(applied);
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
            type: "Heading",
            props: { level: 1, text: "Groxy Desk" },
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
        tag: "section",
        className: "desk-workspace-grid",
        props: { label: "Three-Pane IDE Grid" },
        children: [
          createWorkspacePanelNode(store, actions, doc),
          {
            type: "Section",
            tag: "main",
            id: "desk-main-stage",
            attrs: { id: "desk-main-stage" },
            className: "desk-panel",
            props: { label: "Editor and Diagnostics" },
            children: [
              {
                type: "Heading",
                props: { level: 2, text: "Code Workbench & Grisp Supervisor" },
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
              createSupervisorPanelNode(store, actions, doc),
              {
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
              },
            ],
          },
          createGroxySidebarNode(store, actions),
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
              text: "Groxy Desk v0.1.0 — Pure Grex JS v1.0.0 | Zero Wrapper Divs | Unified Groxy AI",
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
      if (msg && msg.type === "groxy-desk-session") {
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
      webview.postMessage({ type: "groxy-desk-ready" });
    }
  } else if (apiClient.hasSession() && options.autoHydrate !== false) {
    hydrateWorkspace();
  }

  return {
    store,
    actions,
    apiClient,
    hydrateWorkspace,
    toggleTheme,
    element: rendered.element,
    dispose() {
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
