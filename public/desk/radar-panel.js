import { ForEach, signal } from "../grex/grex.js";

export function createRadarPanelNode(store, api, doc) {
  const url = signal("");
  const width = signal("45");
  const genre = signal("");
  const price = signal("");
  const feeds = signal("offline");
  const slug = signal("");
  const title = signal("");
  const setup = signal(null);
  const pending = signal(null);
  const receipt = signal(null);
  const draft = signal(null);
  const message = signal("");
  const busy = signal(false);

  const input = (name, label, value, inputType = "text") => ({
    type: "Input",
    props: { name, label, value, type: inputType },
  });
  const button = (text, onClick, disabled, className) => ({
    type: "Button",
    className,
    props: { type: "button", text, onClick, disabled },
  });
  const paragraph = (text) => ({ type: "Text", tag: "p", props: { text } });

  function native(type, args = {}) {
    const bridge = globalThis.window?.chrome?.webview;
    if (!bridge) {
      message.set("Open Groxy Desk for Windows to use the built-in browser.");
      return;
    }
    bridge.postMessage(JSON.stringify({ type, ...args }));
  }

  async function run(action) {
    if (busy.peek()) return;
    busy.set(true);
    message.set("");
    try {
      await action();
    } catch (error) {
      message.set(error.message);
    } finally {
      busy.set(false);
    }
  }

  const brief = () => pending.get()?.brief;

  const leftBrowserPane = {
    type: "Section",
    tag: "section",
    id: "desk-browser-pane",
    attrs: { id: "desk-browser-pane" },
    className: "desk-panel desk-browser-pane",
    props: { label: "Isolated WebView2 Allowlisted Browser" },
    children: [
      { type: "Heading", props: { level: 3, text: "Browser Co-Pilot" } },
      {
        type: "Stack",
        tag: "section",
        className: "desk-badge-row",
        children: [
          {
            type: "Text",
            tag: "span",
            id: "desk-browser-origin-lock",
            attrs: { id: "desk-browser-origin-lock" },
            className: () =>
              store.browserStatus.get().origin
                ? "desk-badge desk-badge-success"
                : "desk-badge",
            props: {
              text: () =>
                store.browserStatus.get().origin
                  ? `Origin Locked (GRAB-E402): ${store.browserStatus.get().origin}`
                  : "Origin Lock Ready (GRAB-E402 Allowlist)",
            },
          },
        ],
      },
      paragraph(() => store.browserStatus.get().message || store.browserStatus.get().origin || "Browser pane closed."),
      input("research_url", "Allowlisted HTTPS address", url, "url"),
      {
        type: "Stack",
        tag: "section",
        className: "desk-badge-row",
        props: { direction: "row", gap: "sm" },
        children: [
          button("Open browser pane", () => native("groxy-browser-open", { url: url.peek().trim() }), undefined, "desk-btn-primary"),
          button("Close browser pane", () => native("groxy-browser-close")),
          {
            type: "Select",
            props: {
              name: "browser_width",
              label: "Browser width",
              value: width,
              options: ["30", "45", "60"].map((value) => ({ value, label: value + "%" })),
            },
          },
          button("Resize pane", () => native("groxy-browser-resize", { width: Number(width.peek()) })),
        ],
      },
      {
        type: "Stack",
        tag: "section",
        className: "desk-badge-row",
        children: [
          button("Capture selected text", () => native("groxy-browser-capture"), () => !store.browserStatus.get().origin),
          button("Add selection to Groxy AI prompt", () => {
            const selection = store.browserSelection.peek();
            if (!selection?.text) return;
            const reference = `Quoted research reference from ${selection.origin}\n${selection.text}`;
            store.groxyPrompt.set([store.groxyPrompt.peek(), reference].filter(Boolean).join("\n\n"));
            store.browserSelection.set(null);
            message.set("Review the captured text in your Groxy AI prompt before sending.");
          }, () => !store.browserSelection.get()?.text),
        ],
      },
      paragraph(() => {
        const selection = store.browserSelection.get();
        if (!selection) return "";
        return selection.text || "Select text in the browser pane and capture it again.";
      }),
      {
        type: "Stack",
        tag: "section",
        className: "desk-badge-row",
        children: [
          button("Preview business name", () => native("groxy-browser-brand-preview"), () => !store.browserStatus.get().origin),
          button("Fill business name", () => {
            store.browserBrand.set(null);
            native("groxy-browser-brand-prefill");
          }, () => !store.browserBrand.get()?.organization),
        ],
      },
      paragraph(() => {
        const brand = store.browserBrand.get();
        return brand?.message || (brand?.organization ? `${brand.organization} will be shared with ${brand.origin}.` : "");
      }),
      {
        type: "Text",
        tag: "p",
        className: "desk-muted",
        props: {
          text: "Isolated WebView2 profile: zero bridge/CMS/DB credentials exposed. You control all passwords, CAPTCHAs, and form submissions.",
        },
      },
    ],
  };

  const rightRadarPane = {
    type: "Section",
    tag: "section",
    id: "desk-radar-pane",
    attrs: { id: "desk-radar-pane" },
    className: "desk-panel desk-radar-pane",
    props: { label: "Bestseller Radar, BRAND-DNA.md Prefill and GrabLab CMS Draft Handoff" },
    children: [
      { type: "Heading", props: { level: 3, text: "Bestseller Radar" } },
      {
        type: "Stack",
        tag: "section",
        className: "desk-badge-row",
        children: [
          button("Check Radar and CMS setup", () => run(async () => {
            setup.set(await api.radarCapabilities());
          }), busy),
          button("Open CMS Radar", () => native("groxy-browser-open", { url: setup.peek().cms_radar_url }),
            () => !setup.get()?.cms_radar_url),
        ],
      },
      paragraph(() => {
        const state = setup.get();
        if (!state) return "";
        return [
          state.research_configured ? "Radar origin configured." : "Configure GROXY_DESK_RADAR_ORIGIN.",
          state.cms_configured ? "CMS origin and bearer configured." : "Configure GROXY_DESK_CMS_ORIGIN and GROXY_DESK_CMS_MCP_BEARER.",
        ].join(" ");
      }),
      {
        type: "Section",
        tag: "section",
        className: "desk-studio-controls",
        props: { label: "Radar Niche Explorer" },
        children: [
          input("radar_genre", "Genre slug", genre),
          input("radar_price", "Optional price tier in USD", price),
          {
            type: "Select",
            props: {
              name: "radar_source",
              label: "Research sources",
              value: feeds,
              options: [{ value: "offline", label: "Offline playbook" }, { value: "live", label: "Include live feeds" }],
            },
          },
        ],
      },
      button("Research genre", () => run(async () => {
        receipt.set(null);
        draft.set(null);
        pending.set(null);
        const args = { genre_slug: genre.peek().trim(), include_live_feeds: feeds.peek() === "live" };
        if (price.peek().trim()) args.price_tier_usd = price.peek().trim();
        pending.set(await api.radarResearch(args));
      }), busy, "desk-btn-primary"),
      paragraph(() => {
        const data = brief();
        if (!data) return "";
        const source = data.live_feed_used ? "Live feed used" : "Offline playbook";
        return source + " · " + (data.display_name || data.genre_slug || "");
      }),
      ForEach({
        items: () => brief()?.live_citations ?? [],
        key: (citation) => citation,
        parent: { type: "Section", tag: "section", props: { label: "Radar source links" } },
        render: (citation) => button(() => citation.get(), () => native("groxy-browser-open", { url: citation.peek() })),
      }, { document: doc }),
      {
        type: "Text",
        tag: "code",
        className: "desk-code-block",
        props: {
          text: () => (brief() ? JSON.stringify(brief(), null, 2) : ""),
        },
      },
      { type: "Heading", props: { level: 4, text: "BRAND-DNA.md Prefill & GrabLab CMS Draft Handoff" } },
      {
        type: "Text",
        tag: "p",
        id: "desk-brand-dna-preview",
        attrs: { id: "desk-brand-dna-preview" },
        className: "desk-status-banner",
        props: {
          text: () => {
            const brand = store.browserBrand.get();
            if (brand?.organization) {
              return `BRAND-DNA.md Ready: "${brand.organization}" -> ${brand.origin || "allowlisted origin"}`;
            }
            return "BRAND-DNA.md Prefill: Open an allowlisted registration page and click Preview business name.";
          },
        },
      },
      button("Approve Desk draft receipt", () => run(async () => {
        const current = pending.peek();
        receipt.set(await api.radarReceipt({ radar_plan_id: current.radar_plan_id, approval_token: current.approval_token }));
      }), () => busy.get() || !pending.get() || Boolean(receipt.get())),
      {
        type: "Section",
        tag: "section",
        className: "desk-studio-controls",
        props: { label: "GrabLab CMS Draft Metadata" },
        children: [
          input("book_slug", "Draft book slug", slug),
          input("book_title", "Draft book title", title),
        ],
      },
      button("Review CMS draft", () => run(async () => {
        draft.set(await api.radarPlanDraft({
          radar_plan_id: receipt.peek().radar_plan_id,
          slug: slug.peek().trim(),
          title: title.peek().trim(),
        }));
      }), () => busy.get() || !receipt.get()),
      paragraph(() => {
        const proposal = draft.get();
        return proposal ? proposal.title + " (" + proposal.slug + ") → " + proposal.target + " · draft" : "";
      }),
      button("Approve and create CMS draft", () => run(async () => {
        const proposal = draft.peek();
        draft.set(null);
        const result = await api.radarCreateDraft({
          radar_plan_id: receipt.peek().radar_plan_id,
          slug: proposal.slug,
          title: proposal.title,
          approval_token: proposal.approval_token,
        });
        message.set("Created draft " + result.slug + ".");
      }), () => busy.get() || !draft.get(), "desk-btn-primary"),
      { type: "Text", tag: "p", attrs: { role: "status" }, className: "desk-status-banner", props: { text: message } },
    ],
  };

  return {
    type: "Card",
    tag: "section",
    attrs: { id: "desk-radar-panel" },
    className: "desk-card desk-browser-radar-root",
    children: [
      {
        type: "Section",
        tag: "section",
        className: "desk-browser-radar-split",
        props: { label: "Browser Co-Pilot and Bestseller Radar Split Workspace" },
        children: [leftBrowserPane, rightRadarPane],
      },
    ],
  };
}
