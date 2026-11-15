/**
 * Pure Grex JS v1.0.0 Workspace Tree & Built-in grabman v1.0.0 Manager Panel.
 * Zero wrapper div containers, zero raw HTML DOM writes, exact ADR-025c base-10 byte string rendering.
 */
import { ForEach, signal } from "../grex/grex.js";

export function createWorkspacePanelNode(store, actions, doc = globalThis.document) {
  const initSubdir = signal("app-demo");
  const initName = signal("My GRAB Site");
  const initProfile = signal("app");
  const initEdge = signal("caddy");
  const grabmanStatus = signal("Ready to initialize or inspect workspace lockfiles.");

  async function runInitWorkflow() {
    const dir = String(initSubdir.peek() || ".").trim() || ".";
    const profile = String(initProfile.peek() || "app").trim();
    const edgeProxy = String(initEdge.peek() || "caddy").trim();
    const projectName = String(initName.peek() || dir).trim();

    grabmanStatus.set(`Planning grabman init in "${dir}"...`);
    const plan = await actions.grabmanInitPlan({
      target_dir: dir,
      profile,
      project_name: projectName,
      edge_proxy: edgeProxy,
      force: false,
    });
    grabmanStatus.set(
      `Approval token ready for "${plan.target_dir}"; executing scaffold...`
    );
    const res = await actions.grabmanInit({
      approval_token: plan.approval_token,
      target_dir: dir,
      profile,
      project_name: projectName,
      edge_proxy: edgeProxy,
      force: false,
    });
    grabmanStatus.set(res.plain_summary || "grabman init completed.");
    await actions.refreshTree();
  }

  async function runDoctor() {
    grabmanStatus.set("Running grabman doctor...");
    const res = await actions.grabmanDoctor(".");
    grabmanStatus.set(
      `${res.plain_summary || "Doctor complete"} (exit=${res.exit_code})`
    );
  }

  async function runSolveVerify() {
    grabmanStatus.set("Running grabman solve --verify lockfile check...");
    const res = await actions.grabmanSolveVerify(".");
    grabmanStatus.set(
      `${res.plain_summary || "Lockfile check complete"} (exit=${res.exit_code})`
    );
  }

  return {
    type: "Section",
    tag: "aside",
    id: "desk-workspace-sidebar",
    attrs: { id: "desk-workspace-sidebar" },
    className: "desk-panel",
    props: { label: "Workspace and grabman" },
    children: [
      {
        type: "Heading",
        props: { level: 2, text: "Workspace & grabman" },
      },
      {
        type: "Card",
        tag: "section",
        className: "desk-card",
        children: [
          {
            type: "Heading",
            props: { level: 3, text: "Project Explorer" },
          },
          {
            type: "Button",
            id: "desk-refresh-tree-btn",
            attrs: { id: "desk-refresh-tree-btn" },
            props: {
              type: "button",
              text: "Refresh Files",
              onClick: () => actions.refreshTree(),
            },
          },
          ForEach(
            {
              items: store.treeEntries,
              key: (entry) => entry.path,
              parent: {
                type: "Section",
                tag: "section",
                className: "desk-list",
                props: { label: "Workspace Files" },
              },
              empty: () => ({
                type: "Text",
                tag: "p",
                className: "desk-muted",
                props: { text: "No visible files in workspace yet." },
              }),
              render: (entrySig) => {
                const entry = entrySig.peek();
                return {
                  type: "Button",
                  className: "desk-list-item",
                  props: {
                    type: "button",
                    label: `Open ${entry.path}`,
                    text: () =>
                      entrySig.get().is_dir
                        ? `[dir] ${entrySig.get().path}`
                        : `${entrySig.get().path} (${entrySig.get().size_bytes} B)`,
                    onClick: () => {
                      const current = entrySig.peek();
                      if (current && !current.is_dir) {
                        actions.openFile(current.path);
                      }
                    },
                  },
                };
              },
            },
            { document: doc }
          ),
        ],
      },
      {
        type: "Card",
        tag: "section",
        className: "desk-card",
        children: [
          {
            type: "Heading",
            props: { level: 3, text: "grabman v1.0.0 Manager" },
          },
          {
            type: "Form",
            id: "desk-grabman-form",
            attrs: { id: "desk-grabman-form" },
            props: {
              onSubmit: async () => {
                try {
                  await runInitWorkflow();
                } catch (err) {
                  grabmanStatus.set(err && err.message ? err.message : String(err));
                }
              },
            },
            children: [
              {
                type: "Input",
                id: "grabman-dir",
                attrs: { id: "grabman-dir" },
                props: {
                  name: "target_dir",
                  label: "Target Subdirectory",
                  value: initSubdir,
                  required: true,
                },
              },
              {
                type: "Input",
                id: "grabman-name",
                attrs: { id: "grabman-name" },
                props: {
                  name: "project_name",
                  label: "Project Name",
                  value: initName,
                },
              },
              {
                type: "Select",
                id: "grabman-profile",
                attrs: { id: "grabman-profile" },
                props: {
                  name: "profile",
                  label: "Blueprint Profile",
                  value: initProfile,
                  options: [
                    { value: "app", label: "Standard GRAB App (app)" },
                    { value: "cms-stack", label: "GrabLab CMS Stack (cms-stack)" },
                    { value: "ai-stack", label: "Groxy AI Stack (ai-stack)" },
                  ],
                },
              },
              {
                type: "Select",
                id: "grabman-edge",
                attrs: { id: "grabman-edge" },
                props: {
                  name: "edge_proxy",
                  label: "Edge Proxy",
                  value: initEdge,
                  options: [
                    { value: "caddy", label: "Caddy Auto-HTTPS" },
                    { value: "nginx", label: "NGINX Reverse Proxy" },
                    { value: "cloudflare-tunnel", label: "Cloudflare Tunnel" },
                  ],
                },
              },
              {
                type: "Button",
                id: "desk-grabman-init-btn",
                attrs: { id: "desk-grabman-init-btn" },
                className: "desk-btn-primary",
                props: { type: "submit", text: "Init grabman Project" },
              },
            ],
          },
          {
            type: "Stack",
            tag: "section",
            className: "desk-badge-row",
            children: [
              {
                type: "Button",
                id: "desk-grabman-doctor-btn",
                attrs: { id: "desk-grabman-doctor-btn" },
                props: {
                  type: "button",
                  text: "Run doctor",
                  onClick: () => runDoctor().catch((e) => grabmanStatus.set(e.message)),
                },
              },
              {
                type: "Button",
                id: "desk-grabman-solve-btn",
                attrs: { id: "desk-grabman-solve-btn" },
                props: {
                  type: "button",
                  text: "Verify lockfile",
                  onClick: () =>
                    runSolveVerify().catch((e) => grabmanStatus.set(e.message)),
                },
              },
            ],
          },
          {
            type: "Text",
            tag: "p",
            id: "desk-grabman-status",
            attrs: { id: "desk-grabman-status" },
            className: "desk-muted",
            props: { text: grabmanStatus },
          },
        ],
      },
    ],
  };
}
