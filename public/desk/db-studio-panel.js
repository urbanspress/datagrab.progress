import { ForEach, signal } from "../grex/grex.js";

const keywords = {
  sqlite: ["SELECT", "FROM", "WHERE", "JOIN", "GROUP BY", "ORDER BY", "LIMIT", "PRAGMA", "EXPLAIN QUERY PLAN"],
  postgres: ["SELECT", "FROM", "WHERE", "JOIN", "GROUP BY", "ORDER BY", "LIMIT", "ILIKE", "DISTINCT ON", "EXPLAIN"],
  mariadb: ["SELECT", "FROM", "WHERE", "JOIN", "GROUP BY", "ORDER BY", "LIMIT", "SHOW", "DESCRIBE", "EXPLAIN"],
};

const engines = [
  { value: "sqlite", label: "Workspace SQLite" },
  { value: "sqlite-bridge", label: "Remote SQLite via Gashboard" },
  { value: "postgres", label: "PostgreSQL over SSH" },
  { value: "mariadb", label: "MariaDB or MySQL over SSH" },
];

function makeTab(id) {
  return {
    id, engine: signal("sqlite"), path: signal("data/app.db"),
    tunnel: signal(""), user: signal(""), password: signal(""), database: signal(""), schema: signal("public"), bridgeBearer: signal(""),
    sql: signal("SELECT 1 AS ok"), write: signal(""), table: signal("books"), page: signal("1"), limit: signal("25"),
    catalog: signal(null), result: signal(null), message: signal(""), plan: signal(null), applied: signal(null),
    restoreConflict: signal(false),
    maintenanceUser: signal(""), maintenancePassword: signal(""),
    exportPath: signal(""), diagramPath: signal(""), importPath: signal(""), cell: signal(null), replacement: signal(""), format: signal("text"), compare: signal(null),
    busy: signal(false), query: null,
  };
}

function targetFor(tab) {
  const engine = tab.engine.peek();
  if (engine === "sqlite-bridge") {
    return { engine, sqlite_path: tab.path.peek().trim(), tunnel_id: tab.tunnel.peek().trim(), bridge_bearer: tab.bridgeBearer.peek() };
  }
  return engine === "sqlite"
    ? { engine, sqlite_path: tab.path.peek().trim() }
    : { engine, tunnel_id: tab.tunnel.peek().trim(), db_user: tab.user.peek().trim(),
      db_password: tab.password.peek(), db_name: tab.database.peek().trim(), schema_name: tab.schema.peek().trim() };
}

function labelTarget(target) {
  if (target.engine === "sqlite-bridge") return `${target.sqlite_path} via ${target.tunnel_id}`;
  return target.engine === "sqlite" ? target.sqlite_path : `${target.db_name || "Database"} via ${target.tunnel_id}`;
}

function sameTarget(a, b) {
  return JSON.stringify(a) === JSON.stringify(b);
}

const paragraph = (text, attrs) => ({ type: "Text", tag: "p", attrs, props: { text } });
const button = (id, text, onClick, disabled) => ({
  type: "Button", attrs: id ? { id } : {}, props: { type: "button", text, onClick, disabled },
});
const input = (name, label, value, type = "text") => ({
  type: "Input", attrs: { id: name }, props: { name, label, value, type },
});
const select = (name, label, value, options) => ({
  type: "Select", props: { name, label, value, options },
});
const controls = (label, children) => ({
  type: "Section", tag: "section", className: "desk-studio-controls", props: { label }, children,
});

export function createDBStudioPanelNode(store, actions, doc = globalThis.document) {
  let nextTab = 1;
  let nextRender = 0;
  const tabs = signal([makeTab(nextTab)]);
  const active = signal(nextTab);
  const sourceTab = signal("");
  const sshHost = signal("");
  const sshPort = signal("22");
  const sshUser = signal("");
  const knownHosts = signal("");
  const pin = signal("");
  const privateKey = signal("");
  const sshPassword = signal("");
  const targetPort = signal("");
  const onlyActive = () => tabs.get().filter((tab) => tab.id === active.get());
  const repeat = (items, render, label) => ForEach({
    items, key: (item) => item.id, render,
    parent: { type: "Section", tag: "section", props: { label } },
  }, { document: doc });

  async function run(tab, action) {
    if (tab.busy.peek()) return;
    tab.busy.set(true);
    tab.message.set("");
    try {
      await action();
    } catch (error) {
      tab.message.set(error.name === "AbortError" ? "Query canceled." : error.message);
    } finally {
      tab.busy.set(false);
    }
  }

  function renderTab(tab) {
    const disabled = tab.busy;
    const catalogForTarget = () => {
      const catalog = tab.catalog.get();
      for (const field of [tab.engine, tab.path, tab.tunnel, tab.user, tab.password, tab.database, tab.schema, tab.bridgeBearer]) field.get();
      return sameTarget(catalog?.target, targetFor(tab)) ? catalog?.tables : [];
    };
    const fullPostgres = () => catalogForTarget() === tab.catalog.get()?.tables && tab.catalog.get()?.fullPostgres === true;
    const fullMariaDB = () => catalogForTarget() === tab.catalog.get()?.tables && tab.catalog.get()?.fullMariaDB === true;
    const fullWriteTarget = () => {
      const target = targetFor(tab);
      if (target.engine === "postgres" && fullPostgres()) {
        target.maintenance_user = tab.maintenanceUser.peek().trim();
        target.maintenance_password = tab.maintenancePassword.peek();
      }
      return target;
    };
    const canReviewWrite = () => tab.engine.get() === "sqlite" || (fullPostgres() && tab.maintenanceUser.get().trim() !== "") || fullMariaDB();

    function setQueryResult(response, target, table) {
      const rows = response.rows || [];
      const columns = response.columns || Object.keys(rows[0] || {}).map((name) => ({ name }));
      tab.result.set({ id: ++nextRender, target, table, columns, rows });
      tab.cell.set(null);
    }

    const browse = () => run(tab, async () => {
      const target = targetFor(tab);
      const table = tab.table.peek().trim();
      const res = await actions.dbRows({ target, table, page: tab.page.peek().trim(), limit: tab.limit.peek().trim() });
      setQueryResult(res, target, table);
      tab.message.set(`Page ${res.page} of ${res.total_pages}, ${res.total_rows} rows in ${res.table}.`);
    });

    async function planWrite(operation, target = targetFor(tab)) {
      tab.plan.set(null);
      const request = { target, ...operation };
      const plan = await actions.dbMutatePlan(request);
      tab.plan.set({ type: "write", plan, request });
      tab.message.set(plan.plain_summary || "Verified the recovery snapshot.");
    }

    async function reviewRestore(force = false) {
      const applied = tab.applied.peek();
      if (!applied) return;
      tab.plan.set(null);
      const request = force ? { ...applied, force: true } : applied;
      try {
        const plan = await actions.dbRestorePlan(request);
        if (force) request.baseline_sha256 = plan.baseline_sha256;
        tab.plan.set({ type: "restore", plan, request });
        tab.message.set(plan.plain_summary);
      } catch (error) {
        tab.restoreConflict.set(error.status === 409);
        throw error;
      }
    }

    const approve = () => run(tab, async () => {
      const pending = tab.plan.peek();
      if (!pending) return;
      tab.plan.set(null);
      const request = { ...pending.request, approval_token: pending.plan.approval_token };
      if (pending.type === "restore") {
        let result;
        try {
          result = await actions.dbRestore(request);
        } catch (error) {
          tab.restoreConflict.set(error.status === 409);
          throw error;
        }
        tab.applied.set(null);
        tab.restoreConflict.set(false);
        tab.message.set(`Restored the saved data and schema. SHA-256 ${result.restored_sha256}.`);
      } else {
        const result = await (pending.type === "import" ? actions.dbImportApply(request) : actions.dbMutateApply(request));
        tab.applied.set({ target: request.target, snapshot_id: result.snapshot_id });
        tab.restoreConflict.set(false);
        tab.message.set(`Saved ${result.affected_rows} affected rows. A verified snapshot is available for rollback.`);
      }
      tab.result.set(null);
      tab.catalog.set(null);
      tab.cell.set(null);
    });

    function chooseCell(result, row, name, dataType) {
      const table = catalogForTarget()?.find((entry) => entry.name === result.table);
      const keyColumns = (table?.columns || []).filter((entry) => entry.primary_key);
      const key = Object.fromEntries(keyColumns.map((entry) => [entry.name, row[entry.name]]));
      if (!keyColumns.length || Object.values(key).some((value) => typeof value !== "string")) return;
      const before = row[name];
      tab.replacement.set(before ?? "");
      tab.format.set(/DECIMAL|NUMERIC/i.test(dataType) ? "decimal" : /INT/i.test(dataType) ? "integer" : "text");
      tab.cell.set({ result, key, name, before });
    }

    function renderResults(result) {
      const local = result.target.engine === "sqlite";
      const table = result.target.engine !== "sqlite-bridge" && sameTarget(result.target, targetFor(tab))
        ? catalogForTarget()?.find((entry) => entry.name === result.table) : null;
      const columns = new Map((table?.columns || []).map((col) => [col.name, col]));
      return { tag: "section", attrs: { class: "desk-db-grid" }, children: [
        { tag: "table", children: [
          { tag: "caption", text: `${result.table || "Query"} results on ${labelTarget(result.target)}` },
          { tag: "thead", children: [{ tag: "tr", children: result.columns.map((col) =>
            ({ tag: "th", attrs: { scope: "col" }, text: col.name })) }] },
          { tag: "tbody", children: result.rows.map((row, index) => ({
            tag: "tr", children: result.columns.map(({ name }) => {
              const col = columns.get(name);
              const value = row[name];
              const text = value === null ? "NULL" : value === "" ? "(empty string)" : value;
              const primary = (table?.columns || []).filter((entry) => entry.primary_key);
              const editable = col && !col.primary_key && (local ? /INT|TEXT|CHAR|CLOB/i : /INT|TEXT|CHAR|DECIMAL|NUMERIC/i).test(col.type)
                && (local ? primary.length > 0 : primary.length === 1) && primary.every((entry) => typeof row[entry.name] === "string");
              return editable ? { tag: "td", children: [
                { tag: "button", attrs: { type: "button", class: "grex-button", "aria-label": `Edit ${name}, row ${index + 1}: ${text}` },
                  text, on: { click: () => chooseCell(result, row, name, col.type) } },
              ] } : { tag: "td", text };
            }),
          })) },
        ] },
      ] };
    }

    const targetControls = controls("Database target", [
      select("engine", "Engine", tab.engine, engines), input("db-sqlite-path", "SQLite path relative to workspace or bridge root", tab.path),
      input("db-bridge-bearer", "Gashboard bridge bearer", tab.bridgeBearer, "password"),
      input("db-tunnel-id", "SSH tunnel ID", tab.tunnel), input("db-database", "Database name", tab.database),
      input("db-schema", "PostgreSQL schema", tab.schema), input("db-user", "Database user", tab.user),
      input("db-password", "Database password", tab.password, "password"),
    ]);

    const completion = () => {
      const query = tab.sql.get();
      const prefix = /[a-z_][a-z0-9_]*$/i.exec(query)?.[0] || "";
      if (!prefix) return [];
      const schema = catalogForTarget() || [];
      const names = schema.flatMap((table) => [table.name, ...table.columns.map((col) => col.name)]);
      return [...new Set([...(keywords[tab.engine.get()] || []), ...names])]
        .filter((word) => word.toUpperCase().startsWith(prefix.toUpperCase()) && word !== prefix)
        .slice(0, 8).map((word) => ({ id: word }));
    };

    return { type: "Section", tag: "section", props: { label: `Database tab ${tab.id}` },
      on: { keydown: (event) => {
        if (event.key !== "Escape") return;
        if (tab.query) tab.query.abort();
        else if (tab.plan.peek()) tab.plan.set(null);
        else tab.cell.set(null);
      } },
      children: [
      targetControls,
      {
        type: "Accordion", props: { items: [{
          id: `ssh-options-${tab.id}`, summary: "Connect to a database over pinned SSH",
          content: [controls("Pinned SSH connection", [
            input("db-ssh-host", "SSH host", sshHost), input("db-ssh-port", "SSH port", sshPort),
            input("db-ssh-user", "SSH user", sshUser), input("db-ssh-known-hosts", "known_hosts file path", knownHosts),
            input("db-ssh-pin", "SSH host fingerprint (SHA256:…)", pin),
            input("db-ssh-key", "SSH private key file path (optional)", privateKey),
            input("db-ssh-password", "SSH password or key passphrase", sshPassword, "password"),
            input("db-ssh-target-port", "Remote loopback database or Gashboard bridge port", targetPort),
            button(null, "Open pinned SSH tunnel", () => run(tab, async () => {
              const engine = tab.engine.peek();
              const credentials = sshPassword.peek();
              sshPassword.set("");
              const { tunnel } = await actions.dbTunnelOpen({ engine, ssh_host: sshHost.peek().trim(), ssh_port: sshPort.peek().trim(),
                ssh_user: sshUser.peek().trim(), known_hosts_path: knownHosts.peek().trim(), pinned_fingerprint: pin.peek().trim(),
                ssh_private_key_path: privateKey.peek().trim(), ssh_password: credentials,
                remote_db_host: "127.0.0.1", remote_db_port: targetPort.peek().trim() });
              tab.tunnel.set(tunnel.tunnel_id);
              tab.message.set(`Opened a pinned SSH tunnel to ${tunnel.remote_target}.`);
            }), () => tab.busy.get() || tab.engine.get() === "sqlite"),
            button(null, "Close this SSH tunnel", () => run(tab, async () => {
              await actions.dbTunnelClose({ tunnel_id: tab.tunnel.peek() });
              tab.tunnel.set("");
              tab.message.set("SSH tunnel closed.");
            }), () => tab.busy.get() || !tab.tunnel.get()),
          ])],
        }] },
      },
      controls("SQL editor and table browsing", [
        input("db-ro-sql", "Read-only SQL", tab.sql, "textarea"),
        input("db-table-name", "Table name", tab.table),
        input("db-page-num", "Page", tab.page),
        input("db-limit-num", "Page limit (maximum 100)", tab.limit),
      ]),
      repeat(completion, (entry) => button(null, () => entry.get().id, () => {
        tab.sql.set(tab.sql.peek().replace(/[a-z_][a-z0-9_]*$/i, () => entry.peek().id) + " ");
        doc.querySelector("#db-ro-sql textarea")?.focus();
      }), "SQL completions"),
      controls("Database actions", [
        button("db-schema-btn", "Inspect schema", () => run(tab, async () => {
          const target = targetFor(tab);
          const res = await actions.dbSchema(target);
          tab.catalog.set({ id: ++nextRender, target, tables: res.tables || [], fullPostgres: res.postgres_replay_candidate, fullMariaDB: res.mariadb_replay_candidate });
          tab.message.set(`Loaded ${res.tables?.length || 0} tables from ${labelTarget(target)}.`);
        }), disabled),
        button("db-rows-btn", "Browse table rows", browse, disabled),
        button("db-query-btn", "Run read-only query", () => run(tab, async () => {
          const target = targetFor(tab);
          const controller = new AbortController();
          tab.query = controller;
          try {
            const res = await actions.dbQuery({ target, sql: tab.sql.peek() }, controller.signal);
            setQueryResult(res, target, null);
            tab.message.set(`Read-only query returned ${res.row_count} rows.`);
          } finally {
            tab.query = null;
          }
        }), () => tab.busy.get() || tab.engine.get() === "sqlite-bridge"),
        button(null, "Cancel query", () => tab.query?.abort(), () => !tab.busy.get()),
      ]),
      repeat(() => {
        const catalog = tab.catalog.get();
        return catalog ? [{ id: catalog.id, tables: catalog.tables }] : [];
      }, (entry) => {
        const { id, tables } = entry.peek();
        const entities = new Map(tables.map((table, index) => [`${table.schema}.${table.name}`, `db-entity-${tab.id}-${id}-${index}`]));
        return { tag: "section", attrs: { "aria-label": "Entity-relationship diagram", class: "desk-db-diagram" },
          children: tables.map((table) => {
            const name = `${table.schema}.${table.name}`;
            const elementId = entities.get(name);
            return { tag: "article", attrs: { id: elementId, class: "desk-db-entity", "aria-labelledby": `${elementId}-title` }, children: [
              { tag: "h5", attrs: { id: `${elementId}-title`, tabindex: "-1" }, text: name },
              { tag: "ul", attrs: { "aria-label": "Columns" }, children: (table.columns || []).map((col) => ({
                tag: "li", text: [col.name, col.type, col.primary_key ? "primary key" : "", col.nullable ? "nullable" : ""].filter(Boolean).join(" · "),
              })) },
              { tag: "ul", attrs: { "aria-label": "Foreign keys", class: "desk-db-relationships" }, children: (table.foreign_keys || []).map((key) => {
                const linked = `${key.ref_schema || table.schema}.${key.ref_table}`;
                const target = entities.get(linked);
                const label = `${key.column} → ${linked}.${key.ref_column}`;
                return { tag: "li", children: [target ? button(null, label,
                  () => doc.getElementById(`${target}-title`)?.focus()) : { tag: "span", text: label }] };
              }) },
              { tag: "p", text: `Indexes: ${(table.indexes || []).join(", ") || "none"}` },
              button(null, `Select table ${table.name}`, () => tab.table.set(table.name)),
            ] };
          }) };
      }, "Database tables and relationships"),
      repeat(() => {
        const result = tab.result.get();
        if (!result) return [];
        const revision = tab.catalog.get()?.id || 0;
        return [{ ...result, id: `${result.id}-${revision}` }];
      }, (entry) => renderResults(entry.peek()), "Result set"),
      controls("Export a table page", [
        input("db-export-path", "Workspace output path (.csv, .xlsx, .json, or .sql)", tab.exportPath),
        button(null, "Export table page", () => run(tab, async () => {
          const target = targetFor(tab);
          const saved = await actions.dbExportPage({ target, path: tab.exportPath.peek().trim(), table: tab.table.peek().trim(),
            page: tab.page.peek().trim(), limit: tab.limit.peek().trim() });
          tab.message.set(`Exported ${saved.rows_exported} rows from ${saved.table} on ${labelTarget(target)} to ${saved.path}.`);
        }), disabled),
      ]),
      controls("Export the relationship diagram", [
        input("db-diagram-path", "Workspace diagram path (.html)", tab.diagramPath),
        button(null, "Export relationship diagram", () => run(tab, async () => {
          const target = targetFor(tab);
          const saved = await actions.dbExportDiagram({ target, path: tab.diagramPath.peek().trim() });
          tab.message.set(`Exported the diagram for ${labelTarget(target)} to ${saved.path}.`);
        }), disabled),
      ]),
      repeat(() => fullPostgres() ? [tab] : [], () => controls("PostgreSQL database maintenance", [
        input("db-maintenance-user", "Admission administrator on the postgres database", tab.maintenanceUser),
        input("db-maintenance-password", "Maintenance password", tab.maintenancePassword, "password"),
      ]), "PostgreSQL recovery credentials"),
      controls("Import rows or SQL", [
        input("db-import-path", "Workspace import path (.csv, .xlsx, .json, or .sql)", tab.importPath),
        button(null, "Review import", () => run(tab, async () => {
          tab.plan.set(null);
          const request = { target: fullWriteTarget(), path: tab.importPath.peek().trim(), table: tab.table.peek().trim() };
          const plan = await actions.dbImportPlan(request);
          tab.plan.set({ type: "import", plan, request });
          tab.message.set(plan.plain_summary);
        }), () => tab.busy.get() || !canReviewWrite()),
      ]),
      controls("Inline cell edit", [
        paragraph(() => {
          const cell = tab.cell.get();
          if (!cell) return "Inspect the schema and browse rows to select an editable cell.";
          const before = cell.before === null ? "NULL" : JSON.stringify(cell.before);
          return `${cell.result.table}.${cell.name}, key ${JSON.stringify(cell.key)}, current value ${before}.`;
        }),
        input("db-cell-value", "Replacement value", tab.replacement),
        select("db-cell-type", "Value type", tab.format, ["text", "integer", "decimal", "null"].map((value) => ({ value, label: value }))),
        button(null, "Review cell change", () => run(tab, async () => {
          const cell = tab.cell.peek();
          if (!cell) return;
          const kind = tab.format.peek();
          await planWrite({ edit: { table: cell.result.table, column: cell.name, key: cell.key, before: cell.before,
            value: kind === "null" ? null : tab.replacement.peek(), value_kind: kind === "null" ? "" : kind } }, cell.result.target);
        }), () => tab.busy.get() || !tab.cell.get()),
        button(null, "Cancel cell edit", () => tab.cell.set(null), () => !tab.cell.get()),
      ]),
      controls("Guarded database changes", [
        input("db-mutate-sql", "Write SQL", tab.write, "textarea"),
        button("db-mutate-plan-btn", "Review SQL write", () => run(tab, () => planWrite({ sql: tab.write.peek() }, fullWriteTarget())),
          () => tab.busy.get() || !canReviewWrite()),
        button(null, "Undo last database change", () => run(tab, () => reviewRestore()),
          () => tab.busy.get() || !tab.applied.get()),
        button(null, "Review forced restore", () => run(tab, () => reviewRestore(true)), () => {
          const engine = tab.applied.get()?.target.engine;
          return tab.busy.get() || !tab.restoreConflict.get() || !["postgres", "mariadb"].includes(engine);
        }),
        paragraph(() => {
          const pending = tab.plan.get();
          if (!pending) return "";
          const { request } = pending;
          const detail = request.sql || pending.plan.reviewed_sql || (request.path
            ? `${request.path} · ${pending.plan.plain_summary}`
            : request.edit ? JSON.stringify(request.edit) : pending.plan.plain_summary);
          return `${labelTarget(request.target)} — ${detail}`;
        }),
        button("db-mutate-apply-btn", () => {
          const pending = tab.plan.get();
          if (pending?.type !== "restore") return "Approve database change";
          return pending.request.force ? "Approve overwriting the reviewed changes" : "Approve restore";
        },
          approve, () => tab.busy.get() || !tab.plan.get()),
        button(null, "Discard approval", () => tab.plan.set(null), () => !tab.plan.get()),
      ]),
      controls("Compare database schemas", [
        input("db-compare-source", "Source tab number", sourceTab),
        button(null, "Compare source with this tab", () => run(tab, async () => {
          const id = sourceTab.peek().trim();
          const other = tabs.peek().find((candidate) => String(candidate.id) === id && candidate !== tab);
          if (!other) throw new Error("Enter the number of another open SQL tab.");
          const destination = targetFor(tab);
          const comparison = await actions.dbCompare({ source: targetFor(other), destination });
          tab.compare.set({ id: ++nextRender, comparison, destination });
          tab.message.set(`Found ${comparison.changes.length} schema differences.`);
        }), disabled),
      ]),
      repeat(() => tab.compare.get() ? [tab.compare.get()] : [], (entry) => {
        const { comparison, destination } = entry.peek();
        return { tag: "table", children: [
          { tag: "caption", text: `Schema comparison from ${comparison.source.engine} to ${comparison.destination.engine}` },
          { tag: "thead", children: [{ tag: "tr", children: ["Object", "Source", "Destination", "Migration review"].map((text) =>
            ({ tag: "th", attrs: { scope: "col" }, text })) }] },
          { tag: "tbody", children: comparison.changes.map((change) => ({ tag: "tr", children: [
            { tag: "th", attrs: { scope: "row" }, text: `${change.table}${change.object ? "." + change.object : ""}` },
            { tag: "td", text: change.source === undefined ? "—" : JSON.stringify(change.source) },
            { tag: "td", text: change.destination === undefined ? "—" : JSON.stringify(change.destination) },
            { tag: "td", children: [
              paragraph([change.proposed_sql, change.review].filter(Boolean).join(" · ") || "Manual review"),
              ...(change.proposed_sql && (comparison.destination.postgres_replay_candidate || comparison.destination.mariadb_replay_candidate) ? [
                button(null, "Review migration", () => run(tab, () => {
                  if (!sameTarget(destination, targetFor(tab))) throw new Error("Compare the current target before reviewing a migration.");
                  return planWrite({ sql: change.proposed_sql }, fullWriteTarget());
                }), () => tab.busy.get() || !canReviewWrite()),
              ] : []),
            ] },
          ] })) },
        ] };
      }, "Schema comparison results"),
      paragraph(tab.message, { role: "status", "aria-live": "polite" }),
    ] };
  }

  return { type: "Card", tag: "section", attrs: { id: "desk-db-studio-panel" }, className: "desk-card", children: [
    { type: "Heading", props: { level: 3, text: "Visual Database Workbench" } },
    controls("SQL tabs", [
      button(null, "New SQL tab", () => {
        const tab = makeTab(++nextTab);
        tabs.set([...tabs.peek(), tab]);
        active.set(tab.id);
      }),
      button(null, "Close current SQL tab", async () => {
        const current = tabs.peek().find((tab) => tab.id === active.peek());
        const remaining = tabs.peek().filter((tab) => tab !== current);
        if (!remaining.length) return;
        current.query?.abort();
        tabs.set(remaining);
        const next = remaining.at(-1);
        active.set(next.id);
        const tunnel = current.tunnel.peek().trim();
        if (tunnel && !remaining.some((tab) => tab.tunnel.peek().trim() === tunnel)) {
          try {
            await actions.dbTunnelClose({ tunnel_id: tunnel });
          } catch (error) {
            next.message.set(error.message);
          }
        }
      }, () => tabs.get().length < 2),
      repeat(tabs, (entry) => button(null,
        () => `Tab ${entry.get().id}${active.get() === entry.get().id ? " (active)" : ""}`,
        () => active.set(entry.peek().id)), "Open SQL tabs"),
    ]),
    repeat(onlyActive, (entry) => renderTab(entry.peek()), "Active SQL workspace"),
    repeat(() => store.dbTunnels.get().map((tunnel) => ({ ...tunnel, id: tunnel.tunnel_id })),
      (entry) => paragraph(() => {
        const tunnel = entry.get();
        return `${tunnel.tunnel_id} · ${tunnel.engine} · ${tunnel.local_bind_addr}`;
      }), "Active SSH database tunnels"),
  ] };
}
