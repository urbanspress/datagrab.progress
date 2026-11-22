# Groxy Desk v1.4.0

### Release Type & Affected Layers

- **Release Version & SemVer Tier:** `v1.4.0` (`MINOR 1.x.0` — Backwards-Compatible Feature Release)
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
| **Native Desktop Vibe-Coding IDE & Studio** | `urbanspress/groxy-desk` | `>= v1.4.0 (< v2.0.0)` | `Yes` | `No` |

### `v1.4.0` Highlights — True DbGate-Parity Connection Manager, Collapsible Rail & Full-Screen Spreadsheet Grid

- **Saved Connection Profiles & Tabbed Modal (`<dialog id="desk-db-conn-dialog">`, `internal/dbstudio/connections.go`, `GET/POST/DELETE /api/db/connections`):**
  - Replaced raw database/SSH form fields in `#desk-db-sidebar` with a compact **`CONNECTIONS`** manager (`+ New Connection` icon button `#db-new-conn-btn`, Edit / Disconnect / Delete actions, and 1-click connect opening pinned SSH tunnels + loading schema automatically).
  - Moved all database and SSH connection inputs into `<dialog id="desk-db-conn-dialog">` (`General` and `SSH Tunnel` tabs, preserving every existing DOM input ID) with **`Save`**, **`Connect`**, and **`Close`** actions.
  - Passwords and bridge bearers are encrypted at rest in `stateDir/db-connections.json` using `AES-256-GCM` (`v1:...`), never returned in API responses, and never stored in `localStorage`.
- **Collapsible Left Sidebar & Clean `TABLES, VIEWS, TRIGGERS` Tree:**
  - Clicking the active left-rail icon (`#desk-tab-database`, `#desk-tab-code`) toggles the studio's left sidebar open or collapsed (`0px` grid track).
  - Dedicated the lower 75% of `#desk-db-sidebar` exclusively to the filterable schema tree (`#desk-db-object-filter` -> schema namespace `public (N)` -> clickable `Tables`, `Views`, and `Triggers` with row-count badges that immediately load rows into the grid).
- **Edge-to-Edge Excel-Style Spreadsheet Grid & Dedicated Sub-Tabs:**
  - Replaced stacked center form cards with a clean DbGate sub-tab bar (**`Data`** | **`Structure`** | **`SQL Query`** | **`ER Diagram`** | **`Export & Diff`**) and single-line toolbar (`Refresh`, `Inspect schema`, `Structure`, `SQL`, `Export`, `ER Diagram`, `Schema Diff`, `Undo Snapshot`, `Write SQL / Snapshot Guard`, `Inspector`).
  - Default **`Data`** view renders an edge-to-edge full-height `<table class="desk-grid-table">` with sticky row-number gutter (`1, 2, 3...`), two-row sticky headers showing column names + data types and per-column live filter inputs, `1px` cell borders, `28px` rows, alternating shading, `tabular-nums`, and click/double-click cell editing (`ADR-025c`).
- **Collapsed-by-Default Right Inspector Drawer (`#desk-db-inspector`):**
  - Collapsed to `0px` by default so the spreadsheet fills full workspace width, sliding open automatically on cell click/double-click (`ADR-025c`), **`Write SQL / Snapshot Guard`** (`GROXY-SAFE-001`), **`Undo Snapshot`**, or **`Inspector`** toggle.

### Stage 3 & Stage 4 Verification Summary (`SPEC-140` Acceptance Gates 1–4 & Stage 4 Sign-Off: `PASS`)

- **Stage 4 Pre-Release Cross-Layer Security & Contract Sign-Off (`PASS` — `Approved & Frozen — Complete`):** Verified `CGO_ENABLED=0` Windows & Linux amd64 builds/vets, `go mod verify`, `govulncheck ./...` (`0` vulnerabilities), `go test -count=1 ./...` (including `TestV140DBConnectionsEncryptedProfilesAndAuth`), `grab check src/desk.grab views/workbench.grab.html`, canonical `grab format` parity, `node --test tests/*.test.js` (`15/15` pass including `unit_v140_dbgate.test.js`, `divCount === 0`, `innerHTMLWriteCount === 0`, `auditDom()` `0` issues, Light & Dark WCAG AA `>= 4.5:1`), `TestNativeWebView2Isolation`, `python tests/integration_e2e.py`, `python tests/radar_live_e2e.py`, `python tests/network_dump_recovery.py`, and clean `git status --short` across all 10 sibling `v1.x` repositories.
