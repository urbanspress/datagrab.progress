/**
 * Pure Grex JS v1.0.0 1-Click Local Author Mode & Open-Weights Runtime Panel.
 * Zero model-switching dropdowns; exact ADR-025c base-10 ISBN-13, USD price & word count strings.
 */
import { ForEach, signal } from "../grex/grex.js";

export function createAuthorPanelNode(store, actions, doc = globalThis.document) {
  const targetSubdir = signal("author-studio");
  const bookTitle = signal("The Loopback Express");
  const authorName = signal("Independent Author");
  const isbn13 = signal("9780143127550");
  const priceUsd = signal("19.99");
  const wordCount = signal("1450");
  const jsonLdPreview = signal("schema.org/Book JSON-LD preview will appear here cleanly.");
  const statusMsg = signal(
    "1-click Author Mode creates Book/Chapter/CharacterBible/WorldLore schemas with status_enforced=\"draft\"."
  );

  async function onPlanAuthor() {
    try {
      const plan = await actions.authorInitPlan({
        target_subdir: String(targetSubdir.peek() || "author-studio").trim(),
        book_title: String(bookTitle.peek() || "").trim(),
        author_name: String(authorName.peek() || "").trim(),
        isbn_13: String(isbn13.peek() || "").trim(),
        price_usd: String(priceUsd.peek() || "").trim(),
        word_count: String(wordCount.peek() || "").trim(),
      });
      store.authorPlan.set(plan);
      if (plan && plan.schema_org_jsonld) {
        jsonLdPreview.set(plan.schema_org_jsonld);
      }
      statusMsg.set(plan.plain_summary || "Author Mode plan prepared.");
    } catch (err) {
      statusMsg.set(err && err.message ? err.message : String(err));
    }
  }

  async function onApplyAuthor() {
    const plan = store.authorPlan.peek();
    if (!plan || !plan.approval_token) return;
    try {
      const applied = await actions.authorInitApply({
        target_subdir: String(targetSubdir.peek() || "author-studio").trim(),
        book_title: String(bookTitle.peek() || "").trim(),
        author_name: String(authorName.peek() || "").trim(),
        isbn_13: String(isbn13.peek() || "").trim(),
        price_usd: String(priceUsd.peek() || "").trim(),
        word_count: String(wordCount.peek() || "").trim(),
        approval_token: plan.approval_token,
      });
      store.authorPlan.set(null);
      statusMsg.set(
        `Author Mode ready in ${applied.target_subdir} (status_enforced="${applied.status_enforced}").`
      );
    } catch (err) {
      statusMsg.set(err && err.message ? err.message : String(err));
    }
  }

  async function onCheckOllama() {
    try {
      const res = await actions.authorOllamaStatus();
      statusMsg.set(res.plain_summary || "Checked local Ollama loopback status.");
    } catch (err) {
      statusMsg.set(err && err.message ? err.message : String(err));
    }
  }

  async function onStartOllama() {
    try {
      const res = await actions.authorOllamaStart({});
      statusMsg.set(res.plain_summary || "Started local open-weights runtime.");
    } catch (err) {
      statusMsg.set(err && err.message ? err.message : String(err));
    }
  }

  return {
    type: "Card",
    tag: "section",
    id: "desk-author-panel",
    attrs: { id: "desk-author-panel" },
    className: "desk-card",
    children: [
      {
        type: "Heading",
        props: { level: 3, text: "1-Click Local Author Mode & Open-Weights Runtime" },
      },
      {
        type: "Text",
        tag: "p",
        id: "desk-author-status",
        attrs: { id: "desk-author-status" },
        className: "desk-status-banner",
        props: { text: statusMsg },
      },
      {
        type: "Form",
        id: "desk-author-form",
        attrs: { id: "desk-author-form" },
        className: "desk-studio-controls",
        props: { onSubmit: onPlanAuthor },
        children: [
          {
            type: "Input",
            id: "author-subdir",
            attrs: { id: "author-subdir" },
            props: { name: "target_subdir", label: "Author Subdir", value: targetSubdir },
          },
          {
            type: "Input",
            id: "author-book-title",
            attrs: { id: "author-book-title" },
            props: { name: "book_title", label: "Book Title", value: bookTitle },
          },
          {
            type: "Input",
            id: "author-pen-name",
            attrs: { id: "author-pen-name" },
            props: { name: "author_name", label: "Author Name", value: authorName },
          },
          {
            type: "Input",
            id: "author-isbn13",
            attrs: { id: "author-isbn13" },
            props: { name: "isbn_13", label: "ISBN-13 (ADR-025c)", value: isbn13 },
          },
          {
            type: "Input",
            id: "author-price-usd",
            attrs: { id: "author-price-usd" },
            props: { name: "price_usd", label: "List Price USD", value: priceUsd },
          },
          {
            type: "Input",
            id: "author-word-count",
            attrs: { id: "author-word-count" },
            props: { name: "word_count", label: "Starter Word Count", value: wordCount },
          },
          {
            type: "Stack",
            tag: "section",
            props: { direction: "row", gap: "sm" },
            attrs: { "aria-label": "Author Mode and local AI actions" },
            children: [
              {
                type: "Button",
                id: "author-plan-btn",
                attrs: { id: "author-plan-btn" },
                props: {
                  type: "button",
                  text: "Preview Author Mode Plan",
                  onClick: onPlanAuthor,
                },
              },
              {
                type: "Button",
                id: "author-apply-btn",
                attrs: { id: "author-apply-btn" },
                props: {
                  type: "button",
                  text: "Approve & Scaffold Author Mode",
                  onClick: onApplyAuthor,
                  disabled: () => !store.authorPlan.get(),
                },
              },
              {
                type: "Button",
                id: "author-ollama-check-btn",
                attrs: { id: "author-ollama-check-btn" },
                props: {
                  type: "button",
                  text: "Check Local Ollama (127.0.0.1:11434)",
                  onClick: onCheckOllama,
                },
              },
              {
                type: "Button",
                id: "author-ollama-start-btn",
                attrs: { id: "author-ollama-start-btn" },
                props: {
                  type: "button",
                  text: "Launch Local AI Runtime",
                  onClick: onStartOllama,
                },
              },
            ],
          },
        ],
      },
      {
        type: "Text",
        tag: "code",
        id: "author-ollama-badge",
        attrs: { id: "author-ollama-badge" },
        className: "desk-status-banner",
        props: {
          text: () => {
            const st = store.ollamaStatus.get();
            if (!st) return "Ollama Loopback: Not probed yet ($0 cloud API fees)";
            return `Ollama Loopback ${st.loopback_url}: running=${Boolean(st.running)} | detected=${st.detected_count || "0"} | $0 fee`;
          },
        },
      },
      {
        type: "Text",
        tag: "code",
        id: "author-jsonld-preview",
        attrs: { id: "author-jsonld-preview" },
        className: "desk-status-banner",
        props: {
          text: jsonLdPreview,
        },
      },
      ForEach(
        {
          items: () => {
            const p = store.authorPlan.get();
            return p && Array.isArray(p.planned_files) ? p.planned_files : [];
          },
          key: (f) => f.path,
          parent: {
            type: "Section",
            tag: "section",
            className: "desk-list",
            props: { label: "Planned Author Mode Files" },
          },
          empty: () => ({
            type: "Text",
            tag: "p",
            className: "desk-muted",
            props: { text: "Click Preview Author Mode Plan to inspect schemas, SITE-CONTEXT.md & draft chapters." },
          }),
          render: (fSig) => ({
            type: "Card",
            tag: "article",
            className: "desk-diff-row",
            children: [
              {
                type: "Text",
                tag: "strong",
                props: {
                  text: () => {
                    const f = fSig.get();
                    return `${f.path} (${f.byte_count} bytes)`;
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
