# Changelog — Groxy Desk

All notable releases of **Groxy Desk** (Step 6 of the GRAB Post-1.0 Ecosystem) are documented in this file.

---

## `v1.0.1` — 2026-11-18

### Release Type & Affected Layers
- **Release Version & SemVer Tier:** `v1.0.1` (`PATCH 1.0.x — Non-Breaking Bug/Security/Audit Patch`)
- **Affected Layer(s):** `Groxy Desk` (`urbanspress/groxy-desk`)
- **Neighboring Layer Upgrade Required?** `No — 100% drop-in compatible`

| Ecosystem Layer | Repository | Required Version Range | Touched in This Release? | Neighbor Upgrade Required? |
|---|---|---|---|---|
| **Core Language, Bytecode VM & Grisp Server** | `urbanspress/grab` | `>= v1.1.0 (< v2.0.0)` | `No` | `No` |
| **Reactive Frontend Runtime & 20-Element Library** | `urbanspress/grex-js` | `>= v1.0.0 (< v2.0.0)` | `No` | `No` |
| **Flagship Zero-Plugin CMS** | `urbanspress/grablab-cms` | `>= v1.0.1 (< v2.0.0)` | `No` | `No` |
| **Exclusive `mode: "visual"` Theme & Studio** | `urbanspress/grex-builder` | `>= v1.0.0 (< v2.0.0)` | `No` | `No` |
| **Autonomous Design, Content & SEO Agent** | `urbanspress/groxy-ai` | `>= v1.0.0 (< v2.0.0)` | `No` | `No` |
| **Scaffolder, Orchestrator & Discreet VPS Panel** | `urbanspress/grabman`, `urbanspress/grabtaculous`, `urbanspress/gashboard` | `>= v1.0.0 (< v2.0.0)` (aligned with `Grabtaculous v1.1.0`) | `No` | `No` |
| **Native Desktop Vibe-Coding IDE & Studio** | `urbanspress/groxy-desk` | `>= v1.0.0 (< v2.0.0)` | `Yes` | `No` |

### Fixes & Alignment (`specifications/GROXY-DESK-SPEC-101.md`)
- **NGINX `1.30.6` (`http2 on;`) & `GRAB v1.1.0` `--trusted-proxy` Edge Generator Alignment (`internal/dns/dns.go`, `internal/dns/dns_test.go`):**
  - Updated `renderEdgeHTTPSConfig` for `edgeProxy == "nginx"` to replace deprecated NGINX `1.24.x` `listen 443 ssl http2;` with `listen 443 ssl;`, `listen [::]:443 ssl;`, and `http2 on;`, matching official `nginx.org` NGINX `1.30.6` (Stable) deployed by **Grabtaculous `v1.1.0`**.
  - Added full forwarded header tuple inside `location /`: `proxy_set_header Host $host;`, `proxy_set_header X-Real-IP $remote_addr;`, `proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;`, `proxy_set_header X-Forwarded-Proto $scheme;`, and `proxy_set_header X-Forwarded-Host $host;`.
  - Added regression assertions in `TestDNSPlanEmailPreservationPreflightAndHTTPSApply`.
- **Packaged Release & Installer Parity (`Groxy-Desk-Setup-1.0.1.exe`):**
  - Rebuilt static `CGO_ENABLED=0` binaries (`groxy-desk-windows-amd64.exe`, `groxy-deskd-windows-amd64.exe`, `groxy-deskd-linux-amd64`), branded Inno Setup 6 `.exe` installer (`dist/Groxy-Desk-Setup-1.0.1.exe`), [`docs/releases/pdf/Groxy-Desk-v1.0.1-Guide.pdf`](docs/releases/pdf/Groxy-Desk-v1.0.1-Guide.pdf), `groxy-desk-v1.0.1-bundle.zip`, and `SHA256SUMS.txt`.

---

## `v1.0.0` — 2026-11-15

### Release Type & Affected Layers
- **Release Version & SemVer Tier:** `v1.0.0` (`GA 1.0.0 — Production Step 6 Desktop IDE Freeze & Branded Windows Installer`)
- **Affected Layer(s):** `Groxy Desk` (`urbanspress/groxy-desk`)
- **Neighboring Layer Upgrade Required?** `No — 100% drop-in compatible`

| Ecosystem Layer | Repository | Required Version Range | Touched in This Release? | Neighbor Upgrade Required? |
|---|---|---|---|---|
| **Core Language, Bytecode VM & Grisp Server** | `urbanspress/grab` | `>= v1.1.0 (< v2.0.0)` | `No` | `No` |
| **Reactive Frontend Runtime & 20-Element Library** | `urbanspress/grex-js` | `>= v1.0.0 (< v2.0.0)` | `No` | `No` |
| **Flagship Zero-Plugin CMS** | `urbanspress/grablab-cms` | `>= v1.0.1 (< v2.0.0)` | `No` | `No` |
| **Exclusive `mode: "visual"` Theme & Studio** | `urbanspress/grex-builder` | `>= v1.0.0 (< v2.0.0)` | `No` | `No` |
| **Autonomous Design, Content & SEO Agent** | `urbanspress/groxy-ai` | `>= v1.0.0 (< v2.0.0)` | `No` | `No` |
| **Scaffolder, Orchestrator & Discreet VPS Panel** | `urbanspress/grabman`, `urbanspress/grabtaculous`, `urbanspress/gashboard` | `>= v1.0.0 (< v2.0.0)` | `No` | `No` |
| **Native Desktop Vibe-Coding IDE & Studio** | `urbanspress/groxy-desk` | `>= v1.0.0 (< v2.0.0)` | `Yes` | `No` |

### Highlights
- **Pre-`v1.0.0` 7-Layer Audit Sign-Off & Production Contract Freeze (`specifications/GROXY-DESK-SPEC-100.md`):**
  - Freezes all **55 HTTP paths (57 route handlers)** on `127.0.0.1:9490`, all **13 unified `grab mcp` tools** (`grab_check`, `grab_format`, `grab_inspect_routes`, `grab_inspect_schema`, `grab_scaffold`, `list_content_types`, `query_entries`, `create_entry_draft`, `update_entry_layout`, `list_media`, `list_menus`, `studio_design_system`, `studio_apply_layout`), and the complete error taxonomy (`GRAB-E401`, `DESK-E101`, `DESK-E301`–`DESK-E304`, `DESK-E403`, `DESK-E405`, `DESK-E406`, `DESK-E409`, `DESK-E422`, `DESK-E501`–`DESK-E503`).
  - Includes audit-hardening guarantees: clean Ollama child-process teardown via `(*author.Manager).Shutdown()` wired into `(*server.Server).Shutdown()`, and pre-token `DESK-E303` DNS email re-verification + explicit `SPF`/`DKIM`/`DMARC` protection in `ApplyDNS`.
- **Branded Windows Inno Setup 6 Installer (`installer/groxy-desk.iss` -> `dist/Groxy-Desk-Setup-1.0.0.exe`):**
  - Compiles exclusively as an Inno Setup 6 `.exe` installer (**never `.msi`**) bundling `groxy-desk.exe`, `groxy-deskd.exe`, side-by-side `bin/grab.exe` (`GRAB v1.1.0`) + `bin/grabman.exe` (`grabman v1.0.0`), Pure `Grex JS v1.0.0` web assets, and `Groxy-Desk-v1.0.0-Guide.pdf`.
  - Installs to `{localappdata}\Groxy Desk` (`PrivilegesRequired=lowest`, zero mandatory UAC prompt) with automatic side-by-side CLI discovery (`bundledBinFallbacks` in `internal/workspace/workspace.go`) and clean uninstallation that preserves user workspaces, encrypted vaults (`v1:...`), and `GROXY-SAFE-001` Pre-Mutation Snapshots.
- **Full 4-Gate Production Verification (`docs/releases/pdf/Groxy-Desk-v1.0.0-Guide.pdf`):**
  - `100%` pass across Gate 1 (`go build ./...`, `go vet ./...`, `go test -count=1 ./...` across all 11 tested packages, `govulncheck ./...` `0` vulnerabilities, `grab check` + canonical `grab format` byte parity), Gate 2 (`node --test tests/*.test.js` `10/10` pass — `divCount === 0`, `innerHTMLWriteCount === 0`, Light & Dark WCAG AA `>= 4.5:1`, `ADR-025c`, zero model dropdowns), Gate 3, and Gate 4 (`.\dist\groxy-desk-windows-amd64.exe --smoke-test` + `python tests/integration_e2e.py`) with all 8 peer `v1.x` repositories strictly untouched.

---

## `v0.3.0` — 2026-11-15

### Release Type & Affected Layers
- **Release Version & SemVer Tier:** `v0.3.0` (`PRE-1.0 v0.3.0` — Step 6 Integrated Fleet, SSH Database Explorer, Namecheap DNS/Auto-HTTPS & Local Author Mode)
- **Affected Layer(s):** `Groxy Desk` (`urbanspress/groxy-desk`)
- **Neighboring Layer Upgrade Required?** `No — 100% drop-in compatible`

| Ecosystem Layer | Repository | Required Version Range | Touched in This Release? | Neighbor Upgrade Required? |
|---|---|---|---|---|
| **Core Language, Bytecode VM & Grisp Server** | `urbanspress/grab` | `>= v1.1.0 (< v2.0.0)` | `No` | `No` |
| **Reactive Frontend Runtime & 20-Element Library** | `urbanspress/grex-js` | `>= v1.0.0 (< v2.0.0)` | `No` | `No` |
| **Flagship Zero-Plugin CMS** | `urbanspress/grablab-cms` | `>= v1.0.1 (< v2.0.0)` | `No` | `No` |
| **Exclusive `mode: "visual"` Theme & Studio** | `urbanspress/grex-builder` | `>= v1.0.0 (< v2.0.0)` | `No` | `No` |
| **Autonomous Design, Content & SEO Agent** | `urbanspress/groxy-ai` | `>= v1.0.0 (< v2.0.0)` | `No` | `No` |
| **Scaffolder, Orchestrator & Discreet VPS Panel** | `urbanspress/grabman`, `urbanspress/grabtaculous`, `urbanspress/gashboard` | `>= v1.0.0 (< v2.0.0)` | `No` | `No` |
| **Native Desktop Vibe-Coding IDE & Studio** | `urbanspress/groxy-desk` | `>= v0.1.0` | `Yes` | `No` |

### Highlights
- **Step 5 Fleet & `Grabtaculous` Bridge (`internal/fleet/`, `public/desk/fleet-panel.js`):**
  - Connects over `known_hosts`-pinned SSH (`SHA256:...` fingerprint verification — `403 DESK-E405` on mismatch, zero `InsecureIgnoreHostKey`, zero public VPS admin portals) to remote `grabtaculousd` (`127.0.0.1:9440`).
  - Integrates `grabman` (`doctor`, `solve --verify`, `Site DNA` validation `422 DESK-E302`) and curated `--read-only` non-root `@sha256` deployments behind target-bound single-use `approval_token` + `X-CSRF-Token` (`GROXY-SAFE-001`) and automatic SHA-256 Pre-Deploy Snapshot (`snap_deploy_*`) rollback.
- **Pinned `127.0.0.1` SSH Database Tunnels & Visual DB Explorer (`internal/dbstudio/`, `public/desk/db-studio-panel.js`):**
  - Manages up to 8 concurrent `127.0.0.1:15432–15439` SSH tunnels (`409 DESK-E409`) for PostgreSQL & MariaDB plus workspace-confined local SQLite (`GRAB-E401`).
  - Implements schema/table inspection, bounded `<= 100` row pagination preserving exact `ADR-025c` base-10 wire strings, strict read-only `SELECT`/`EXPLAIN` queries (`403 DESK-E406` on unapproved writes), and `snap_db_*` gated writes/imports + 1-click rollback.
- **Encrypted Namecheap Vault & Domain -> DNS -> Auto-HTTPS Wizard (`internal/dns/`, `public/desk/dns-panel.js`):**
  - Encrypts Namecheap API keys/credentials at rest via `use crypto` `AES-256-GCM` (`v1:...`) with masked status responses (`nc_****...4321`).
  - Validates DNS record diffs to preserve existing `MX`/`TXT`/`SPF`/`DKIM`/`DMARC`/`CAA`/`SRV` email records byte-identical (`422 DESK-E303`), checks live DNS propagation before ACME issuance (`422 DESK-E304`), and configures Caddy, Nginx + Certbot, or Cloudflare Tunnel Auto-HTTPS to `127.0.0.1:8765–8772` with `snap_dns_*` rollback.
- **1-Click Local "Author Mode" & Optional Open-Weights Model Launcher (`internal/author/`, `public/desk/author-panel.js`):**
  - Scaffolds local **GrabLab CMS + Groxy AI** author workspaces (`Book`, `Chapter`, `CharacterBible`, `WorldLore` schemas, `status_enforced: "draft"`, `SITE-CONTEXT.md`, XSS-safe `schema.org/Book` JSON-LD with exact `ADR-025c` strings) alongside a loopback `127.0.0.1:11434` Ollama status check/launcher (zero required external fees, zero model-switching dropdowns).
- **Pure `Grex JS v1.0.0` UI Parity & Verification (`docs/releases/pdf/Groxy-Desk-v0.3.0-Guide.pdf`):**
  - `100%` pass across Gate 1 (`go build`, `go vet`, `go test -count=1 ./...` across all 11 test packages, `govulncheck ./...` `0` vulnerabilities, `grab check` + canonical `grab format` parity), Gate 2 (`node --test tests/*.test.js` `10/10` pass — `divCount === 0`, `innerHTMLWriteCount === 0`, Light & Dark WCAG AA `>= 4.5:1`), Gate 3, and Gate 4 (`.\dist\groxy-desk-windows-amd64.exe --smoke-test` + `python tests/integration_e2e.py`) while leaving all 8 peer `v1.x` repositories completely untouched.

---

## `v0.2.0` — 2026-11-15

### Release Type & Affected Layers
- **Release Version & SemVer Tier:** `v0.2.0` (`PRE-1.0` — Step 6 Full-Parity `grab mcp` Studio, Split-Preview Canvas & 22-Rule Booster)
- **Affected Layer(s):** `Groxy Desk` (`urbanspress/groxy-desk`)
- **Neighboring Layer Upgrade Required?** `No — 100% drop-in compatible`

| Ecosystem Layer | Repository | Required Version Range | Touched in This Release? | Neighbor Upgrade Required? |
|---|---|---|---|---|
| **Core Language, Bytecode VM & Grisp Server** | `urbanspress/grab` | `>= v1.1.0 (< v2.0.0)` | `No` | `No` |
| **Reactive Frontend Runtime & 20-Element Library** | `urbanspress/grex-js` | `>= v1.0.0 (< v2.0.0)` | `No` | `No` |
| **Flagship Zero-Plugin CMS** | `urbanspress/grablab-cms` | `>= v1.0.1 (< v2.0.0)` | `No` | `No` |
| **Exclusive `mode: "visual"` Theme & Studio** | `urbanspress/grex-builder` | `>= v1.0.0 (< v2.0.0)` | `No` | `No` |
| **Autonomous Design, Content & SEO Agent** | `urbanspress/groxy-ai` | `>= v1.0.0 (< v2.0.0)` | `No` | `No` |
| **Scaffolder, Orchestrator & Discreet VPS Panel** | `urbanspress/grabman`, `urbanspress/grabtaculous`, `urbanspress/gashboard` | `>= v1.0.0 (< v2.0.0)` | `No` | `No` |
| **Native Desktop Vibe-Coding IDE & Studio** | `urbanspress/groxy-desk` | `>= v0.1.0` | `Yes` | `No` |

### Added
- **Unified 13-Tool `grab mcp` Studio & Loopback Ecosystem Bridge (`internal/mcpbridge/`, `internal/server/v020_handlers.go`):**
  - Core GRAB `v1.1.0` stdio JSON-RPC 2.0 client (`grab mcp --workspace <root>`) exposing `grab_check`, `grab_format`, `grab_inspect_routes`, `grab_inspect_schema`, and `grab_scaffold` via structured `argv` slices (`exec.CommandContext`), strict `GRAB-E401` path/symlink/secret confinement, and read-only default (`--allow-write` gated strictly behind consumed `GROXY-SAFE-001` approval tokens).
  - GrabLab CMS `v1.0.1` loopback HTTP MCP client enforcing `DESK-E501` (rejecting non-loopback URLs), mandatory `status_enforced: "draft"` on `create_entry_draft`, and `HTTP 422 / JSON-RPC -32602` validation errors on invalid semantic/WCAG trees in `update_entry_layout`.
  - Grex Theme Builder `v1.0.0` studio proxy bridging `GET /api/design-system.md`, `SITE-CONTEXT.md`, and `POST /api/mcp/layout` with `actor: "groxy-ai"` and `applyAiLayout` `Ctrl+Z` command-stack inverse undo.
- **22-Rule Deterministic Auditor, 8 Style Packs & 🚀 `"Let Groxy Boost This"` Diff Engine (`internal/auditor/`, `internal/groxybridge/`):**
  - Evaluates all 4 equal-weight pillars (`on_page_seo`, `content_a11y`, `performance_dom`, `linking_readiness` — weight `"25"` each), enforces hard `F` on `<div>`, inline scripts, or inline `on*` event attributes, generates XSS-escaped `schema.org` JSON-LD previews, and preserves `ADR-025c` base-10 numeric wire strings.
  - Synthesizes WCAG AA Light/Dark (`>= 4.5:1`) zero-`<div>` layouts across all `8 Style Packs × 5 Blueprints` (`40` combinations) and powers 1-click `"Let Groxy Boost This"` remediation with before/after diffs, Pre-Mutation SHA-256 Snapshots, single-use target-bound `approval_token` (`403 DESK-E403` / `409 DESK-E409`), and 1-click rollback.
- **Interactive Split-Preview Workbench & Origin-Checked Bridge (`public/desk/studio-panel.js`, `public/desk/scorecard-panel.js`, `public/desk/workbench.js`, `public/desk/api-client.js`):**
  - Adds center workbench switcher (`Code` / `Split` / `Visual & Scorecard`), `Base` / `Tablet (768px)` / `Mobile (375px)` viewports, `GrexPreviewBridge` (`PREVIEW_CHANNEL = "grex-preview-v1"`) with strict loopback `allowedOrigins` enforcement (rejecting wildcard `"*"`), and full Pure `Grex JS v1.0.0` Light/Dark parity (`divCount === 0`, `innerHTMLWriteCount === 0`, zero `eval()`/`new Function()`, zero model-switching dropdowns).
- **Walkthrough Guide & Verification (`docs/releases/pdf/Groxy-Desk-v0.2.0-Guide.pdf`):**
  - Verified across Gate 1 (`go vet`, `go build`, `go test -count=1 ./...`, `govulncheck ./...`, `grab check`, canonical `grab format` parity), Gate 2 (`node --test tests/*.test.js`), Gate 3 (`GRAB-E401`, `DESK-E501`, `DESK-E403`, `DESK-E409`, `422/-32602`, `status_enforced: "draft"`, snapshot rollback), and Gate 4 (`python tests/integration_e2e.py`).

---

## `v0.1.0` — 2026-11-15

### Release Type & Affected Layers
- **Release Version & SemVer Tier:** `v0.1.0` (`PRE-1.0` — Step 6 Native Desktop IDE Foundation)
- **Affected Layer(s):** `Groxy Desk` (`urbanspress/groxy-desk`)
- **Neighboring Layer Upgrade Required?** `No — 100% drop-in compatible`

| Ecosystem Layer | Repository | Required Version Range | Touched in This Release? | Neighbor Upgrade Required? |
|---|---|---|---|---|
| **Core Language, Bytecode VM & Grisp Server** | `urbanspress/grab` | `>= v1.1.0 (< v2.0.0)` | `No` | `No` |
| **Reactive Frontend Runtime & 20-Element Library** | `urbanspress/grex-js` | `>= v1.0.0 (< v2.0.0)` | `No` | `No` |
| **Flagship Zero-Plugin CMS** | `urbanspress/grablab-cms` | `>= v1.0.1 (< v2.0.0)` | `No` | `No` |
| **Exclusive `mode: "visual"` Theme & Studio** | `urbanspress/grex-builder` | `>= v1.0.0 (< v2.0.0)` | `No` | `No` |
| **Autonomous Design, Content & SEO Agent** | `urbanspress/groxy-ai` | `>= v1.0.0 (< v2.0.0)` | `No` | `No` |
| **Scaffolder, Orchestrator & Discreet VPS Panel** | `urbanspress/grabman`, `urbanspress/grabtaculous`, `urbanspress/gashboard` | `>= v1.0.0 (< v2.0.0)` | `No` | `No` |
| **Native Desktop Vibe-Coding IDE & Studio** | `urbanspress/groxy-desk` | `>= v0.1.0` | `Yes` | `No` |

### Added
- **Loopback Host Daemon & Native Windows WebView2 Launcher (`cmd/groxy-deskd/`, `cmd/groxy-desk/`, `internal/server/`, `internal/security/`):**
  - Strictly binds to `127.0.0.1:9490` (fail-closed `DESK-E501` on `0.0.0.0`, `::`, or non-loopback hosts) using `CGO_ENABLED=0` Go + `go-webview2` COM desktop integration.
  - Enforces constant-time `Authorization: Bearer` + `X-CSRF-Token` validation (`403 DESK-E403`), memory-only WebView2 postMessage session injection, CSP `script-src 'self'`, `X-Frame-Options: DENY`, and `GRAB-E401` workspace confinement (blocking `..`, NTFS directory junctions/symlinks, `.env`, `.git`, `*.pem`, `*.key`, `id_rsa`, `id_ed25519`).
- **Local `Grisp` (`grab.exe v1.1.0`) Multi-Worker Supervisor (`internal/supervisor/`):**
  - Manages up to 8 concurrent GRAB workers on loopback ports `127.0.0.1:8765–8772` with start, stop, `grab check`-gated atomic reload (preserving last-known-good worker on syntax error), live `/_health` and `/_ready` polling, secret-redacted 256-line ring-buffer logs, and graceful shutdown.
- **Built-In `grabman v1.0.0` Workspace Manager (`internal/workspace/`):**
  - Integrates `grabman init` (behind single-use target-bound `approval_token` gating), `grabman doctor`, `grabman solve --verify`, confined tree exploration, and `grab check` / `grab format` canonical byte parity directly into the left project sidebar.
- **Unified `Groxy AI` Default Agent & Adaptive Reasoning Router (`internal/groxybridge/`, `public/desk/groxy-agent.js`):**
  - Permanent sole assistant identity (`groxy-ai`, zero model-switching dropdowns) with deterministic 22-rule SEO/A11Y/DOM/Link audit (`fast` path) and `GROXY-SAFE-001` Plan -> Pre-Mutation SHA-256 Snapshot -> single-use target-bound `approval_token` (`403 DESK-E403` mismatch / `409 DESK-E409` replay) -> Execute + 1-click snapshot rollback (`deep` path).
- **Pure `Grex JS v1.0.0` Light/Dark IDE Workbench (`views/`, `public/desk/`, `public/grex/`):**
  - Vendored byte-identical `grex.js` + `grex.elements.json` rendering strictly via the 20 semantic Grex elements (`divCount === 0`, `innerHTMLWriteCount === 0`, zero `eval()` / `new Function()`, 100% WCAG AA Light/Dark `>= 4.5:1` contrast, exact `ADR-025c` base-10 wire strings, zero-FOUC theme persistence).
- **Release Packaging & Walkthrough Guide (`docs/releases/pdf/Groxy-Desk-v0.1.0-Guide.pdf`):**
  - Static `CGO_ENABLED=0` binaries (`groxy-desk-windows-amd64.exe`, `groxy-deskd-windows-amd64.exe`, `groxy-deskd-linux-amd64`), walkthrough PDF, release ZIP bundle, and `SHA256SUMS.txt`.
