import assert from "node:assert/strict";
import test from "node:test";

globalThis.__GROXY_DESK_NO_AUTO_MOUNT__ = true;

import { auditDom } from "../public/grex/grex.js";
import { mountGroxyDeskWorkbench } from "../public/desk/workbench.js";
import { createDomShim } from "./helpers/dom-shim.js";

function installMockLocalStorage() {
  const map = new Map();
  const prev = globalThis.localStorage;
  globalThis.localStorage = {
    getItem: (k) => (map.has(String(k)) ? map.get(String(k)) : null),
    setItem: (k, v) => map.set(String(k), String(v)),
    removeItem: (k) => map.delete(String(k)),
    clear: () => map.clear(),
  };
  return {
    map,
    restore: () => {
      globalThis.localStorage = prev;
    },
  };
}

function setControlValue(el, val) {
  const ctrl = el && el.localName === "label" ? el.querySelector("input, textarea") : el;
  ctrl.value = val;
  return ctrl.dispatchEvent({ type: "input", target: ctrl });
}

test("v1.4.0 DbGate-parity connection manager, tabbed modal, collapsible sidebar, spreadsheet grid, and collapsed inspector", async () => {
  const storageHook = installMockLocalStorage();
  try {
    const shim = createDomShim("dark");
    const root = shim.document.createElement("section");
    root.setAttribute("id", "desk-root");
    shim.document.body.appendChild(root);

    let savedConnections = [];
    const calls = {
      tunnelOpen: [],
      tunnelClose: [],
      schema: [],
      rows: [],
      deletedIds: [],
    };

    const mockApi = {
      hasSession: () => true,
      getTheme: async () => ({ theme: "dark" }),
      workspaceTree: async () => ({ entries: [] }),
      listWorkers: async () => ({ workers: [] }),
      groxySnapshots: async () => ({ snapshots: [] }),
      mcpTools: async () => ({ tools: [] }),
      studioDesignSystem: async () => ({ style_packs: [], blueprints: [] }),
      fleetNodes: async () => ({ nodes: [] }),
      authorOllamaStatus: async () => ({ ok: true, running: false }),
      dbTunnels: async () => ({ tunnels: [] }),
      dbConnectionsList: async () => ({ ok: true, connections: savedConnections }),
      dbConnectionSave: async (payload) => {
        const view = {
          id: payload.id || "conn-pg-1",
          name: payload.name,
          engine: payload.engine,
          sqlite_path: payload.sqlite_path || "",
          db_host: payload.db_host || "127.0.0.1",
          db_port: payload.db_port || "5432",
          db_user: payload.db_user || "",
          db_name: payload.db_name || "",
          schema_name: payload.schema_name || "public",
          has_db_password: Boolean(payload.db_password),
          db_password_masked: payload.db_password ? "********" : "",
          has_bridge_bearer: Boolean(payload.bridge_bearer),
          ssh_enabled: Boolean(payload.ssh_enabled),
          ssh_host: payload.ssh_host || "",
          ssh_port: payload.ssh_port || "22",
          ssh_user: payload.ssh_user || "",
          ssh_private_key_path: payload.ssh_private_key_path || "",
          known_hosts_path: payload.known_hosts_path || "",
          pinned_fingerprint: payload.pinned_fingerprint || "",
          remote_db_port: payload.remote_db_port || "5432",
          has_ssh_password: Boolean(payload.ssh_password),
        };
        savedConnections = [...savedConnections.filter((c) => c.id !== view.id), view];
        return { ok: true, connection: view, connections: savedConnections };
      },
      dbConnectionDelete: async (id) => {
        calls.deletedIds.push(id);
        savedConnections = savedConnections.filter((c) => c.id !== id);
        return { ok: true, deleted: true, id, connections: savedConnections };
      },
      dbTunnelOpen: async (payload) => {
        calls.tunnelOpen.push(payload);
        return {
          ok: true,
          tunnel: {
            tunnel_id: "tun-v140-1",
            engine: payload.engine,
            ssh_host: payload.ssh_host,
            pinned_fingerprint: payload.pinned_fingerprint,
            local_bind_addr: "127.0.0.1:15432",
            remote_target: "127.0.0.1:5432",
          },
        };
      },
      dbTunnelClose: async (payload) => {
        calls.tunnelClose.push(payload);
        return { ok: true, closed: true };
      },
      dbSchema: async (target) => {
        calls.schema.push(target);
        return {
          ok: true,
          engine: target.engine,
          tables: [
            {
              schema: "public",
              name: "books",
              row_count: "2",
              columns: [
                { name: "id", type: "bigint", primary_key: true, nullable: false },
                { name: "title", type: "varchar(128)", primary_key: false, nullable: false },
                { name: "price", type: "DECIMAL(10,2)", primary_key: false, nullable: false },
              ],
              foreign_keys: [],
              indexes: ["books_pkey"],
            },
          ],
          views: [{ name: "active_books" }],
          triggers: [{ name: "books_updated_trig" }],
        };
      },
      dbRows: async (payload) => {
        calls.rows.push(payload);
        return {
          ok: true,
          table: payload.table,
          page: "1",
          limit: "25",
          total_pages: "1",
          total_rows: "2",
          columns: [
            { name: "id", type: "bigint" },
            { name: "title", type: "varchar(128)" },
            { name: "price", type: "DECIMAL(10,2)" },
          ],
          rows: [
            { id: "1", title: "Clockwork Harbor", price: "19.99" },
            { id: "2", title: "Lantern Foundry", price: "24.50" },
          ],
        };
      },
    };

    const mounted = mountGroxyDeskWorkbench(root, {
      document: shim.document,
      apiClient: mockApi,
      initialStudio: "database",
      autoHydrate: false,
    });
    await mounted.hydrateWorkspace();

    // 1. Sidebar has NO raw SSH/DB form inputs; they live inside <dialog id="desk-db-conn-dialog">
    const sidebar = root.querySelector("#desk-db-sidebar");
    const dialog = root.querySelector("#desk-db-conn-dialog");
    assert.ok(sidebar, "Missing #desk-db-sidebar");
    assert.ok(dialog, "Missing <dialog id='desk-db-conn-dialog'>");
    assert.equal(dialog.localName, "dialog");
    assert.equal(sidebar.querySelector("#db-ssh-host"), null);
    assert.equal(sidebar.querySelector("#db-password"), null);
    for (const preservedId of [
      "db-conn-name",
      "db-server",
      "db-port",
      "db-user",
      "db-password",
      "db-database",
      "db-schema",
      "db-sqlite-path",
      "db-bridge-bearer",
      "db-tunnel-id",
      "db-ssh-host",
      "db-ssh-port",
      "db-ssh-user",
      "db-ssh-key",
      "db-ssh-known-hosts",
      "db-ssh-pin",
      "db-ssh-password",
      "db-ssh-target-port",
    ]) {
      assert.ok(dialog.querySelector(`#${preservedId}`), `Missing #${preservedId} inside #desk-db-conn-dialog`);
    }

    // 2. Right inspector starts collapsed by default
    const inspector = root.querySelector("#desk-db-inspector");
    assert.equal(inspector.getAttribute("data-collapsed"), "true");
    assert.equal(inspector.getAttribute("aria-hidden"), "true");

    // 3. Open modal via + New Connection, fill General + SSH Tunnel, Save -> non-secret localStorage + list
    const newConnection = root.querySelector("#db-new-conn-btn");
    assert.equal(newConnection.getAttribute("aria-haspopup"), "dialog");
    assert.equal(newConnection.getAttribute("aria-controls"), dialog.id);
    assert.ok(newConnection.querySelector("svg path"));
    assert.ok(sidebar.querySelector(".desk-search-control svg"));
    assert.ok(sidebar.querySelector(".desk-search-control #desk-db-object-filter"));
    await newConnection.click();
    assert.equal(dialog.open, true);

    await setControlValue(dialog.querySelector("#db-conn-name"), "Prod Postgres");
    const engineSelect = dialog.querySelector('select[name="engine"]');
    engineSelect.value = "postgres";
    await engineSelect.dispatchEvent({ type: "change", target: engineSelect });
    await setControlValue(dialog.querySelector("#db-server"), "127.0.0.1");
    await setControlValue(dialog.querySelector("#db-port"), "5432");
    await setControlValue(dialog.querySelector("#db-user"), "app_user");
    await setControlValue(dialog.querySelector("#db-password"), "secret-db-pass-123");
    await setControlValue(dialog.querySelector("#db-database"), "grablab");
    await setControlValue(dialog.querySelector("#db-ssh-host"), "203.0.113.10");
    await setControlValue(dialog.querySelector("#db-ssh-user"), "deploy");
    await setControlValue(dialog.querySelector("#db-ssh-pin"), "SHA256:testPinnedFingerprintValue123456");
    await setControlValue(dialog.querySelector("#db-ssh-password"), "secret-ssh-pass-456");
    await setControlValue(dialog.querySelector("#db-ssh-target-port"), "5432");

    await dialog.querySelector("#db-conn-save-btn").click();
    await dialog.querySelector("#db-conn-close-btn").click();
    assert.equal(dialog.open, false);

    const cachedRaw = storageHook.map.get("groxy_desk_db_connections") || "";
    assert.equal(cachedRaw.includes("Prod Postgres"), true);
    assert.equal(cachedRaw.includes("secret-db-pass-123"), false, "localStorage must NEVER store db_password");
    assert.equal(cachedRaw.includes("secret-ssh-pass-456"), false, "localStorage must NEVER store ssh_password");

    // 4. 1-click saved connection opens pinned SSH tunnel and loads schema tree
    const connBtn = root.querySelector("#desk-db-connections-list .desk-conn-select-btn");
    assert.ok(connBtn, "Expected saved connection button in #desk-db-connections-list");
    const databaseRailIcon = root.querySelector("#desk-tab-database svg path");
    assert.ok(databaseRailIcon);
    assert.equal(connBtn.querySelector("svg path").getAttribute("d"), databaseRailIcon.getAttribute("d"));
    await connBtn.click();
    assert.equal(calls.tunnelOpen.length, 1);
    assert.equal(calls.schema.length, 1);
    assert.equal(sidebar.textContent.includes("public (3)"), true);
    assert.equal(sidebar.textContent.includes("books (2)"), true);

    const schemaPane = sidebar.querySelector(".desk-db-schema-pane");
    assert.ok(schemaPane.querySelector(".desk-db-connections-pane"));
    assert.equal(schemaPane.querySelector("details").hasAttribute("open"), false);
    assert.equal(root.querySelector("#desk-status-db").parentNode.parentNode.localName, "header");
    assert.equal(root.querySelector("#desk-status-bar").hasAttribute("hidden"), true);
    assert.equal(root.querySelector("#db-subtab-structure").hasAttribute("hidden"), true);
    assert.equal(root.querySelector("#db-rows-btn").getAttribute("aria-label"), "Refresh rows");
    assert.equal(root.querySelector("#db-schema-btn").getAttribute("aria-label"), "Refresh schema");

    // 5. Clicking a table in schema tree immediately loads rows into default Data spreadsheet grid
    const tableBtn = sidebar.querySelectorAll(".desk-tree-item").find((b) => b.textContent.includes("books"));
    assert.ok(tableBtn, "Expected clickable books table button");
    await tableBtn.click();
    assert.equal(calls.rows.length, 1);

    const dataSubpanel = root.querySelector("#db-subpanel-data");
    const structureSubpanel = root.querySelector("#db-subpanel-structure");
    const sqlSubpanel = root.querySelector("#db-subpanel-sql");
    const erSubpanel = root.querySelector("#db-subpanel-er");
    const exportSubpanel = root.querySelector("#db-subpanel-export");
    assert.equal(dataSubpanel.hasAttribute("hidden"), false);
    assert.equal(structureSubpanel.hasAttribute("hidden"), true);
    assert.equal(sqlSubpanel.hasAttribute("hidden"), true);
    assert.equal(erSubpanel.hasAttribute("hidden"), true);
    assert.equal(exportSubpanel.hasAttribute("hidden"), true);

    const gridTable = dataSubpanel.querySelector(".desk-grid-table");
    assert.ok(gridTable, "Expected .desk-grid-table inside #db-subpanel-data");
    assert.equal(gridTable.textContent.includes("varchar(128)"), true);
    assert.equal(gridTable.textContent.includes("DECIMAL(10,2)"), true);
    assert.equal(gridTable.textContent.includes("Clockwork Harbor"), true);
    assert.equal(gridTable.textContent.includes("Lantern Foundry"), true);

    // 6. Per-column live filter input filters rows in real time
    const titleFilter = gridTable
      .querySelectorAll("input")
      .find((el) => el.getAttribute("aria-label") === "Filter column title");
    assert.ok(titleFilter, "Expected per-column live filter input for title");
    await setControlValue(titleFilter, "Lantern");
    const tbodyText = gridTable.querySelector("tbody").textContent;
    assert.equal(tbodyText.includes("Lantern Foundry"), true);
    assert.equal(tbodyText.includes("Clockwork Harbor"), false);

    // Clear filter and click editable cell -> inspector slides open with exact ADR-025c value
    await setControlValue(titleFilter, "");
    const priceCellBtn = gridTable.querySelectorAll(".desk-grid-cell-btn").find((b) => b.textContent === "19.99");
    assert.ok(priceCellBtn, "Expected editable price cell button");
    await priceCellBtn.click();
    assert.equal(inspector.getAttribute("data-collapsed"), "false");
    assert.equal(root.querySelector("#db-cell-value input").value, "19.99");

    await root.querySelector("#db-inspector-close-btn").click();
    assert.equal(inspector.getAttribute("data-collapsed"), "true");

    // 7. Clicking Write SQL / Snapshot Guard opens inspector
    await root.querySelector("#db-save-btn").click();
    assert.equal(root.querySelector("#desk-db-inspector").hasAttribute("hidden"), false);
    await root.querySelector("#db-inspector-close-btn").click();
    assert.equal(root.querySelector("#desk-db-inspector").hasAttribute("hidden"), true);
    await root.querySelector("#db-toolbar-guard-btn").click();
    assert.equal(inspector.getAttribute("data-collapsed"), "false");

    // 8. Sub-view switching renders Structure, SQL Query, ER Diagram, Export & Diff in their own sub-tabs
    await root.querySelector("#db-subtab-er").click();
    assert.equal(dataSubpanel.hasAttribute("hidden"), true);
    assert.equal(erSubpanel.hasAttribute("hidden"), false);
    assert.equal(erSubpanel.textContent.includes("public.books"), true);

    await root.querySelector("#db-subtab-structure").click();
    assert.equal(structureSubpanel.hasAttribute("hidden"), false);
    assert.equal(structureSubpanel.textContent.includes("books_pkey") || structureSubpanel.textContent.includes("varchar(128)"), true);

    // 9. Clicking active left-rail icon toggles sidebar collapsed/open (database & code)
    assert.equal(sidebar.getAttribute("data-collapsed"), "false");
    await root.querySelector("#desk-tab-database").click();
    assert.equal(sidebar.getAttribute("data-collapsed"), "true");
    await root.querySelector("#desk-tab-database").click();
    assert.equal(sidebar.getAttribute("data-collapsed"), "false");

    await root.querySelector("#desk-tab-code").click();
    const codeRail = root.querySelector(".desk-code-left-rail");
    assert.equal(codeRail.getAttribute("data-collapsed"), "false");
    await root.querySelector("#desk-tab-code").click();
    assert.equal(codeRail.getAttribute("data-collapsed"), "true");
    await root.querySelector("#desk-tab-code").click();
    assert.equal(codeRail.getAttribute("data-collapsed"), "false");
    await root.querySelector("#desk-tab-database").click();

    // 10. Edit -> Connect in modal + Disconnect & Delete connection profile
    const editBtn = root
      .querySelectorAll("#desk-db-connections-list .desk-conn-action-btn")
      .find((b) => b.textContent === "Edit");
    assert.ok(editBtn, "Expected Edit button on saved connection");
    assert.ok(editBtn.querySelector("svg path"));
    assert.ok(root.querySelector("#db-toolbar-inspector-btn svg path"));
    assert.ok(root.querySelector("#db-toolbar-undo-btn svg path"));
    await editBtn.click();
    assert.equal(dialog.open, true);
    assert.equal(dialog.querySelector("#db-conn-name input").value, "Prod Postgres");
    await setControlValue(dialog.querySelector("#db-conn-name"), "Prod Postgres Edited");
    await dialog.querySelector("#db-conn-connect-btn").click();
    assert.equal(dialog.open, false);
    assert.equal(
      root.querySelector("#desk-db-connections-list").textContent.includes("Prod Postgres Edited"),
      true
    );

    const actionBtns = root.querySelectorAll("#desk-db-connections-list .desk-conn-action-btn");
    const disconnectBtn = actionBtns.find((b) => b.textContent === "Disconnect");
    const deleteBtn = actionBtns.find((b) => b.textContent === "Delete");
    await disconnectBtn.click();
    assert.equal(calls.tunnelClose.length, 1);
    await deleteBtn.click();
    assert.equal(calls.deletedIds.length, 1);

    // 11. Pure Grex JS invariants hold 100%
    assert.equal(root.querySelectorAll("*").filter((e) => e.localName === "div").length, 0);
    assert.equal(shim.innerHTMLWriteCount, 0);
    const domAudit = auditDom(root);
    assert.equal(domAudit.ok, true, `auditDom failed: ${JSON.stringify(domAudit.issues)}`);
    assert.equal(domAudit.issues.length, 0, `Unexpected auditDom warnings: ${JSON.stringify(domAudit.issues)}`);

    mounted.dispose();
  } finally {
    storageHook.restore();
  }
});
