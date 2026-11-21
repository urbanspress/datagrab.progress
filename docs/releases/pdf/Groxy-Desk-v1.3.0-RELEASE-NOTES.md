# Groxy Desk v1.3.0

### Release Type & Affected Layers

- **Release Version & SemVer Tier:** `v1.3.0` (`MINOR 1.x.0` — Backwards-Compatible Feature Release)
- **Affected Layer(s):** `Groxy Desk` (`urbanspress/groxy-desk`)
- **Neighboring Layer Upgrade Required?** `No — 100% drop-in compatible` (existing workflows use frozen `v1.x` baselines; optional Bestseller Radar and GrabLab CMS book draft handoffs use `Groxy AI v1.8.0` and `GrabLab CMS v1.4.0`/`v1.5.0`)

| Ecosystem Layer | Repository | Required Version Range | Touched in This Release? | Neighbor Upgrade Required? |
|---|---|---|---|---|
| **Core Language, Bytecode VM & Grisp Server** | `urbanspress/grab` | `>= v1.1.0 (< v2.0.0)` | `No` | `No` |
| **Reactive Frontend Runtime & 20-Element Library** | `urbanspress/grex-js` | `>= v1.0.0 (< v2.0.0)` | `No` | `No` |
| **Flagship Zero-Plugin CMS** | `urbanspress/grablab-cms` | `>= v1.0.1 (< v2.0.0)` (`v1.4.0`/`v1.5.0` for optional book drafts) | `No` | `No` |
| **Exclusive `mode: "visual"` Theme & Studio** | `urbanspress/grex-builder` | `>= v1.0.0 (< v2.0.0)` | `No` | `No` |
| **Autonomous Design, Content & SEO Agent** | `urbanspress/groxy-ai` | `>= v1.0.0 (< v2.0.0)` (`v1.8.0` for optional Bestseller Radar) | `No` | `No` |
| **Scaffolder, Orchestrator & Discreet VPS Panel** | `urbanspress/grabman`, `urbanspress/grabtaculous`, `urbanspress/gashboard` | `>= v1.0.0 (< v2.0.0)` (bundle pins `grabman v1.0.0`) | `No` | `No` |
| **Native Desktop Vibe-Coding IDE & Studio** | `urbanspress/groxy-desk` | `>= v1.3.0 (< v2.0.0)` | `Yes` | `No` |

### `v1.3.0` Highlights — VS Code / DbGate / Coolify High-Density `100vh` Desktop IDE Shell

- **Locked `100vh` Desktop IDE Shell & Dark Obsidian / Clean Light Palettes (`public/desk/desk.css`, `public/desk/theme.js`):**
  - Locked `html, body, #desk-root` to `height: 100vh; max-height: 100vh; overflow: hidden;` with `display: flex; flex-direction: column;` so outer viewports never scroll and only inner panes scroll independently.
  - Added WCAG AA (`>= 4.5:1` text, `>= 3.0:1` UI) CSS tokens for **Dark Obsidian** (`--grex-base: #0f1115`, `--grex-surface: #181b20`) and **Clean Light** (`--grex-base: #f8fafc`, `--grex-surface: #ffffff`), `1px solid var(--grex-border)` hairline pane dividers, layout variables (`--desk-header-h: 32px`, `--desk-statusbar-h: 24px`, `--desk-rail-w: 48px`, `--desk-sidebar-w: 240px`, `--desk-inspector-w: 280px`, `--desk-row-h: 28px`), and `26px–28px` monospace form controls.
- **`32px` Top IDE Menu & Command Bar (`<header id="desk-header" class="desk-header desk-ide-header" role="banner">`):**
  - Compact `<nav role="menubar">` with `File`, `Edit`, `View`, `Database`, `Browser`, `Tools`, and `Help` shortcuts, centered `#desk-command-search` (`Ctrl+K`), active workspace badge (`#desk-workspace-badge`), `#desk-host-status-badge` (`#desk-connection-badge`), and `#desk-theme-toggle`.
- **`48px` Far-Left Vertical Activity Icon Rail (`<nav id="desk-activity-rail" class="desk-activity-rail" role="tablist" aria-label="Activity Rail">`):**
  - Crisp `grab-ide-icons` SVG navigation buttons rendered via Pure `Grex JS v1.0.0` `Icon` (`createElementNS`) for `#desk-tab-code`, `#desk-tab-database`, `#desk-tab-browser`, `#desk-tab-grex`, `#desk-tab-fleet`, `#desk-action-settings`, and `#desk-action-shortcuts` with WAI-ARIA keyboard navigation and allowlisted `#studio=database|browser|code` + `sessionStorage` persistence.
- **Locked Main Stage & `28px`-Row Tabular Spreadsheet Grid (`.desk-stage-locked`, `public/desk/db-studio-panel.js`):**
  - **Database Studio (`#desk-panel-database`):** `240px` left sidebar (`#desk-db-sidebar`) with live schema object filtering (`#desk-db-object-filter`), center SQL/toolbar workspace (`#desk-db-center`) over a `28px`-row-height sticky-header zebra-striped spreadsheet grid (`.desk-grid-table`, `tabular-nums`, exact `ADR-025c` base-10 wire strings), and `280px` right inspector (`#desk-db-inspector`).
  - **Browser & Radar (`#desk-panel-browser`):** Edge-to-edge 50/50 split pane with `1px solid var(--grex-border)` center divider.
  - **Code, Grex Builder & Server/Fleet (`#desk-panel-code`):** Edge-to-edge 3-pane studio with Coolify-density Server & Fleet tools, live `Grex` preview canvas, and Unified `Groxy AI` (zero model-switching dropdowns).
- **`24px` Pinned Bottom IDE Status Bar (`<footer id="desk-status-bar" class="desk-footer desk-ide-statusbar" role="status">`):**
  - Live `#desk-status-db` connection pill (1-click switch to Database Studio), `#desk-status-ssh` pinned `SHA256:...` fingerprint, `#desk-status-safety` (`GROXY-SAFE-001` snapshot guard), `#desk-status-grisp` active worker pool, and `#desk-status-adr025c` exact-wire indicator.

### Stage 3 & Stage 4 Verification Summary (`SPEC-130` Acceptance Gates 1–4 & Stage 4 Sign-Off: `PASS`)

- **Stage 4 Pre-Release Cross-Layer Security & Contract Sign-Off (`PASS` — `Approved & Frozen — Complete`):** Verified `7df23c8` implementation baseline, `dist/SHA256SUMS.txt` byte-for-byte parity across all 6 packaged assets (`Groxy-Desk-Setup-1.3.0.exe`, `Groxy-Desk-v1.3.0-Guide.pdf`, `groxy-desk-v1.3.0-bundle.zip`, `groxy-desk-windows-amd64.exe`, `groxy-deskd-linux-amd64`, `groxy-deskd-windows-amd64.exe`), byte-for-byte unmodified `public/grex/grex.js` + `public/grex/grex.elements.json`, `trustedWorkbenchSource` fragment allowlisting (`cmd/groxy-desk/browser_policy.go`), and clean `git status --short` across all 10 sibling `v1.x` repositories.
- `CGO_ENABLED=0` `go build ./...` and `go vet ./...` (Windows amd64 & Linux amd64), `go mod verify`, `govulncheck ./...` (`0` vulnerabilities), and `go test -count=1 ./...` across all tested Go packages passed `100%`.
- `grab check src/desk.grab views/workbench.grab.html` and canonical `grab format` byte-for-byte parity passed `100%`.
- `node --test tests/*.test.js` (`14/14` pass) verified `divCount === 0`, `innerHTMLWriteCount === 0`, zero `eval()` / `new Function()`, Dark Obsidian & Clean Light WCAG AA (`>= 4.5:1` text, `>= 3.0:1` UI), `auditDom()` zero issues/zero heading skips, `ADR-025c` fidelity, `100vh` shell tokens, 5 chrome layers, live object filtering, and 100% preservation of all 21 frozen `v1.2.0` DOM IDs.
- `GROXY_DESK_TEST_WEBVIEW2=1` `TestNativeWebView2Isolation`, `python tests/integration_e2e.py`, `python tests/radar_live_e2e.py`, and `python tests/network_dump_recovery.py` passed `100%`, with all 10 sibling `v1.x` repositories verified clean (`git status --short`).
