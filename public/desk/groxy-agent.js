/**
 * Unified Groxy AI Default Agent & Automatic Adaptive Reasoning Sidebar.
 * Strict Invariant: Groxy AI is the permanent sole identity — ZERO model-switching dropdowns!
 */
import { signal } from "../grex/grex.js";

export function createGroxySidebarNode(store, actions) {
  const promptText = signal("");
  const groxyStatus = signal(
    "Groxy AI ready (Adaptive Reasoning: Fast & Deep gated workflows)."
  );

  async function submitPrompt(customPrompt = null) {
    const prompt = String(
      customPrompt !== null ? customPrompt : promptText.peek() || ""
    ).trim();
    if (!prompt) {
      groxyStatus.set("Enter an instruction or click a quick action.");
      return;
    }
    try {
      groxyStatus.set("Groxy AI analyzing request...");
      await actions.askGroxy({
        prompt,
        target_path: store.activePath.peek() || "",
      });
      const latest = store.groxyResponse.peek();
      if (latest) {
        groxyStatus.set(
          `[${latest.reasoning_mode.toUpperCase()}] ${latest.summary}`
        );
      }
    } catch (err) {
      groxyStatus.set(err && err.message ? err.message : String(err));
    }
  }

  async function approveAndApply() {
    const latest = store.groxyResponse.peek();
    if (!latest || !latest.approval_token || !latest.target_path) {
      groxyStatus.set("No pending gated mutation plan to apply.");
      return;
    }
    try {
      const applied = await actions.applyGroxyMutation({
        approval_token: latest.approval_token,
        target_path: latest.target_path,
        content: latest.proposed_content,
      });
      groxyStatus.set(
        `Applied to ${applied.target_path} (Snapshot ${applied.snapshot_id} saved for 1-click rollback).`
      );
    } catch (err) {
      groxyStatus.set(err && err.message ? err.message : String(err));
    }
  }

  async function rollbackLatestSnapshot() {
    const latest = store.groxyResponse.peek();
    const snapId = latest && latest.snapshot ? latest.snapshot.snapshot_id : "";
    if (!snapId) {
      groxyStatus.set("No snapshot selected for rollback.");
      return;
    }
    try {
      const restored = await actions.restoreSnapshot(snapId);
      groxyStatus.set(
        `Restored ${restored.target_path} to snapshot ${restored.snapshot_id}.`
      );
    } catch (err) {
      groxyStatus.set(err && err.message ? err.message : String(err));
    }
  }

  return {
    type: "Section",
    tag: "aside",
    id: "desk-groxy-sidebar",
    attrs: { id: "desk-groxy-sidebar" },
    className: "desk-panel",
    props: { label: "Unified Groxy AI Co-Pilot" },
    children: [
      {
        type: "Heading",
        props: { level: 2, text: "Groxy AI" },
      },
      {
        type: "Stack",
        tag: "section",
        className: "desk-badge-row",
        children: [
          {
            type: "Text",
            tag: "span",
            id: "desk-groxy-identity-badge",
            attrs: { id: "desk-groxy-identity-badge" },
            className: "desk-badge desk-badge-success",
            props: { text: "Unified Identity: Groxy AI" },
          },
          {
            type: "Text",
            tag: "span",
            id: "desk-groxy-mode-badge",
            attrs: { id: "desk-groxy-mode-badge" },
            className: "desk-badge",
            props: {
              text: () => {
                const resp = store.groxyResponse.get();
                return resp
                  ? `Mode: ${resp.reasoning_mode}`
                  : "Adaptive Router: Auto";
              },
            },
          },
        ],
      },
      {
        type: "Card",
        tag: "section",
        className: "desk-card",
        children: [
          {
            type: "Heading",
            props: { level: 3, text: "Adaptive Reasoning Co-Pilot" },
          },
          {
            type: "Stack",
            tag: "section",
            className: "desk-badge-row",
            children: [
              {
                type: "Button",
                id: "desk-groxy-audit-btn",
                attrs: { id: "desk-groxy-audit-btn" },
                props: {
                  type: "button",
                  text: "Audit 22 Rules (Fast)",
                  onClick: () =>
                    submitPrompt("Run deterministic 22-rule audit on active file"),
                },
              },
              {
                type: "Button",
                id: "desk-groxy-check-btn",
                attrs: { id: "desk-groxy-check-btn" },
                props: {
                  type: "button",
                  text: "grab check (Fast)",
                  onClick: () => submitPrompt("Run grab check and format"),
                },
              },
            ],
          },
          {
            type: "Form",
            id: "desk-groxy-form",
            attrs: { id: "desk-groxy-form" },
            props: {
              onSubmit: async () => {
                await submitPrompt();
              },
            },
            children: [
              {
                type: "Input",
                id: "groxy-prompt-input",
                attrs: { id: "groxy-prompt-input" },
                props: {
                  type: "textarea",
                  name: "prompt",
                  label: "Ask Groxy AI in plain English",
                  placeholder:
                    "e.g. Audit current view OR Fix wrapper elements safely with snapshot...",
                  value: promptText,
                },
              },
              {
                type: "Button",
                id: "desk-groxy-submit-btn",
                attrs: { id: "desk-groxy-submit-btn" },
                className: "desk-btn-primary",
                props: { type: "submit", text: "Ask Groxy AI" },
              },
            ],
          },
          {
            type: "Text",
            tag: "p",
            id: "desk-groxy-status",
            attrs: { id: "desk-groxy-status" },
            className: "desk-muted",
            props: { text: groxyStatus },
          },
        ],
      },
      {
        type: "Card",
        tag: "section",
        id: "desk-groxy-plan-card",
        attrs: { id: "desk-groxy-plan-card" },
        className: "desk-card",
        children: [
          {
            type: "Heading",
            props: { level: 3, text: "Audit Scorecard & Safety Gate" },
          },
          {
            type: "Text",
            tag: "p",
            id: "desk-groxy-audit-summary",
            attrs: { id: "desk-groxy-audit-summary" },
            className: "desk-muted",
            props: {
              text: () => {
                const resp = store.groxyResponse.get();
                if (!resp) {
                  return "Ask Groxy AI to view 22-rule scorecards or gated mutation plans.";
                }
                if (resp.audit) {
                  return `Score ${resp.audit.score}/100 (Grade ${resp.audit.grade}, PublishSafe=${resp.audit.publish_safe}, divCount=${resp.audit.div_count})`;
                }
                return resp.summary || "Completed.";
              },
            },
          },
          {
            type: "Text",
            tag: "code",
            id: "desk-groxy-proposal-preview",
            attrs: { id: "desk-groxy-proposal-preview" },
            className: "desk-code-block",
            props: {
              text: () => {
                const resp = store.groxyResponse.get();
                if (!resp) {
                  return "";
                }
                if (resp.proposed_content) {
                  return resp.proposed_content;
                }
                if (resp.plan && resp.plan.length > 0) {
                  return resp.plan
                    .map((s) => `${s.step_number}. ${s.title} — ${s.description}`)
                    .join("\n");
                }
                return resp.summary || "";
              },
            },
          },
          {
            type: "Stack",
            tag: "section",
            className: "desk-badge-row",
            children: [
              {
                type: "Button",
                id: "desk-groxy-approve-btn",
                attrs: { id: "desk-groxy-approve-btn" },
                className: "desk-btn-primary",
                props: {
                  type: "button",
                  text: "Approve & Apply",
                  disabled: () => {
                    const r = store.groxyResponse.get();
                    return !(r && r.approval_required && r.approval_token);
                  },
                  onClick: () => approveAndApply(),
                },
              },
              {
                type: "Button",
                id: "desk-groxy-rollback-btn",
                attrs: { id: "desk-groxy-rollback-btn" },
                props: {
                  type: "button",
                  text: "Rollback Snapshot",
                  disabled: () => {
                    const r = store.groxyResponse.get();
                    return !(r && r.snapshot && r.snapshot.snapshot_id);
                  },
                  onClick: () => rollbackLatestSnapshot(),
                },
              },
            ],
          },
        ],
      },
    ],
  };
}
