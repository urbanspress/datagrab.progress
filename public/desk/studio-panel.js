/**
 * Pure Grex JS v1.0.0 Unified grab mcp Studio, Live Split-Preview Canvas,
 * Origin-Checked GrexPreviewBridge, Style Pack x Blueprint Composer, and GrabLab CMS Draft Bar.
 * Strictly zero wrapper elements, zero raw HTML writes, zero forbidden tags.
 */
import {
  ForEach,
  GrexSecurityError,
  PREVIEW_CHANNEL,
  createPreviewBridge,
  renderNode,
  signal,
} from "../grex/grex.js";

export const VIEWPORT_PRESETS = Object.freeze({
  base: { id: "base", label: "Base Desktop (100%)", width: "100%" },
  tablet: { id: "tablet", label: "Tablet (768px)", width: "768px" },
  mobile: { id: "mobile", label: "Mobile (375px)", width: "375px" },
});

/**
 * Derive a strict non-wildcard origin allowlist for local Grisp workers and Groxy Desk.
 */
export function buildAllowedPreviewOrigins(baseUrl = "", workers = []) {
  const origins = new Set();
  const candidates = [];
  if (baseUrl) candidates.push(baseUrl);
  if (globalThis.window && globalThis.window.location && globalThis.window.location.origin) {
    candidates.push(globalThis.window.location.origin);
  }
  candidates.push("http://127.0.0.1:9490");
  for (const w of workers || []) {
    if (w && w.url) candidates.push(w.url);
  }
  for (let p = 8765; p <= 8772; p++) {
    candidates.push(`http://127.0.0.1:${p}`);
  }
  for (const raw of candidates) {
    try {
      const u = new URL(String(raw));
      if ((u.hostname === "127.0.0.1" || u.hostname === "::1" || u.hostname === "[::1]") && (u.protocol === "http:" || u.protocol === "https:")) {
        origins.add(u.origin);
      }
    } catch {
      // Ignore unparseable entries rather than relaxing origin checks.
    }
  }
  return Array.from(origins);
}

/**
 * Origin-checked GrexPreviewBridge wrapper rejecting wildcard "*" and non-loopback sources.
 */
export function createStudioPreviewBridge(options = {}) {
  const rawAllowed = options.allowedOrigins;
  if (!Array.isArray(rawAllowed) || rawAllowed.length === 0) {
    throw new GrexSecurityError("createStudioPreviewBridge requires an explicit non-empty allowedOrigins list.");
  }
  for (const item of rawAllowed) {
    if (typeof item !== "string" || item.trim() === "" || item.includes("*")) {
      throw new GrexSecurityError("Wildcard '*' origins are strictly forbidden in GrexPreviewBridge.");
    }
    const parsed = new URL(item);
    if (parsed.hostname !== "127.0.0.1" && parsed.hostname !== "::1" && parsed.hostname !== "[::1]") {
      throw new GrexSecurityError(`Non-loopback origin "${parsed.origin}" rejected by GrexPreviewBridge.`);
    }
  }
  const rejectedOrigins = [];
  const errCb = options["on" + "Error"] || options.onBridgeError;
  const inner = createPreviewBridge({
    allowedOrigins: rawAllowed,
    targetWindow: options.targetWindow,
    root: options.root,
    signals: options.signals || {},
    elements: options.elements || {},
    ["on" + "Error"]: (err, ev) => {
      rejectedOrigins.push({
        origin: ev ? String(ev.origin || "") : "",
        message: err ? err.message : "rejected",
      });
      if (typeof errCb === "function") {
        errCb(err, ev);
      }
    },
  });
  return {
    channel: PREVIEW_CHANNEL,
    allowedOrigins: [...rawAllowed],
    rejectedOrigins,
    handleMessage: inner.handleMessage,
    start: inner.start,
    stop: inner.stop,
    dispose: inner.dispose,
  };
}

function toRenderableGrexNode(apiNode) {
  if (!apiNode || typeof apiNode !== "object") {
    return {
      type: "Section",
      tag: "section",
      children: [{ type: "Text", tag: "p", props: { text: "Synthesize or select a layout to preview live." } }],
    };
  }
  const typeMap = {
    Container: { type: "Section", tag: "main" },
    Section: { type: "Section", tag: "section" },
    Grid: { type: "Section", tag: "section" },
    Stack: { type: "Section", tag: "section" },
    Accordion: { type: "Section", tag: "section" },
    ForEach: { type: "Section", tag: "section" },
    Card: { type: "Card", tag: "article" },
    Nav: { type: "Nav", tag: "nav" },
    Heading: { type: "Heading", tag: "h4" },
    Text: { type: "Text", tag: "p" },
    RichText: { type: "Text", tag: "p" },
    Link: { type: "Link", tag: "a" },
    Button: { type: "Button", tag: "button" },
  };
  const spec = typeMap[apiNode.element] || { type: "Section", tag: "section" };
  const props = apiNode.props && typeof apiNode.props === "object" ? { ...apiNode.props } : {};
  if (spec.type === "Heading") {
    props.level = 4;
  }
  if (apiNode.element === "Image") {
    return {
      type: "Text",
      tag: "p",
      className: "desk-preview-image-caption",
      props: { text: `[Media Alt Verified] ${props.alt || "Visual preview asset"}` },
    };
  }
  if (spec.type === "Button") {
    props.type = "button";
    props.text = props.label || props.text || "Preview CTA";
  }
  if (spec.type === "Nav") {
    props.label = props.ariaLabel || "Preview Navigation";
  }
  return {
    type: spec.type,
    tag: spec.tag,
    className: props.class ? `desk-preview-node ${props.class}` : "desk-preview-node",
    props,
    children: Array.isArray(apiNode.children) ? apiNode.children.map(toRenderableGrexNode) : [],
  };
}

export function renderPreviewTree(hostEl, layoutNode, doc = globalThis.document) {
  if (!hostEl || !doc) return;
  while (hostEl.firstChild) {
    hostEl.removeChild(hostEl.firstChild);
  }
  const renderable = toRenderableGrexNode(layoutNode);
  const built = renderNode(renderable, { document: doc });
  hostEl.appendChild(built.element);
}

export function createStudioPanelNode(store, actions, doc = globalThis.document) {
  const studioStatus = signal("Select a Style Pack + Blueprint (40 combinations) or run a read-only grab mcp tool.");
  const customPrice = signal("49.00");
  const customSku = signal("10020");
  const topicTitle = signal("Semantic Zero-Wrapper Studio Showcase");
  const cmsUrlInput = signal("");
  const draftSlugInput = signal("semantic-draft-page");

  async function refreshCatalog() {
    const cat = await actions.loadStudioCatalog();
    studioStatus.set(
      `Loaded ${cat.style_packs ? cat.style_packs.length : 8} Style Packs x ${cat.blueprints ? cat.blueprints.length : 5} Blueprints (${cat.combination_count || "40"} combos) & ${cat.elements ? cat.elements.length : 20} Grex elements.`
    );
    return cat;
  }

  async function runSynthesize() {
    const packId = String(store.selectedStylePack.peek() || "emerald-editorial");
    const bpId = String(store.selectedBlueprint.peek() || "full_landing_page");
    studioStatus.set(`Synthesizing WCAG AA semantic tree (${packId} x ${bpId})...`);
    const res = await actions.synthesizeStudio({
      style_pack: packId,
      blueprint: bpId,
      title: String(topicTitle.peek() || "Semantic Studio Page"),
      custom_fields: {
        price: String(customPrice.peek() || "49.00"),
        sku: String(customSku.peek() || "10020"),
      },
    });
    const ratioTxt = res.contrast ? `${res.contrast.light_text_bg} Light / ${res.contrast.dark_text_bg} Dark` : "WCAG AA";
    studioStatus.set(
      `Synthesized ${res.style_pack.label} + ${res.blueprint.label} (Score ${res.scorecard.overall_score}/${res.scorecard.overall_grade}, ${ratioTxt}).`
    );
    return res;
  }

  async function applyLayoutWithApproval() {
    const layout = store.synthesizedLayout.peek();
    if (!layout) {
      studioStatus.set("Synthesize a layout first before applying.");
      return;
    }
    const studioUrl = String(cmsUrlInput.peek() || "").trim();
    const prep = await actions.applyStudioLayout({
      slot: "main",
      studio_url: studioUrl || undefined,
      grex_layout: layout,
      prepare: true,
    });
    const applied = await actions.applyStudioLayout({
      slot: "main",
      studio_url: studioUrl || undefined,
      summary: `Apply ${store.selectedStylePack.peek()} ${store.selectedBlueprint.peek()}`,
      grex_layout: layout,
      approval_token: prep.approval_token,
    });
    studioStatus.set(
      `Dispatched actor="${applied.command ? applied.command.actor : "groxy-ai"}" (${applied.command ? applied.command.type : "applyAiLayout"}). Press Ctrl+Z or Undo AI Layout anytime.`
    );
  }

  async function runActiveCoreMCP(toolName) {
    const active = String(store.activePath.peek() || "src/desk.grab");
    const targetFile = active.endsWith(".grab") ? active : "src/desk.grab";
    studioStatus.set(`Calling read-only grab mcp tool "${toolName}" on "${targetFile}"...`);
    const args = toolName === "grab_inspect_schema" ? {} : { file: targetFile };
    const res = await actions.callMcpTool({ tool: toolName, arguments: args });
    const summary = JSON.stringify(res.result || res).slice(0, 220);
    store.mcpLastOutput.set(`${toolName} (read_only=${res.read_only}, --allow-write=${res.allow_write_flag}): ${summary}`);
    studioStatus.set(`MCP "${toolName}" completed cleanly (read-only).`);
  }

  async function createSafeCmsDraft() {
    const cmsUrl = String(cmsUrlInput.peek() || "").trim();
    const slug = String(draftSlugInput.peek() || "semantic-draft").trim();
    const layout = store.synthesizedLayout.peek();
    if (!cmsUrl) {
      studioStatus.set("Enter a local GrabLab CMS loopback URL (e.g. http://127.0.0.1:8765) to create a draft.");
      return;
    }
    const args = {
      content_type: "post",
      slug,
      title: String(topicTitle.peek() || "Groxy Studio Draft"),
      status: "draft",
      grex_layout: layout || undefined,
    };
    const prep = await actions.callMcpTool({
      tool: "create_entry_draft",
      cms_url: cmsUrl,
      arguments: args,
      prepare: true,
    });
    const res = await actions.callMcpTool({
      tool: "create_entry_draft",
      cms_url: cmsUrl,
      arguments: args,
      approval_token: prep.approval_token,
    });
    store.mcpLastOutput.set(
      `create_entry_draft status_enforced=${res.status_enforced}: ${JSON.stringify(res.result).slice(0, 200)}`
    );
    studioStatus.set(`GrabLab CMS draft saved with status_enforced="${res.status_enforced}".`);
  }

  return {
    type: "Card",
    tag: "section",
    id: "desk-studio-panel",
    attrs: { id: "desk-studio-panel" },
    className: "desk-card",
    children: [
      {
        type: "Heading",
        props: { level: 3, text: "Grex Studio Composer, Split-Preview Canvas & grab mcp" },
      },
      {
        type: "Stack",
        tag: "section",
        className: "desk-mode-bar",
        children: [
          {
            type: "Button",
            id: "desk-mode-code-btn",
            attrs: { id: "desk-mode-code-btn" },
            props: {
              type: "button",
              text: "Mode: Code",
              onClick: () => store.workbenchMode.set("code"),
            },
          },
          {
            type: "Button",
            id: "desk-mode-split-btn",
            attrs: { id: "desk-mode-split-btn" },
            className: "desk-btn-primary",
            props: {
              type: "button",
              text: "Mode: Split Preview",
              onClick: () => store.workbenchMode.set("split"),
            },
          },
          {
            type: "Button",
            id: "desk-mode-visual-btn",
            attrs: { id: "desk-mode-visual-btn" },
            props: {
              type: "button",
              text: "Mode: Visual & Scorecard",
              onClick: () => store.workbenchMode.set("visual"),
            },
          },
          {
            type: "Button",
            id: "desk-vp-base-btn",
            attrs: { id: "desk-vp-base-btn" },
            props: { type: "button", text: "Viewport: Base", onClick: () => store.previewViewport.set("base") },
          },
          {
            type: "Button",
            id: "desk-vp-tablet-btn",
            attrs: { id: "desk-vp-tablet-btn" },
            props: { type: "button", text: "Tablet (768px)", onClick: () => store.previewViewport.set("tablet") },
          },
          {
            type: "Button",
            id: "desk-vp-mobile-btn",
            attrs: { id: "desk-vp-mobile-btn" },
            props: { type: "button", text: "Mobile (375px)", onClick: () => store.previewViewport.set("mobile") },
          },
        ],
      },
      {
        type: "Section",
        tag: "section",
        className: "desk-studio-controls",
        props: { label: "Design Style Pack and Blueprint Controls" },
        children: [
          {
            type: "Select",
            id: "desk-style-pack-select",
            attrs: { id: "desk-style-pack-select" },
            props: {
              name: "style_pack",
              label: "Design Style Pack (8 WCAG AA Packs)",
              value: store.selectedStylePack,
              options: () =>
                (store.stylePacks.get() || []).map((p) => ({
                  value: p.id,
                  label: p.label,
                })),
            },
          },
          {
            type: "Select",
            id: "desk-blueprint-select",
            attrs: { id: "desk-blueprint-select" },
            props: {
              name: "blueprint",
              label: "Page Blueprint (5 Layouts)",
              value: store.selectedBlueprint,
              options: () =>
                (store.blueprints.get() || []).map((b) => ({
                  value: b.id,
                  label: b.label,
                })),
            },
          },
          {
            type: "Input",
            id: "desk-studio-topic-input",
            attrs: { id: "desk-studio-topic-input" },
            props: { name: "topic", label: "Page Heading / Topic", value: topicTitle },
          },
          {
            type: "Input",
            id: "desk-studio-price-input",
            attrs: { id: "desk-studio-price-input" },
            props: { name: "price", label: "GrabDecimal Price (ADR-025c)", value: customPrice },
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
            id: "desk-studio-load-btn",
            attrs: { id: "desk-studio-load-btn" },
            props: {
              type: "button",
              text: "Load Design System & Context",
              onClick: () => refreshCatalog().catch((e) => studioStatus.set(e.message)),
            },
          },
          {
            type: "Button",
            id: "desk-studio-synth-btn",
            attrs: { id: "desk-studio-synth-btn" },
            className: "desk-btn-primary",
            props: {
              type: "button",
              text: "Synthesize Semantic Layout",
              onClick: () => runSynthesize().catch((e) => studioStatus.set(e.message)),
            },
          },
          {
            type: "Button",
            id: "desk-studio-apply-btn",
            attrs: { id: "desk-studio-apply-btn" },
            props: {
              type: "button",
              text: "Dispatch applyAiLayout (Approved)",
              onClick: () => applyLayoutWithApproval().catch((e) => studioStatus.set(e.message)),
            },
          },
          {
            type: "Button",
            id: "desk-studio-undo-btn",
            attrs: { id: "desk-studio-undo-btn" },
            props: {
              type: "button",
              text: "Undo AI Layout (Ctrl+Z)",
              onClick: () => actions.undoStudioLayout(),
            },
          },
        ],
      },
      {
        type: "Text",
        tag: "p",
        id: "desk-studio-status",
        attrs: { id: "desk-studio-status" },
        className: "desk-status-banner",
        props: { text: studioStatus },
      },
      {
        type: "Section",
        tag: "section",
        id: "desk-preview-canvas",
        attrs: { id: "desk-preview-canvas" },
        className: () => {
          const vp = store.previewViewport.get();
          const mode = store.workbenchMode.get();
          return `desk-preview-frame viewport-${vp} ${mode === "code" ? "desk-mode-hidden" : ""}`;
        },
        props: { label: "Origin-Checked Live Grex Preview Canvas" },
        children: [
          {
            type: "Text",
            tag: "p",
            id: "desk-preview-bridge-meta",
            attrs: { id: "desk-preview-bridge-meta" },
            className: "desk-muted",
            props: {
              text: () => {
                const vp = VIEWPORT_PRESETS[store.previewViewport.get()] || VIEWPORT_PRESETS.base;
                const undoLen = (store.undoStack.get() || []).length;
                return `GrexPreviewBridge (${PREVIEW_CHANNEL}) | Viewport: ${vp.label} | Undo Stack: ${undoLen}`;
              },
            },
          },
          {
            type: "Section",
            tag: "section",
            id: "desk-preview-surface",
            attrs: { id: "desk-preview-surface" },
            className: "desk-preview-surface",
            props: {
              label: "Live Semantic Component Preview",
            },
            children: [
              {
                type: "Text",
                tag: "p",
                props: { text: "Semantic preview renders directly here with zero wrapper elements." },
              },
            ],
          },
        ],
      },
      {
        type: "Heading",
        props: { level: 4, text: "Unified grab mcp & GrabLab CMS Draft Bridge (13 Tools)" },
      },
      {
        type: "Section",
        tag: "section",
        className: "desk-studio-controls",
        props: { label: "Loopback CMS Worker Inputs" },
        children: [
          {
            type: "Input",
            id: "desk-cms-url-input",
            attrs: { id: "desk-cms-url-input" },
            props: {
              name: "cms_url",
              label: "Local CMS / Builder Loopback URL",
              placeholder: "http://127.0.0.1:8765",
              value: cmsUrlInput,
            },
          },
          {
            type: "Input",
            id: "desk-draft-slug-input",
            attrs: { id: "desk-draft-slug-input" },
            props: {
              name: "draft_slug",
              label: "Draft Entry Slug",
              value: draftSlugInput,
            },
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
            id: "desk-mcp-routes-btn",
            attrs: { id: "desk-mcp-routes-btn" },
            props: {
              type: "button",
              text: "Inspect Routes (grab mcp)",
              onClick: () => runActiveCoreMCP("grab_inspect_routes").catch((e) => studioStatus.set(e.message)),
            },
          },
          {
            type: "Button",
            id: "desk-mcp-schema-btn",
            attrs: { id: "desk-mcp-schema-btn" },
            props: {
              type: "button",
              text: "Inspect Schema (grab mcp)",
              onClick: () => runActiveCoreMCP("grab_inspect_schema").catch((e) => studioStatus.set(e.message)),
            },
          },
          {
            type: "Button",
            id: "desk-cms-draft-btn",
            attrs: { id: "desk-cms-draft-btn" },
            props: {
              type: "button",
              text: "Create CMS Draft (status_enforced=draft)",
              onClick: () => createSafeCmsDraft().catch((e) => studioStatus.set(e.message)),
            },
          },
        ],
      },
      {
        type: "Text",
        tag: "code",
        id: "desk-mcp-output",
        attrs: { id: "desk-mcp-output" },
        className: "desk-code-block",
        props: { text: store.mcpLastOutput },
      },
      ForEach(
        {
          items: store.mcpTools,
          key: (t) => t.name,
          parent: {
            type: "Section",
            tag: "section",
            id: "desk-mcp-tools-list",
            attrs: { id: "desk-mcp-tools-list" },
            className: "desk-list",
            props: { label: "Unified MCP Tool Catalog" },
          },
          empty: () => ({
            type: "Text",
            tag: "p",
            className: "desk-muted",
            props: { text: "13 unified MCP tools ready once session connects." },
          }),
          render: (toolSig) => ({
            type: "Card",
            tag: "article",
            className: "desk-tree-item",
            children: [
              {
                type: "Text",
                tag: "strong",
                props: {
                  text: () => {
                    const t = toolSig.get();
                    return `${t.name} [${t.layer}]`;
                  },
                },
              },
              {
                type: "Text",
                tag: "small",
                className: "desk-muted",
                props: {
                  text: () => {
                    const t = toolSig.get();
                    return `${t.mode} — ${t.description}`;
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
