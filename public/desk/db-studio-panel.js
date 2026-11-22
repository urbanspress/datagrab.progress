import { Accordion, ForEach, Icon, bindElement, signal } from "../grex/grex.js";

const CONN_STORAGE_KEY = "groxy_desk_db_connections";

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

const PLUS_ICON_PATHS = [
  { tag: "line", attrs: { x1: "12", y1: "5", x2: "12", y2: "19", stroke: "currentColor", "stroke-width": "2", "stroke-linecap": "round" } },
  { tag: "line", attrs: { x1: "5", y1: "12", x2: "19", y2: "12", stroke: "currentColor", "stroke-width": "2", "stroke-linecap": "round" } },
];

function sanitizeConnectionForCache(conn) {
  if (!conn || typeof conn !== "object") return null;
  const id = String(conn.id || "").trim();
  const engine = String(conn.engine || "sqlite").trim();
  if (!id) return null;
  return {
    id,
    name: String(conn.name || id).trim(),
    engine,
    sqlite_path: String(conn.sqlite_path || "").trim(),
    db_host: String(conn.db_host || "127.0.0.1").trim(),
    db_port: String(conn.db_port || "").trim(),
    db_user: String(conn.db_user || "").trim(),
    db_name: String(conn.db_name || "").trim(),
    schema_name: String(conn.schema_name || "").trim(),
    has_db_password: Boolean(conn.has_db_password),
    has_bridge_bearer: Boolean(conn.has_bridge_bearer),
    ssh_enabled: Boolean(conn.ssh_enabled),
    ssh_host: String(conn.ssh_host || "").trim(),
    ssh_port: String(conn.ssh_port || "22").trim(),
    ssh_user: String(conn.ssh_user || "").trim(),
    ssh_private_key_path: String(conn.ssh_private_key_path || "").trim(),
    known_hosts_path: String(conn.known_hosts_path || "").trim(),
    pinned_fingerprint: String(conn.pinned_fingerprint || "").trim(),
    remote_db_port: String(conn.remote_db_port || "").trim(),
    has_ssh_password: Boolean(conn.has_ssh_password),
  };
}

function readCachedConnections() {
  try {
    const raw = globalThis.localStorage?.getItem(CONN_STORAGE_KEY);
    if (!raw) return [];
    const parsed = JSON.parse(raw);
    if (!Array.isArray(parsed)) return [];
    return parsed.map(sanitizeConnectionForCache).filter(Boolean);
  } catch {
    return [];
  }
}

function writeCachedConnections(list) {
  try {
    const clean = (Array.isArray(list) ? list : []).map(sanitizeConnectionForCache).filter(Boolean);
    globalThis.localStorage?.setItem(CONN_STORAGE_KEY, JSON.stringify(clean));
  } catch {
    // Ignore storage errors in restricted contexts.
  }
}

function makeTab(id) {
  return {
    id,
    connectionId: signal(""),
    engine: signal("sqlite"),
    path: signal("data/app.db"),
    tunnel: signal(""),
    user: signal(""),
    password: signal(""),
    database: signal(""),
    schema: signal("public"),
    bridgeBearer: signal(""),
    sql: signal("SELECT 1 AS ok"),
    write: signal(""),
    table: signal("books"),
    page: signal("1"),
    limit: signal("25"),
    centerView: signal("data"),
    columnFilters: signal({}),
    catalog: signal(null),
    result: signal(null),
    message: signal(""),
    plan: signal(null),
    applied: signal(null),
    restoreConflict: signal(false),
    maintenanceUser: signal(""),
    maintenancePassword: signal(""),
    exportPath: signal(""),
    diagramPath: signal(""),
    importPath: signal(""),
    cell: signal(null),
    replacement: signal(""),
    format: signal("text"),
    compare: signal(null),
    busy: signal(false),
    query: null,
  };
}

function targetFor(tab) {
  const engine = tab.engine.peek();
  const connId = tab.connectionId.peek().trim();
  const base = connId ? { connection_id: connId } : {};
  if (engine === "sqlite-bridge") {
    return {
      ...base,
      engine,
      sqlite_path: tab.path.peek().trim(),
      tunnel_id: tab.tunnel.peek().trim(),
      bridge_bearer: tab.bridgeBearer.peek(),
    };
  }
  return engine === "sqlite"
    ? { ...base, engine, sqlite_path: tab.path.peek().trim() }
    : {
        ...base,
        engine,
        tunnel_id: tab.tunnel.peek().trim(),
        db_user: tab.user.peek().trim(),
        db_password: tab.password.peek(),
        db_name: tab.database.peek().trim(),
        schema_name: tab.schema.peek().trim(),
      };
}

function labelTarget(target) {
  if (target.engine === "sqlite-bridge") return `${target.sqlite_path} via ${target.tunnel_id}`;
  return target.engine === "sqlite" ? target.sqlite_path : `${target.db_name || "Database"} via ${target.tunnel_id}`;
}

function sameTarget(a, b) {
  return JSON.stringify(a) === JSON.stringify(b);
}

function engineBadgeText(engine) {
  if (engine === "postgres") return "PG";
  if (engine === "mariadb") return "MY";
  if (engine === "sqlite-bridge") return "BR";
  return "SQ";
}

const paragraph = (text, attrs) => ({ type: "Text", tag: "p", attrs, props: { text } });
const button = (id, text, onClick, disabled, className, attrs = {}) => ({
  type: "Button",
  attrs: id ? { id, ...attrs } : attrs,
  className,
  props: { type: "button", text, onClick, disabled },
});
const input = (name, label, value, type = "text", placeholder = "") => ({
  type: "Input",
  attrs: { id: name },
  props: { name, label, value, type, ...(placeholder ? { placeholder } : {}) },
});
const select = (name, label, value, options) => ({
  type: "Select",
  props: { name, label, value, options },
});
const controls = (label, children) => ({
  type: "Section",
  tag: "section",
  className: "desk-studio-controls",
  props: { label },
  children,
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
  const objectFilter = signal("");

  const connDialogOpen = signal(false);
  const connFormId = signal("");
  const connFormName = signal("");
  const connFormHost = signal("127.0.0.1");
  const connFormPort = signal("");
  const activeConnId = signal("");
  const inspectorOpen = signal(false);
  const localConnections = signal(readCachedConnections());

  const mergedConnections = () => {
    const remote = store.dbConnections && typeof store.dbConnections.get === "function"
      ? store.dbConnections.get()
      : [];
    const cached = localConnections.get();
    const byId = new Map();
    for (const c of cached) {
      const clean = sanitizeConnectionForCache(c);
      if (clean) byId.set(clean.id, clean);
    }
    for (const c of remote) {
      const clean = sanitizeConnectionForCache(c);
      if (clean) byId.set(clean.id, clean);
    }
    return Array.from(byId.values());
  };

  const syncLocalCache = (list) => {
    const clean = list.map(sanitizeConnectionForCache).filter(Boolean);
    localConnections.set(clean);
    writeCachedConnections(clean);
    if (store.dbConnections && typeof store.dbConnections.set === "function") {
      store.dbConnections.set(clean);
    }
  };

  const onlyActive = () => tabs.get().filter((tab) => tab.id === active.get());
  const repeat = (items, render, label) =>
    ForEach(
      {
        items,
        key: (item) => item.id,
        render,
        parent: { type: "Section", tag: "section", props: { label } },
      },
      { document: doc }
    );

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
    const currentCatalog = () => {
      const catalog = tab.catalog.get();
      for (const field of [tab.connectionId, tab.engine, tab.path, tab.tunnel, tab.user, tab.password, tab.database, tab.schema, tab.bridgeBearer]) {
        field.get();
      }
      return sameTarget(catalog?.target, targetFor(tab)) ? catalog : null;
    };
    const catalogForTarget = () => currentCatalog()?.tables || [];
    const viewsForTarget = () => currentCatalog()?.views || [];
    const triggersForTarget = () => currentCatalog()?.triggers || [];
    const fullPostgres = () => currentCatalog()?.fullPostgres === true;
    const fullMariaDB = () => currentCatalog()?.fullMariaDB === true;
    const fullWriteTarget = () => {
      const target = targetFor(tab);
      if (target.engine === "postgres" && fullPostgres()) {
        target.maintenance_user = tab.maintenanceUser.peek().trim();
        target.maintenance_password = tab.maintenancePassword.peek();
      }
      return target;
    };
    const canReviewWrite = () => tab.engine.get() === "sqlite" || (fullPostgres() && tab.maintenanceUser.get().trim() !== "") || fullMariaDB();

    const schemaNamespaceLabel = () => {
      const eng = tab.engine.get();
      if (eng === "postgres") return tab.schema.get().trim() || "public";
      if (eng === "mariadb") return tab.database.get().trim() || "database";
      return "main";
    };

    function setQueryResult(response, target, table) {
      const rows = response.rows || [];
      const columns = response.columns || Object.keys(rows[0] || {}).map((name) => ({ name }));
      tab.columnFilters.set({});
      tab.result.set({
        id: ++nextRender,
        target,
        table,
        page: response.page || "1",
        limit: response.limit || String(rows.length || 25),
        columns,
        rows,
      });
      tab.cell.set(null);
    }

    const inspectSchema = () =>
      run(tab, async () => {
        const target = targetFor(tab);
        const res = await actions.dbSchema(target);
        tab.catalog.set({
          id: ++nextRender,
          target,
          tables: res.tables || [],
          views: res.views || [],
          triggers: res.triggers || [],
          fullPostgres: res.postgres_replay_candidate,
          fullMariaDB: res.mariadb_replay_candidate,
        });
        tab.message.set(`Loaded ${res.tables?.length || 0} tables from ${labelTarget(target)}.`);
      });

    const browse = () =>
      run(tab, async () => {
        const target = targetFor(tab);
        const table = tab.table.peek().trim();
        const res = await actions.dbRows({ target, table, page: tab.page.peek().trim(), limit: tab.limit.peek().trim() });
        setQueryResult(res, target, table);
        tab.message.set(`Page ${res.page} of ${res.total_pages}, ${res.total_rows} rows in ${res.table}.`);
      });

    const runReadOnlyQuery = () =>
      run(tab, async () => {
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
      });

    async function planWrite(operation, target = targetFor(tab)) {
      tab.plan.set(null);
      const request = { target, ...operation };
      const plan = await actions.dbMutatePlan(request);
      tab.plan.set({ type: "write", plan, request });
      inspectorOpen.set(true);
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
        inspectorOpen.set(true);
        tab.message.set(plan.plain_summary);
      } catch (error) {
        tab.restoreConflict.set(error.status === 409);
        throw error;
      }
    }

    const approve = () =>
      run(tab, async () => {
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
      inspectorOpen.set(true);
    }

    function openNewConnectionDialog() {
      connFormId.set("");
      connFormName.set("");
      connFormHost.set("127.0.0.1");
      connFormPort.set("");
      tab.connectionId.set("");
      tab.password.set("");
      tab.bridgeBearer.set("");
      sshPassword.set("");
      connDialogOpen.set(true);
    }

    function populateFormFromConnection(conn) {
      connFormId.set(conn.id || "");
      connFormName.set(conn.name || "");
      tab.connectionId.set(conn.id || "");
      tab.engine.set(conn.engine || "sqlite");
      tab.path.set(conn.sqlite_path || "data/app.db");
      connFormHost.set(conn.db_host || "127.0.0.1");
      connFormPort.set(conn.db_port || conn.remote_db_port || "");
      tab.user.set(conn.db_user || "");
      tab.password.set("");
      tab.database.set(conn.db_name || "");
      tab.schema.set(conn.schema_name || "public");
      tab.bridgeBearer.set("");
      sshHost.set(conn.ssh_host || "");
      sshPort.set(conn.ssh_port || "22");
      sshUser.set(conn.ssh_user || "");
      knownHosts.set(conn.known_hosts_path || "");
      pin.set(conn.pinned_fingerprint || "");
      privateKey.set(conn.ssh_private_key_path || "");
      sshPassword.set("");
      targetPort.set(conn.remote_db_port || conn.db_port || "");
    }

    async function saveCurrentConnection() {
      const eng = tab.engine.peek();
      const id = connFormId.peek().trim() || `conn-${Date.now().toString(36)}`;
      const remotePort = targetPort.peek().trim() || connFormPort.peek().trim();
      const payload = {
        id,
        name:
          connFormName.peek().trim() ||
          (eng === "sqlite" ? tab.path.peek().trim() || "Workspace SQLite" : tab.database.peek().trim() || sshHost.peek().trim() || eng),
        engine: eng,
        sqlite_path: tab.path.peek().trim(),
        db_host: connFormHost.peek().trim() || "127.0.0.1",
        db_port: connFormPort.peek().trim(),
        db_user: tab.user.peek().trim(),
        db_password: tab.password.peek(),
        db_name: tab.database.peek().trim(),
        schema_name: tab.schema.peek().trim() || "public",
        bridge_bearer: tab.bridgeBearer.peek(),
        ssh_enabled: eng !== "sqlite" || Boolean(sshHost.peek().trim() || pin.peek().trim()),
        ssh_host: sshHost.peek().trim(),
        ssh_port: sshPort.peek().trim() || "22",
        ssh_user: sshUser.peek().trim(),
        ssh_private_key_path: privateKey.peek().trim(),
        known_hosts_path: knownHosts.peek().trim(),
        pinned_fingerprint: pin.peek().trim(),
        remote_db_port: remotePort,
        ssh_password: sshPassword.peek(),
      };

      let savedView = sanitizeConnectionForCache({
        ...payload,
        has_db_password: Boolean(payload.db_password),
        has_ssh_password: Boolean(payload.ssh_password),
        has_bridge_bearer: Boolean(payload.bridge_bearer),
      });

      if (typeof actions.saveDBConnection === "function") {
        const remoteRes = await actions.saveDBConnection(payload);
        if (remoteRes) {
          savedView = sanitizeConnectionForCache(remoteRes);
        }
      }

      const existing = mergedConnections().filter((c) => c.id !== savedView.id);
      syncLocalCache([...existing, savedView]);
      connFormId.set(savedView.id);
      tab.connectionId.set(savedView.id);
      tab.message.set(`Saved connection profile "${savedView.name}".`);
      return savedView;
    }

    async function connectSavedProfile(conn) {
      populateFormFromConnection(conn);
      const eng = conn.engine || "sqlite";
      if (eng !== "sqlite") {
        const existingTunnel = (store.dbTunnels.peek() || []).find(
          (t) =>
            t.engine === eng &&
            (!conn.ssh_host || t.ssh_host === conn.ssh_host) &&
            (!conn.pinned_fingerprint || t.pinned_fingerprint === conn.pinned_fingerprint)
        );
        if (existingTunnel) {
          tab.tunnel.set(existingTunnel.tunnel_id);
        } else if (typeof actions.dbTunnelOpen === "function") {
          const { tunnel } = await actions.dbTunnelOpen({
            connection_id: conn.id,
            engine: eng,
            ssh_host: conn.ssh_host || "",
            ssh_port: conn.ssh_port || "22",
            ssh_user: conn.ssh_user || "",
            known_hosts_path: conn.known_hosts_path || "",
            pinned_fingerprint: conn.pinned_fingerprint || "",
            ssh_private_key_path: conn.ssh_private_key_path || "",
            ssh_password: sshPassword.peek(),
            remote_db_host: conn.db_host || "127.0.0.1",
            remote_db_port: conn.remote_db_port || conn.db_port || "",
          });
          sshPassword.set("");
          if (tunnel && tunnel.tunnel_id) {
            tab.tunnel.set(tunnel.tunnel_id);
          }
        }
      } else {
        tab.tunnel.set("");
      }
      const target = targetFor(tab);
      const res = await actions.dbSchema(target);
      tab.catalog.set({
        id: ++nextRender,
        target,
        tables: res.tables || [],
        views: res.views || [],
        triggers: res.triggers || [],
        fullPostgres: res.postgres_replay_candidate,
        fullMariaDB: res.mariadb_replay_candidate,
      });
      activeConnId.set(conn.id);
      tab.centerView.set("data");
      tab.message.set(`Connected to ${conn.name} (${res.tables?.length || 0} tables).`);
    }

    async function disconnectProfile(conn) {
      const tunId = tab.tunnel.peek().trim();
      if (tunId && typeof actions.dbTunnelClose === "function") {
        await actions.dbTunnelClose({ tunnel_id: tunId });
        tab.tunnel.set("");
      }
      if (activeConnId.peek() === conn.id) {
        activeConnId.set("");
      }
      tab.catalog.set(null);
      tab.result.set(null);
      tab.cell.set(null);
      tab.message.set(`Disconnected from ${conn.name}.`);
    }

    async function deleteProfile(conn) {
      if (activeConnId.peek() === conn.id) {
        await disconnectProfile(conn).catch(() => {});
      }
      if (typeof actions.deleteDBConnection === "function") {
        await actions.deleteDBConnection(conn.id).catch(() => {});
      }
      const next = mergedConnections().filter((c) => c.id !== conn.id);
      syncLocalCache(next);
      if (tab.connectionId.peek() === conn.id) {
        tab.connectionId.set("");
      }
      tab.message.set(`Deleted connection profile "${conn.name}".`);
    }

    function renderResults(result) {
      const local = result.target.engine === "sqlite";
      const table =
        result.target.engine !== "sqlite-bridge" && sameTarget(result.target, targetFor(tab))
          ? catalogForTarget()?.find((entry) => entry.name === result.table)
          : null;
      const columns = new Map((table?.columns || []).map((col) => [col.name, col]));
      const pageNum = Math.max(1, Number.parseInt(String(result.page || "1"), 10) || 1);
      const limitNum = Math.max(1, Number.parseInt(String(result.limit || "25"), 10) || 25);
      const rowOffset = (pageNum - 1) * limitNum;

      const filteredRows = () => {
        const filters = tab.columnFilters.get() || {};
        const activeFilters = Object.entries(filters).filter(([, v]) => String(v || "").trim() !== "");
        const indexed = result.rows.map((row, idx) => ({ __rowKey: `r-${idx}`, __rowNum: rowOffset + idx + 1, row }));
        if (activeFilters.length === 0) return indexed;
        return indexed.filter(({ row }) =>
          activeFilters.every(([colName, needle]) => {
            const raw = row[colName];
            const haystack = raw === null ? "null" : String(raw ?? "").toLowerCase();
            return haystack.includes(String(needle).trim().toLowerCase());
          })
        );
      };

      if (typeof tab._disposeGridBody === "function") {
        tab._disposeGridBody();
      }
      const tbodyHandle = ForEach(
        {
          items: filteredRows,
          key: (item) => item.__rowKey,
          parent: { tag: "tbody" },
          render: (rowEntrySig) => {
            const { __rowNum, row } = rowEntrySig.peek();
            return {
              tag: "tr",
              children: [
                { tag: "th", attrs: { scope: "row", class: "desk-grid-gutter" }, text: String(__rowNum) },
                ...result.columns.map(({ name }) => {
                  const col = columns.get(name);
                  const value = row[name];
                  const text = value === null ? "NULL" : value === "" ? "(empty string)" : value;
                  const primary = (table?.columns || []).filter((entry) => entry.primary_key);
                  const editable =
                    col &&
                    !col.primary_key &&
                    (local ? /INT|TEXT|CHAR|CLOB/i : /INT|TEXT|CHAR|DECIMAL|NUMERIC/i).test(col.type) &&
                    (local ? primary.length > 0 : primary.length === 1) &&
                    primary.every((entry) => typeof row[entry.name] === "string");
                  return editable
                    ? {
                        tag: "td",
                        attrs: { class: "desk-grid-cell-editable" },
                        on: { dblclick: () => chooseCell(result, row, name, col.type) },
                        children: [
                          {
                            tag: "button",
                            attrs: {
                              type: "button",
                              class: "grex-button desk-grid-cell-btn",
                              "aria-label": `Edit ${name}, row ${__rowNum}: ${text}`,
                            },
                            text,
                            on: {
                              click: () => chooseCell(result, row, name, col.type),
                              dblclick: () => chooseCell(result, row, name, col.type),
                            },
                          },
                        ],
                      }
                    : { tag: "td", text };
                }),
              ],
            };
          },
        },
        { document: doc }
      );
      tab._disposeGridBody = tbodyHandle.dispose;

      return {
        tag: "section",
        attrs: { class: "desk-db-grid" },
        children: [
          {
            tag: "table",
            attrs: { class: "desk-grid-table" },
            children: [
              { tag: "caption", attrs: { class: "desk-sr-only" }, text: `${result.table || "Query"} results on ${labelTarget(result.target)}` },
              {
                tag: "thead",
                children: [
                  {
                    tag: "tr",
                    attrs: { class: "desk-grid-header-row" },
                    children: [
                      { tag: "th", attrs: { scope: "col", class: "desk-grid-gutter" }, text: "#" },
                      ...result.columns.map((col) => {
                        const meta = columns.get(col.name);
                        const colType = col.type || meta?.type || "";
                        return {
                          tag: "th",
                          attrs: { scope: "col" },
                          children: [
                            { tag: "span", attrs: { class: "desk-col-name" }, text: col.name },
                            ...(colType ? [{ tag: "span", attrs: { class: "desk-col-type" }, text: colType }] : []),
                          ],
                        };
                      }),
                    ],
                  },
                  {
                    tag: "tr",
                    attrs: { class: "desk-grid-filter-row" },
                    children: [
                      { tag: "th", attrs: { class: "desk-grid-gutter" }, text: "" },
                      ...result.columns.map((col) => ({
                        tag: "th",
                        attrs: { class: "desk-col-filter-cell" },
                        children: [
                          {
                            tag: "input",
                            attrs: {
                              type: "search",
                              class: "desk-col-filter",
                              "aria-label": `Filter column ${col.name}`,
                              placeholder: "Filter...",
                              value: String((tab.columnFilters.peek() || {})[col.name] || ""),
                            },
                            on: {
                              input: (ev) => {
                                const val = ev && ev.target ? String(ev.target.value || "") : "";
                                const next = { ...(tab.columnFilters.peek() || {}), [col.name]: val };
                                tab.columnFilters.set(next);
                              },
                            },
                          },
                        ],
                      })),
                    ],
                  },
                ],
              },
              tbodyHandle.element,
            ],
          },
        ],
      };
    }

    const focusSectionControl = (selector) => {
      if (doc && typeof doc.querySelector === "function") {
        const el = doc.querySelector(selector);
        if (el && typeof el.focus === "function") {
          el.focus();
        }
      }
    };

    const matchesFilter = (name) => {
      const q = String(objectFilter.get() || "").trim().toLowerCase();
      if (!q) return true;
      return String(name || "").toLowerCase().includes(q);
    };

    const newConnPlusIcon = Icon(
      {
        className: "desk-inline-icon",
        props: { size: "14", viewBox: "0 0 24 24", paths: PLUS_ICON_PATHS },
      },
      { document: doc }
    ).element;

    const connectionsPane = {
      type: "Section",
      tag: "section",
      className: "desk-db-connections-pane",
      props: { label: "Saved Connections" },
      children: [
        {
          tag: "section",
          attrs: { class: "desk-db-section-header" },
          children: [
            { tag: "span", attrs: { class: "desk-db-section-title" }, text: "CONNECTIONS" },
            {
              tag: "button",
              attrs: {
                id: "db-new-conn-btn",
                type: "button",
                class: "grex-button desk-icon-btn",
                "aria-label": "New Connection",
                title: "New Connection",
              },
              on: { click: () => openNewConnectionDialog() },
              children: [newConnPlusIcon],
            },
          ],
        },
        {
          type: "Section",
          tag: "section",
          id: "desk-db-connections-list",
          attrs: { id: "desk-db-connections-list" },
          className: "desk-db-connections-list",
          props: { label: "Saved Database Connections" },
          children: [
            ForEach(
              {
                items: () => mergedConnections(),
                key: (item) => item.id,
                parent: { type: "Stack", tag: "ul", className: "desk-db-conn-ul", props: { label: "Saved connection profiles" } },
                empty: () => ({
                  tag: "li",
                  attrs: { class: "desk-db-conn-empty" },
                  children: [
                    {
                      tag: "button",
                      attrs: {
                        id: "db-quick-sqlite-btn",
                        type: "button",
                        class: "grex-button desk-conn-select-btn",
                        disabled: () => tab.busy.get(),
                      },
                      text: "Workspace SQLite (data/app.db)",
                      on: {
                        click: () =>
                          run(tab, () =>
                            connectSavedProfile({
                              id: "workspace-sqlite",
                              name: "Workspace SQLite",
                              engine: "sqlite",
                              sqlite_path: "data/app.db",
                            })
                          ),
                      },
                    },
                  ],
                }),
                render: (entrySig) => {
                  const conn = entrySig.peek();
                  const isConnected = () => activeConnId.get() === conn.id;
                  return {
                    tag: "li",
                    attrs: {
                      class: () => (isConnected() ? "desk-db-conn-item is-connected" : "desk-db-conn-item"),
                    },
                    children: [
                      {
                        tag: "button",
                        attrs: {
                          type: "button",
                          class: "grex-button desk-conn-select-btn",
                          "aria-label": () => {
                            const c = entrySig.get();
                            return `Connect to ${c.name} (${c.engine})`;
                          },
                        },
                        on: { click: () => run(tab, () => connectSavedProfile(entrySig.peek())) },
                        children: [
                          { tag: "span", attrs: { class: "desk-conn-dot" }, text: () => (isConnected() ? "●" : "○") },
                          { tag: "span", attrs: { class: "desk-badge desk-conn-engine" }, text: () => engineBadgeText(entrySig.get().engine) },
                          {
                            tag: "span",
                            attrs: { class: "desk-conn-name" },
                            text: () => {
                              const c = entrySig.get();
                              const d = c.engine === "sqlite" ? c.sqlite_path || "data/app.db" : c.db_name || c.ssh_host || c.engine;
                              return `${c.name} · ${d}`;
                            },
                          },
                        ],
                      },
                      {
                        tag: "section",
                        attrs: { class: "desk-conn-actions" },
                        children: [
                          {
                            tag: "button",
                            attrs: { type: "button", class: "grex-button desk-conn-action-btn", disabled: () => tab.busy.get() },
                            text: "Edit",
                            on: {
                              click: () => {
                                populateFormFromConnection(entrySig.peek());
                                connDialogOpen.set(true);
                              },
                            },
                          },
                          {
                            tag: "button",
                            attrs: {
                              type: "button",
                              class: "grex-button desk-conn-action-btn",
                              disabled: () => tab.busy.get() || (!isConnected() && !tab.tunnel.get()),
                            },
                            text: "Disconnect",
                            on: { click: () => run(tab, () => disconnectProfile(entrySig.peek())) },
                          },
                          {
                            tag: "button",
                            attrs: { type: "button", class: "grex-button desk-conn-action-btn", disabled: () => tab.busy.get() },
                            text: "Delete",
                            on: { click: () => run(tab, () => deleteProfile(entrySig.peek())) },
                          },
                        ],
                      },
                    ],
                  };
                },
              },
              { document: doc }
            ),
          ],
        },
        repeat(
          () => store.dbTunnels.get().map((tunnel) => ({ ...tunnel, id: tunnel.tunnel_id })),
          (entry) =>
            paragraph(() => {
              const tunnel = entry.get();
              return `Tunnel ${tunnel.tunnel_id} · ${tunnel.engine} · ${tunnel.local_bind_addr}`;
            }, { class: "desk-muted desk-mono" }),
          "Active SSH database tunnels"
        ),
      ],
    };

    const schemaTree = Accordion({
      className: "desk-db-accordion",
      props: {
        items: [
          {
            id: `db-tree-tables-${tab.id}`,
            summary: "Tables",
            open: false,
            content: [
              repeat(
                () =>
                  catalogForTarget()
                    .filter((table) => matchesFilter(table.name))
                    .map((table) => ({ ...table, id: `tbl-${table.schema || "main"}-${table.name}` })),
                (entry) => {
                  const table = entry.peek();
                  const colCount = (table.columns || []).length;
                  const hasRowCount = table.row_count !== undefined && table.row_count !== null && String(table.row_count) !== "";
                  const label = hasRowCount ? `${table.name} (${table.row_count})` : `${table.name} (${colCount} cols)`;
                  return button(
                    null,
                    label,
                    () => {
                      tab.table.set(table.name);
                      tab.centerView.set("data");
                      tab.message.set(`Loading ${table.name}...`);
                      browse();
                    },
                    disabled,
                    "desk-tree-item",
                    {
                      "data-selected": () => (tab.table.get() === table.name ? "true" : "false"),
                      title: `${table.name} (${colCount} cols${hasRowCount ? `, ${table.row_count} rows` : ""})`,
                    }
                  );
                },
                "Tables list"
              ),
            ],
          },
          {
            id: `db-tree-views-${tab.id}`,
            summary: "Views",
            open: false,
            content: [
              paragraph(() => `Views (${viewsForTarget().length})`),
              repeat(
                () =>
                  viewsForTarget()
                    .filter((v) => matchesFilter(v.name || v))
                    .map((v, idx) => ({ id: `view-${idx}-${v.name || v}`, label: v.name || String(v) })),
                (entry) => paragraph(() => entry.get().label),
                "Views list"
              ),
            ],
          },
          {
            id: `db-tree-triggers-${tab.id}`,
            summary: "Triggers",
            open: false,
            content: [
              paragraph(() => `Triggers (${triggersForTarget().length})`),
              repeat(
                () =>
                  triggersForTarget()
                    .filter((tr) => matchesFilter(tr.name || tr))
                    .map((tr, idx) => ({ id: `trig-${idx}-${tr.name || tr}`, label: tr.name || String(tr) })),
                (entry) => paragraph(() => entry.get().label),
                "Triggers list"
              ),
            ],
          },
        ],
      },
    }, { document: doc });
    const tableSummary = bindElement(schemaTree.element.querySelector("summary"), {
      text: () => `Tables (${catalogForTarget().length})`,
    });

    const leftSidebar = {
      type: "Section",
      tag: "aside",
      id: "desk-db-sidebar",
      className: "desk-db-sidebar",
      attrs: {
        id: "desk-db-sidebar",
        "data-collapsed": () => (store.dbSidebarCollapsed && store.dbSidebarCollapsed.get() ? "true" : "false"),
        inert: () => Boolean(store.dbSidebarCollapsed?.get()),
      },
      props: { label: "Connections and Schema Object Tree" },
      children: [
        { type: "Heading", className: "desk-sr-only", props: { level: 4, text: "Connections & Schema" } },
        {
          type: "Section",
          tag: "section",
          className: "desk-db-schema-pane",
          props: { label: "Schema Object Tree" },
          children: [
            {
              type: "Input",
              id: "desk-db-object-filter-field",
              className: "desk-db-filter-field",
              props: {
                inputId: "desk-db-object-filter",
                name: "db-object-filter",
                label: "Filter schema objects",
                type: "search",
                placeholder: "Search in tables, views...",
                value: objectFilter,
                bare: true,
              },
            },
            {
              type: "Text",
              tag: "p",
              className: "desk-db-namespace-label desk-mono",
              attrs: { hidden: true },
              props: {
                text: () => {
                  const total = catalogForTarget().length + viewsForTarget().length + triggersForTarget().length;
                  return `${schemaNamespaceLabel()} (${total})`;
                },
              },
            },
            {
              element: schemaTree.element,
              dispose() {
                tableSummary.dispose();
                schemaTree.dispose();
              },
            },
            connectionsPane,
          ],
        },
      ],
    };

    const completion = () => {
      const query = tab.sql.get();
      const prefix = /[a-z_][a-z0-9_]*$/i.exec(query)?.[0] || "";
      if (!prefix) return [];
      const schema = catalogForTarget() || [];
      const names = schema.flatMap((table) => [table.name, ...table.columns.map((col) => col.name)]);
      return [...new Set([...(keywords[tab.engine.get()] || []), ...names])]
        .filter((word) => word.toUpperCase().startsWith(prefix.toUpperCase()) && word !== prefix)
        .slice(0, 8)
        .map((word) => ({ id: word }));
    };

    const activeTableSchema = () => {
      const name = tab.table.get().trim();
      return catalogForTarget().find((entry) => entry.name === name) || null;
    };

    const subTabButton = (viewId, domId, label) => ({
      tag: "button",
      attrs: {
        id: domId,
        type: "button",
        hidden: () => viewId === "structure" && tab.centerView.get() !== viewId,
        "aria-controls": `db-subpanel-${viewId}`,
        role: "tab",
        "aria-selected": () => (tab.centerView.get() === viewId ? "true" : "false"),
        class: () => (tab.centerView.get() === viewId ? "desk-db-subtab is-active" : "desk-db-subtab"),
      },
      on: { click: () => tab.centerView.set(viewId) },
      text: label,
    });

    const openWriteInspector = () => {
      inspectorOpen.set(true);
      focusSectionControl(tab.cell.peek() ? "#db-cell-value input" : "#db-mutate-sql textarea");
    };

    const centerWorkspace = {
      type: "Section",
      tag: "section",
      id: "desk-db-center",
      attrs: { id: "desk-db-center" },
      className: "desk-db-center",
      props: { label: "Data Grid, Structure, SQL Query, ER Diagram and Export Workspace" },
      children: [
        { type: "Heading", className: "desk-sr-only", props: { level: 4, text: "SQL Editor & Data Grid" } },
        {
          type: "Section",
          tag: "section",
          className: "desk-db-subtabs",
          children: [
            {
              type: "Stack",
              tag: "nav",
              id: "desk-db-subtabs",
              attrs: { id: "desk-db-subtabs", role: "tablist", "aria-label": "Database Center Sub-Views" },
              className: "desk-db-view-tabs",
              children: [
                subTabButton("data", "db-subtab-data", "Data"),
                subTabButton("structure", "db-subtab-structure", "Structure"),
                subTabButton("sql", "db-subtab-sql", "SQL"),
                subTabButton("er", "db-subtab-er", "ER Diagram"),
                subTabButton("export", "db-subtab-export", "Export"),
              ],
            },
            button("db-save-btn", "Save", openWriteInspector, disabled, "desk-btn-primary"),
            button("db-rows-btn", "Refresh", browse, disabled, "desk-btn-primary", {
              title: "Refresh rows",
              "aria-label": "Refresh rows",
            }),
            button("db-schema-btn", "Refresh", inspectSchema, disabled, "desk-btn-primary", {
              title: "Refresh schema",
              "aria-label": "Refresh schema",
            }),
          ],
        },
        // Sub-View 1: Data (Default Full-Screen Spreadsheet Grid)
        {
          type: "Section",
          tag: "section",
          id: "db-subpanel-data",
          attrs: {
            id: "db-subpanel-data",
            role: "tabpanel",
            "aria-labelledby": "db-subtab-data",
            hidden: () => tab.centerView.get() !== "data",
            "aria-hidden": () => (tab.centerView.get() !== "data" ? "true" : "false"),
          },
          className: "desk-db-subpanel desk-db-subpanel-data",
          children: [
            repeat(
              () => {
                const result = tab.result.get();
                if (!result) return [];
                const revision = tab.catalog.get()?.id || 0;
                return [{ ...result, id: `${result.id}-${revision}` }];
              },
              (entry) => renderResults(entry.peek()),
              "Spreadsheet data grid"
            ),
            {
              type: "Section",
              tag: "section",
              attrs: { hidden: () => Boolean(tab.result.get()) },
              className: "desk-db-empty-grid",
              children: [
                paragraph(() =>
                  catalogForTarget().length > 0
                    ? `Select a table in the left tree or click Refresh to load rows for "${tab.table.get() || "table"}".`
                    : "Choose a saved connection on the left (or click + New Connection), then click any table to browse rows."
                ),
                button(null, "Load Selected Table", browse, disabled, "desk-btn-primary"),
              ],
            },
          ],
        },
        // Sub-View 2: Structure
        {
          type: "Section",
          tag: "section",
          id: "db-subpanel-structure",
          attrs: {
            id: "db-subpanel-structure",
            role: "tabpanel",
            "aria-labelledby": "db-subtab-structure",
            hidden: () => tab.centerView.get() !== "structure",
            "aria-hidden": () => (tab.centerView.get() !== "structure" ? "true" : "false"),
          },
          className: "desk-db-subpanel",
          children: [
            repeat(
              () => {
                const tbl = activeTableSchema();
                return tbl ? [{ ...tbl, id: `struct-${tbl.schema}-${tbl.name}-${tab.catalog.get()?.id || 0}` }] : [];
              },
              (entry) => {
                const tbl = entry.peek();
                return {
                  tag: "section",
                  attrs: { class: "desk-db-grid" },
                  children: [
                    {
                      tag: "table",
                      attrs: { class: "desk-grid-table" },
                      children: [
                        { tag: "caption", text: `Columns for ${tbl.schema || "main"}.${tbl.name}` },
                        {
                          tag: "thead",
                          children: [
                            {
                              tag: "tr",
                              children: ["#", "Column", "Type", "Nullable", "Primary Key", "Default"].map((h) => ({
                                tag: "th",
                                attrs: { scope: "col" },
                                text: h,
                              })),
                            },
                          ],
                        },
                        {
                          tag: "tbody",
                          children: (tbl.columns || []).map((col, idx) => ({
                            tag: "tr",
                            children: [
                              { tag: "th", attrs: { scope: "row", class: "desk-grid-gutter" }, text: String(idx + 1) },
                              { tag: "td", text: col.name },
                              { tag: "td", text: col.type || "" },
                              { tag: "td", text: col.nullable ? "YES" : "NO" },
                              { tag: "td", text: col.primary_key ? "PK" : "—" },
                              { tag: "td", text: col.default_val || "—" },
                            ],
                          })),
                        },
                      ],
                    },
                  ],
                };
              },
              "Table structure"
            ),
          ],
        },
        // Sub-View 3: SQL Query
        {
          type: "Section",
          tag: "section",
          id: "db-subpanel-sql",
          attrs: {
            id: "db-subpanel-sql",
            role: "tabpanel",
            "aria-labelledby": "db-subtab-sql",
            hidden: () => tab.centerView.get() !== "sql",
            "aria-hidden": () => (tab.centerView.get() !== "sql" ? "true" : "false"),
          },
          className: "desk-db-subpanel",
          children: [
            actionsDrawer("Database actions", [
              button("db-toolbar-structure-btn", "Structure", () => tab.centerView.set("structure"), disabled),
              button("db-toolbar-sql-btn", "SQL", () => {
                tab.centerView.set("sql");
                focusSectionControl("#db-ro-sql textarea");
              }),
              button("db-toolbar-export-btn", "Export", () => {
                tab.centerView.set("export");
                focusSectionControl("#db-export-path input");
              }),
              button("db-toolbar-er-btn", "ER Diagram", () => {
                tab.centerView.set("er");
                if (!currentCatalog()) {
                  inspectSchema();
                }
              }),
              button("db-toolbar-diff-btn", "Schema Diff", () => {
                tab.centerView.set("export");
                focusSectionControl("#db-compare-source input");
              }),
              button("db-toolbar-undo-btn", "Undo Snapshot", () => run(tab, () => reviewRestore()), () => tab.busy.get() || !tab.applied.get()),
              button("db-toolbar-guard-btn", "Write SQL / Snapshot Guard", openWriteInspector),
              button("db-toolbar-inspector-btn", () => (inspectorOpen.get() ? "Hide Inspector" : "Inspector"), () => {
                inspectorOpen.set(!inspectorOpen.peek());
              }),
            ], "desk-db-toolbar"),
            buildSQLTabControls(),
            controls("SQL editor and table browsing", [
              input("db-ro-sql", "Read-only SQL", tab.sql, "textarea"),
              input("db-table-name", "Table name", tab.table),
              input("db-page-num", "Page", tab.page),
              input("db-limit-num", "Page limit (maximum 100)", tab.limit),
            ]),
            {
              type: "Section",
              tag: "section",
              className: "desk-badge-row",
              children: [
                button("db-query-btn", "Run Query", runReadOnlyQuery, () => tab.busy.get() || tab.engine.get() === "sqlite-bridge", "desk-btn-primary"),
                button(null, "Cancel query", () => tab.query?.abort(), () => !tab.busy.get()),
              ],
            },
            repeat(
              completion,
              (entry) =>
                button(null, () => entry.get().id, () => {
                  tab.sql.set(tab.sql.peek().replace(/[a-z_][a-z0-9_]*$/i, () => entry.peek().id) + " ");
                  doc.querySelector("#db-ro-sql textarea")?.focus();
                }),
              "SQL completions"
            ),
            repeat(
              () => {
                const result = tab.result.get();
                if (!result || result.table) return [];
                const revision = tab.catalog.get()?.id || 0;
                return [{ ...result, id: `sql-${result.id}-${revision}` }];
              },
              (entry) => renderResults(entry.peek()),
              "SQL query results"
            ),
          ],
        },
        // Sub-View 4: ER Diagram
        {
          type: "Section",
          tag: "section",
          id: "db-subpanel-er",
          attrs: {
            id: "db-subpanel-er",
            role: "tabpanel",
            "aria-labelledby": "db-subtab-er",
            hidden: () => tab.centerView.get() !== "er",
            "aria-hidden": () => (tab.centerView.get() !== "er" ? "true" : "false"),
          },
          className: "desk-db-subpanel",
          children: [
            repeat(
              () => {
                const catalog = tab.catalog.get();
                return catalog ? [{ id: catalog.id, tables: catalog.tables }] : [];
              },
              (entry) => {
                const { id, tables } = entry.peek();
                const entities = new Map(tables.map((table, index) => [`${table.schema}.${table.name}`, `db-entity-${tab.id}-${id}-${index}`]));
                return {
                  type: "Section",
                  tag: "section",
                  props: { label: "Entity-relationship diagram" },
                  children: [
                    { type: "Heading", props: { level: 4, text: "Entity-Relationship Diagram" } },
                    {
                      type: "Section",
                      tag: "section",
                      className: "desk-db-diagram",
                      children: tables.map((table) => {
                        const name = `${table.schema}.${table.name}`;
                        const elementId = entities.get(name);
                        return {
                          tag: "article",
                          attrs: { id: elementId, class: "desk-db-entity", "aria-labelledby": `${elementId}-title` },
                          children: [
                            { tag: "h5", attrs: { id: `${elementId}-title`, tabindex: "-1" }, text: name },
                            {
                              tag: "ul",
                              attrs: { "aria-label": "Columns" },
                              children: (table.columns || []).map((col) => ({
                                tag: "li",
                                text: [col.name, col.type, col.primary_key ? "primary key" : "", col.nullable ? "nullable" : ""].filter(Boolean).join(" · "),
                              })),
                            },
                            {
                              tag: "ul",
                              attrs: { "aria-label": "Foreign keys", class: "desk-db-relationships" },
                              children: (table.foreign_keys || []).map((key) => {
                                const linked = `${key.ref_schema || table.schema}.${key.ref_table}`;
                                const target = entities.get(linked);
                                const label = `${key.column} → ${linked}.${key.ref_column}`;
                                return {
                                  tag: "li",
                                  children: [
                                    target
                                      ? {
                                          tag: "button",
                                          attrs: { type: "button", class: "grex-button" },
                                          text: label,
                                          on: { click: () => doc.getElementById(`${target}-title`)?.focus() },
                                        }
                                      : { tag: "span", text: label },
                                  ],
                                };
                              }),
                            },
                            { tag: "p", text: `Indexes: ${(table.indexes || []).join(", ") || "none"}` },
                            {
                              tag: "button",
                              attrs: { type: "button", class: "grex-button" },
                              text: `Select table ${table.name}`,
                              on: {
                                click: () => {
                                  tab.table.set(table.name);
                                  tab.centerView.set("data");
                                  browse();
                                },
                              },
                            },
                          ],
                        };
                      }),
                    },
                  ],
                };
              },
              "Database tables and relationships"
            ),
          ],
        },
        // Sub-View 5: Export & Diff
        {
          type: "Section",
          tag: "section",
          id: "db-subpanel-export",
          attrs: {
            id: "db-subpanel-export",
            role: "tabpanel",
            "aria-labelledby": "db-subtab-export",
            hidden: () => tab.centerView.get() !== "export",
            "aria-hidden": () => (tab.centerView.get() !== "export" ? "true" : "false"),
          },
          className: "desk-db-subpanel",
          children: [
            controls("Export a table page", [
              input("db-export-path", "Workspace output path (.csv, .xlsx, .json, or .sql)", tab.exportPath),
              button(
                null,
                "Export table page",
                () =>
                  run(tab, async () => {
                    const target = targetFor(tab);
                    const saved = await actions.dbExportPage({
                      target,
                      path: tab.exportPath.peek().trim(),
                      table: tab.table.peek().trim(),
                      page: tab.page.peek().trim(),
                      limit: tab.limit.peek().trim(),
                    });
                    tab.message.set(`Exported ${saved.rows_exported} rows from ${saved.table} on ${labelTarget(target)} to ${saved.path}.`);
                  }),
                disabled
              ),
            ]),
            controls("Export the relationship diagram", [
              input("db-diagram-path", "Workspace diagram path (.html)", tab.diagramPath),
              button(
                null,
                "Export relationship diagram",
                () =>
                  run(tab, async () => {
                    const target = targetFor(tab);
                    const saved = await actions.dbExportDiagram({ target, path: tab.diagramPath.peek().trim() });
                    tab.message.set(`Exported the diagram for ${labelTarget(target)} to ${saved.path}.`);
                  }),
                disabled
              ),
            ]),
            controls("Import rows or SQL", [
              input("db-import-path", "Workspace import path (.csv, .xlsx, .json, or .sql)", tab.importPath),
              button(
                null,
                "Review import",
                () =>
                  run(tab, async () => {
                    tab.plan.set(null);
                    const request = { target: fullWriteTarget(), path: tab.importPath.peek().trim(), table: tab.table.peek().trim() };
                    const plan = await actions.dbImportPlan(request);
                    tab.plan.set({ type: "import", plan, request });
                    inspectorOpen.set(true);
                    tab.message.set(plan.plain_summary);
                  }),
                () => tab.busy.get() || !canReviewWrite()
              ),
            ]),
            controls("Compare database schemas", [
              input("db-compare-source", "Source tab number", sourceTab),
              button(
                null,
                "Compare source with this tab",
                () =>
                  run(tab, async () => {
                    const id = sourceTab.peek().trim();
                    const other = tabs.peek().find((candidate) => String(candidate.id) === id && candidate !== tab);
                    if (!other) throw new Error("Enter the number of another open SQL tab.");
                    const destination = targetFor(tab);
                    const comparison = await actions.dbCompare({ source: targetFor(other), destination });
                    tab.compare.set({ id: ++nextRender, comparison, destination });
                    tab.message.set(`Found ${comparison.changes.length} schema differences.`);
                  }),
                disabled
              ),
            ]),
            repeat(
              () => (tab.compare.get() ? [tab.compare.get()] : []),
              (entry) => {
                const { comparison, destination } = entry.peek();
                return {
                  tag: "table",
                  attrs: { class: "desk-grid-table" },
                  children: [
                    { tag: "caption", text: `Schema comparison from ${comparison.source.engine} to ${comparison.destination.engine}` },
                    {
                      tag: "thead",
                      children: [
                        {
                          tag: "tr",
                          children: ["Object", "Source", "Destination", "Migration review"].map((text) => ({
                            tag: "th",
                            attrs: { scope: "col" },
                            text,
                          })),
                        },
                      ],
                    },
                    {
                      tag: "tbody",
                      children: comparison.changes.map((change) => ({
                        tag: "tr",
                        children: [
                          { tag: "th", attrs: { scope: "row" }, text: `${change.table}${change.object ? "." + change.object : ""}` },
                          { tag: "td", text: change.source === undefined ? "—" : JSON.stringify(change.source) },
                          { tag: "td", text: change.destination === undefined ? "—" : JSON.stringify(change.destination) },
                          {
                            tag: "td",
                            children: [
                              { tag: "p", text: [change.proposed_sql, change.review].filter(Boolean).join(" · ") || "Manual review" },
                              ...(change.proposed_sql && (comparison.destination.postgres_replay_candidate || comparison.destination.mariadb_replay_candidate)
                                ? [
                                    {
                                      tag: "button",
                                      attrs: {
                                        type: "button",
                                        class: "grex-button",
                                        disabled: () => tab.busy.get() || !canReviewWrite(),
                                      },
                                      text: "Review migration",
                                      on: {
                                        click: () =>
                                          run(tab, () => {
                                            if (!sameTarget(destination, targetFor(tab))) {
                                              throw new Error("Compare the current target before reviewing a migration.");
                                            }
                                            return planWrite({ sql: change.proposed_sql }, fullWriteTarget());
                                          }),
                                      },
                                    },
                                  ]
                                : []),
                            ],
                          },
                        ],
                      })),
                    },
                  ],
                };
              },
              "Schema comparison results"
            ),
          ],
        },
        paragraph(tab.message, { role: "status", "aria-live": "polite", class: "desk-db-center-status desk-muted" }),
      ],
    };

    const rightInspector = {
      type: "Section",
      tag: "aside",
      id: "desk-db-inspector",
      className: "desk-db-inspector",
      attrs: {
        id: "desk-db-inspector",
        "data-collapsed": () => (inspectorOpen.get() ? "false" : "true"),
        "aria-hidden": () => (inspectorOpen.get() ? "false" : "true"),
        hidden: () => !inspectorOpen.get(),
      },
      props: { label: "Row Inspector, Pre-Mutation Snapshot Guard and GROXY-SAFE-001 Drawer" },
      children: [
        {
          type: "Section",
          tag: "section",
          className: "desk-db-inspector-header",
          children: [
            { type: "Heading", props: { level: 4, text: "Row Inspector & Safety Gate" } },
            button("db-inspector-close-btn", "Close", () => inspectorOpen.set(false), undefined, "desk-conn-action-btn"),
          ],
        },
        controls("Inline cell edit", [
          paragraph(() => {
            const cell = tab.cell.get();
            if (!cell) return "Click any editable cell in the grid to inspect or edit exact ADR-025c values.";
            const before = cell.before === null ? "NULL" : JSON.stringify(cell.before);
            return `${cell.result.table}.${cell.name}, key ${JSON.stringify(cell.key)}, current value ${before}.`;
          }),
          input("db-cell-value", "Replacement value", tab.replacement),
          select(
            "db-cell-type",
            "Value type",
            tab.format,
            ["text", "integer", "decimal", "null"].map((value) => ({ value, label: value }))
          ),
          button(
            null,
            "Review cell change",
            () =>
              run(tab, async () => {
                const cell = tab.cell.peek();
                if (!cell) return;
                const kind = tab.format.peek();
                await planWrite(
                  {
                    edit: {
                      table: cell.result.table,
                      column: cell.name,
                      key: cell.key,
                      before: cell.before,
                      value: kind === "null" ? null : tab.replacement.peek(),
                      value_kind: kind === "null" ? "" : kind,
                    },
                  },
                  cell.result.target
                );
              }),
            () => tab.busy.get() || !tab.cell.get()
          ),
          button(null, "Cancel cell edit", () => tab.cell.set(null), () => !tab.cell.get()),
        ]),
        repeat(
          () => (fullPostgres() ? [tab] : []),
          () =>
            controls("PostgreSQL database maintenance", [
              input("db-maintenance-user", "Admission administrator on the postgres database", tab.maintenanceUser),
              input("db-maintenance-password", "Maintenance password", tab.maintenancePassword, "password"),
            ]),
          "PostgreSQL recovery credentials"
        ),
        {
          type: "Text",
          tag: "p",
          className: "desk-status-banner",
          props: {
            text: () => {
              const engine = tab.engine.get();
              if (engine === "mariadb") {
                return "Pre-Mutation Snapshot Guard: MariaDB requires target-only least-privilege grants (409 write_fence_unavailable if any global *.* write exists).";
              }
              if (engine === "postgres") {
                return "Pre-Mutation Snapshot Guard: PostgreSQL writes require a maintenance login to fence target admission with SHA-256 recovery.";
              }
              return "Pre-Mutation Snapshot Guard: GROXY-SAFE-001 captures a verified SHA-256 snapshot before any approved write.";
            },
          },
        },
        controls("Guarded database changes", [
          input("db-mutate-sql", "Write SQL", tab.write, "textarea"),
          button(
            "db-mutate-plan-btn",
            "Review SQL write",
            () => run(tab, () => planWrite({ sql: tab.write.peek() }, fullWriteTarget())),
            () => tab.busy.get() || !canReviewWrite()
          ),
          button(null, "Undo last database change", () => run(tab, () => reviewRestore()), () => tab.busy.get() || !tab.applied.get()),
          button(null, "Review forced restore", () => run(tab, () => reviewRestore(true)), () => {
            const engine = tab.applied.get()?.target.engine;
            return tab.busy.get() || !tab.restoreConflict.get() || !["postgres", "mariadb"].includes(engine);
          }),
          paragraph(() => {
            const pending = tab.plan.get();
            if (!pending) return "";
            const { request } = pending;
            const detail =
              request.sql ||
              pending.plan.reviewed_sql ||
              (request.path
                ? `${request.path} · ${pending.plan.plain_summary}`
                : request.edit
                  ? JSON.stringify(request.edit)
                  : pending.plan.plain_summary);
            return `${labelTarget(request.target)} — ${detail}`;
          }),
          button(
            "db-mutate-apply-btn",
            () => {
              const pending = tab.plan.get();
              if (pending?.type !== "restore") return "Approve database change";
              return pending.request.force ? "Approve overwriting the reviewed changes" : "Approve restore";
            },
            approve,
            () => tab.busy.get() || !tab.plan.get(),
            "desk-btn-primary"
          ),
          button(null, "Discard approval", () => tab.plan.set(null), () => !tab.plan.get()),
        ]),
      ],
    };

    const connectionDialog = {
      type: "Modal",
      id: "desk-db-conn-dialog",
      attrs: {
        id: "desk-db-conn-dialog",
        "aria-labelledby": "desk-db-conn-dialog-title",
      },
      className: "desk-db-conn-dialog",
      props: {
        open: connDialogOpen,
        onClose: () => connDialogOpen.set(false),
      },
      children: [
        {
          type: "Heading",
          id: "desk-db-conn-dialog-title",
          attrs: { id: "desk-db-conn-dialog-title" },
          props: { level: 4, text: "Database Connection" },
        },
        {
          type: "Tabs",
          id: "desk-db-conn-tabs",
          className: "desk-db-conn-tabs",
          props: {
            label: "Connection profile tabs",
            tabs: [
              {
                id: "general",
                label: "General",
                content: [
                  controls("General connection settings", [
                    input("db-conn-name", "Connection name", connFormName, "text", "e.g. Production PostgreSQL"),
                    select("engine", "Engine", tab.engine, engines),
                    input("db-server", "Server", connFormHost, "text", "127.0.0.1"),
                    input("db-port", "Port", connFormPort, "text", "5432"),
                    input("db-user", "User", tab.user),
                    input("db-password", "Password", tab.password, "password"),
                    input("db-database", "Default database", tab.database),
                    input("db-schema", "PostgreSQL schema", tab.schema),
                    input("db-sqlite-path", "SQLite path relative to workspace or bridge root", tab.path),
                    input("db-bridge-bearer", "Gashboard bridge bearer", tab.bridgeBearer, "password"),
                    input("db-tunnel-id", "SSH tunnel ID", tab.tunnel),
                  ]),
                ],
              },
              {
                id: "ssh",
                label: "SSH Tunnel",
                content: [
                  controls("Pinned SSH tunnel settings", [
                    input("db-ssh-host", "SSH host", sshHost),
                    input("db-ssh-port", "SSH port", sshPort),
                    input("db-ssh-user", "SSH user", sshUser),
                    input("db-ssh-key", "Private key path", privateKey),
                    input("db-ssh-known-hosts", "known_hosts path", knownHosts),
                    input("db-ssh-pin", "SHA256 fingerprint", pin),
                    input("db-ssh-password", "SSH password or key passphrase", sshPassword, "password"),
                    input("db-ssh-target-port", "Remote port", targetPort),
                    button(
                      null,
                      "Open pinned SSH tunnel",
                      () =>
                        run(tab, async () => {
                          const engine = tab.engine.peek();
                          const credentials = sshPassword.peek();
                          sshPassword.set("");
                          const { tunnel } = await actions.dbTunnelOpen({
                            connection_id: tab.connectionId.peek().trim(),
                            engine,
                            ssh_host: sshHost.peek().trim(),
                            ssh_port: sshPort.peek().trim(),
                            ssh_user: sshUser.peek().trim(),
                            known_hosts_path: knownHosts.peek().trim(),
                            pinned_fingerprint: pin.peek().trim(),
                            ssh_private_key_path: privateKey.peek().trim(),
                            ssh_password: credentials,
                            remote_db_host: connFormHost.peek().trim() || "127.0.0.1",
                            remote_db_port: targetPort.peek().trim() || connFormPort.peek().trim(),
                          });
                          tab.tunnel.set(tunnel.tunnel_id);
                          tab.message.set(`Opened a pinned SSH tunnel to ${tunnel.remote_target}.`);
                        }),
                      () => tab.busy.get() || tab.engine.get() === "sqlite"
                    ),
                    button(
                      null,
                      "Close this SSH tunnel",
                      () =>
                        run(tab, async () => {
                          await actions.dbTunnelClose({ tunnel_id: tab.tunnel.peek() });
                          tab.tunnel.set("");
                          tab.message.set("SSH tunnel closed.");
                        }),
                      () => tab.busy.get() || !tab.tunnel.get()
                    ),
                  ]),
                ],
              },
            ],
          },
        },
        {
          type: "Section",
          tag: "section",
          className: "desk-db-dialog-footer",
          children: [
            button("db-conn-save-btn", "Save", () => run(tab, () => saveCurrentConnection()), disabled),
            button(
              "db-conn-connect-btn",
              "Connect",
              () =>
                run(tab, async () => {
                  const saved = await saveCurrentConnection();
                  await connectSavedProfile(saved);
                  connDialogOpen.set(false);
                }),
              disabled,
              "desk-btn-primary"
            ),
            button("db-conn-close-btn", "Close", () => connDialogOpen.set(false)),
          ],
        },
      ],
    };

    return {
      type: "Section",
      tag: "section",
      className: "desk-db-three-pane",
      attrs: {
        "data-sidebar-collapsed": () => (store.dbSidebarCollapsed && store.dbSidebarCollapsed.get() ? "true" : "false"),
        "data-inspector-open": () => (inspectorOpen.get() ? "true" : "false"),
      },
      props: { label: `Database tab ${tab.id}` },
      on: {
        keydown: (event) => {
          if (event.key !== "Escape") return;
          if (connDialogOpen.peek()) connDialogOpen.set(false);
          else if (tab.query) tab.query.abort();
          else if (tab.plan.peek()) tab.plan.set(null);
          else if (tab.cell.peek()) tab.cell.set(null);
          else if (inspectorOpen.peek()) inspectorOpen.set(false);
        },
      },
      children: [leftSidebar, centerWorkspace, rightInspector, connectionDialog],
    };
  }

  const actionsDrawer = (label, content, id) => ({
    type: "Accordion",
    id,
    attrs: id ? { id } : undefined,
    className: "desk-db-actions-drawer",
    props: { items: [{ id: label, summary: label, content }] },
  });

  const buildSQLTabControls = () => actionsDrawer("SQL tabs", [
    button(null, "New SQL tab", () => {
      const tab = makeTab(++nextTab);
      tab.centerView.set("sql");
      tabs.set([...tabs.peek(), tab]);
      active.set(tab.id);
    }),
    button(
      null,
      "Close current SQL tab",
      async () => {
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
      },
      () => tabs.get().length < 2
    ),
    repeat(
      tabs,
      (entry) =>
        button(
          null,
          () => `Tab ${entry.get().id}${active.get() === entry.get().id ? " (active)" : ""}`,
          () => active.set(entry.peek().id)
        ),
      "Open SQL tabs"
    ),
  ]);

  return {
    type: "Card",
    tag: "section",
    attrs: { id: "desk-db-studio-panel" },
    className: "desk-card desk-db-studio-root",
    children: [
      { type: "Heading", props: { level: 3, text: "Visual Database Workbench" } },
      repeat(onlyActive, (entry) => renderTab(entry.peek()), "Active SQL workspace"),
    ],
  };
}
