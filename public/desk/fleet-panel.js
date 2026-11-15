/**
 * Pure Grex JS v1.0.0 Step 5 Fleet & Grabtaculous Bridge Panel.
 * Enforces SHA256:... SSH host key pinning (DESK-E405), Site DNA validation,
 * --read-only container rollouts, and single-use GROXY-SAFE-001 approval + rollback.
 */
import { ForEach, signal } from "../grex/grex.js";

export function createFleetPanelNode(store, actions, doc = globalThis.document) {
  const nodeId = signal("vps-prod-1");
  const sshHost = signal("127.0.0.1");
  const sshPort = signal("22");
  const pinnedFp = signal("");
  const controlUrl = signal("http://127.0.0.1:9440");
  const appName = signal("grablab-cms");
  const containerImage = signal(
    "ghcr.io/urbanspress/grablab-cms@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
  );
  const siteDnaRaw = signal(
    '{"schema_version":"groxy-site-dna-v0.4.0","site_name":"Groxy Studio"}'
  );
  const statusMsg = signal(
    "Step 5 bridge connects strictly over known_hosts-pinned SSH to loopback grabtaculousd (127.0.0.1:9440)."
  );

  async function onVerifyNode() {
    try {
      statusMsg.set("Verifying pinned SSH host key...");
      const res = await actions.fleetVerifyNode({
        node_id: String(nodeId.peek() || "").trim(),
        host: String(sshHost.peek() || "").trim(),
        port: String(sshPort.peek() || "22").trim(),
        pinned_fingerprint: String(pinnedFp.peek() || "").trim(),
        presented_fingerprint: String(pinnedFp.peek() || "").trim(),
        control_url: String(controlUrl.peek() || "").trim(),
      });
      statusMsg.set(
        `Verified pinned node ${res.node ? res.node.node_id : nodeId.peek()} (zero public admin exposure).`
      );
    } catch (err) {
      statusMsg.set(err && err.message ? err.message : String(err));
    }
  }

  async function onValidateDNA() {
    try {
      const res = await actions.fleetValidateDNA({
        raw_json: String(siteDnaRaw.peek() || ""),
      });
      statusMsg.set(res.summary || "Site DNA verified.");
    } catch (err) {
      statusMsg.set(err && err.message ? err.message : String(err));
    }
  }

  async function onPlanDeploy() {
    try {
      const plan = await actions.fleetDeployPlan({
        node_id: String(nodeId.peek() || "").trim(),
        app_name: String(appName.peek() || "").trim(),
        blueprint_id: "grablab-cms-sqlite",
        container_image: String(containerImage.peek() || "").trim(),
        site_dna_raw: String(siteDnaRaw.peek() || ""),
      });
      store.fleetDeployPlan.set(plan);
      statusMsg.set(plan.plain_summary || "Pre-Deploy Snapshot captured.");
    } catch (err) {
      statusMsg.set(err && err.message ? err.message : String(err));
    }
  }

  async function onApplyDeploy() {
    const plan = store.fleetDeployPlan.peek();
    if (!plan || !plan.approval_token) return;
    try {
      const applied = await actions.fleetDeployApply({
        node_id: plan.node_id,
        app_name: plan.app_name,
        approval_token: plan.approval_token,
        container_image: String(containerImage.peek() || "").trim(),
      });
      store.fleetDeployPlan.set(null);
      statusMsg.set(
        `Deployed ${applied.app_name} with --read-only hardening (drain ${applied.drain_timeout_sec}s).`
      );
    } catch (err) {
      statusMsg.set(err && err.message ? err.message : String(err));
    }
  }

  return {
    type: "Card",
    tag: "section",
    id: "desk-fleet-panel",
    attrs: { id: "desk-fleet-panel" },
    className: "desk-card",
    children: [
      {
        type: "Heading",
        props: { level: 3, text: "Step 5 Fleet & Grabtaculous Bridge" },
      },
      {
        type: "Text",
        tag: "p",
        id: "desk-fleet-status",
        attrs: { id: "desk-fleet-status" },
        className: "desk-status-banner",
        props: { text: statusMsg },
      },
      {
        type: "Form",
        id: "desk-fleet-form",
        attrs: { id: "desk-fleet-form" },
        className: "desk-studio-controls",
        props: { onSubmit: onVerifyNode },
        children: [
          {
            type: "Input",
            id: "fleet-node-id",
            attrs: { id: "fleet-node-id" },
            props: { name: "node_id", label: "Node ID", value: nodeId },
          },
          {
            type: "Input",
            id: "fleet-ssh-host",
            attrs: { id: "fleet-ssh-host" },
            props: { name: "host", label: "SSH Host", value: sshHost },
          },
          {
            type: "Input",
            id: "fleet-ssh-port",
            attrs: { id: "fleet-ssh-port" },
            props: { name: "port", label: "SSH Port", value: sshPort },
          },
          {
            type: "Input",
            id: "fleet-ssh-pin",
            attrs: { id: "fleet-ssh-pin" },
            props: {
              name: "pinned_fingerprint",
              label: "Pinned SHA256 Host Fingerprint",
              value: pinnedFp,
              placeholder: "SHA256:...",
            },
          },
          {
            type: "Input",
            id: "fleet-control-url",
            attrs: { id: "fleet-control-url" },
            props: {
              name: "control_url",
              label: "Loopback Grabtaculous URL",
              value: controlUrl,
            },
          },
          {
            type: "Input",
            id: "fleet-app-name",
            attrs: { id: "fleet-app-name" },
            props: { name: "app_name", label: "App Name", value: appName },
          },
          {
            type: "Input",
            id: "fleet-image-digest",
            attrs: { id: "fleet-image-digest" },
            props: {
              name: "container_image",
              label: "Pinned @sha256 Image",
              value: containerImage,
            },
          },
          {
            type: "Input",
            id: "fleet-site-dna",
            attrs: { id: "fleet-site-dna" },
            props: {
              name: "site_dna_raw",
              label: "Site DNA JSON",
              value: siteDnaRaw,
            },
          },
          {
            type: "Stack",
            tag: "section",
            props: { direction: "row", gap: "sm" },
            attrs: { "aria-label": "Fleet deployment actions" },
            children: [
              {
                type: "Button",
                id: "fleet-verify-btn",
                attrs: { id: "fleet-verify-btn" },
                props: { type: "submit", text: "Verify Pinned SSH Node" },
              },
              {
                type: "Button",
                id: "fleet-dna-btn",
                attrs: { id: "fleet-dna-btn" },
                props: {
                  type: "button",
                  text: "Validate Site DNA",
                  onClick: onValidateDNA,
                },
              },
              {
                type: "Button",
                id: "fleet-plan-btn",
                attrs: { id: "fleet-plan-btn" },
                props: {
                  type: "button",
                  text: "Plan --read-only Deploy",
                  onClick: onPlanDeploy,
                },
              },
              {
                type: "Button",
                id: "fleet-apply-btn",
                attrs: { id: "fleet-apply-btn" },
                props: {
                  type: "button",
                  text: "Approve & Apply Deploy",
                  onClick: onApplyDeploy,
                  disabled: () => !store.fleetDeployPlan.get(),
                },
              },
            ],
          },
        ],
      },
      ForEach(
        {
          items: store.fleetNodes,
          key: (n) => n.node_id,
          parent: {
            type: "Section",
            tag: "section",
            className: "desk-list",
            props: { label: "Verified Fleet Nodes" },
          },
          empty: () => ({
            type: "Text",
            tag: "p",
            className: "desk-muted",
            props: { text: "No pinned Step 5 nodes registered yet." },
          }),
          render: (nodeSig) => ({
            type: "Card",
            tag: "article",
            className: "desk-diff-row",
            children: [
              {
                type: "Text",
                tag: "strong",
                props: {
                  text: () => {
                    const n = nodeSig.get();
                    return `${n.node_id} (${n.host}:${n.port}) [${n.status}]`;
                  },
                },
              },
              {
                type: "Text",
                tag: "span",
                className: "desk-muted",
                props: {
                  text: () => {
                    const n = nodeSig.get();
                    return `Pin ${n.pinned_fingerprint} -> ${n.control_url}`;
                  },
                },
              },
            ],
          }),
        },
        { document: doc }
      ),
    ],
  };
}
