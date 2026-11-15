/**
 * Pure Grex JS v1.0.0 22-Rule / 4-Pillar Scorecard & "Let Groxy Boost This" Diff Panel.
 * Zero wrapper nodes, zero raw HTML DOM writes, exact ADR-025c base-10 wire preservation.
 */
import { ForEach, signal } from "../grex/grex.js";

const PILLAR_KEYS = [
  { key: "on_page_seo", label: "On-Page SEO (25%)" },
  { key: "content_a11y", label: "Content & WCAG AA (25%)" },
  { key: "performance_dom", label: "Performance & Clean DOM (25%)" },
  { key: "linking_readiness", label: "Internal Linking & Schema (25%)" },
];

export function createScorecardPanelNode(store, actions, doc = globalThis.document) {
  const auditFeedback = signal("Click Run 22-Rule Audit or Let Groxy Boost This to evaluate.");
  const boostStatus = signal("Ready to compute side-by-side semantic & WCAG AA diffs.");

  async function runAudit() {
    const targetPath = String(store.activePath.peek() || "").trim();
    const layout = store.synthesizedLayout ? store.synthesizedLayout.peek() : null;
    auditFeedback.set(`Auditing ${targetPath || "current layout"} across 22 deterministic rules...`);
    const report = await actions.runV2Audit({
      target_path: targetPath || undefined,
      html: targetPath && targetPath.endsWith(".grab.html") ? undefined : String(store.activeFileContent.peek() || ""),
      grex_layout: layout || undefined,
    });
    auditFeedback.set(
      `Scorecard: ${report.overall_score}/100 (Grade ${report.overall_grade}, PublishSafe=${report.publish_safe}, divCount=${report.div_count})`
    );
    return report;
  }

  async function runBoostProposal() {
    const targetPath = String(store.activePath.peek() || "views/workbench.grab.html").trim();
    const packId = String(store.selectedStylePack ? store.selectedStylePack.peek() : "emerald-editorial");
    boostStatus.set(`Proposing Deep remediation for "${targetPath}"...`);
    const res = await actions.proposeBoost({
      target_path: targetPath,
      style_pack: packId,
      html: targetPath.endsWith(".grab.html") ? undefined : String(store.activeFileContent.peek() || ""),
      grex_layout: store.synthesizedLayout ? store.synthesizedLayout.peek() : undefined,
    });
    const d = res && res.proposal && res.proposal.projected_delta ? res.proposal.projected_delta : null;
    if (d) {
      boostStatus.set(
        `Proposed Boost ready: ${d.before_score} (${d.before_grade}) -> ${d.after_score} (${d.after_grade}) [${d.delta}] | Snapshot ${res.snapshot ? res.snapshot.snapshot_id : ""}`
      );
    }
    return res;
  }

  async function approveBoost() {
    const boost = store.boostProposal.peek();
    if (!boost || !boost.approval_token) {
      boostStatus.set("Generate a Boost proposal first before approving.");
      return;
    }
    boostStatus.set(`Applying approved Boost to "${boost.target_path}" with Pre-Mutation Snapshot...`);
    const applied = await actions.approveBoost({
      approval_token: boost.approval_token,
      target_path: boost.target_path,
    });
    boostStatus.set(
      `Boost applied safely (SHA-256 ${String(applied.applied_sha256 || "").slice(0, 12)}...). Click Rollback Snapshot if needed.`
    );
  }

  async function rollbackLastBoost() {
    const boost = store.boostProposal.peek();
    const snapId = boost && boost.snapshot ? boost.snapshot.snapshot_id : "";
    if (!snapId) {
      boostStatus.set("No Pre-Mutation Snapshot recorded to roll back.");
      return;
    }
    const res = await actions.restoreSnapshot(snapId);
    boostStatus.set(
      `Snapshot "${res.snapshot_id}" rolled back (SHA-256 ${String(res.restored_sha256 || "").slice(0, 12)}...).`
    );
  }

  return {
    type: "Card",
    tag: "section",
    id: "desk-scorecard-panel",
    attrs: { id: "desk-scorecard-panel" },
    className: () =>
      store.workbenchMode.get() === "code" ? "desk-card desk-mode-hidden" : "desk-card",
    children: [
      {
        type: "Heading",
        props: { level: 3, text: "22-Rule Deterministic Scorecard & Let Groxy Boost This" },
      },
      {
        type: "Stack",
        tag: "section",
        className: "desk-badge-row",
        children: [
          {
            type: "Button",
            id: "desk-v2-audit-btn",
            attrs: { id: "desk-v2-audit-btn" },
            props: {
              type: "button",
              text: "Run 22-Rule Audit",
              onClick: () => runAudit().catch((e) => auditFeedback.set(e.message)),
            },
          },
          {
            type: "Button",
            id: "desk-boost-propose-btn",
            attrs: { id: "desk-boost-propose-btn" },
            className: "desk-btn-primary",
            props: {
              type: "button",
              text: "Let Groxy Boost This",
              onClick: () => runBoostProposal().catch((e) => boostStatus.set(e.message)),
            },
          },
          {
            type: "Button",
            id: "desk-boost-approve-btn",
            attrs: { id: "desk-boost-approve-btn" },
            props: {
              type: "button",
              text: "Approve & Apply Boost",
              onClick: () => approveBoost().catch((e) => boostStatus.set(e.message)),
            },
          },
          {
            type: "Button",
            id: "desk-boost-rollback-btn",
            attrs: { id: "desk-boost-rollback-btn" },
            props: {
              type: "button",
              text: "Rollback Boost Snapshot",
              onClick: () => rollbackLastBoost().catch((e) => boostStatus.set(e.message)),
            },
          },
        ],
      },
      {
        type: "Text",
        tag: "p",
        id: "desk-scorecard-summary",
        attrs: { id: "desk-scorecard-summary" },
        className: "desk-status-banner",
        props: { text: auditFeedback },
      },
      {
        type: "Section",
        tag: "section",
        id: "desk-pillar-grid",
        attrs: { id: "desk-pillar-grid" },
        className: "desk-score-grid",
        props: { label: "Four Equal Scorecard Pillars" },
        children: PILLAR_KEYS.map((p) => ({
          type: "Card",
          tag: "article",
          className: "desk-pillar-card",
          children: [
            {
              type: "Heading",
              props: { level: 4, text: p.label },
            },
            {
              type: "Text",
              tag: "p",
              props: {
                text: () => {
                  const rep = store.scorecardReport.get();
                  if (!rep || !rep.categories || !rep.categories[p.key]) {
                    return "Pending audit";
                  }
                  const c = rep.categories[p.key];
                  return `Score ${c.score}/100 — Grade ${c.grade} (Weight ${c.weight}%)`;
                },
              },
            },
          ],
        })),
      },
      {
        type: "Text",
        tag: "p",
        id: "desk-boost-status",
        attrs: { id: "desk-boost-status" },
        className: "desk-muted",
        props: { text: boostStatus },
      },
      {
        type: "Text",
        tag: "code",
        id: "desk-schema-org-preview",
        attrs: { id: "desk-schema-org-preview" },
        className: "desk-code-block",
        props: {
          text: () => {
            const rep = store.scorecardReport.get();
            if (!rep || !rep.schema_org || !rep.schema_org.json_text) {
              return "Schema.org JSON-LD preview will appear after running 22-Rule Audit.";
            }
            return `Schema.org (${rep.schema_org.type}): ${rep.schema_org.json_text}`;
          },
        },
      },
      {
        type: "Heading",
        props: { level: 4, text: "Side-by-Side Boost Diffs & Rule Findings" },
      },
      ForEach(
        {
          items: () => {
            const boost = store.boostProposal.get();
            if (boost && boost.proposal && Array.isArray(boost.proposal.diffs) && boost.proposal.diffs.length > 0) {
              return boost.proposal.diffs.map((d, idx) => ({
                id: `${d.field}-${idx}`,
                title: `${d.field} (${(d.rule_ids || []).join(", ")})`,
                detail: `${d.summary} | Before: ${d.before} -> After: ${d.after}`,
              }));
            }
            const rep = store.scorecardReport.get();
            if (rep && Array.isArray(rep.findings)) {
              return rep.findings.slice(0, 22).map((f) => ({
                id: f.rule_id,
                title: `${f.rule_id} [${f.status}] ${f.points_earned}/${f.max_points}`,
                detail: `${f.title}: ${f.message}`,
              }));
            }
            return [];
          },
          key: (item) => item.id,
          parent: {
            type: "Section",
            tag: "section",
            id: "desk-findings-list",
            attrs: { id: "desk-findings-list" },
            className: "desk-list",
            props: { label: "Side-by-Side Diffs and Rule Findings" },
          },
          empty: () => ({
            type: "Text",
            tag: "p",
            className: "desk-muted",
            props: { text: "No findings or diffs loaded yet." },
          }),
          render: (itemSig) => ({
            type: "Card",
            tag: "article",
            className: "desk-diff-row",
            children: [
              { type: "Text", tag: "strong", props: { text: () => itemSig.get().title } },
              { type: "Text", tag: "p", className: "desk-muted", props: { text: () => itemSig.get().detail } },
            ],
          }),
        },
        { document: doc }
      ),
    ],
  };
}
