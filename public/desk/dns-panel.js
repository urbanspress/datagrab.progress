/**
 * Pure Grex JS v1.0.0 Namecheap AES-256-GCM DNS Vault & 1-Click Auto-HTTPS Wizard Panel.
 * Preserves MX/TXT/SPF/DKIM email records byte-identical (422 DESK-E303) and blocks
 * ACME Let's Encrypt issuance until DNS propagation converges (422 DESK-E304).
 */
import { ForEach, signal } from "../grex/grex.js";

export function createDNSPanelNode(store, actions, doc = globalThis.document) {
  const apiUser = signal("");
  const apiKey = signal("");
  const clientIp = signal("");
  const domain = signal("example.org");
  const targetIp = signal("203.0.113.42");
  const edgeProxy = signal("caddy");
  const upstreamPort = signal("8765");
  const statusMsg = signal(
    "Namecheap credentials stay encrypted via AES-256-GCM (v1:...); MX/TXT/SPF/DKIM email records preserved byte-identical."
  );

  async function onSaveVault() {
    try {
      const res = await actions.dnsVaultSave({
        api_user: String(apiUser.peek() || "").trim(),
        api_key: String(apiKey.peek() || "").trim(),
        client_ip: String(clientIp.peek() || "").trim(),
      });
      apiKey.set("");
      statusMsg.set(
        `Saved encrypted vault (${res.api_key_masked || "AES-256-GCM"}).`
      );
    } catch (err) {
      statusMsg.set(err && err.message ? err.message : String(err));
    }
  }

  async function onPlanDNS() {
    try {
      const plan = await actions.dnsPlan({
        domain: String(domain.peek() || "").trim(),
        target_ip: String(targetIp.peek() || "").trim(),
        configure_www: true,
      });
      store.dnsPlan.set(plan);
      statusMsg.set(plan.plain_summary || "DNS plan prepared.");
    } catch (err) {
      statusMsg.set(err && err.message ? err.message : String(err));
    }
  }

  async function onApplyDNS() {
    const plan = store.dnsPlan.peek();
    if (!plan || !plan.approval_token) return;
    try {
      const applied = await actions.dnsApply({
        domain: plan.domain,
        approval_token: plan.approval_token,
        proposed_records: plan.proposed_records || [],
      });
      store.dnsPlan.set(null);
      statusMsg.set(
        `Applied ${applied.record_count} DNS records (${applied.preserved_email_count} email records kept byte-identical).`
      );
    } catch (err) {
      statusMsg.set(err && err.message ? err.message : String(err));
    }
  }

  async function onPreflightHTTPS() {
    try {
      const res = await actions.httpsPreflight({
        domain: String(domain.peek() || "").trim(),
        expected_ip: String(targetIp.peek() || "").trim(),
        edge_proxy: String(edgeProxy.peek() || "caddy").trim(),
        upstream_port: String(upstreamPort.peek() || "8765").trim(),
      });
      store.httpsPlan.set(res);
      statusMsg.set(
        `DNS converged for ${res.domain}; ${res.edge_proxy} -> ${res.upstream} Auto-HTTPS ready.`
      );
    } catch (err) {
      statusMsg.set(err && err.message ? err.message : String(err));
    }
  }

  async function onApplyHTTPS() {
    const plan = store.httpsPlan.peek();
    if (!plan || !plan.approval_token) return;
    try {
      const res = await actions.httpsApply({
        domain: plan.domain,
        expected_ip: String(targetIp.peek() || "").trim(),
        edge_proxy: plan.edge_proxy,
        upstream_port: String(upstreamPort.peek() || "8765").trim(),
        approval_token: plan.approval_token,
      });
      store.httpsPlan.set(null);
      statusMsg.set(
        `Configured ${res.edge_proxy} Auto-HTTPS for ${res.domain} -> ${res.upstream}.`
      );
    } catch (err) {
      statusMsg.set(err && err.message ? err.message : String(err));
    }
  }

  return {
    type: "Card",
    tag: "section",
    id: "desk-dns-panel",
    attrs: { id: "desk-dns-panel" },
    className: "desk-card",
    children: [
      {
        type: "Heading",
        props: { level: 3, text: "Namecheap DNS Vault & Auto-HTTPS Wizard" },
      },
      {
        type: "Text",
        tag: "p",
        id: "desk-dns-status",
        attrs: { id: "desk-dns-status" },
        className: "desk-status-banner",
        props: { text: statusMsg },
      },
      {
        type: "Form",
        id: "desk-dns-vault-form",
        attrs: { id: "desk-dns-vault-form" },
        className: "desk-studio-controls",
        props: { onSubmit: onSaveVault },
        children: [
          {
            type: "Input",
            id: "dns-api-user",
            attrs: { id: "dns-api-user" },
            props: { name: "api_user", label: "Namecheap API User", value: apiUser },
          },
          {
            type: "Input",
            id: "dns-api-key",
            attrs: { id: "dns-api-key" },
            props: {
              name: "api_key",
              type: "password",
              label: "Namecheap API Key (AES-256-GCM)",
              value: apiKey,
            },
          },
          {
            type: "Input",
            id: "dns-client-ip",
            attrs: { id: "dns-client-ip" },
            props: { name: "client_ip", label: "Whitelisted Client IP", value: clientIp },
          },
          {
            type: "Input",
            id: "dns-domain",
            attrs: { id: "dns-domain" },
            props: { name: "domain", label: "Domain (FQDN)", value: domain },
          },
          {
            type: "Input",
            id: "dns-target-ip",
            attrs: { id: "dns-target-ip" },
            props: { name: "target_ip", label: "Target VPS IPv4", value: targetIp },
          },
          {
            type: "Select",
            id: "dns-edge-proxy",
            attrs: { id: "dns-edge-proxy" },
            props: {
              name: "edge_proxy",
              label: "Edge TLS Proxy",
              value: edgeProxy,
              options: [
                { value: "caddy", label: "Caddy Auto-HTTPS" },
                { value: "nginx", label: "Nginx + Certbot" },
                { value: "cloudflare-tunnel", label: "Cloudflare Tunnel" },
              ],
            },
          },
          {
            type: "Input",
            id: "dns-upstream-port",
            attrs: { id: "dns-upstream-port" },
            props: {
              name: "upstream_port",
              label: "Grisp Loopback Port (8765-8772)",
              value: upstreamPort,
            },
          },
          {
            type: "Stack",
            tag: "section",
            props: { direction: "row", gap: "sm" },
            attrs: { "aria-label": "DNS and Auto-HTTPS actions" },
            children: [
              {
                type: "Button",
                id: "dns-vault-save-btn",
                attrs: { id: "dns-vault-save-btn" },
                props: { type: "submit", text: "Encrypt & Save Vault" },
              },
              {
                type: "Button",
                id: "dns-plan-btn",
                attrs: { id: "dns-plan-btn" },
                props: {
                  type: "button",
                  text: "Plan Email-Safe DNS Diff",
                  onClick: onPlanDNS,
                },
              },
              {
                type: "Button",
                id: "dns-apply-btn",
                attrs: { id: "dns-apply-btn" },
                props: {
                  type: "button",
                  text: "Approve & Apply DNS",
                  onClick: onApplyDNS,
                  disabled: () => !store.dnsPlan.get(),
                },
              },
              {
                type: "Button",
                id: "dns-https-preflight-btn",
                attrs: { id: "dns-https-preflight-btn" },
                props: {
                  type: "button",
                  text: "Check DNS Propagation",
                  onClick: onPreflightHTTPS,
                },
              },
              {
                type: "Button",
                id: "dns-https-apply-btn",
                attrs: { id: "dns-https-apply-btn" },
                props: {
                  type: "button",
                  text: "Enable Auto-HTTPS",
                  onClick: onApplyHTTPS,
                  disabled: () => !store.httpsPlan.get(),
                },
              },
            ],
          },
        ],
      },
      ForEach(
        {
          items: () => {
            const p = store.dnsPlan.get();
            return p && Array.isArray(p.diffs) ? p.diffs : [];
          },
          key: (d) => `${d.action}:${d.type}:${d.name}:${d.address}`,
          parent: {
            type: "Section",
            tag: "section",
            className: "desk-list",
            props: { label: "Planned DNS Records Diff" },
          },
          empty: () => ({
            type: "Text",
            tag: "p",
            className: "desk-muted",
            props: { text: "Run Plan Email-Safe DNS Diff to verify MX/TXT/DKIM preservation before applying." },
          }),
          render: (diffSig) => ({
            type: "Card",
            tag: "article",
            className: "desk-diff-row",
            children: [
              {
                type: "Text",
                tag: "strong",
                props: {
                  text: () => {
                    const d = diffSig.get();
                    return `[${d.action}] ${d.type} ${d.name} -> ${d.address}`;
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
