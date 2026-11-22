# Groxy Desk Roadmap

**Repositories:** `urbanspress/groxy-desk.progress` (active development) / `urbanspress/groxy-desk.stable` (verified releases)  
**Foundational Baselines:** **GRAB `v1.1.0`**, **Grex JS `v1.0.0`**, **GrabLab CMS `v1.0.1`**, **Grex Theme Builder `v1.0.0`**, **Groxy AI `v1.0.0`**, **`grabman v1.0.0`**, **Grabtaculous `v1.0.0`**, and **Gashboard `v1.0.0`**

**Status Labels:**
- **Approved target; specification in review** — Scope approved and the specification awaiting separate implementation approval.
- **Specification in Review** — Specification drafted and awaiting explicit project-owner approval before any Go, GRAB, C/Win32, or JS application code is written.
- **Approved** — Authorized by the project owner for implementation.
- **Complete** — Implemented, all 4 verification gates passing `100%`, walkthrough PDF generated, and synchronized to `origin` + `stable`.
- **Planned** — Future milestone awaiting prior milestone completion and spec drafting.

---

## `v0.1.0` — Native Desktop Shell, Local `Grisp` Multi-Worker Supervisor, Workspace Manager & Unified `Groxy AI` Host Bridge

**Status:** Complete ([`specifications/GROXY-DESK-SPEC-001.md`](../specifications/GROXY-DESK-SPEC-001.md)) — Released `v0.1.0` (All 4 Gates Passed)

Target Scope:
- **Native Windows WebView2 + `Grex JS v1.0.0` Zero-`<div>` Workbench:** Lightweight native desktop shell paired with a `127.0.0.1:9490` Go host daemon (`CGO_ENABLED=0`), zero-FOUC Light/Dark WCAG AA (`>= 4.5:1`) theme parity, and semantic 20-element layout (`divCount === 0`).
- **Local `Grisp` (`grab.exe`) Multi-Worker Supervisor:** 1-click start, stop, and atomic `--reload` for local GRAB / GrabLab / Grex sites on private loopback ports (`127.0.0.1:8765–8772`) with live `/_health` and `/_ready` telemetry and structured log streaming.
- **Workspace Project Manager (`grabman v1.0.0`):** Integrated UI for `grabman init`, `grabman doctor`, and `grabman solve --verify` with `GRAB-E401` path confinement and plain-English diagnostics.
- **Unified `Groxy AI` Default Agent Bar & Adaptive Reasoning Router:** Single permanent built-in agent identity (zero model-switching dropdowns) automatically routing instant daily checks vs. deep multi-step reasoning locally via Ollama/OpenCode or optional encrypted provider keys.

---

## `v0.2.0` — Full-Parity `Groxy AI` Vibe-Coding Co-Pilot, Native `grab mcp` Bridge & Live `Grex` Split-Preview Canvas

**Status:** Complete ([`specifications/GROXY-DESK-SPEC-002.md`](../specifications/GROXY-DESK-SPEC-002.md) | [Walkthrough PDF](releases/pdf/Groxy-Desk-v0.2.0-Guide.pdf)) — Released as `v0.2.0`

Target Scope:
- **Full `grab mcp` Tool Parity:** Native execution of Core GRAB (`grab_check`, `grab_format`, `grab_inspect_routes`, `grab_inspect_schema`, `grab_scaffold`), GrabLab CMS (`list_content_types`, `query_entries`, `create_entry_draft` with `status_enforced: "draft"`, `update_entry_layout`, `list_media`, `list_menus`), and Grex Theme Builder (`GET /api/design-system.md`, `POST /api/mcp/layout` with `actor: "groxy-ai"`).
- **22-Rule Scorecard & 🚀 `"Let Groxy Boost This"`:** Embedded 4-pillar SEO/WCAG/DOM/Link audit (`A+`–`F` / `0–100`), side-by-side remediation diffs, `SITE-CONTEXT.md` internal linking & `schema.org` JSON-LD generation, media `alt`-text enrichment, and all 8 Design Style Packs (40 blueprint combinations).
- **Live Split-Screen `Grex` Preview Canvas:** Real-time code/visual split pane with 1-click `Ctrl+Z` (`applyAiLayout`) command-stack undo parity and `GROXY-SAFE-001` target-bound approval tokens.

---

## `v0.3.0` — Integrated `Gashboard` + `Grabtaculous` Fleet Deployer, Pinned SSH DB Tunnels, Visual DB Explorer, Namecheap DNS + Auto-HTTPS & 1-Click Local Author Mode

**Status:** Complete ([`specifications/GROXY-DESK-SPEC-003.md`](../specifications/GROXY-DESK-SPEC-003.md) | [Walkthrough PDF](releases/pdf/Groxy-Desk-v0.3.0-Guide.pdf)) — Released as `v0.3.0`

Target Scope:
- **Step 5 Fleet & `Grabtaculous` Bridge (`internal/fleet/`):** `known_hosts`-pinned SSH (`SHA256:...` fingerprint verification, zero `InsecureIgnoreHostKey`) to remote `grabtaculousd` (`127.0.0.1:9440`), `grabman` (`doctor`, `solve --verify`, `dna validate`/`diff`), 1-click `--read-only` Marketplace stack deployments, `groxy-site-dna-v0.4.0` fleet rollouts, and atomic SHA-256 Pre-Deploy Snapshot rollbacks.
- **Pinned `127.0.0.1` SSH Database Tunnels & Visual DB Explorer (`internal/dbstudio/`):** Built-in `known_hosts`-pinned loopback SSH tunnels (`<= 8` concurrent on `127.0.0.1:15432–15439`) + confined local SQLite (`GRAB-E401`), read-only-default schema/row explorer (`<= 100` rows/page, `ADR-025c` exact base-10 wire strings), and `GROXY-SAFE-001` single-use `approval_token` + Pre-Mutation Snapshots for any write/DDL/SQL import.
- **Encrypted Namecheap DNS Vault & Auto-HTTPS Wizard (`internal/dns/`):** `use crypto` `AES-256-GCM` (`v1:...`) credential vault, email-safe (`MX`/`TXT`/`SPF`/`DKIM`-preserving) DNS updates, `422` ACME propagation preflight (protecting Let's Encrypt rate limits), and Caddy / Nginx+Certbot / Cloudflare Tunnel wiring with 1-click rollback.
- **1-Click Local "Author Mode" & Optional Open-Weights Model Launcher (`internal/author/`):** 1-click local setup card initializing **GrabLab CMS + Groxy AI** for **Author Mode** (`Book`, `Chapter`, `CharacterBible`, `WorldLore` schemas + `schema.org/Book` JSON-LD preview) alongside an optional local Ollama open-weights status check/launcher (zero required external API fees, zero model-switching dropdowns).

---

## `v1.0.0` — Independent Cross-Layer Audit, Branded Inno Setup Installer & Production Freeze

**Status:** Approved & Frozen — Complete ([`specifications/GROXY-DESK-SPEC-100.md`](../specifications/GROXY-DESK-SPEC-100.md) | [Walkthrough PDF](releases/pdf/Groxy-Desk-v1.0.0-Guide.pdf)) — Released as `v1.0.0`

Target Scope:
- Pre-`v1.0.0` cross-layer security, IPC (`GrexPreviewBridge`), SSH pinning/tunnels, DB isolation, Namecheap DNS/HTTPS, Author Mode, Pure `Grex JS v1.0.0` DOM/WCAG AA, `ADR-025c`, and `GROXY-SAFE-001` audit (**Completed**, including `author.Manager.Shutdown()` teardown wiring in `server.Shutdown` and pre-token `DESK-E303` DNS email re-verification + explicit `SPF`/`DKIM`/`DMARC` protection; Gates 1–4 `100%` passing).
- Branded flicker-free Inno Setup Windows installer (`installer/groxy-desk.iss` -> `dist/Groxy-Desk-Setup-1.0.0.exe`, Inno Setup `.exe` only), `SHA256SUMS.txt`, plain-English walkthrough PDF (`docs/releases/pdf/Groxy-Desk-v1.0.0-Guide.pdf`), frozen `55` path / `57` route handler + `13` `grab mcp` tool contracts, and synchronized `v1.0.0` promotion to `origin` and `stable`.

---

## `v1.0.1` — NGINX `1.30.6` Edge Generator Alignment & Trusted-Proxy Header Completeness (`PATCH 1.0.x`)

**Status:** Complete ([`specifications/GROXY-DESK-SPEC-101.md`](../specifications/GROXY-DESK-SPEC-101.md) | [Walkthrough PDF](releases/pdf/Groxy-Desk-v1.0.1-Guide.pdf)) — Released as `v1.0.1`

Completed Scope:
- Updated `renderEdgeHTTPSConfig` in `internal/dns/dns.go` (`edgeProxy == "nginx"`) to emit NGINX `1.30.6` (`listen 443 ssl;`, `listen [::]:443 ssl;`, `http2 on;`) instead of deprecated `listen 443 ssl http2;`, and added full `Host`, `X-Real-IP`, `X-Forwarded-For`, `X-Forwarded-Proto $scheme;`, and `X-Forwarded-Host $host;` forwarding headers for `GRAB v1.1.0` `--trusted-proxy` parity with **Grabtaculous `v1.1.0`**.
- Verified regression test coverage in `internal/dns/dns_test.go`, passed all 4 verification gates (`go build`, `go vet`, `go test -count=1`, `govulncheck`, `grab check`/`format`, `node --test`, `python tests/integration_e2e.py`), packaged `dist/Groxy-Desk-Setup-1.0.1.exe` + `Groxy-Desk-v1.0.1-Guide.pdf` + `SHA256SUMS.txt`, and synchronized `v1.0.1` across `origin` and `stable`.

---

## `v1.1.0` — Built-In Browser Co-Pilot, Bestseller Radar Side-Panel & DbGate-Parity Visual Database Workbench

**Status:** Approved & Frozen — Complete (**Stage 4 Pre-Release Cross-Layer Security & Contract Sign-Off: PASS — Released as `v1.1.0`**) ([`specifications/GROXY-DESK-SPEC-110.md`](../specifications/GROXY-DESK-SPEC-110.md) | [Walkthrough PDF](releases/pdf/Groxy-Desk-v1.1.0-Guide.pdf))

Completed Stage 3 Scope (All Section 9 Acceptance Gates Passed):
- **Full PostgreSQL 18 & Least-Privilege MariaDB 11.8 Recovery Gate Lifted:** Verified full logical replay, writer exclusion across the pre-check → restore → post-verify window (leaving unrelated databases connected), conflict rejection (`409`), forced recovery, CSV/XLSX/JSON/SQL import/export round-trips, cross-dialect schema-sync migrations, and MariaDB partial-DDL failure rollback to exact pre-mutation SHA-256 (tables, views, and triggers intact) through Desk's HTTP approval and rollback endpoints. MariaDB targets with any account holding global `*.*` write privileges continue to fail closed with HTTP `409 write_fence_unavailable` before approval.

Target Scope:
- **WebView2 Browser Co-Pilot:** Side-by-side research and registration browsing with approved `BRAND-DNA.md` form prefill. Users control passwords, CAPTCHAs, and submission.
- **Bestseller Radar:** Groxy AI `v1.8.0` consumes the single-use `rtok_res_<hex>` approval and returns a Desk-bound draft receipt. Separate approval creates a GrabLab CMS `v1.4.0`/`v1.5.0` book draft through the allowlisted MCP endpoint.
- **SSH-Tunneled Loopback Connections (`127.0.0.1`):** Workspace-confined SQLite, capability-gated remote SQLite, and `known_hosts` `SHA256:...`-pinned connections for PostgreSQL and MariaDB/MySQL on native Ubuntu `systemd` or containerized services.
- **Multi-Tab Autocomplete SQL Editor & Spreadsheet-Style Inline Cell Editing:** Schema-aware SQL autocomplete across tabs plus inline spreadsheet-style cell editing preserving exact `ADR-025c` base-10 `GrabDecimal` and `GrabInteger` wire strings (zero IEEE-754 float drift).
- **Visual ER Schema Diagrams & 1-Click Import/Export:** Interactive entity-relationship (ER) schema diagrams rendered in pure `Grex JS v1.0.0` (`divCount === 0`, `innerHTMLWriteCount === 0`, Light/Dark WCAG AA `>= 4.5:1`), plus 1-click **CSV**, **Excel (`.xlsx`)**, **JSON**, and **SQL dump** import/export.
- **Cross-Environment Schema Diffing & `GROXY-SAFE-001` Safety Gate:** Side-by-side schema diffing between **Local SQLite** and **VPS PostgreSQL/MariaDB**, with every mutating write, DDL change, import, or schema sync gated by a single-use `approval_token`, `X-CSRF-Token`, and automatic **SHA-256 Pre-Mutation Snapshot** with 1-click rollback.

---

## `v1.2.0` — DbGate-Parity 3-Tab Dedicated Workspace Studio & Visual Polish (`Database Studio` | `Browser & Radar` | `Code & Grex Studio`)

**Status:** Approved & Frozen — Complete (**Stage 4 Pre-Release Cross-Layer Security & Contract Sign-Off: PASS — Released as `v1.2.0`**) ([`specifications/GROXY-DESK-SPEC-120.md`](../specifications/GROXY-DESK-SPEC-120.md) | [Walkthrough PDF](releases/pdf/Groxy-Desk-v1.2.0-Guide.pdf))

Target Scope:
- **Sticky Top Workspace Mode Switcher & `Urbanist` Typography:** Clean sticky top header with `Urbanist` geometric typography, authenticated loopback status badge, Light/Dark theme toggle, and 3 dedicated full-viewport workspace tabs (`role="tablist"` with `ArrowLeft`/`ArrowRight`/`Home`/`End` keyboard navigation and URL hash/session persistence) while preserving shared workspace context, active SSH tunnels, and Unified `Groxy AI` state.
- **Tab 1 — `Database Studio` (True DbGate 3-Pane Layout):** Left sidebar (saved/active `PostgreSQL`, `MariaDB`, `SQLite` connections + expandable `Tables`/`Views`/`Triggers` schema tree), center workspace (focused action toolbar: `Run Query`, `Export CSV/XLSX/JSON/SQL`, `ER Diagram`, `Schema Diff`, `Undo Snapshot` + multi-tab autocomplete SQL editor over spreadsheet-style data grid), and right inspector (`ADR-025c` exact-decimal editor, pre-mutation snapshot guard, and `GROXY-SAFE-001` approval/rollback drawer).
- **Tab 2 — `Browser & Radar` (Full-Height 50/50 Split Pane):** Isolated WebView2 allowlisted browser with origin lock badge on the left, paired with Bestseller Radar niche explorer, `BRAND-DNA.md` prefill preview, and `GrabLab CMS` draft handoff panel on the right.
- **Tab 3 — `Code & Grex Studio` (Clean 3-Pane Vibe-Coding Studio):** Workspace file explorer & `grabman` tools on the left, active `.grab`/`.html` code workbench & responsive `Grex` visual preview canvas in the center, and Unified `Groxy AI` Adaptive Reasoning co-pilot & 22-rule audit scorecard on the right.
- **Zero Regression & Pure `Grex JS v1.0.0` Conformance:** 100% preservation of existing `v1.1.0` DOM element IDs, ARIA attributes, and API bindings (`divCount === 0`, `innerHTMLWriteCount === 0`, zero `eval()`, Light & Dark WCAG AA `>= 4.5:1` contrast).

---

## `v1.3.0` — VS Code / DbGate / Coolify High-Density Desktop IDE Shell

**Status:** Approved & Frozen — Complete (**Stage 4 Pre-Release Cross-Layer Security & Contract Sign-Off: PASS — Released as `v1.3.0`**) ([`specifications/GROXY-DESK-SPEC-130.md`](../specifications/GROXY-DESK-SPEC-130.md) | [Walkthrough PDF](releases/pdf/Groxy-Desk-v1.3.0-Guide.pdf))

Target Scope:
- **Locked `100vh` High-Density IDE Shell & Dark Obsidian / Clean Light Palettes (`public/desk/desk.css`, `public/desk/theme.js`):** Zero page scroll (`height: 100vh; max-height: 100vh; overflow: hidden` on `html, body, #desk-root`), Dark Obsidian (`#0f1115` / `#181b20`) and Clean Light (`#f8fafc` / `#ffffff`) token system with WCAG AA `>= 4.5:1` text and `>= 3.0:1` UI contrast, `1px solid var(--grex-border)` hairline pane dividers, and custom `26px–28px` dark/light semantic inputs (zero unstyled white browser inputs).
- **`32px` Top IDE Menu & Command Bar (`<header id="desk-header" class="desk-ide-header" role="banner">`):** Compact application menu bar (`<nav role="menubar">` with `File`, `Edit`, `View`, `Database`, `Browser`, `Tools`, `Help` and keyboard access keys), centered command search bar (`#desk-command-search`, `Ctrl+K`), active workspace badge, `#desk-host-status-badge` (`#desk-connection-badge`), and `#desk-theme-toggle`.
- **`48px` Far-Left Vertical Activity Icon Rail (`<nav id="desk-activity-rail" class="desk-activity-rail" role="tablist" aria-label="Activity Rail">`):** Crisp SVG icons (`grab-ide-icons` via Pure Grex JS `Icon`) switching panes cleanly (`#desk-tab-code` Explorer & Code, `#desk-tab-database` Database Studio, `#desk-tab-browser` Browser & Radar, `#desk-tab-grex` Grex Visual Builder, `#desk-tab-fleet` Server & Fleet, plus bottom-anchored `#desk-action-settings` and `#desk-action-shortcuts`) with WAI-ARIA keyboard navigation and `#studio=database|browser|code` hash/session persistence.
- **Main Locked Stage (`.desk-stage-locked`, `height: calc(100vh - var(--desk-header-h) - var(--desk-statusbar-h))`):** `240px` collapsible left sidebars (`#desk-db-sidebar` with `#desk-db-object-filter`), `28px`-row-height sticky-header zebra-striped spreadsheet grid (`.desk-grid-table`, `font-variant-numeric: tabular-nums`), `280px` right inspector (`#desk-db-inspector`), and Coolify-density Server & Fleet health pills.
- **`24px` Pinned Bottom IDE Status Bar (`<footer id="desk-status-bar" class="desk-ide-statusbar" role="status">`):** Live `#desk-status-db` connection pill (1-click connect/disconnect), `#desk-status-ssh` pinned `SHA256:...` fingerprint, `#desk-status-safety` (`GROXY-SAFE-001` guard state), `#desk-status-grisp` active worker pool, and `#desk-status-adr025c` exact-wire indicator, preserving 100% of frozen `v1.2.0` DOM element IDs and invariants.

---

## `v1.3.1` — Remote SSH schema inspection latency

**Status:** Approved & Frozen — Complete ([SPEC-131](../specifications/GROXY-DESK-SPEC-131.md) | [Walkthrough PDF](releases/pdf/Groxy-Desk-v1.3.1-Guide.pdf)).

- Batch catalog metadata and exact row counts for PostgreSQL and MariaDB/MySQL over pinned SSH. Apply a 20-second inspection deadline.
- Verify the delayed 40-table regression, existing compatibility and safety gates, and synchronized installer and release assets.
