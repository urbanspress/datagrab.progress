# GROXY-DESK-SPEC-120 — Groxy Desk v1.2.0

**DbGate-Parity 3-Tab Dedicated Workspace Studio & Visual Polish (`Database Studio` | `Browser & Radar` | `Code & Grex Studio`)**

- **Status:** Approved & Frozen — Complete (**Stage 4 Pre-Release Cross-Layer Security & Contract Sign-Off: PASS — Released `v1.2.0`**)
- **Release tier:** Backwards-compatible minor feature (`ADR-027c`)
- **Base Frozen Release:** `v1.1.0` (`9aa3ff4`, in-sync across `origin` and `stable`)

---

## 1. Stage 1 Scope, Compatibility & Frozen Upstream Guarantees

1. **Additive UI & Workspace Architecture Only:** `v1.2.0` reorganizes Groxy Desk's single-page vertical stack into a clean, full-viewport 3-tab studio (`Database Studio` | `Browser & Radar` | `Code & Grex Studio`) with `Urbanist` geometric typography and DbGate-parity 3-pane ergonomics.
2. **Frozen `v1.1.0` Backend & Peer Contracts Protected:** All existing `v1.0.0`, `v1.0.1`, and `v1.1.0` Go loopback daemon routes (`127.0.0.1:9490`), `grab mcp` bridge contracts, WebView2 isolation boundaries, PostgreSQL 18 / least-privilege MariaDB 11.8 write fences (`409 write_fence_unavailable`), `GROXY-SAFE-001` single-use `approval_token` + SHA-256 Pre-Mutation Snapshot rollbacks, and all 10 frozen `v1.x` peer repositories (`GRAB v1.1.0`, `grex-js v1.0.0`, `grablab-cms`, `grex-builder v1.0.0`, `groxy-ai`, `grabman v1.1.0`, `grabtaculous v1.1.1`, `gashboard v1.0.0`, `grab-ide-icons`, `plotific-press`) remain untouched and 100% backwards-compatible.
3. **Zero-`<div>` Pure `Grex JS v1.0.0` & Unified `Groxy AI`:** Every template, tabpanel, toolbar, tree, data grid, and inspector drawer MUST use strictly the 20 semantic elements from `grex.elements.json` and semantic HTML5 tags (`divCount === 0`, `innerHTMLWriteCount === 0`, zero `eval()` / `new Function()`, zero unsafe URLs). **Groxy AI** remains the sole permanent built-in agent identity with Automatic Adaptive Reasoning Routing and zero model-switching dropdowns.

---

## 2. Sticky Top Workspace Mode Switcher & `Urbanist` Typography

### 2.1 Sticky Header & Typography
- **Sticky Top Header (`.desk-header`):** Remains pinned to the top of the viewport (`position: sticky; top: 0; z-index: 20`) with a subtle border and surface contrast token (`--grex-surface` / `--grex-border`) across both Light and Dark themes.
- **`Urbanist` Geometric Typography:** Apply `Urbanist` as the primary geometric display and UI font family (`"Urbanist", "Segoe UI", system-ui, -apple-system, sans-serif`) for headings, tab labels, badges, and workspace chrome, keeping monospace (`"JetBrains Mono", "Cascadia Code", Consolas, monospace`) for code, SQL, exact-decimal cells, and SHA-256 digests. Bundle or resolve `Urbanist` locally/offline-safe with zero external unpinned runtime network dependencies and zero unsafe URL schemes.
- **Header Controls (Preserved + Tab Switcher):**
  - Brand heading (`Groxy Desk`) and version subtitle.
  - **3-Tab Workspace Mode Switcher (`role="tablist"`, `aria-label="Workspace Studio Modes"`):**
    1. `Database Studio` (`#desk-tab-database`, `aria-controls="desk-panel-database"`)
    2. `Browser & Radar` (`#desk-tab-browser`, `aria-controls="desk-panel-browser"`)
    3. `Code & Grex Studio` (`#desk-tab-code`, `aria-controls="desk-panel-code"`)
  - Authenticated loopback status badge (`127.0.0.1:9490 Authenticated` / `Waiting for Host Session`).
  - Light/Dark theme toggle (`#desk-theme-toggle`) preserving zero-FOUC startup and `>= 4.5:1` WCAG AA contrast.

### 2.2 Keyboard Navigation, Hash/Session Persistence & Full-Viewport Layout
- **WAI-ARIA Tablist Keyboard Model:**
  - Active tab has `aria-selected="true"` and `tabindex="0"`; inactive tabs have `aria-selected="false"` and `tabindex="-1"`.
  - `ArrowRight` / `ArrowLeft` move focus and activate the next/previous tab (wrapping at ends).
  - `Home` / `End` jump to the first (`Database Studio`) or last (`Code & Grex Studio`) tab.
  - Visible `:focus-visible` ring (`--grex-focus-ring`) on all tabs and controls.
- **URL Hash & Session Persistence:**
  - Active studio tab persists to `window.location.hash` (`#studio=database` | `#studio=browser` | `#studio=code`) and `sessionStorage` (`groxy_desk_active_studio`), defaulting cleanly to `code` (`Code & Grex Studio`) or the persisted choice on reload without losing shared `store` signals, active SSH tunnels, `Grisp` workers, or `Groxy AI` conversation state.
- **Full-Viewport Tabpanels (`role="tabpanel"`):**
  - Replace the cluttered single-page vertical stack with three full-viewport `role="tabpanel"` containers (`#desk-panel-database`, `#desk-panel-browser`, `#desk-panel-code`) sized to fill the remaining viewport height below the sticky header.
  - Keep all three tabpanels mounted in the DOM (toggling the semantic `hidden` attribute and `aria-hidden` state) so shared reactive signals, live preview effects, and existing `v1.1.0` element ID queries continue to work regardless of which tab is currently visible.

---

## 3. Tab 1 — `Database Studio` (True DbGate 3-Pane Layout)

Full-height 3-pane split workspace (`#desk-panel-database`, `aria-labelledby="desk-tab-database"`):

1. **Left Sidebar — Connections & Expandable Schema Object Tree:**
   - Saved and active connections list (`PostgreSQL`, `MariaDB`, `SQLite`, including `known_hosts` `SHA256:...`-pinned SSH loopback tunnels on `127.0.0.1:15432–15439` and capability-gated remote SQLite).
   - Expandable schema tree grouped by `Tables`, `Views`, and `Triggers` (with column/index metadata), using accessible semantic `<details>`/`<summary>` or tree buttons (`divCount === 0`).
2. **Center Workspace — Action Toolbar, Multi-Tab SQL Editor & Spreadsheet Data Grid:**
   - **Focused Action Toolbar:** `Run Query`, `Export CSV/XLSX/JSON/SQL` (plus import entry points), `ER Diagram`, `Schema Diff`, and `Undo Snapshot`.
   - **Top Split — Multi-Tab Autocomplete SQL Editor:** Dialect-aware (`PostgreSQL`, `MariaDB`, `SQLite`), schema-aware SQL autocomplete across open query tabs with bounded execution and cancellation.
   - **Bottom Split — Clean Spreadsheet-Style Data Grid:** Paginated, keyboard-navigable result grid (`<= 100` rows/page) with inline cell editing, null vs. empty distinction, and exact `ADR-025c` base-10 wire string rendering (`"0.00"`, large integers above `Number.MAX_SAFE_INTEGER`).
3. **Right Inspector — Exact-Decimal Editor, Snapshot Guard & Approval Drawer:**
   - Selected row/cell `ADR-025c` exact-decimal editor with before/after diff preview and freshness check.
   - Pre-mutation SHA-256 snapshot guard status (including fail-closed `409 write_fence_unavailable` notice if a MariaDB target has global `*.*` write privileges).
   - `GROXY-SAFE-001` target-bound `approval_token` confirmation and 1-click rollback drawer.

---

## 4. Tab 2 — `Browser & Radar` (Full-Height 50/50 Split Pane)

Full-height 50/50 split workspace (`#desk-panel-browser`, `aria-labelledby="desk-tab-browser"`):

1. **Left Pane — Isolated WebView2 Allowlisted Browser:**
   - Dedicated untrusted WebView2 research profile and controller (zero privileged host objects, zero bridge/CMS/DB credentials exposed to page content).
   - Live origin lock badge (`GRAB-E402` scheme/host/port verification), navigation bar, and explicit user-confirmed text selection / `BRAND-DNA.md` prefill trigger (users retain full control of passwords, CAPTCHAs, and form submission).
2. **Right Pane — Bestseller Radar, `BRAND-DNA.md` Prefill & `GrabLab CMS` Draft Handoff:**
   - Bestseller Radar niche explorer (genre selector, optional price tier, live vs. offline fallback indicator, exact decimal unit economics, and source freshness).
   - `BRAND-DNA.md` allowlisted prefill preview.
   - Desk-bound receipt staging (`rtok_res_<hex>`, `groxy-csrf`) and separately approved `GrabLab CMS` (`v1.4.0`/`v1.5.0`) `create_book_draft` handoff panel (`status_enforced: "draft"`).

---

## 5. Tab 3 — `Code & Grex Studio` (Clean 3-Pane Vibe-Coding Studio)

Full-height 3-pane split workspace (`#desk-panel-code`, `aria-labelledby="desk-tab-code"`):

1. **Left Pane — Workspace File Explorer, `grabman`, `Grisp` & Step 5 Tools:**
   - `GRAB-E401`-confined workspace file tree (`workspace-panel.js`) and `grabman v1.0.0` project diagnostics (`init`, `doctor`, `solve --verify`).
   - Organized collapsible sections (`<details>`/`<summary>`) for Local `Grisp` Multi-Worker Supervisor (`supervisor-panel.js`), Step 5 Fleet (`fleet-panel.js`), Namecheap DNS + Auto-HTTPS (`dns-panel.js`), and 1-Click Local Author Mode (`author-panel.js`) so secondary tools stay one click away without vertical clutter.
2. **Center Pane — Active `.grab`/`.html` Code Workbench & Responsive `Grex` Preview Canvas:**
   - Active file preview/editor (`#desk-editor-card`, `#desk-active-file-meta`, `#desk-run-grab-check-btn`, `#desk-editor-code`, `#desk-check-diagnostics`).
   - Embedded split-screen `Grex` visual preview canvas (`studio-panel.js`, `#desk-preview-surface`, responsive viewport toggles, 8 Design Style Packs × 5 Blueprints, `POST /api/mcp/layout`, and 1-click `Ctrl+Z` `applyAiLayout` undo stack).
3. **Right Pane — Unified `Groxy AI` Co-Pilot, 22-Rule Scorecard & Safety Snapshots:**
   - Unified `Groxy AI` Adaptive Reasoning co-pilot sidebar (`groxy-agent.js`, zero model-switching dropdowns).
   - 22-rule 4-pillar SEO/WCAG/DOM/Link audit scorecard (`scorecard-panel.js`, `A+`–`F` / `0–100`, 🚀 `"Let Groxy Boost This"` remediation diffs).
   - Pre-Mutation Snapshots & 1-click rollback list (`#desk-snapshots-card`).

---

## 6. Zero Regression & Pure `Grex JS v1.0.0` Conformance

1. **100% Preserved DOM IDs & API Bindings:** Every existing `v1.1.0` DOM element ID (`#desk-root`, `#desk-workspace-stage`, `#desk-theme-toggle`, `#desk-main-stage`, `#desk-editor-card`, `#desk-active-file-meta`, `#desk-run-grab-check-btn`, `#desk-editor-code`, `#desk-check-diagnostics`, `#desk-preview-surface`, `#desk-snapshots-card`, and all panel IDs inside `db-studio-panel.js`, `radar-panel.js`, `studio-panel.js`, `scorecard-panel.js`, `workspace-panel.js`, `supervisor-panel.js`, `fleet-panel.js`, `dns-panel.js`, `author-panel.js`, and `groxy-agent.js`), ARIA attributes, and `api-client.js` / `GrexPreviewBridge` contracts MUST remain intact.
2. **Strict DOM & Security Hygiene:**
   - `divCount === 0`, `innerHTMLWriteCount === 0`, zero `outerHTML` / `insertAdjacentHTML`, zero `eval()` / `new Function()`, zero inline `on*=` attributes, and zero unsafe URL schemes (`javascript:`, `vbscript:`, `data:text/html:`).
   - Light and Dark themes (`data-grex-theme="light|dark"`) MUST pass `computeContrastRatio() >= 4.5:1` (normal text) and `>= 3.0:1` (large text), `100%` `auditAccessibility()`, and zero FOUC on startup.

---

## 7. Acceptance Gates (Stage 3 Verification Plan)

| Gate | Required Evidence |
|---|---|
| **1. Go Host & Launcher** | `go build ./...`, `go vet ./...`, `go test -count=1 ./...`, `govulncheck ./...` (`0` vulnerabilities), `go mod verify`, and Windows WebView2 launcher `--smoke-test` pass `100%`. |
| **2. GRAB, Grex & 3-Tab Studio Conformance** | `grab check` and `grab format` byte parity on `src/desk.grab` and `views/workbench.grab.html`; `node --test tests/*.test.js` `100%` pass verifying `divCount === 0`, `innerHTMLWriteCount === 0`, zero `eval()`, Light/Dark WCAG AA `>= 4.5:1`, `ADR-025c` numeric fidelity, `role="tablist"` keyboard navigation (`ArrowLeft`/`ArrowRight`/`Home`/`End`), URL hash/session persistence, and 100% preservation of all `v1.1.0` DOM IDs. |
| **3. Data Fidelity & Recovery** | Exact `"0.00"` and large-integer round trips, inline cell editing, CSV/XLSX/JSON/SQL import/export, ER diagram + keyboard-accessible relationship view, schema diff, and `GROXY-SAFE-001` snapshot/rollback continue passing `100%`. |
| **4. Cross-Layer Windows E2E** | Full `python tests/integration_e2e.py`, `python tests/radar_live_e2e.py`, `TestNativeWebView2Isolation`, and pinned DB checks pass `100%` with all 10 sibling `v1.x` repositories clean. |

---

## 8. Stage 2 Pre-Implementation Cross-Layer Security & Contract Audit (PASS)

On 2026-11-21, the project owner approved `GROXY-DESK-SPEC-120.md` as written. Cross-layer inspection of `views/workbench.grab.html`, `src/desk.grab`, `public/desk/*.js`, `public/desk/desk.css`, `public/grex/grex.js`, `internal/server/server.go`, `internal/dbstudio/`, `cmd/groxy-desk/browser_policy.go`, `cmd/groxy-desk/webview_windows.go`, and `tests/*.test.js` confirms `v1.2.0` can be implemented strictly inside `C:\Development\groxy-desk` with zero modifications to any frozen `v1.1.0` backend route contract or any of the 10 frozen sibling `v1.x` repositories. **Result: `PASS`** with the following binding Stage 3 guardrails:

1. **Sticky 3-Tab Switcher, Keep-Mounted Panels & WebView2 Fragment Policy (`§§2, 4, 6`):**
   - Render the sticky `.desk-header` switcher via Pure `Grex JS v1.0.0` semantic `<nav role="tablist" aria-label="Workspace Studio Modes">` + `<button type="button" role="tab" id="desk-tab-database|browser|code" aria-controls="desk-panel-database|browser|code" aria-selected="true|false" tabindex="0|-1">` with wrapping `ArrowLeft`/`ArrowRight` and `Home`/`End` keyboard navigation, keeping all three `<section role="tabpanel" id="desk-panel-database|browser|code" tabindex="0">` containers mounted in the DOM (toggling `hidden` + `aria-hidden`, backed by `.desk-tabpanel[hidden] { display: none; }` in `public/desk/desk.css` so grid/flex layout never overrides `[hidden]`).
   - Strictly allowlist-validate `window.location.hash` (`#studio=database` | `#studio=browser` | `#studio=code`) and `sessionStorage` (`groxy_desk_active_studio`), defaulting to `"code"`, guarding `window`/`sessionStorage`/`focus()` in headless Node shims, and NEVER storing bearer, CSRF, approval, or DB credentials in `sessionStorage` or URLs.
   - **Binding WebView2 IPC Fix (`cmd/groxy-desk/browser_policy.go`):** `trustedWorkbenchSource(raw, base)` currently checks `raw == base + "/"`. Because Chromium `ICoreWebView2::get_Source()` includes the URL fragment (`http://127.0.0.1:<port>/#studio=browser`), Stage 3 MUST update `trustedWorkbenchSource` to validate `origin == base && u.Path == "/" && u.RawQuery == "" && u.User == nil` while allowing only the safe fragment allowlist (`""`, `"studio=database"`, `"studio=browser"`, `"studio=code"`, `"desk-workspace-stage"`), preserving `TestBrowserPaneExactOriginAndWorkbenchMessages` and `GRAB-E402` isolation.
2. **Offline-Safe `Urbanist` Typography & Host CSP/MIME Parity (`§2.1`):**
   - `internal/server/server.go` enforces `Content-Security-Policy: default-src 'self'; script-src 'self'; style-src 'self'; ...` and `X-Content-Type-Options: nosniff`. Stage 3 MUST NOT reference external font CDNs (`fonts.googleapis.com` / `fonts.gstatic.com`) or `data:` script/HTML URLs. Bundle `Urbanist` `.woff2` font asset(s) locally under `public/desk/fonts/` (embedded automatically by `assets.go` `//go:embed views public`) and add explicit `case ".woff2": ctype = "font/woff2"` (and `case ".woff": ctype = "font/woff"`) in `handleStaticPublic` in `internal/server/server.go`, with fallback stack `"Urbanist", "Segoe UI", system-ui, -apple-system, sans-serif` and monospace `"JetBrains Mono", "Cascadia Code", Consolas, monospace`.
3. **WAI-ARIA Heading Hierarchy (`GREX-A11Y-004`) & Single `<main>` Landmark (`§§3–6`):**
   - `public/grex/grex.js` `auditDom(root)` enforces strict sequential heading levels (`h1` -> `h2` -> `h3` -> `h4` -> `h5` -> `h6`) across the entire mounted DOM including inactive tabpanels. Each tabpanel (`#desk-panel-database`, `#desk-panel-browser`, `#desk-panel-code`) MUST open with an `<h2>` heading before its `<h3>` cards, and `db-studio-panel.js` MUST NOT skip from `<h3>` directly to `<h5>` in the ER diagram (introduce an `<h4>` section heading or use `<h4>` for entity titles).
   - Keep a single always-visible `<main id="desk-workspace-stage">` landmark wrapping the three tabpanels and render `#desk-main-stage` inside Tab 3 as a semantic `<section id="desk-main-stage">`.
4. **DbGate 3-Pane `Database Studio` & Preserved `v1.1.0` IDs/Contracts (`§§3, 6`):**
   - Keep `#desk-db-studio-panel` as the root container of Tab 1's 3-pane layout so existing queries (`root.querySelector("#desk-db-studio-panel").textContent`) and all `db-studio-panel.js` IDs, `ADR-025c` exact base-10 wire strings, PostgreSQL 18 / least-privilege MariaDB 11.8 write fences (`409 write_fence_unavailable`), and `GROXY-SAFE-001` approval/rollback flows remain 100% intact.
   - Populate the `Tables` / `Views` / `Triggers` tree from `/api/db/schema` (`res.tables` + optional `res.views || []` / `res.triggers || []`) without breaking existing `v1.1.0` `/api/db/schema` response contracts, and replace the undefined `--desk-border` custom property in `public/desk/desk.css` (lines 376, 395) with `var(--grex-border)`.
5. **50/50 `Browser & Radar`, 3-Pane `Code & Grex Studio`, & Zero-Regression Guarantees (`§§4–6`):**
   - Preserve 100% of `v1.1.0` DOM IDs, ARIA attributes, `GrexPreviewBridge` origin allowlists, `#desk-groxy-sidebar` / `#desk-author-panel` zero-`<select>` invariant (zero model-switching dropdowns), `divCount === 0`, `innerHTMLWriteCount === 0`, zero `eval()` / `new Function()`, zero unsafe URLs, and Light/Dark WCAG AA (`>= 4.5:1`).
   - All 10 sibling `v1.x` repositories (`GRAB`, `grex-js`, `grablab-cms`, `grex-builder`, `groxy-ai`, `grabman`, `grabtaculous`, `gashboard`, `grab-ide-icons`, `plotific-press`) verified clean (`git status --short`) and untouched.

---

## 9. Stage 4 Pre-Release Cross-Layer Security & Contract Sign-Off (PASS)

- **Sign-Off Status & Baseline Commit:** `PASS` (`Approved & Frozen — Complete` on 2026-11-21; implementation verified at `ef9357d`).
- **Sticky 3-Tab Switcher, WAI-ARIA Conformance & WebView2 Fragment Allowlisting (`§§2, 6, 8.1, 8.3`):** Verified `trustedWorkbenchSource` in `cmd/groxy-desk/browser_policy.go` enforces exact loopback origin, `u.Path == "/"`, empty query, `u.User == nil`, and the strict 5-value fragment allowlist (`""`, `"studio=database"`, `"studio=browser"`, `"studio=code"`, `"desk-workspace-stage"`). Verified `<nav role="tablist">` wrapping `ArrowLeft`/`ArrowRight` + `Home`/`End` keyboard navigation, allowlisted `#studio=database|browser|code` URL hash and `sessionStorage` (`groxy_desk_active_studio`) persistence, single `<main id="desk-workspace-stage">` landmark, sequential `h1` -> `h2` -> `h3` -> `h4` -> `h5` heading hierarchy (`GREX-A11Y-004`), and 100% preserved `v1.1.0` DOM IDs across all 3 mounted `role="tabpanel"` containers.
- **Offline `Urbanist` Typography & CSP/MIME Enforcement (`§§2.1, 8.2`):** Verified locally bundled `Urbanist` (`400`/`500`/`600`/`700`) `.woff2` font files under `public/desk/fonts/` (embedded via `assets.go`), explicit `font/woff2` and `font/woff` MIME mappings in `handleStaticPublic` (`internal/server/server.go`), strict `Content-Security-Policy: default-src 'self'; script-src 'self'; style-src 'self'; ...` and `X-Content-Type-Options: nosniff` compliance with zero external CDN requests, and `--desk-border` -> `var(--grex-border)` remediation in `public/desk/desk.css`.
- **Pure `Grex JS v1.0.0`, `ADR-025c`, `GROXY-SAFE-001` & Cross-Layer E2E Parity (`§§3–7, 8.4, 8.5`):** Verified `grab check src/desk.grab views/workbench.grab.html`, canonical `grab format` byte parity, `13/13` Node DOM/WCAG AA tests (`divCount === 0`, `innerHTMLWriteCount === 0`, zero `eval()`, Light & Dark WCAG AA `>= 4.5:1`, zero `<select>` model pickers), `TestNativeWebView2Isolation`, `python tests/integration_e2e.py`, `python tests/radar_live_e2e.py`, and `python tests/network_dump_recovery.py` at `100%` pass.
- **Release Packaging & Cross-Repo Hygiene:** Verified `dist/SHA256SUMS.txt` byte-for-byte across all 6 packaged assets (`Groxy-Desk-Setup-1.2.0.exe`, `Groxy-Desk-v1.2.0-Guide.pdf`, `groxy-desk-v1.2.0-bundle.zip`, `groxy-desk-windows-amd64.exe`, `groxy-deskd-linux-amd64`, `groxy-deskd-windows-amd64.exe`) and clean `git status --short` across all 10 sibling `v1.x` repositories (`GRAB`, `grex-js`, `grablab-cms`, `grex-builder`, `groxy-ai`, `grabman`, `grabtaculous`, `gashboard`, `grab-ide-icons`, `plotific-press`).
