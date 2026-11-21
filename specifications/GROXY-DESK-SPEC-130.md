# GROXY-DESK-SPEC-130 — Groxy Desk v1.3.0

**VS Code / DbGate / Coolify High-Density Desktop IDE Shell**

- **Status:** Approved; Stage 3 Acceptance Complete (**Stage 2 Pre-Implementation Cross-Layer Security & Contract Audit: PASS | Stage 3 Gates 1–4: PASS**)
- **Release tier:** Backwards-compatible minor feature (`ADR-027c`, `MINOR 1.x.0`)
- **Base Frozen Release:** `v1.2.0` (`dbd957a`, verified via `git ls-remote stable refs/heads/main` and `git describe --tags --abbrev=0 stable/main`, in-sync across `origin` and `stable`)

---

## 1. Stage 1 Scope, Visual Audit Remediation & Frozen Upstream Guarantees

### 1.1 Why `v1.3.0` Replaces the `v1.2.0` Vertical-Card Layout
A visual inspection of `v1.2.0` (`desk1.png`–`desk5.png`) identified five desktop-ergonomics gaps compared to **VS Code**, **DbGate**, and **Coolify**:
1. **Oversized Vertical Cards & Low Canvas Density:** Floating cards with `14px–24px` padding, rounded card borders, and wide grid gaps consume over 40% of the viewport on whitespace and gutters instead of edge-to-edge structural panes separated by `1px solid var(--grex-border)` hairline dividers.
2. **Vertical Document Scrolling:** The outer document scrolled vertically, pushing operational tools (`Grisp` Multi-Worker Supervisor, Step 5 Fleet, Namecheap DNS Wizard, and Pre-Mutation Snapshots) below the fold. In `v1.3.0`, `html, body, #desk-root` are strictly locked to `100vh` (`max-height: 100vh; overflow: hidden`), and only inner pane containers scroll.
3. **Unstyled Native Browser Form Controls:** Inputs and `<select>` boxes rendered with spacious browser-default styling instead of compact `26px–28px` dark/light semantic IDE controls with monospace typography and `2px solid var(--grex-focus-ring)` focus indicators.
4. **Missing Top Menu/Command Bar & Vertical Activity Icon Rail:** `v1.2.0` placed bulky horizontal pill tabs in a tall header and lacked an application menu bar (`File`, `Edit`, `View`, `Database`, `Browser`, `Tools`, `Help`), a centered `Ctrl+K` command search bar (`#desk-command-search`), and a `48px` far-left vertical activity icon rail (`#desk-activity-rail`).
5. **Missing Pinned Bottom IDE Status Bar:** `v1.2.0` used a static text footer instead of a `24px` live bottom status bar (`#desk-status-bar`) surfacing `#desk-status-db`, `#desk-status-ssh`, `#desk-status-safety`, `#desk-status-grisp`, and `#desk-status-adr025c`.

```
Target v1.3.0 Desktop IDE Architecture (VS Code / DbGate / Coolify Density)
┌──────────────────────────────────────────────────────────────────────────────┐
│ File  Edit  View  Database  Browser  Tools  Help │ [Ctrl+K Search] │ 127.0.0.1:9490 [☾] │ ← 32px Menu & Command Bar
├────┬─────────────────────────┬────────────────────────────────┬──────────────┤
│[≡] │ 240px Left Sidebar      │ Center Workspace (Tabbed)      │ 280px Right  │ ← 100vh Locked Viewport
│[🗄]│ ├─ Connections          │ ├─ SQL Editor Tabs (26px)      │ ├─ ADR-025c  │   (Zero page scroll)
│[🌐]│ └─ Schema Tree + Filter │ ├─ Spreadsheet Grid (28px rows)│ │  Inspector │
│[⚡]│    ├─ Tables (36)       │ └─ Split Live Canvas           │ └─ Snapshots │
│[⚙] │    └─ Views / Triggers  │                                │              │
├────┴─────────────────────────┴────────────────────────────────┴──────────────┤
│ ● Connected: PostgreSQL 18 (127.0.0.1:15432) │ SHA256:... │ GROXY-SAFE-001 │ Grisp │ ADR-025c │ ← 24px Status Bar
└──────────────────────────────────────────────────────────────────────────────┘
```

### 1.2 Frozen Upstream & Backend Guarantees
- **Frozen `v1.2.0` Go Host Daemon Routes Untouched:** All existing `127.0.0.1:9490` loopback daemon routes, constant-time bearer + `X-CSRF-Token` checks (`403 DESK-E403`), `GRAB-E401` workspace confinement, `GRAB-E402` WebView2 isolation, PostgreSQL 18 / least-privilege MariaDB 11.8 recovery fences (`409 write_fence_unavailable`), and `GROXY-SAFE-001` single-use `approval_token` + SHA-256 Pre-Mutation Snapshot rollbacks remain 100% intact.
- **10 Frozen `v1.x` Sibling Repositories Untouched:** `GRAB v1.1.0`, `grex-js v1.0.0`, `grablab-cms`, `grex-builder v1.0.0`, `groxy-ai`, `grabman v1.1.0`, `grabtaculous v1.1.1`, `gashboard v1.0.0`, `grab-ide-icons`, and `plotific-press` remain strictly clean (`git status --short` empty).
- **Single Unified Default Agent (`Groxy AI` Only):** Zero model-switching dropdowns anywhere in the IDE (`model_switching_allowed: false`, zero `<select>` elements inside `#desk-groxy-sidebar` or `#desk-author-panel`).

---

## 2. High-Density Design Tokens & WCAG AA Palettes (`public/desk/desk.css` & `public/desk/theme.js`)

### 2.1 Dark Obsidian & Clean Light Token Palettes
Both palettes define the high-density `v1.3.0` tokens while maintaining backward-compatible aliases (`--grex-bg`, `--grex-surface-alt`, `--grex-muted`, `--grex-accent-text`, `--grex-code-bg`) so existing components and `verifyPaletteContrast()` pass `100%`:

| Token | Dark Obsidian (`data-grex-theme="dark"`) | Clean Light (`data-grex-theme="light"`) | Purpose & Verified WCAG AA Contrast |
|---|---|---|---|
| `--grex-base` (`--grex-bg`) | `#0f1115` | `#f8fafc` | Locked `100vh` workbench canvas & recessed input/code background |
| `--grex-surface` | `#181b20` | `#ffffff` | `32px` header, `48px` activity rail, sidebars, toolbars, `24px` status bar |
| `--grex-surface-hover` (`--grex-surface-alt`) | `#22262e` | `#f1f5f9` | Row/button hover surface & zebra-stripe alternate surface |
| `--grex-surface-active` | `#2b303a` | `#e2e8f0` | Active activity-rail button, selected tree node, or active tab surface |
| `--grex-border` | `#262a33` | `#cbd5e1` | `1px solid` hairline structural pane dividers and control borders |
| `--grex-text` | `#e6edf3` | `#0f172a` | Primary text (`15.99:1` Dark on base, `14.61:1` Dark on surface; `17.06:1` Light on base, `17.85:1` Light on surface) |
| `--grex-text-muted` (`--grex-muted`) | `#8b949e` | `#475569` | Secondary labels & metadata (`6.14:1` Dark on base, `5.61:1` Dark on surface, `4.93:1` Dark on hover; `7.58:1` Light on surface) |
| `--grex-accent` | `#10b981` | `#059669` | GRAB Emerald brand accent (`6.81:1` Dark on surface; `3.77:1 >= 3.0:1` Light UI component/active border contrast) |
| `--grex-accent-hover` | `#059669` | `#047857` | Darker Emerald hover & Light high-contrast accent fill (`5.48:1` with `#ffffff` text) |
| `--grex-focus-ring` | `#3b82f6` | `#2563eb` | Standardized `2px solid` `:focus-visible` keyboard ring |
| `--desk-header-h` | `32px` | `32px` | Locked top IDE menu & command bar height |
| `--desk-statusbar-h` | `24px` | `24px` | Locked bottom IDE status bar height |
| `--desk-rail-w` | `48px` | `48px` | Far-left vertical activity icon rail width |
| `--desk-sidebar-w` | `240px` | `240px` | Collapsible left sidebar width |
| `--desk-inspector-w` | `280px` | `280px` | Right inspector / co-pilot width |
| `--desk-row-h` | `28px` | `28px` | High-density spreadsheet grid row & sticky header height |

### 2.2 Locked `100vh` Shell & Compact Semantic Form Controls
- **Zero Page Scroll (`100vh` Lock):**
  - `html, body, #desk-root` enforce `height: 100vh; max-height: 100vh; overflow: hidden; margin: 0; padding: 0; background-color: var(--grex-base); color: var(--grex-text); font-family: "Urbanist", "Segoe UI", system-ui, sans-serif; font-size: 13px; line-height: 1.4;`.
- **Semantic Dark/Light Form Controls (`26px–28px` Height):**
  - All `<input>`, `<select>`, and single-line controls inside `.grex-field`, `.desk-ide-header`, `.desk-db-toolbar`, and `.desk-db-sidebar` use `height: 26px` to `28px`, `padding: 2px 8px`, `border-radius: 4px`, `border: 1px solid var(--grex-border)`, `background-color: var(--grex-base)`, `color: var(--grex-text)`, and monospace typography (`"JetBrains Mono", "Cascadia Code", Consolas, monospace`) for paths, ports, SQL, and `ADR-025c` decimals—eliminating unstyled white browser inputs.

---

## 3. Desktop IDE Shell Architecture (`100vh` Locked Viewport)

### 3.1 `32px` Top IDE Menu & Command Bar (`<header id="desk-header" class="desk-ide-header" role="banner">`)
- **Dimensions & Surface:** `height: var(--desk-header-h)` (`32px`), `background: var(--grex-surface)`, `border-bottom: 1px solid var(--grex-border)`, `padding: 0 8px`, `display: flex; align-items: center; justify-content: space-between; gap: 8px;`.
- **Left Group — Compact Brand & Application Menu Bar (`<nav role="menubar" aria-label="Application Menu">`):**
  - Compact `<h1>Groxy Desk</h1>` (`13px`, bold, zero margin) + `v1.3.0` badge so the document retains a single primary `<h1>` (`GREX-A11Y-004`).
  - Semantic `<nav role="menubar" aria-label="Application Menu">` containing compact `role="menuitem"` buttons (`File`, `Edit`, `View`, `Database`, `Browser`, `Tools`, `Help`) with keyboard access keys (`accesskey="f|e|v|d|b|t|h"`, `Alt+F`..`Alt+H`) triggering workspace refresh/check, undo (`Ctrl+Z`), studio view switching, database schema inspection, browser co-pilot focus, `grabman doctor`, and shortcut help.
- **Center Group — Omnipresent Command Search Bar (`#desk-command-search`):**
  - Centered `Ctrl+K` command search input (`id="desk-command-search"`, `type="search"`, `24px` height, monospace placeholder `Search files, run SQL, or ask Groxy AI (Ctrl+K)...`) with global `Ctrl+K` focus shortcut and instant command dispatch (switching studios, filtering files/tables, or staging a `Groxy AI` prompt).
- **Right Group — Active Workspace, Host Status & Theme Toggle:**
  - Active workspace badge (`#desk-workspace-badge`) displaying the active workspace/file scope.
  - Host status badge (`#desk-host-status-badge` and backward-compatible `#desk-connection-badge` text `127.0.0.1:9490 Authenticated` / `Waiting for Host Session`).
  - Compact theme toggle button (`#desk-theme-toggle`) switching Dark Obsidian (`#0f1115`) and Clean Light (`#f8fafc`).

### 3.2 `48px` Far-Left Vertical Activity Icon Rail (`<nav id="desk-activity-rail" class="desk-activity-rail" role="tablist" aria-label="Activity Rail">`)
- **Dimensions & Surface:** `width: var(--desk-rail-w)` (`48px`), `min-width: 48px`, `height: 100%`, `background: var(--grex-surface)`, `border-right: 1px solid var(--grex-border)`.
- **Activity Rail Buttons (`grab-ide-icons` Crisp SVG Vectors via Grex `Icon`):**
  1. **Explorer & Code (`#desk-tab-code`):** `role="tab"`, `aria-controls="desk-panel-code"`, switches to `Code & Grex Studio` (`code` view).
  2. **Database Studio (`#desk-tab-database`):** `role="tab"`, `aria-controls="desk-panel-database"`, switches to `Database Studio` (`database` view).
  3. **Browser & Radar (`#desk-tab-browser`):** `role="tab"`, `aria-controls="desk-panel-browser"`, switches to `Browser & Radar` (`browser` view).
  4. **Grex Visual Builder (`#desk-tab-grex`):** `role="tab"`, `aria-controls="desk-panel-code"`, activates `#desk-panel-code` focused on the embedded `Grex` split-preview canvas (`#desk-preview-surface`).
  5. **Server & Fleet (`#desk-tab-fleet`):** `role="tab"`, `aria-controls="desk-panel-code"`, activates `#desk-panel-code` and opens the Coolify-density `Grisp` Supervisor & Step 5 Fleet drawer.
  6. **Bottom-Anchored Quick Actions:** `#desk-action-settings` (toggles theme/workspace preferences) and `#desk-action-shortcuts` (focuses command search / displays keyboard shortcuts).
- **Keyboard & Hash/Session Persistence Parity:**
  - `ArrowRight` / `ArrowDown` and `ArrowLeft` / `ArrowUp` cycle cleanly through the studio tabs (`database`, `browser`, `code`) with `Home` / `End` jump support, updating `window.location.hash` (`#studio=database|browser|code`) and `sessionStorage` (`groxy_desk_active_studio`).
  - Active rail button displays a `2px solid var(--grex-accent)` left indicator bar and `var(--grex-surface-active)` background.

### 3.3 Main Locked Stage (`.desk-stage-locked`, `height: calc(100vh - var(--desk-header-h) - var(--desk-statusbar-h))`)
- **Outer Container:** Wraps `#desk-activity-rail` and `<main id="desk-workspace-stage" class="desk-main">` in a locked flex row (`height: calc(100vh - 56px); overflow: hidden;`).
- **Edge-to-Edge Panes:** All three mounted `role="tabpanel"` containers (`#desk-panel-database`, `#desk-panel-browser`, `#desk-panel-code`) fill `100%` of `<main id="desk-workspace-stage">` with `1px solid var(--grex-border)` hairline dividers and independent `overflow-y: auto` pane scrolling.
- **Database Studio (`#desk-panel-database`, wrapping `#desk-db-studio-panel`):**
  - **`240px` Collapsible Left Sidebar (`#desk-db-sidebar`):** Quick-switch connection selector (`PostgreSQL 18`, `MariaDB 11.8`, `SQLite`), inline schema object filter input (`#desk-db-object-filter`), and expandable `Tables`, `Views`, and `Triggers` tree nodes.
  - **Center Workspace (`#desk-db-center`):** `24px`-button compact action toolbar (`Run Query`, `Export CSV/XLSX/JSON/SQL`, `ER Diagram`, `Schema Diff`, `Undo Snapshot`), `26px` multi-tab autocomplete SQL editor, and `28px`-row-height (`--desk-row-h: 28px`) sticky-header zebra-striped spreadsheet grid (`.desk-grid-table`, `font-variant-numeric: tabular-nums`) preserving exact `ADR-025c` base-10 wire strings (`"0.00"`).
  - **`280px` Right Inspector (`#desk-db-inspector`):** Selected cell `ADR-025c` exact-decimal editor, pre-mutation SHA-256 snapshot status, and `GROXY-SAFE-001` approval/rollback drawer.
- **Browser & Radar (`#desk-panel-browser`):**
  - Edge-to-edge 50/50 split pane with `1px solid var(--grex-border)` center divider: isolated WebView2 browser controller + `GRAB-E402` origin lock badge on the left; Bestseller Radar niche explorer, `BRAND-DNA.md` prefill preview, and `GrabLab CMS` draft handoff (`status_enforced: "draft"`) on the right.
- **Code, Grex Visual Builder & Coolify-Density Server/Fleet (`#desk-panel-code`):**
  - **`240px` Left Sidebar:** `GRAB-E401`-confined workspace file tree, `grabman` tools, and Coolify-density Server & Fleet health pills (`● Grisp :8765 (Healthy)` with 1-click reload/stop, Step 5 Fleet, Namecheap DNS + Auto-HTTPS, and 1-Click Local Author Mode) using `#181b20`/`#ffffff` sidebar surfaces, `#0f1115`/`#f8fafc` nested cards, and `1px solid var(--grex-border)` borders.
  - **Center Pane (`#desk-main-stage`):** Active `.grab`/`.grab.html` code workbench (`#desk-editor-card`, `#desk-active-file-meta`, `#desk-run-grab-check-btn`, `#desk-editor-code`, `#desk-check-diagnostics`) paired with the embedded split-screen `Grex` visual preview canvas (`#desk-preview-surface`, 8 Style Packs × 5 Blueprints, `Ctrl+Z` `applyAiLayout` undo).
  - **`280px` Right Inspector:** Unified `Groxy AI` Adaptive Reasoning co-pilot (`#desk-groxy-sidebar`, zero model-switching dropdowns), 22-rule 4-pillar audit scorecard + 🚀 `"Let Groxy Boost This"`, and Pre-Mutation Snapshots (`#desk-snapshots-card`).

### 3.4 `24px` Pinned Bottom IDE Status Bar (`<footer id="desk-status-bar" class="desk-ide-statusbar" role="status">`)
- **Dimensions & Surface:** `height: var(--desk-statusbar-h)` (`24px`), `min-height: 24px`, `background: var(--grex-surface)`, `border-top: 1px solid var(--grex-border)`, `padding: 0 8px`, `font-size: 11px`, `color: var(--grex-text-muted)`.
- **Five Live Status Indicators:**
  1. **`#desk-status-db` (Database Connection Pill):** Interactive 1-click button showing live DB connection state (e.g., `● Connected: SQLite (data/app.db)` / `● Connected: PostgreSQL (tunnel)` or `○ DB Studio Ready — Click to Connect`), activating `#desk-panel-database` on click.
  2. **`#desk-status-ssh` (Pinned SSH Fingerprint Pill):** Shows active `SHA256:...` host key fingerprint from `store.dbTunnels` or `SSH: Loopback Only (127.0.0.1)`.
  3. **`#desk-status-safety` (`GROXY-SAFE-001` Guard State):** Live indicator `GROXY-SAFE-001: Armed (<N> Snapshots)` reflecting `store.snapshots`.
  4. **`#desk-status-grisp` (Active `Grisp` Worker Pool):** Live worker count and ports from `store.workers` (e.g., `Grisp: 1 Active (:8765)` or `Grisp: 0 Active (Idle)`).
  5. **`#desk-status-adr025c` (Exact-Wire & Runtime Indicator):** `ADR-025c Exact Wire | UTF-8 | Grex JS v1.0.0`.

---

## 4. Preservation of Frozen `v1.2.0` Contracts & Invariants

| Contract Layer | Frozen Baseline | Invariant Preservation Strategy in `v1.3.0` |
|---|---|---|
| **21 Frozen DOM Element IDs** | `v1.2.0` / `SPEC-120 §6` | **100% Preserved:** `#desk-root`, `#desk-header`, `#desk-workspace-stage`, `#desk-theme-toggle`, `#desk-main-stage`, `#desk-tab-database`, `#desk-tab-browser`, `#desk-tab-code`, `#desk-panel-database`, `#desk-panel-browser`, `#desk-panel-code`, `#desk-db-studio-panel`, `#desk-editor-card`, `#desk-active-file-meta`, `#desk-run-grab-check-btn`, `#desk-editor-code`, `#desk-check-diagnostics`, `#desk-preview-surface`, `#desk-snapshots-card`, `#desk-groxy-sidebar`, and `#desk-author-panel` (plus `#desk-connection-badge` alongside `#desk-host-status-badge`, and all `db-studio-panel.js`, `radar-panel.js`, `studio-panel.js`, `scorecard-panel.js`, `workspace-panel.js`, `supervisor-panel.js`, `fleet-panel.js`, and `dns-panel.js` control IDs). |
| **New `v1.3.0` Shell IDs** | `SPEC-130 §3` | Additive IDs: `#desk-command-search`, `#desk-host-status-badge`, `#desk-activity-rail`, `#desk-tab-grex`, `#desk-tab-fleet`, `#desk-action-settings`, `#desk-action-shortcuts`, `#desk-db-sidebar`, `#desk-db-object-filter`, `#desk-db-center`, `#desk-db-inspector`, `#desk-status-bar`, `#desk-status-db`, `#desk-status-ssh`, `#desk-status-safety`, `#desk-status-grisp`, and `#desk-status-adr025c`. |
| **Pure `Grex JS v1.0.0`** | Pillar 4 / `GREX-SCHEMA-001` | `divCount === 0`, `innerHTMLWriteCount === 0`, zero `outerHTML` / `insertAdjacentHTML`, zero `eval()` / `new Function()`, zero inline `on*=` attributes, zero unsafe URL schemes, and byte-for-byte unmodified `public/grex/grex.js` + `public/grex/grex.elements.json`. |
| **Go Host & WebView2 Isolation** | Pillar 3 / `SPEC-110` / `SPEC-120` | Strictly `127.0.0.1` loopback binding (`DESK-E501`), constant-time bearer + `X-CSRF-Token` (`403 DESK-E403`), `GRAB-E401` path confinement, and `cmd/groxy-desk/browser_policy.go` exact-origin + fragment allowlist (`GRAB-E402`). |
| **Exact Numeric Wire & Safety** | Pillar 5 / `ADR-025c` / `GROXY-SAFE-001` | Base-10 wire strings (`"0.00"`, `> 2^53` integers) preserved without IEEE-754 float drift; single-use `approval_token` + SHA-256 Pre-Mutation Snapshot required before any write, DDL, import, layout apply, or DNS/fleet mutation. |

---

## 5. Acceptance Gates (Stage 3 Verification Plan)

| Gate | Required Evidence |
|---|---|
| **1. Go Host & Launcher** | `CGO_ENABLED=0` `go build ./...`, `go vet ./...`, `go test -count=1 ./...`, `govulncheck ./...` (`0` vulnerabilities), `go mod verify`, and Windows WebView2 launcher `--smoke-test` pass `100%`. |
| **2. GRAB, Grex & `v1.3.0` IDE Shell Conformance** | `grab check` and canonical `grab format` byte parity on `src/desk.grab` and `views/workbench.grab.html`; `node --test tests/*.test.js` `100%` pass verifying `divCount === 0`, `innerHTMLWriteCount === 0`, zero `eval()`, Dark Obsidian & Clean Light WCAG AA contrast (`>= 4.5:1` text, `>= 3.0:1` UI), `100vh` locked shell tokens, `#desk-header` (`32px`, `role="menubar"`, `#desk-command-search`), `#desk-activity-rail` (`48px`, `role="tablist"`, SVG icons, keyboard nav), `#desk-db-object-filter`, `#desk-status-bar` (`24px`, all 5 live indicators), and 100% preservation of all `v1.2.0` DOM IDs. |
| **3. Data Fidelity & Recovery** | Exact `"0.00"` and large-integer round trips, inline cell editing in `.desk-grid-table`, CSV/XLSX/JSON/SQL import/export, ER diagram, schema diff, and `GROXY-SAFE-001` snapshot/rollback continue passing `100%`. |
| **4. Cross-Layer Windows E2E** | Full `python tests/integration_e2e.py`, `python tests/radar_live_e2e.py`, `TestNativeWebView2Isolation`, and pinned DB recovery checks pass `100%` with all 10 sibling `v1.x` repositories strictly clean. |

---

## 6. Stage 2 Pre-Implementation Cross-Layer Security & Contract Audit (PASS)

On 2026-11-21, the project owner approved `GROXY-DESK-SPEC-130.md` for Stage 1 & Stage 2 and authorized recording `Approved; implementation in progress` and `Stage 2 Pre-Implementation Cross-Layer Security & Contract Audit (PASS)`. A line-by-line cross-layer inspection of `views/workbench.grab.html`, `src/desk.grab`, `public/desk/desk.css`, `public/desk/theme.js`, `public/desk/workbench.js`, `public/desk/db-studio-panel.js`, `public/grex/grex.js`, `cmd/groxy-desk/browser_policy.go`, `tests/helpers/dom-shim.js`, `tests/unit_ui_dom.test.js`, `tests/unit_v120_tabs.test.js`, and `tests/integration_e2e.py` confirms `v1.3.0` can be implemented cleanly inside `C:\Development\groxy-desk` with zero modifications to frozen backend route contracts or any of the 10 frozen sibling `v1.x` repositories.

**Audit Verdict: `PASS`** with the following five binding Stage 3 implementation guardrails:

1. **100% Preservation of Frozen `v1.2.0` DOM Element IDs & Dual Status Badge Compatibility (`§§3.1–3.4, 4`):**
   - Stage 3 MUST preserve all 21 core `v1.2.0` DOM element IDs (`#desk-root`, `#desk-header`, `#desk-workspace-stage`, `#desk-theme-toggle`, `#desk-main-stage`, `#desk-tab-database`, `#desk-tab-browser`, `#desk-tab-code`, `#desk-panel-database`, `#desk-panel-browser`, `#desk-panel-code`, `#desk-db-studio-panel`, `#desk-editor-card`, `#desk-active-file-meta`, `#desk-run-grab-check-btn`, `#desk-editor-code`, `#desk-check-diagnostics`, `#desk-preview-surface`, `#desk-snapshots-card`, `#desk-groxy-sidebar`, `#desk-author-panel`) and all existing form/button IDs inside the child panel modules.
   - In `#desk-header`, render `#desk-host-status-badge` (`127.0.0.1:9490 Authenticated` / `Waiting for Host Session`) while preserving `#desk-connection-badge` (or keeping `#desk-connection-badge` as the status pill inside `#desk-host-status-badge`) so any selector targeting either ID succeeds.
   - Keep all three `<section role="tabpanel" id="desk-panel-database|browser|code" tabindex="0">` containers permanently mounted in the DOM (toggling `hidden` + `aria-hidden`, backed by `.desk-tabpanel[hidden] { display: none; }` in `public/desk/desk.css`).

2. **Pure `Grex JS v1.0.0` `Icon` (`createElementNS`) & Headless DOM Shim Parity (`§§3.2, 4`):**
   - `public/grex/grex.js` (`C1E28D74A7ECF19182E225644FA244AA8600658DD1FF8895C133BFC64976B6D6`) renders `Icon` elements using `doc.createElementNS("http://www.w3.org/2000/svg", "svg")` and allowlisted SVG child primitives (`path`, `circle`, `rect`, `line`, `polyline`, `polygon`) with zero `innerHTML`.
   - Note that `Button` (`type: "Button"`) in `public/grex/grex.js` renders a text-based `<button>` and does not traverse `config.children`, whereas `renderElement({ tag: "button", attrs: ..., on: ..., children: [...] })` (used by ` renderNode` when `type` is omitted) renders `<button>` with child `Icon` (`<svg>`) and `<span>` nodes via `createElement` / `createElementNS`. When embedding `Icon` inside activity-rail buttons, either render the `Icon` via `renderNode({ type: "Icon", ... })` inside `children` of a `<button>` element spec or pair `Button` with CSS/SVG so `divCount === 0` and `innerHTMLWriteCount === 0` hold unconditionally.
   - **Binding Test Shim Update (`tests/helpers/dom-shim.js` & `tests/unit_ui_dom.test.js`):** Add `createElementNS(_ns, tag)` to `createDomShim` in `tests/helpers/dom-shim.js` (delegating to `new ShimElement(tag, doc, counter)`) and add the allowlisted SVG shape tags (`"path"`, `"circle"`, `"rect"`, `"line"`, `"polyline"`, `"polygon"`) and table tags (`"table"`, `"thead"`, `"tr"`, `"th"`, `"td"`, `"caption"`) to `ALLOWED_SEMANTIC_TAGS` in `tests/unit_ui_dom.test.js`, while keeping `public/grex/grex.js` and `public/grex/grex.elements.json` 100% unmodified.

3. **WCAG AA Contrast Math Verification & `theme.js` Token Alias Compatibility (`§2.1`):**
   - Live verification via `computeContrastRatio()` in `public/grex/grex.js` confirms:
     - **Dark Obsidian:** `--grex-text` (`#e6edf3`) achieves `15.99:1` on `--grex-base` (`#0f1115`), `14.61:1` on `--grex-surface` (`#181b20`), `12.83:1` on `--grex-surface-hover` (`#22262e`), and `11.20:1` on `--grex-surface-active` (`#2b303a`). `--grex-text-muted` (`#8b949e`) achieves `6.14:1` on `--grex-base`, `5.61:1` on `--grex-surface`, and `4.93:1` on `--grex-surface-hover` (all `>= 4.5:1`). `--grex-accent` (`#10b981`) achieves `7.45:1` on `--grex-base`, `6.81:1` on `--grex-surface`, and `7.45:1` against dark accent text `#0f1115`.
     - **Clean Light:** `--grex-text` (`#0f172a`) achieves `17.06:1` on `--grex-base` (`#f8fafc`) and `17.85:1` on `--grex-surface` (`#ffffff`). `--grex-text-muted` (`#475569`) achieves `7.24:1` on `--grex-base`, `7.58:1` on `--grex-surface`, `6.92:1` on `--grex-surface-hover` (`#f1f5f9`), and `6.15:1` on `--grex-surface-active` (`#e2e8f0`). Because `--grex-accent: #059669` on `#ffffff` is `3.77:1` (`>= 3.0:1` WCAG AA UI component/graphical object threshold, below `4.5:1` normal text), Stage 3 MUST use `--grex-accent-hover: #047857` (`5.48:1` on `#ffffff` and `5.48:1` with `--grex-accent-text: #ffffff`) for normal-size accent text and solid primary button fills in Light mode (or `--grex-accent-text: #0f172a` on `#10b981` at `7.04:1`), ensuring `verifyPaletteContrast()` in `public/desk/theme.js` and `tests/unit_v120_tabs.test.js` pass `100%` at `>= 4.5:1`.

4. **`browser_policy.go` Fragment Allowlisting (`GRAB-E402`) & Activity Rail `role="tablist"` Semantics (`§§3.1, 3.2, 4`):**
   - `tests/unit_v120_tabs.test.js` queries `root.querySelector('nav[role="tablist"]')` and dispatches `ArrowRight`, `ArrowLeft`, `Home`, and `End` keydown events expecting `STUDIO_MODES = ["database", "browser", "code"]` order (`database` -> `browser` -> `code` -> `database`, with `Home` -> `database` and `End` -> `code`).
   - Stage 3 MUST keep `role="tablist"` exclusively on `<nav id="desk-activity-rail" class="desk-activity-rail" role="tablist" aria-label="Activity Rail">` (using `<nav role="menubar" aria-label="Application Menu">` for the top `32px` menu bar so `root.querySelector('nav[role="tablist"]')` resolves unambiguously to `#desk-activity-rail`), preserve the `ArrowRight`/`ArrowLeft`/`Home`/`End` keyboard cycle across `database`, `browser`, and `code`, and ensure `#desk-tab-grex` and `#desk-tab-fleet` activate `#desk-panel-code` (`#studio=code`) while focusing their respective sub-views (`#desk-preview-surface` or the Server & Fleet section).
   - `trustedWorkbenchSource` in `cmd/groxy-desk/browser_policy.go` continues to enforce exact loopback origin, `u.Path == "/"`, empty query, `u.User == nil`, and the strict fragment allowlist (`""`, `"studio=database"`, `"studio=browser"`, `"studio=code"`, `"desk-workspace-stage"`).

5. **Sequential Heading Hierarchy (`GREX-A11Y-004`), Single `<main>` Landmark, `ADR-025c` & `GROXY-SAFE-001` (`§§3.3, 3.4, 4`):**
   - `auditDom(root)` enforces a single `<h1>` (in `#desk-header`) followed by sequential `<h2>` -> `<h3>` -> `<h4>` -> `<h5>` headings across the entire mounted DOM without level skips. Each tabpanel (`#desk-panel-database`, `#desk-panel-browser`, `#desk-panel-code`) MUST open with its `<h2>` heading before any `<h3>`/`<h4>` subheadings, and neither `#desk-header`, `#desk-activity-rail`, nor `#desk-status-bar` may introduce an `<h3>`+ heading before the first `<h2>`.
   - Maintain a single `<main id="desk-workspace-stage">` landmark, keep `#desk-db-studio-panel` wrapping `#desk-db-sidebar` (`with #desk-db-object-filter`), `#desk-db-center` (with `.desk-grid-table`), and `#desk-db-inspector`, preserve exact `ADR-025c` base-10 wire strings (`"0.00"`), and preserve all `GROXY-SAFE-001` single-use `approval_token` + SHA-256 Pre-Mutation Snapshot and rollback workflows.

---

## 7. Stage 3 Implementation & Verification Evidence (PASS)

- **Locked `100vh` High-Density IDE Shell & Palettes (`public/desk/desk.css`, `public/desk/theme.js`, `views/workbench.grab.html`):**
  - Locked `html, body, #desk-root` to `height: 100vh; max-height: 100vh; overflow: hidden; display: flex; flex-direction: column;`.
  - Configured Dark Obsidian (`#0f1115` / `#181b20`) and Clean Light (`#f8fafc` / `#ffffff`) tokens + layout dimension variables (`--desk-header-h: 32px`, `--desk-statusbar-h: 24px`, `--desk-rail-w: 48px`, `--desk-sidebar-w: 240px`, `--desk-inspector-w: 280px`, `--desk-row-h: 28px`) and `26px–28px` monospace form controls passing `verifyPaletteContrast()` (`>= 4.5:1` text, `>= 3.0:1` UI).
- **Five Coordinated IDE Chrome & Studio Layers (`public/desk/workbench.js`, `public/desk/db-studio-panel.js`):**
  - Built `32px` top `#desk-header` (`<nav role="menubar">` with `File`, `Edit`, `View`, `Database`, `Browser`, `Tools`, `Help`, `#desk-command-search` `Ctrl+K`, `#desk-workspace-badge`, `#desk-host-status-badge` + `#desk-connection-badge`, and `#desk-theme-toggle`), `48px` far-left `#desk-activity-rail` (`role="tablist"`, `grab-ide-icons` SVG vectors via Pure Grex JS `Icon` `createElementNS`, `#desk-tab-code`, `#desk-tab-database`, `#desk-tab-browser`, `#desk-tab-grex`, `#desk-tab-fleet`, `#desk-action-settings`, `#desk-action-shortcuts`), locked main stage (`.desk-stage-locked` with `#desk-db-sidebar`, `#desk-db-object-filter`, `28px`-row `.desk-grid-table` with `tabular-nums`, `#desk-db-center`, `#desk-db-inspector`), and `24px` bottom `#desk-status-bar` (`#desk-status-db`, `#desk-status-ssh`, `#desk-status-safety`, `#desk-status-grisp`, `#desk-status-adr025c`).
- **Verification Gates 1–4 (`100%` Pass):**
  - Gate 1: `CGO_ENABLED=0` `go build ./...` & `go vet ./...` (Windows amd64 & Linux amd64), `go mod verify`, `govulncheck ./...` (`0` vulnerabilities), `go test -count=1 ./...`, and `grab check` + `grab format` byte parity.
  - Gate 2: `node --test tests/*.test.js` (`14/14` pass, `divCount === 0`, `innerHTMLWriteCount === 0`, `auditDom()` `0` issues, Light & Dark WCAG AA, `ADR-025c`, and all 21 frozen `v1.2.0` DOM IDs + 17 additive `v1.3.0` shell IDs verified).
  - Gate 3 & Gate 4: `GROXY_DESK_TEST_WEBVIEW2=1` `TestNativeWebView2Isolation`, `python tests/integration_e2e.py`, `python tests/radar_live_e2e.py`, `python tests/network_dump_recovery.py`, packaged `--smoke-test`, and clean `git status --short` across all 10 sibling `v1.x` repositories.
