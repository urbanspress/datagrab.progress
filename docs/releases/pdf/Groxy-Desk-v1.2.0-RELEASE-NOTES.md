# Groxy Desk v1.2.0

### Release Type & Affected Layers

- **Release Version & SemVer Tier:** `v1.2.0` (`MINOR 1.x.0` — Backwards-Compatible Feature Release)
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
| **Native Desktop Vibe-Coding IDE & Studio** | `urbanspress/groxy-desk` | `>= v1.2.0 (< v2.0.0)` | `Yes` | `No` |

### `v1.2.0` Highlights — DbGate-Parity 3-Tab Dedicated Workspace Studio & Visual Polish

- **Sticky Top 3-Tab Workspace Switcher & Offline-Safe `Urbanist` Typography:**
  - Bundled `Urbanist` (weights `400`, `500`, `600`, `700`) `.woff2` font files locally under `public/desk/fonts/` (embedded via `assets.go`) and registered `.woff2` (`font/woff2`) / `.woff` (`font/woff`) MIME types in `handleStaticPublic` (`internal/server/server.go`) with zero external CDN requests and `X-Content-Type-Options: nosniff` compliance.
  - Added a sticky top `.desk-header` housing the brand title, `nav[role="tablist"]` workspace switcher (`Database Studio` | `Browser & Radar` | `Code & Grex Studio`), authenticated loopback badge, and Light/Dark toggle, with wrapping `ArrowLeft`/`ArrowRight` + `Home`/`End` keyboard navigation and strictly allowlisted `#studio=database|browser|code` URL hash + `sessionStorage` (`groxy_desk_active_studio`) persistence.
  - Updated `trustedWorkbenchSource` in `cmd/groxy-desk/browser_policy.go` to allow only exact loopback root fragments (`""`, `"studio=database"`, `"studio=browser"`, `"studio=code"`, `"desk-workspace-stage"`) while preserving `GRAB-E402` and WebView2 IPC isolation.
- **Tab 1 — `Database Studio` (DbGate 3-Pane Full-Height Workspace):**
  - Left sidebar: saved/active connections (`PostgreSQL`, `MariaDB`, `SQLite`) + pinned SSH tunnels and expandable `Tables` / `Views` / `Triggers` schema tree (`Grex` `Accordion`).
  - Center workspace: focused action toolbar (`Run Query`, `Browse table rows`, `Inspect schema`, `Cancel query`, `Export CSV/XLSX/JSON/SQL`, `ER Diagram`, `Schema Diff`, `Undo Snapshot`), multi-tab autocomplete SQL editor, spreadsheet-style data grid, interactive ER diagram (fixing heading hierarchy `<h4>` -> `<h5>`), export/import, and cross-environment schema diff.
  - Right inspector: `ADR-025c` exact-decimal cell editor, Pre-Mutation Snapshot Guard status (including MariaDB least-privilege `409 write_fence_unavailable` notice), and `GROXY-SAFE-001` approval/rollback drawer.
- **Tab 2 — `Browser & Radar` (Full-Height 50/50 Split Pane):**
  - Left: isolated WebView2 allowlisted browser controls with live `GRAB-E402` origin lock badge, explicit selection capture, and `BRAND-DNA.md` organization prefill.
  - Right: Bestseller Radar niche explorer, `BRAND-DNA.md` prefill preview, and separately approved `GrabLab CMS` draft handoff (`status_enforced: "draft"`).
- **Tab 3 — `Code & Grex Studio` (Clean 3-Pane Vibe-Coding Studio):**
  - Left: confined workspace file explorer, `grabman v1.0.0` manager, and collapsible accordions for Local `Grisp` Supervisor, Step 5 Fleet, Namecheap DNS + Auto-HTTPS, and 1-Click Author Mode.
  - Center: active `.grab`/`.html` code workbench (`#desk-main-stage`) and responsive `Grex` live split-preview canvas.
  - Right: Unified `Groxy AI` Adaptive Reasoning co-pilot (zero model-switching dropdowns), 22-rule audit scorecard (`Let Groxy Boost This`), and Pre-Mutation Snapshots.

### Stage 3 & Stage 4 Verification Summary (`SPEC-120` Acceptance Gates — Stage 4 Pre-Release Cross-Layer Security & Contract Sign-Off: PASS)

- `CGO_ENABLED=0` `go build ./...` and `go vet ./...` (Windows amd64 & Linux amd64), `go mod verify`, `govulncheck ./...` (`0` vulnerabilities), and `go test -count=1 ./...` across all tested Go packages passed `100%`.
- `grab check src/desk.grab views/workbench.grab.html` and canonical `grab format` byte-for-byte parity passed `100%`.
- `node --test tests/*.test.js` (`13/13` pass) verified `divCount === 0`, `innerHTMLWriteCount === 0`, zero `eval()` / `new Function()`, Light & Dark WCAG AA (`>= 4.5:1`), `auditDom()` zero issues/zero heading skips, `ADR-025c` fidelity, WAI-ARIA 3-tab keyboard navigation, allowlisted hash/session persistence, and zero `<select>` model pickers.
- `GROXY_DESK_TEST_WEBVIEW2=1` `TestNativeWebView2Isolation`, `python tests/integration_e2e.py`, `python tests/radar_live_e2e.py`, and `python tests/network_dump_recovery.py` passed `100%`, with all 10 sibling `v1.x` repositories verified clean (`git status --short`).
