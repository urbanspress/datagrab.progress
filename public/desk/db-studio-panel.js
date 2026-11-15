/**
 * Pure Grex JS v1.0.0 Loopback SSH DB Tunnels & Built-In Visual DB Explorer Panel.
 * Preserves exact ADR-025c base-10 wire strings across ports, pagination, and SQL row values.
 */
import { ForEach, signal } from "../grex/grex.js";

export function createDBStudioPanelNode(store, actions, doc = globalThis.document) {
  const engine = signal("sqlite");
  const sqlitePath = signal("data/app.db");
  const tunnelId = signal("");
  const tableName = signal("books");
  const pageNum = signal("1");
  const limitNum = signal("25");
  const readOnlySQL = signal("SELECT 1 AS ok");
  const mutateSQL = signal("");
  const statusMsg = signal(
    "Read-only DB Explorer active; up to 8 loopback SSH tunnels (127.0.0.1:15432-15439)."
  );

  function buildTarget() {
    const eng = String(engine.peek() || "sqlite").trim();
    if (eng === "sqlite") {
      return {
        engine: "sqlite",
        sqlite_path: String(sqlitePath.peek() || "").trim(),
      };
    }
    return {
      engine: eng,
      tunnel_id: String(tunnelId.peek() || "").trim(),
    };
  }

  async function onInspectSchema() {
    try {
      const res = await actions.dbSchema(buildTarget());
      statusMsg.set(
        `Loaded ${Array.isArray(res.tables) ? res.tables.length : 0} table(s) from ${res.target_label || res.engine}.`
      );
    } catch (err) {
      statusMsg.set(err && err.message ? err.message : String(err));
    }
  }

  async function onBrowseRows() {
    try {
      const res = await actions.dbRows({
        target: buildTarget(),
        table: String(tableName.peek() || "").trim(),
        page: String(pageNum.peek() || "1").trim(),
        limit: String(limitNum.peek() || "25").trim(),
      });
      statusMsg.set(
        `Showing page ${res.page}/${res.total_pages} (${res.total_rows} total rows) for ${res.table}.`
      );
    } catch (err) {
      statusMsg.set(err && err.message ? err.message : String(err));
    }
  }

  async function onRunQuery() {
    try {
      const res = await actions.dbQuery({
        target: buildTarget(),
        sql: String(readOnlySQL.peek() || ""),
      });
      statusMsg.set(
        `Read-only ${res.verb} returned ${res.row_count} row(s) cleanly.`
      );
    } catch (err) {
      statusMsg.set(err && err.message ? err.message : String(err));
    }
  }

  async function onPlanMutate() {
    try {
      const plan = await actions.dbMutatePlan({
        target: buildTarget(),
        sql: String(mutateSQL.peek() || ""),
      });
      store.dbMutatePlan.set(plan);
      statusMsg.set(plan.plain_summary || "Captured Pre-Mutation DB Snapshot.");
    } catch (err) {
      statusMsg.set(err && err.message ? err.message : String(err));
    }
  }

  async function onApplyMutate() {
    const plan = store.dbMutatePlan.peek();
    if (!plan || !plan.approval_token) return;
    try {
      const res = await actions.dbMutateApply({
        target: buildTarget(),
        sql: String(mutateSQL.peek() || ""),
        approval_token: plan.approval_token,
      });
      store.dbMutatePlan.set(null);
      statusMsg.set(
        `Applied ${res.statements_executed} SQL statement(s); affected rows: ${res.affected_rows}.`
      );
    } catch (err) {
      statusMsg.set(err && err.message ? err.message : String(err));
    }
  }

  return {
    type: "Card",
    tag: "section",
    id: "desk-db-studio-panel",
    attrs: { id: "desk-db-studio-panel" },
    className: "desk-card",
    children: [
      {
        type: "Heading",
        props: { level: 3, text: "SSH DB Tunnels & Visual DB Explorer" },
      },
      {
        type: "Text",
        tag: "p",
        id: "desk-db-status",
        attrs: { id: "desk-db-status" },
        className: "desk-status-banner",
        props: { text: statusMsg },
      },
      {
        type: "Section",
        tag: "section",
        className: "desk-studio-controls",
        props: { label: "Database target controls" },
        children: [
          {
            type: "Select",
            id: "db-engine-select",
            attrs: { id: "db-engine-select" },
            props: {
              name: "engine",
              label: "DB Engine",
              value: engine,
              options: [
                { value: "sqlite", label: "Workspace SQLite (GRAB-E401)" },
                { value: "postgres", label: "PostgreSQL (SSH Tunnel)" },
                { value: "mariadb", label: "MariaDB (SSH Tunnel)" },
              ],
            },
          },
          {
            type: "Input",
            id: "db-sqlite-path",
            attrs: { id: "db-sqlite-path" },
            props: {
              name: "sqlite_path",
              label: "SQLite Workspace Path",
              value: sqlitePath,
            },
          },
          {
            type: "Input",
            id: "db-tunnel-id",
            attrs: { id: "db-tunnel-id" },
            props: {
              name: "tunnel_id",
              label: "SSH Tunnel ID",
              value: tunnelId,
            },
          },
          {
            type: "Input",
            id: "db-table-name",
            attrs: { id: "db-table-name" },
            props: { name: "table", label: "Table Name", value: tableName },
          },
          {
            type: "Input",
            id: "db-page-num",
            attrs: { id: "db-page-num" },
            props: { name: "page", label: "Page", value: pageNum },
          },
          {
            type: "Input",
            id: "db-limit-num",
            attrs: { id: "db-limit-num" },
            props: { name: "limit", label: "Limit (<= 100)", value: limitNum },
          },
          {
            type: "Input",
            id: "db-ro-sql",
            attrs: { id: "db-ro-sql" },
            props: {
              name: "sql",
              label: "Read-Only SQL (SELECT / EXPLAIN)",
              value: readOnlySQL,
            },
          },
          {
            type: "Input",
            id: "db-mutate-sql",
            attrs: { id: "db-mutate-sql" },
            props: {
              name: "mutate_sql",
              label: "Guarded Write SQL (Requires Snapshot + Token)",
              value: mutateSQL,
            },
          },
        ],
      },
      {
        type: "Stack",
        tag: "section",
        props: { direction: "row", gap: "sm" },
        attrs: { "aria-label": "Database explorer actions" },
        children: [
          {
            type: "Button",
            id: "db-schema-btn",
            attrs: { id: "db-schema-btn" },
            props: {
              type: "button",
              text: "Inspect Schema",
              onClick: onInspectSchema,
            },
          },
          {
            type: "Button",
            id: "db-rows-btn",
            attrs: { id: "db-rows-btn" },
            props: {
              type: "button",
              text: "Browse Bounded Rows",
              onClick: onBrowseRows,
            },
          },
          {
            type: "Button",
            id: "db-query-btn",
            attrs: { id: "db-query-btn" },
            props: {
              type: "button",
              text: "Run Read-Only Query",
              onClick: onRunQuery,
            },
          },
          {
            type: "Button",
            id: "db-mutate-plan-btn",
            attrs: { id: "db-mutate-plan-btn" },
            props: {
              type: "button",
              text: "Plan Guarded Write",
              onClick: onPlanMutate,
            },
          },
          {
            type: "Button",
            id: "db-mutate-apply-btn",
            attrs: { id: "db-mutate-apply-btn" },
            props: {
              type: "button",
              text: "Approve & Execute Write",
              onClick: onApplyMutate,
              disabled: () => !store.dbMutatePlan.get(),
            },
          },
        ],
      },
      ForEach(
        {
          items: store.dbTunnels,
          key: (t) => t.tunnel_id,
          parent: {
            type: "Section",
            tag: "section",
            className: "desk-list",
            props: { label: "Active Loopback DB Tunnels" },
          },
          empty: () => ({
            type: "Text",
            tag: "p",
            className: "desk-muted",
            props: { text: "No active 127.0.0.1 SSH database tunnels." },
          }),
          render: (tunSig) => ({
            type: "Card",
            tag: "article",
            className: "desk-diff-row",
            children: [
              {
                type: "Text",
                tag: "strong",
                props: {
                  text: () => {
                    const t = tunSig.get();
                    return `${t.tunnel_id} [${t.engine}] -> ${t.local_bind_addr} (TTL ${t.ttl_seconds}s)`;
                  },
                },
              },
            ],
          }),
        },
        { document: doc }
      ),
      ForEach(
        {
          items: () =>
            (store.dbRowsList.get() || []).map((row, idx) => ({
              _id: `row-${idx}`,
              row,
            })),
          key: (item) => item._id,
          parent: {
            type: "Section",
            tag: "section",
            className: "desk-list",
            props: { label: "Database Query Rows" },
          },
          empty: () => ({
            type: "Text",
            tag: "p",
            className: "desk-muted",
            props: { text: "Browse rows or run a read-only SELECT query to inspect data." },
          }),
          render: (rowSig) => ({
            type: "Card",
            tag: "article",
            className: "desk-diff-row",
            children: [
              {
                type: "Text",
                tag: "code",
                props: {
                  text: () => JSON.stringify(rowSig.get().row),
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
