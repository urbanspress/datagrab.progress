# GROXY-DESK-SPEC-140 — Groxy Desk v1.4.0

**True DbGate-Parity Connection Manager, Collapsible Rail & Full-Screen Spreadsheet Grid**

- **Status:** Approved & Frozen — Complete (**Stage 2 Pre-Implementation Cross-Layer Security & Contract Audit: PASS | Stage 3 Gates 1–4: PASS | Stage 4 Pre-Release Cross-Layer Security & Contract Sign-Off: PASS — Released as `v1.4.0`**)
- **Release tier:** Backwards-compatible minor feature (`ADR-027c`, `MINOR 1.x.0`)
- **Base Frozen Release:** `v1.3.1` (`7ffc8f2`, verified via `git ls-remote stable refs/heads/main` and `git describe --tags --abbrev=0 stable/main`, in-sync across `origin` and `stable`)

---

## 1. Stage 1 Scope, DbGate Comparison Remediation & Frozen Upstream Guarantees

### 1.1 Four Database Studio Gaps Identified Against DbGate Community
Side-by-side review of `Groxy Desk v1.3.1` against **DbGate Community** identified four specific UI/UX gaps in Database Studio that `v1.4.0` resolves cleanly without breaking any frozen `v1.x` contract:

1. **Raw Connection/SSH Form Inputs Cluttering `#desk-db-sidebar`:**
   - Replace all raw database and SSH tunnel input fields inside `#desk-db-sidebar` with a compact **`CONNECTIONS`** manager (`+ New Connection` icon button `#db-new-conn-btn`, saved connection list `#desk-db-connections-list` with **Edit**, **Disconnect**, and **Delete** actions) backed by non-secret `localStorage` metadata and the encrypted state-directory endpoint `GET/POST/DELETE /api/db/connections`.
   - **1 click on a saved connection** automatically opens its pinned SSH tunnel (`POST /api/db/tunnels/open` when remote) and loads its schema (`POST /api/db/schema`).
   - Move all connection fields (`Connection name`, `Engine`, `Server`, `Port`, `User`, `Password`, `Default database`, `PostgreSQL schema`, `SQLite path`, `Gashboard bridge bearer`, `SSH tunnel ID` on the **`General`** tab; `SSH host`, `SSH port`, `SSH user`, `Private key path`, `known_hosts path`, `SHA256 fingerprint`, `SSH password / passphrase`, `Remote port` on the **`SSH Tunnel`** tab) into a crisp `<dialog id="desk-db-conn-dialog">` modal with **`Save`**, **`Connect`**, and **`Close`** buttons, preserving every existing input DOM ID inside the dialog.

2. **Non-Collapsible Left Sidebar & Cramped Schema Tree:**
   - Clicking the active left-rail icon (`#desk-tab-database`, `#desk-tab-code`) toggles the studio's left sidebar (`#desk-db-sidebar` / `.desk-code-left-rail`) open or collapsed so users can maximize center workspace width anytime.
   - Dedicate the lower ~75% of `#desk-db-sidebar` exclusively to the filterable schema tree (`#desk-db-object-filter` with placeholder `Search in tables, views...` -> schema namespace header `public (N)` / `main (N)` -> clickable `Tables`, `Views`, and `Triggers` rows with row-count badges). Clicking any table immediately loads its rows into the center spreadsheet grid (`Data` view).

3. **Stacked Vertical Form Cards Above/Below the Center Grid:**
   - Eliminate all vertically stacked form boxes (`Entity-Relationship Diagram` cards, `Export CSV/XLSX/JSON/SQL` accordion card, `Cross-Environment Schema Diff` card) from the default center view.
   - Give `#desk-db-center` a clean DbGate sub-tab bar (**`Data`** | **`Structure`** | **`SQL Query`** | **`ER Diagram`** | **`Export & Diff`**) + compact single-line toolbar (`Refresh`, `Inspect Schema`, `Structure`, `SQL`, `Export`, `Undo Snapshot`, `Write SQL / Snapshot Guard`, `Inspector`).
   - Make **`Data`** the default edge-to-edge full-height spreadsheet grid (`<table class="desk-grid-table">`) featuring:
     - Sticky row-number gutter (`1, 2, 3...`),
     - Two-row sticky `<thead>` showing column name + data type (`varchar(128)`, `text`, `bigint`, `DECIMAL(10,2)`) in row 1 and **per-column live filter inputs** in row 2,
     - Crisp `1px` spreadsheet cell borders, `28px` compact row height, alternating row shading, `tabular-nums`, and click / double-click cell inspection (`ADR-025c`).
   - Render **`Structure`**, **`SQL Query`**, **`ER Diagram`**, and **`Export & Diff`** exclusively inside their own dedicated sub-tabs (`[hidden]` when inactive)—never stacked above or below the data grid.

4. **Always-Open Right Inspector Drawer Reducing Grid Width:**
   - Collapse `#desk-db-inspector` by default (`0px` / `.is-collapsed` / `data-collapsed="true"`) so the spreadsheet grid fills the full workspace width by default, and slide it open automatically whenever a user clicks/double-clicks an editable cell (`ADR-025c`), clicks **`Write SQL / Snapshot Guard`** (`GROXY-SAFE-001`), clicks **`Undo Snapshot`**, reviews an import/migration, or toggles **`Inspector`**.

### 1.2 Frozen Upstream & Sibling Repository Guarantees
- **10 Frozen Sibling `v1.x` Repositories Untouched:** `GRAB`, `grex-js`, `grablab-cms`, `grex-builder`, `groxy-ai`, `grabman`, `grabtaculous`, `gashboard`, `grab-ide-icons`, and `plotific-press` remain 100% untouched (`git status --short` clean).
- **Vendored `Grex JS v1.0.0` Untouched:** `public/grex/grex.js` and `public/grex/grex.elements.json` remain byte-for-byte identical to upstream `grex-js v1.0.0`.
- **Unified `Groxy AI` Only:** Zero model-switching dropdowns anywhere in the IDE (`model_switching_allowed: false`).
- **Loopback-Only, Confinement & Safety Gates Preserved:** `127.0.0.1` bind (`DESK-E501`), constant-time `Bearer` + `X-CSRF-Token` (`DESK-E403`), `GRAB-E401` path confinement, `GRAB-E402` WebView2 isolation, `DESK-E405` pinned SSH `SHA256:...` verification, `DESK-E406` read-only SQL enforcement, `ADR-025c` exact base-10 wire strings, and `GROXY-SAFE-001` single-use `approval_token` + SHA-256 Pre-Mutation Snapshot recovery.

---

## 2. Encrypted Saved Connection Profiles (`GET / POST / DELETE /api/db/connections`)

### 2.1 State-Directory Storage & Encryption (`internal/dbstudio/connections.go`)
- Store saved connection profiles outside the user workspace inside `stateDir/db-connections.json` with `0o600` permissions (never inside the project workspace).
- Encrypt all secret fields (`db_password_enc`, `ssh_password_enc`, `bridge_bearer_enc`) using `security.ResolveMasterKey(stateDir)` + `security.EncryptAEAD` (`v1:<nonce_b64>:<ciphertext_b64>` AES-256-GCM) and register incoming secret values with `security.SecretRedactor`.
- **Zero-Secret API Responses (`ConnectionView`):** `GET /api/db/connections` and `POST /api/db/connections` MUST NEVER return plaintext `db_password`, `ssh_password`, or `bridge_bearer`. Responses return safe metadata plus boolean flags (`has_db_password`, `has_ssh_password`, `has_bridge_bearer`) and masked previews (`db_password_masked: "****"`).
- **Safe Update Semantics:** When updating an existing saved connection (`POST /api/db/connections` with an existing `id`), leaving `db_password` / `ssh_password` / `bridge_bearer` empty preserves the existing encrypted secret unless an explicit `clear_db_password` / `clear_ssh_password` / `clear_bridge_bearer` flag is `true`.
- **SSH Pin & Loopback Validation:** Validate `engine` (`sqlite`, `sqlite-bridge`, `postgres`, `mariadb`), safe `id` (`^[A-Za-z0-9][A-Za-z0-9_.-]{0,63}$`), and `pinned_fingerprint` (`security.NormalizeSSHFingerprint` -> `403 DESK-E405` if malformed when provided).
- **Server-Side Secret Resolution for 1-Click Connect:** `OpenTunnelRequest` and `TargetSpec` accept optional `connection_id` so the Go host daemon resolves decrypted secrets directly in memory when opening a pinned SSH tunnel or querying/mutating a saved profile without ever returning secrets to the browser DOM or `localStorage`.

### 2.2 Browser-Side Non-Secret Metadata Cache (`localStorage`)
- `public/desk/db-studio-panel.js` caches non-secret connection profile metadata (`id`, `name`, `engine`, `sqlite_path`, `db_host`, `db_port`, `db_user`, `db_name`, `schema_name`, `ssh_enabled`, `ssh_host`, `ssh_port`, `ssh_user`, `ssh_private_key_path`, `known_hosts_path`, `pinned_fingerprint`, `remote_db_port`) under `localStorage["groxy_desk_db_connections"]`, explicitly stripping `db_password`, `ssh_password`, and `bridge_bearer` before persistence.
- On workspace hydration (`hydrateWorkspace()`), `workbench.js` fetches `GET /api/db/connections` and reconciles the daemon's encrypted connection list with the UI store (`store.dbConnections`).

---

## 3. UI Architecture, DOM ID Preservation & Accessibility (`public/desk/`)

### 3.1 `<dialog id="desk-db-conn-dialog">` Tabbed Connection Modal
- Rendered via Pure `Grex JS v1.0.0` `Modal` (`<dialog id="desk-db-conn-dialog" aria-modal="true">`) containing `Tabs` (`General` | `SSH Tunnel`):
  - **`General` tab:** `#db-conn-name` (`Connection name`), `select[name="engine"]` (`Engine`: `sqlite`, `sqlite-bridge`, `postgres`, `mariadb`), `#db-server` (`Server`), `#db-port` (`Port`), `#db-user` (`User`), `#db-password` (`Password`), `#db-database` (`Default database`), `#db-schema` (`PostgreSQL schema`), `#db-sqlite-path` (`SQLite path relative to workspace or bridge root`), `#db-bridge-bearer` (`Gashboard bridge bearer`), `#db-tunnel-id` (`SSH tunnel ID`).
  - **`SSH Tunnel` tab:** `#db-ssh-host`, `#db-ssh-port`, `#db-ssh-user`, `#db-ssh-key`, `#db-ssh-known-hosts`, `#db-ssh-pin`, `#db-ssh-password`, `#db-ssh-target-port`, plus `Open pinned SSH tunnel` and `Close this SSH tunnel` buttons.
  - **Dialog footer actions:** **`Save`** (`#db-conn-save-btn`), **`Connect`** (`#db-conn-connect-btn` — saves if named, opens pinned SSH tunnel if remote, loads schema, closes modal), and **`Close`** (`#db-conn-close-btn`).

### 3.2 Collapsible Sidebars & Right Inspector
- **Activity Rail Toggle:** Clicking `#desk-tab-database` when already on `"database"` toggles `store.dbSidebarCollapsed`; clicking `#desk-tab-code` when already on `"code"` toggles `store.codeSidebarCollapsed`. Both update `data-collapsed` and CSS grid track sizing (`0px` when collapsed, `var(--desk-sidebar-w)` when open) while keeping DOM nodes mounted for accessibility and test selectors.
- **Right Inspector (`#desk-db-inspector`):** Starts collapsed (`inspectorOpen = signal(false)`, `0px` width, `is-collapsed`, `data-collapsed="true"`, `aria-hidden="true"`). Opens automatically on cell click/double-click (`chooseCell`), **`Write SQL / Snapshot Guard`** (`#db-toolbar-guard-btn`), **`Undo Snapshot`** (`#db-toolbar-undo-btn`), **`Inspector`** (`#db-toolbar-inspector-btn`), or any pending `GROXY-SAFE-001` plan. Includes a close button (`#db-inspector-close-btn`).

### 3.3 Center Sub-Tab Bar & Full-Height Spreadsheet Grid
- **Sub-Tabs (`#desk-db-subtabs`):** `Data` (`#db-subtab-data`), `Structure` (`#db-subtab-structure`), `SQL Query` (`#db-subtab-sql`), `ER Diagram` (`#db-subtab-er`), `Export & Diff` (`#db-subtab-export`).
- **Single-Line Toolbar (`#desk-db-toolbar`):** `Refresh` (`#db-rows-btn`), `Inspect schema` (`#db-schema-btn`), `Structure` (`#db-toolbar-structure-btn`), `SQL` (`#db-toolbar-sql-btn`), `Export` (`#db-toolbar-export-btn`), `ER Diagram` (`#db-toolbar-er-btn`), `Schema Diff` (`#db-toolbar-diff-btn`), `Undo Snapshot` (`#db-toolbar-undo-btn`), `Write SQL / Snapshot Guard` (`#db-toolbar-guard-btn`), `Inspector` (`#db-toolbar-inspector-btn`).
- **Spreadsheet Grid (`<table class="desk-grid-table">`):**
  - Sticky row-number gutter (`#`, `1, 2, 3...`),
  - Two-row sticky `<thead>`: column name + type metadata in row 1, per-column `<input type="search" aria-label="Filter ...">` live filter controls in row 2,
  - `1px solid var(--grex-border)` full cell borders, `28px` row height, alternating row shading, `tabular-nums`, and click / double-click cell editing (`ADR-025c`).

---

## 4. Stage 2 Pre-Implementation Cross-Layer Security & Contract Audit (PASS)

On 2026-11-21, the project owner authorized `Milestone v1.4.0` (`GROXY-DESK-SPEC-140.md`) for full implementation, verification, and synchronized release. A cross-layer inspection of `internal/dbstudio/`, `internal/server/v030_handlers.go`, `internal/server/server.go`, `internal/security/crypto.go`, `public/desk/db-studio-panel.js`, `public/desk/workbench.js`, `public/desk/desk.css`, `public/grex/grex.js`, `tests/helpers/dom-shim.js`, and existing test suites confirms `v1.4.0` requires zero changes to any frozen `v1.x` sibling repository or `public/grex/*`.

**Audit Verdict: `PASS`** with the following five binding Stage 3 guardrails:

1. **Zero Secret Leakage in `GET/POST /api/db/connections` & `localStorage` (`§2`):**
   - `ConnectionStore` MUST encrypt `db_password`, `ssh_password`, and `bridge_bearer` at rest in `stateDir/db-connections.json` using `security.EncryptAEAD` (`v1:...`) and NEVER return plaintext secrets in JSON responses or store them in `localStorage`.
   - `DELETE /api/db/connections` and `POST /api/db/connections` MUST run behind `s.authCSRF(...)` (`403 DESK-E403` without constant-time Bearer + `X-CSRF-Token`).

2. **100% Preservation of Existing DOM IDs & Test Contracts (`§§1.1, 3`):**
   - Preserve all 21 frozen `v1.2.0` DOM IDs, all 17 `v1.3.0` shell IDs, and every existing Database Studio ID (`#db-sqlite-path`, `#db-bridge-bearer`, `#db-tunnel-id`, `#db-database`, `#db-schema`, `#db-user`, `#db-password`, `#db-ssh-host`, `#db-ssh-port`, `#db-ssh-user`, `#db-ssh-known-hosts`, `#db-ssh-pin`, `#db-ssh-key`, `#db-ssh-password`, `#db-ssh-target-port`, `#db-ro-sql`, `#db-table-name`, `#db-page-num`, `#db-limit-num`, `#db-query-btn`, `#db-rows-btn`, `#db-schema-btn`, `#db-export-path`, `#db-diagram-path`, `#db-import-path`, `#db-compare-source`, `#db-cell-value`, `#db-cell-type`, `#db-mutate-sql`, `#db-mutate-plan-btn`, `#db-mutate-apply-btn`).
   - Keep `main.books` / `priced_books` / `books_audit_trig` and `Tables (N)` / `Views (N)` / `Triggers (N)` present cleanly, and render `(${cols} cols)` when `row_count` is omitted (or include both row-count badge and column count) so existing schema-tree and filter assertions pass.

3. **Sequential Heading Hierarchy (`GREX-A11Y-004`) Inside `<dialog>` & Hidden Sub-Tabs (`§3.1`):**
   - `Grex.Modal` renders an `<h2>` if `props.title` is passed; to avoid introducing an `h2` -> `h4` skip in `auditDom(root)`, either omit `props.title` and render a `level: 4` `<Heading>` inside `#desk-db-conn-dialog` (with `aria-labelledby`), or place `#desk-db-conn-dialog` after the `<h4>` sections with sequential headings. Using `level: 4` + `aria-labelledby` inside `#desk-db-conn-dialog` guarantees zero heading-level skips in `auditDom`.
   - Every per-column filter `<input>` inside the spreadsheet `<thead>` MUST include `aria-label` (`GREX-A11Y-003`).

4. **Pure `Grex JS v1.0.0` & Light/Dark WCAG AA (`>= 4.5:1`) Conformance (`§§1.2, 3`):**
   - Zero `<div>`, zero `innerHTML`/`outerHTML`/`insertAdjacentHTML`, zero `eval()`/`new Function()`, zero inline `on*=` attributes, zero unsafe URLs, and zero `<select>` elements inside `#desk-groxy-sidebar` or `#desk-author-panel`.
   - Add `showModal()` and `close()` methods to `ShimElement` in `tests/helpers/dom-shim.js` so `<dialog id="desk-db-conn-dialog">` exercises real `showModal()`/`close()` lifecycle in Node unit tests.

5. **Activity Rail `role="tablist"` & WebView2 Fragment Allowlist (`GRAB-E402`):**
   - Keep `nav[role="tablist"]` on `#desk-activity-rail` first in DOM order so keyboard navigation (`ArrowRight`/`ArrowLeft`/`Home`/`End` across `database`, `browser`, `code`) and `browser_policy.go` fragment allowlisting (`#studio=database|browser|code`) remain 100% intact.

---

## 5. Acceptance Gates (Stage 3 Verification Plan)

| Gate | Required Evidence |
|---|---|
| **1. Go Host & Launcher** | `CGO_ENABLED=0` `go build ./...` & `go vet ./...` (Windows & Linux amd64), `go test -count=1 ./...` (including new `GET/POST/DELETE /api/db/connections` AEAD encryption & auth tests), `govulncheck ./...` (`0` vulnerabilities), `go mod verify`, and Windows launcher `--smoke-test` pass `100%`. |
| **2. GRAB, Grex & `v1.4.0` DbGate-Parity UI** | `grab check` + `grab format` parity on `src/desk.grab` and `views/workbench.grab.html`; `node --test tests/*.test.js` `100%` pass including new `v1.4.0` tests (saved connections + `localStorage` non-secret persistence + `<dialog id="desk-db-conn-dialog">` General/SSH tabs + collapsible rail/sidebar + 2-row sticky spreadsheet grid with row gutter, types, and per-column live filters + sub-tab switcher + collapsed-by-default `#desk-db-inspector` auto-open on cell edit / Safety Guard). |
| **3. Data Fidelity & Recovery** | Exact `ADR-025c` `"0.00"` and `>2^53` integer round trips, inline cell editing -> `GROXY-SAFE-001` plan/approve/rollback, CSV/XLSX/JSON/SQL import/export, ER diagram, and schema diff pass `100%`. |
| **4. Cross-Layer Windows E2E** | `TestNativeWebView2Isolation`, `python tests/integration_e2e.py`, `python tests/radar_live_e2e.py`, `python tests/network_dump_recovery.py`, and clean `git status --short` across all 10 sibling `v1.x` repositories pass `100%`. |

---

## 6. Stage 3 Implementation & Verification Evidence (PASS)

- **Encrypted Saved Connection Profiles & Tabbed Connection Modal (`internal/dbstudio/connections.go`, `internal/server/v030_handlers.go`, `internal/server/server.go`, `public/desk/api-client.js`, `public/desk/db-studio-panel.js`):**
  - Implemented `ConnectionStore` persisting saved database/SSH profiles to `stateDir/db-connections.json` (`0o600`) with `AES-256-GCM` (`v1:...`) secret encryption, zero plaintext secrets in API responses, server-side `connection_id` secret resolution across `OpenTunnelRequest` and `TargetSpec`, and `GET/POST/DELETE /api/db/connections` behind `authCSRF`.
  - Replaced raw sidebar inputs with a compact **`CONNECTIONS`** manager (`+ New Connection`, 1-click connect opening pinned SSH tunnel + schema load, Edit, Disconnect, Delete) + non-secret `localStorage` cache + `<dialog id="desk-db-conn-dialog">` (`General` and `SSH Tunnel` tabs preserving all existing input DOM IDs).
- **Collapsible Rail Sidebars, Full-Screen Spreadsheet Grid & Collapsed Inspector (`public/desk/workbench.js`, `public/desk/db-studio-panel.js`, `public/desk/desk.css`):**
  - Clicking active left-rail icons (`#desk-tab-database`, `#desk-tab-code`) toggles the studio's left sidebar (`data-collapsed="true|false"`).
  - Replaced stacked center form cards with DbGate sub-tabs (`Data` | `Structure` | `SQL Query` | `ER Diagram` | `Export & Diff`), single-line toolbar, edge-to-edge `.desk-grid-table` (sticky row-number gutter, two-row sticky headers with column types + per-column live filter inputs, `1px` borders, `28px` rows, alternating shading, `tabular-nums`, click/double-click cell edit), and collapsed-by-default `#desk-db-inspector` auto-opening on cell edit or `Write SQL / Snapshot Guard`.
- **Verification Gates 1–4 (`100%` Pass):**
  - Gate 1: `CGO_ENABLED=0` `go build ./...` & `go vet ./...` (Windows amd64 & Linux amd64), `go mod verify`, `govulncheck ./...` (`0` vulnerabilities), `go test -count=1 ./...` (including `TestV140DBConnectionsEncryptedProfilesAndAuth`).
  - Gate 2: `grab check src/desk.grab views/workbench.grab.html`, canonical `grab format` parity, and `node --test tests/*.test.js` (`15/15` pass including `tests/unit_v140_dbgate.test.js`, `divCount === 0`, `innerHTMLWriteCount === 0`, `auditDom()` `0` issues, Light & Dark WCAG AA `>= 4.5:1`, `ADR-025c`).
  - Gate 3 & Gate 4: `GROXY_DESK_TEST_WEBVIEW2=1` `TestNativeWebView2Isolation`, `python tests/integration_e2e.py`, `python tests/radar_live_e2e.py`, `python tests/network_dump_recovery.py`, and clean `git status --short` across all 10 sibling `v1.x` repositories.

---

## 7. Stage 4 Pre-Release Cross-Layer Security & Contract Sign-Off (PASS)

- **Sign-Off Status:** `PASS` (`Approved & Frozen — Complete` on 2026-11-21).
- **Security & Contract Verification:** Verified `GET/POST/DELETE /api/db/connections` `authCSRF` enforcement (`403 DESK-E403`), `AES-256-GCM` (`v1:...`) at-rest encryption in `stateDir/db-connections.json` with zero plaintext secrets in HTTP responses or `localStorage`, `DESK-E405` pinned SSH fingerprint checks, byte-for-byte unmodified `public/grex/grex.js` + `public/grex/grex.elements.json`, `100%` preservation of all frozen DOM IDs, `auditDom()` zero issues, and clean `git status --short` across all 10 sibling `v1.x` repositories.
- **Release Packaging:** Verified `Groxy-Desk-Setup-1.4.0.exe`, `Groxy-Desk-v1.4.0-Guide.pdf`, `groxy-desk-v1.4.0-bundle.zip`, `groxy-desk-windows-amd64.exe`, `groxy-deskd-windows-amd64.exe`, `groxy-deskd-linux-amd64`, and `dist/SHA256SUMS.txt` digest parity.
