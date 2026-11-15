# GROXY-DESK-SPEC-100: Groxy Desk `v1.0.0` — Cross-Layer Audit Sign-Off, GA Contract Freeze & Branded Inno Setup Installer

- **Status:** Specification in Review (Awaiting Project-Owner Approval — **Pre-`v1.0.0` 7-Layer Audit Complete**)
- **Target Release:** `v1.0.0` (`GA 1.0.0` — Production Freeze & Branded Windows Installer for **Step 6 of the Post-1.0 GRAB Ecosystem**)
- **Foundational Baselines:**
  - **GRAB:** `v1.1.0` (`urbanspress/grab` — `Grisp`, `grab serve`, `grab check`, `grab format`, `grab mcp`, `ADR-005`, `ADR-017`, `ADR-025c`)
  - **Grex JS:** `v1.0.0` (`urbanspress/grex-js` — 20 semantic elements, reactive signals, `createTheme`, `computeContrastRatio`, `auditAccessibility`, `GREX-SCHEMA-001`)
  - **GrabLab CMS:** `v1.0.1` (`urbanspress/grablab-cms`; 100% forwards-compatible with planned minor `v1.4.0` `mode: "author"`)
  - **Grex Theme Builder:** `v1.0.0` (`urbanspress/grex-builder` — `GET /api/design-system.md`, `POST /api/mcp/layout`, `applyAiLayout` undo stack, 8 Design Style Packs / 40 presets)
  - **Groxy AI:** `v1.0.0` (`urbanspress/groxy-ai`; 100% forwards-compatible with planned minor `v1.7.0` Multi-Genre Author Brain & `schema.org/Book` JSON-LD; `groxy-site-dna-v0.4.0`, 22-rule 4-pillar Scorecard, Automatic Adaptive Reasoning Router)
  - **`grabman`, Grabtaculous & Gashboard:** `v1.0.0` Step 5 Freeze (`urbanspress/grabman`, `urbanspress/grabtaculous`, `urbanspress/gashboard`)
- **Governance:** Read alongside [`GROXY-DESK-EXPERT.md`](../GROXY-DESK-EXPERT.md), [`AGENTS.md`](../AGENTS.md), [`docs/ROADMAP.md`](../docs/ROADMAP.md), and [`GROXY-DESK-SPEC-001.md`](GROXY-DESK-SPEC-001.md) through [`GROXY-DESK-SPEC-003.md`](GROXY-DESK-SPEC-003.md).

---

## 1. Executive Summary & Plain-English Overview

**Groxy Desk `v1.0.0`** promotes our completed, verified desktop IDE from pre-`1.0` (`v0.3.0`) to a **Production `v1.0.0` GA Release** with a zero-hassle **Inno Setup Windows Installer (`Groxy-Desk-Setup-1.0.0.exe`)** and an independent **Pre-`v1.0.0` 7-Layer Security, SSH, DB, DNS, Author Mode & Pure `Grex JS` Audit Sign-Off**.

From day one on a clean Windows PC, double-clicking **`Groxy-Desk-Setup-1.0.0.exe`** gives a creator or developer:
- **Zero-Config Loopback Desktop App:** Lightweight native Windows WebView2 shell (~15–25 MB UI overhead, zero Electron bloat) paired with the static Go `groxy-deskd` daemon (`127.0.0.1:9490`) and bundled **`grab.exe` (`v1.1.0`)** + **`grabman.exe` (`v1.0.0`)** CLIs.
- **One Unified AI Partner (`Groxy AI`) — Zero Model-Switching Dropdowns:** The **Automatic Adaptive Reasoning Router** chooses between fast daily checks and deep multi-step architecture behind the scenes, with local open-weights **Ollama (`127.0.0.1:11434`)** support and zero mandatory cloud API fees.
- **Complete Post-1.0 Studio & Step 5 Operations:** Local `Grisp` multi-worker supervisor (`127.0.0.1:8765–8772`), all **13** unified `grab mcp` tools across Core GRAB + GrabLab CMS + Grex Theme Builder, live split-screen preview canvas (`Ctrl+Z` `applyAiLayout` undo), 🚀 **"Let Groxy Boost This"** 22-rule SEO/WCAG/DOM/Link improvements, Step 5 `known_hosts`-pinned Fleet deploys, `127.0.0.1:15432–15439` SSH DB tunnels + read-only-default Visual DB Explorer, encrypted Namecheap DNS Vault + Auto-HTTPS Wizard, and 1-Click Local **Author Mode**.

> **Zero Frozen Upstream Edits:** `Groxy Desk v1.0.0` does **not** alter any frozen `v1.x` code or contracts in `GRAB`, `Grex JS`, `GrabLab CMS`, `Grex Theme Builder`, `Groxy AI`, `grabman`, `grabtaculous`, or `gashboard`.

---

## 2. Pre-`v1.0.0` 7-Layer Audit Summary & Remediations Landed

Prior to drafting this specification, all Go packages (`cmd/*`, `internal/*`), `.grab` / `.grab.html` files, Pure `Grex JS v1.0.0` UI modules (`public/desk/*`), and integration suites (`tests/*`) completed a full pre-`v1.0.0` cross-layer security, IPC, SSH, DB, DNS, Author Mode, `ADR-025c`, and DOM audit.

### 2.1 Audit Findings & Edge-Case Hardening Applied Ahead of `v1.0.0`
1. **Clean Local Ollama Subprocess Teardown on Shutdown (`internal/author/author.go`, `internal/server/server.go`):**
   - Added `AuthorManager.Shutdown()` (`(*Manager).Shutdown()` in `internal/author/author.go`) and wired it directly into `(*Server).Shutdown(ctx)` in `internal/server/server.go` alongside `dbStudio.CloseAll()` and `supervisorMgr.ShutdownAll()` so any daemon-started `ollama serve` child process is cleanly terminated and reaped when Groxy Desk exits.
2. **Pre-Token DNS Email Preservation Re-Verification & Explicit `SPF`/`DKIM`/`DMARC` Protection (`internal/dns/dns.go`, `internal/dns/dns_test.go`):**
   - Hardened `ApplyDNS` to decrypt the Namecheap vault, fetch live `existing` records, and execute `VerifyEmailRecordsPreserved(existing, req.ProposedRecords)` **before** calling `approvals.ConsumeToken` or `namecheapSetHosts`, preventing single-use approval token consumption or remote writes if an apply payload was tampered with after planning (`422 DESK-E303`).
   - Added explicit `"SPF"`, `"DKIM"`, and `"DMARC"` RR type handling alongside `"MX"`, `"TXT"`, `"CAA"`, `"SRV"`, `"NS"`, and email `CNAME`s (`_domainkey`, `autodiscover`, `mail`, `_dmarc`) in `isProtectedEmailOrServiceRecord` and `stripEmailRecords`, with new regression assertions verified in `TestDNSPlanEmailPreservationPreflightAndHTTPSApply`.

### 2.2 Verified Pre-`v1.0.0` Security & Isolation Guarantees
- **Loopback Control Plane & IPC (`ADR-005`, `ADR-017`):**
  - `ValidateLoopbackBind` fails closed (`DESK-E501`) on `0.0.0.0`, `::`, wildcards, or LAN/public hosts; `groxy-deskd` binds exclusively to `127.0.0.1` (`127.0.0.1:9490` default).
  - Constant-time per-session `Authorization: Bearer` validation (`subtle.ConstantTimeCompare`) + `X-CSRF-Token` on mutating `POST` routes (`403 DESK-E403`), kept strictly in JS closure memory (`createDeskApiClient`).
  - Browser CSP emits `default-src 'self'; script-src 'self'; style-src 'self'; object-src 'none'; frame-ancestors 'none'` and `X-Content-Type-Options: nosniff`.
  - `GrexPreviewBridge` (`PREVIEW_CHANNEL = "grex-preview-v1"`) rejects wildcard `"*"` and non-loopback `postMessage` origins, allowing only dynamically allowlisted `127.0.0.1` worker origins.
  - All CLI/supervisor/MCP/Git/Ollama subprocesses spawn strictly via `exec.Command` / `exec.CommandContext` structured `argv` slices (`ValidateSafeCommandBinary` blocks shells like `cmd`, `powershell`, `sh`, `bash`).
- **SSH Host-Key Pinning, Tunnels & Visual DB Explorer Safety:**
  - Zero `InsecureIgnoreHostKey`; Fleet SSH and DB tunnels verify `known_hosts` `SHA256:...` fingerprints (`403 DESK-E405` on mismatch/absence).
  - Maximum `8` concurrent SSH DB tunnels bound exclusively to `127.0.0.1:15432–15439` (`409 DESK-E409`) with clean shutdown cleanup.
  - Workspace-confined SQLite (`GRAB-E401`), `limit <= 100` pagination (`400 DESK-E101`), strict read-only `SELECT`/`EXPLAIN` default blocking multi-statement SQL / DDL / DML / `ATTACH DATABASE` / write `PRAGMA` (`403 DESK-E406`), and target-bound `approval_token` + SHA-256 Pre-Mutation Snapshot (`snap_db_*`, `snap_deploy_*`) gating (`GROXY-SAFE-001`).
- **Encrypted Namecheap Vault & Auto-HTTPS:**
  - `use crypto` `AES-256-GCM` (`v1:...`) credential vault never echoes raw API keys (`secret_masked`), preserves protected email/service records byte-for-byte (`422 DESK-E303`), validates live DNS propagation before ACME provisioning (`422 DESK-E304`), and emits Caddy / Nginx + Certbot / Cloudflare Tunnel configurations targeting `127.0.0.1:8765–8772` with `snap_dns_*` rollback.
- **Pure `Grex JS v1.0.0`, `ADR-025c` Precision & Zero Model Dropdowns:**
  - Vendored `public/grex/grex.js` and `public/grex/grex.elements.json` match `C:\Development\grex-js` byte-for-byte via SHA-256 verification.
  - Zero `<div>`, zero `innerHTML` writes, zero `eval()`/`new Function()`, zero unsafe URLs, zero `<select>` model dropdowns across both `#desk-groxy-sidebar` and `#desk-author-panel`, `100%` WCAG AA (`>= 4.5:1`) across Light & Dark palettes, and exact base-10 wire string preservation (`GrabDecimal`/`GrabInteger`) in DB Studio tables, Site DNA diagnostics, Boost diffs, and XSS-safe `schema.org/Book` JSON-LD.

---

## 3. Frozen `v1.0.0` Loopback Daemon Contract (`127.0.0.1:9490`)

In `v1.0.0`, Groxy Desk freezes its **55 HTTP paths (57 route handlers)** across `internal/server/server.go`, `internal/server/v020_handlers.go`, and `internal/server/v030_handlers.go`. All post-`1.0.0` backwards-compatible additions MUST remain additive (`MINOR` `1.x.0`).

| Group | Method(s) & Path | Auth & Safety Gate | Frozen Contract |
|---|---|---|---|
| **Health & Shell** | `GET /_health`, `GET /_ready`, `GET /`, `GET /public/{rest...}` | Loopback local (public static `/_health`, `/_ready` redact internals) | `GET /_health` reports `"service": "groxy-deskd"`, `"version": "1.0.0"`, `"bind": "127.0.0.1"`. `GET /` injects per-session memory tokens into CSP-protected WebView2 shell. |
| **Theme & Workspace & `grabman`** | `GET`/`POST /api/preferences/theme`, `GET /api/workspace/tree`, `GET /api/workspace/read`, `POST /api/workspace/check`, `POST /api/grabman/init/plan`, `POST /api/grabman/init`, `POST /api/grabman/doctor`, `POST /api/grabman/solve-verify` | Bearer (`GET`), Bearer + CSRF (`POST`), + `approval_token` (`grabman/init`) | `GRAB-E401` path/symlink/secret confinement (`<= 10 MiB` reads); structured `argv` wrappers for `grab check`, `grab format --check`, and `grabman init\|doctor\|solve --verify`. |
| **`Grisp` Supervisor** | `GET /api/grisp/workers`, `POST /api/grisp/workers/start`, `POST /api/grisp/workers/reload`, `POST /api/grisp/workers/stop`, `GET /api/grisp/workers/logs` | Bearer (`GET`), Bearer + CSRF (`POST`) | Supervises up to `8` local `grab serve` workers on `127.0.0.1:8765–8772`, `/_health` + `/_ready` probes, atomic reload after `grab check`, and redacted log ring buffers. |
| **`Groxy AI` & Scorecard** | `POST /api/groxy/ask`, `POST /api/groxy/approvals/apply`, `GET /api/groxy/snapshots`, `POST /api/groxy/snapshots/restore`, `POST /api/groxy/audit`, `POST /api/groxy/boost`, `POST /api/groxy/boost/approve` | Bearer (`GET`), Bearer + CSRF (`POST`), + `approval_token` (`approvals/apply`, `snapshots/restore`, `boost/approve`) | Unified **Groxy AI** (`Adaptive Reasoning Router`, `v1` instant vs deep tiers), 22-rule 4-pillar SEO/WCAG/DOM/Link audit, 🚀 Boost diff, and SHA-256 Pre-Mutation Snapshot (`snap_*`) rollback. |
| **Unified `grab mcp` & Studio** | `GET /api/mcp/tools`, `POST /api/mcp/call`, `GET /api/studio/design-system`, `POST /api/studio/synthesize`, `POST /api/studio/layout/apply` | Bearer (`GET`), Bearer + CSRF (`POST`), + `approval_token` for write tools & `studio/layout/apply` | Exposes all `13` whitelisted tools; forwards live `GRAB-DESIGN-SYSTEM.md`, 8x5 Grex styles, and `POST /api/mcp/layout` (`actor: "groxy-ai"`, `applyAiLayout` `Ctrl+Z` undo stack). |
| **Step 5 Fleet** | `GET /api/fleet/nodes`, `POST /api/fleet/nodes/verify`, `POST /api/fleet/dna/validate`, `POST /api/fleet/deploy/plan`, `POST /api/fleet/deploy/apply`, `POST /api/fleet/snapshots/restore` | Bearer (`GET`), Bearer + CSRF (`POST`), + `approval_token` (`deploy/apply`, `snapshots/restore`) | `v1:...` encrypted node vault, `known_hosts` `SHA256:...` pinning (`403 DESK-E405`), `groxy-site-dna-v0.4.0` + `grabman doctor` gate (`422 DESK-E302`), `--read-only` deploy + `snap_deploy_*` rollback. |
| **DB Studio & Tunnels** | `GET /api/db/tunnels`, `POST /api/db/tunnels/open`, `POST /api/db/tunnels/close`, `GET`/`POST /api/db/schema`, `POST /api/db/rows`, `POST /api/db/query`, `POST /api/db/mutate/plan`, `POST /api/db/mutate/apply`, `POST /api/db/snapshots/restore` | Bearer (`GET`), Bearer + CSRF (`POST`), + `approval_token` (`mutate/apply`, `snapshots/restore`) | `<= 8` pinned `127.0.0.1:15432–15439` SSH tunnels (`409 DESK-E409`), workspace SQLite (`GRAB-E401`), `limit <= 100` pagination, `ADR-025c` string fidelity, read-only SQL default (`403 DESK-E406`), `snap_db_*` rollback. |
| **Namecheap DNS & HTTPS** | `GET /api/dns/vault`, `POST /api/dns/vault/save`, `POST /api/dns/plan`, `POST /api/dns/apply`, `POST /api/dns/https/preflight`, `POST /api/dns/https/apply`, `POST /api/dns/snapshots/restore` | Bearer (`GET`), Bearer + CSRF (`POST`), + `approval_token` (`apply`, `https/apply`, `snapshots/restore`) | `AES-256-GCM` (`v1:...`) vault, masked secrets, strict email record preservation (`422 DESK-E303`), live propagation check (`422 DESK-E304`), Auto-HTTPS proxy config + `snap_dns_*` rollback. |
| **Author Mode & Ollama** | `POST /api/author/init/plan`, `POST /api/author/init/apply`, `GET /api/author/ollama/status`, `POST /api/author/ollama/start` | Bearer (`GET`), Bearer + CSRF (`POST`), + `approval_token` (`init/apply`) | Scaffolds `Book`, `Chapter`, `CharacterBible`, `WorldLore`, `SITE-CONTEXT.md`, `status_enforced: "draft"`, `schema.org/Book` JSON-LD (`ADR-025c`), and loopback Ollama (`127.0.0.1:11434`) lifecycle. |

---

## 4. Frozen Unified `grab mcp` 13-Tool Catalog & Error Taxonomy

### 4.1 All 13 Frozen `grab mcp` Bridge Tools (`internal/mcpbridge/mcpbridge.go`)
| # | Tool Name | Layer | Default Mode | `GROXY-SAFE-001` Approval Required? |
|---|---|---|---|---|
| 1 | `grab_check` | `core-grab` | `read-only` | No |
| 2 | `grab_format` | `core-grab` | `read-only` | When `write: true` (`dry_run` default is read-only) |
| 3 | `grab_inspect_routes` | `core-grab` | `read-only` | No |
| 4 | `grab_inspect_schema` | `core-grab` | `read-only` | No |
| 5 | `grab_scaffold` | `core-grab` | `write` | Yes (unless `dry_run: true`) |
| 6 | `list_content_types` | `grablab-cms` | `read-only` | No |
| 7 | `query_entries` | `grablab-cms` | `read-only` | No |
| 8 | `create_entry_draft` | `grablab-cms` | `write` | Yes (`status_enforced: "draft"`) |
| 9 | `update_entry_layout` | `grablab-cms` | `write` | Yes (20-element & WCAG AA validated) |
| 10 | `list_media` | `grablab-cms` | `read-only` | No |
| 11 | `list_menus` | `grablab-cms` | `read-only` | No |
| 12 | `studio_design_system` | `grex-builder` | `read-only` | No |
| 13 | `studio_apply_layout` | `grex-builder` | `write` | Yes (`actor: "groxy-ai"`, `applyAiLayout` undo metadata) |

*JSON-RPC Initialize Handshake:* `clientInfo` updates from `"0.3.0"` to `"name": "groxy-desk", "version": "1.0.0"`.

### 4.2 Frozen Error Code Taxonomy (`internal/security/security.go`, `internal/mcpbridge/mcpbridge.go`)
| Code | HTTP Status | Trigger Condition |
|---|---|---|
| `GRAB-E401` | `403` | Workspace confinement failure (`..` traversal, UNC `\\`/`//`, escaping symlink, NUL byte, `.git`/`.env*`/`.ssh`/private key read/write, SQLite outside workspace, or non-loopback `GrexPreviewBridge` origin). |
| `DESK-E101` | `400` | Request payload/parameter validation failure, unsupported MCP tool, or Visual DB row pagination `limit > 100`. |
| `DESK-E201` | `—` | Reserved diagnostic constant in `internal/security/security.go` preserved for API parity. |
| `DESK-E301` | `422` | Missing upstream binary (`grab.exe`, `grabman.exe`, `ollama`) or `grab check` / `grab format` / `grabman` execution failure. |
| `DESK-E302` | `422` | Fleet `Site DNA` (`groxy-site-dna-v0.4.0`), `grabman doctor` / `solve --verify`, or non-root / `--read-only` deployment check failure. |
| `DESK-E303` | `422` | Namecheap DNS plan/apply dropped or altered protected email/service records (`MX`, `TXT`, `SPF`, `DKIM`, `DMARC`, `CAA`, `SRV`, `NS`, email `CNAME`). |
| `DESK-E304` | `422` | Live DNS propagation preflight failed prior to Auto-HTTPS ACME certificate provisioning. |
| `DESK-E403` | `403` | Missing/invalid Bearer token, missing/invalid `X-CSRF-Token`, or target-bound `approval_token` scope mismatch. |
| `DESK-E405` | `403` | Missing or mismatched `known_hosts` `SHA256:...` SSH host-key fingerprint (or missing pinned DB tunnel). |
| `DESK-E406` | `403` | Read-only SQL policy block (`POST /api/db/query` rejected multi-statement SQL, DDL/DML writes, `ATTACH DATABASE`, or write `PRAGMA`). |
| `DESK-E409` | `409` | Single-use `approval_token` replay, duplicate active worker/tunnel ID, > `8` active `Grisp` workers, > `8` active SSH DB tunnels, or exhausted loopback port range. |
| `DESK-E422` | `422` | Upstream `grab mcp`, GrabLab CMS, or Grex Theme Builder layout/WCAG rejection. |
| `DESK-E501` | `403` / `500` | Fail-closed non-loopback host/URL rejection (`0.0.0.0`, `::`, wildcards, or remote addresses). |
| `DESK-E502` | `502` | SSH dial failure, Namecheap XML gateway error, or `Grisp` process/stdio pipe failure. |
| `DESK-E503` | `503` | Local Ollama (`127.0.0.1:11434`) unreachable, failed startup, or server error. |

---

## 5. `v1.0.0` Code & Packaging Implementation Plan (Activated Only After Approval)

Once the project owner approves `specifications/GROXY-DESK-SPEC-100.md`, `v1.0.0` implementation covers two focused deliverables:

### 5.1 Version Bump Inventory (`0.3.0` -> `1.0.0`)
Update the hardcoded `0.3.0` (and legacy `package.json` `0.1.0`) version strings cleanly to `1.0.0`:
1. `internal/server/server.go` (`"version": "1.0.0"` in `GET /_health`).
2. `internal/mcpbridge/mcpbridge.go` (`clientInfo.version: "1.0.0"` in stdio MCP `initialize`).
3. `package.json` (`"version": "1.0.0"`).
4. `src/desk.grab` & `views/workbench.grab.html` (`v1.0.0` metadata & status bar string, re-formatted canonically with `grab format`).
5. `public/desk/workbench.js` & `public/desk/api-client.js` (header/footer `v1.0.0` display text & comments).
6. `internal/server/v020_server_test.go`, `internal/server/v030_server_test.go`, and `tests/integration_e2e.py` (assert `"version": "1.0.0"`).
7. `docs/releases/generate_release_guides.py` (`v1.0.0` walkthrough guide template generating `docs/releases/html/Groxy-Desk-v1.0.0-Guide.html` & `docs/releases/pdf/Groxy-Desk-v1.0.0-Guide.pdf`).
8. `docs/badges/release-v1.0.0.svg` (emerald `#10B981` SVG badge) & `README.md` (`v1.0.0` badge + quickstart/installer docs).

### 5.2 Branded Windows Inno Setup Installer (`installer/groxy-desk.iss` -> `Groxy-Desk-Setup-1.0.0.exe`)
- **Format Invariant:** Compiled **exclusively with Inno Setup 6 (`ISCC.exe`)** as `dist/Groxy-Desk-Setup-1.0.0.exe` (**never `.msi`** per `AGENTS.md`).
- **Build Prerequisite Note:** Release packaging builds `Groxy-Desk-Setup-1.0.0.exe` via Inno Setup 6 (`ISCC.exe`, e.g., `winget install JRSoftware.InnoSetup`). `scripts/package_release.py` will auto-detect `ISCC.exe` (failing loudly when missing on `--require-installer`, or cleanly reporting clear installation steps) and stage all verified payload binaries before invoking `ISCC.exe`.
- **Installer Bundled Payload:**
  - `groxy-desk.exe` (`cmd/groxy-desk` static WebView2 launcher)
  - `groxy-deskd.exe` (`cmd/groxy-deskd` static `CGO_ENABLED=0` loopback host daemon)
  - Side-by-side Ecosystem CLIs in `{app}\bin\`:
    - `grab.exe` (verified SHA-256 copy of `GRAB v1.1.0` from `C:\Development\GRAB\dist\grab-windows-amd64.exe`)
    - `grabman.exe` (verified SHA-256 copy of `grabman v1.0.0` from `C:\Development\grabman\dist\grabman-windows-amd64.exe`)
  - Web Assets & Docs: `public\grex\`, `public\desk\`, `src\desk.grab`, `views\workbench.grab.html`, and `docs\releases\pdf\Groxy-Desk-v1.0.0-Guide.pdf`.
- **Desktop UX & Security Guarantees:**
  - `PrivilegesRequired=lowest` default install to `{localappdata}\Groxy Desk` (zero mandatory UAC elevation prompt; works smoothly for standard Windows users).
  - Flicker-free modern Inno Setup wizard (`WizardStyle=modern`, optional desktop shortcut, Start Menu shortcut, and WebView2 Runtime detection without shipping Chromium/Electron).
  - Clean uninstaller removes application binaries and shortcuts while **preserving** user workspaces, encrypted credential vaults, and `GROXY-SAFE-001` Pre-Mutation Snapshots.
- **Release Dist Artifacts (`dist/` + `SHA256SUMS.txt`):**
  - `Groxy-Desk-Setup-1.0.0.exe`
  - `groxy-desk-windows-amd64.exe`
  - `groxy-deskd-windows-amd64.exe`
  - `groxy-desk-v1.0.0-bundle.zip`
  - `Groxy-Desk-v1.0.0-Guide.pdf`
  - `SHA256SUMS.txt` (cryptographic SHA-256 manifest covering every release artifact)

---

## 6. 7-Layer Ecosystem Compatibility Matrix (`v1.0.0` Target)

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

---

## 7. Verification Gates & Dual-Repo `v1.0.0` Promotion Checklist

1. **Gate 1 (Go Static Build, Vet, Tests & Security Scan):** `go build ./...`, `go vet ./...`, `go test -count=1 ./...` across all 11 tested packages, and `govulncheck ./...` (`0` errors, `0` vulnerabilities).
2. **Gate 2 (GRAB Contract Check & Canonical Format Parity):** `grab check src/desk.grab views/workbench.grab.html` and canonical `grab format --write` byte-for-byte SHA-256 parity.
3. **Gate 3 (Pure `Grex JS v1.0.0` DOM, WCAG AA & Contract Tests):** `node --test tests/*.test.js` (`divCount === 0`, `innerHTMLWriteCount === 0`, Light/Dark `>= 4.5:1`, `GrexPreviewBridge` origin firewall, zero model dropdowns, `ADR-025c` fidelity).
4. **Gate 4 (Full Cross-Layer E2E Suite):** `python tests/integration_e2e.py` verifying all 8 peer `v1.x` repositories untouched, static `CGO_ENABLED=0` binaries, `--smoke-test`, live `Grisp`, unified `Groxy AI`, `13` MCP tools, Studio, Boost, Fleet, SSH Tunnels + DB Studio, Namecheap DNS/HTTPS, and Author Mode (`100%` pass).
5. **Dual-Repo Promotion (`origin` & `stable`) Only After 100% Completion + Explicit Approval:**
   - Verify `Groxy-Desk-Setup-1.0.0.exe` clean install & uninstall on Windows (`ProgramData`/workspace preservation + `/_health` `"version": "1.0.0"`).
   - Run `sync-notebook-sources.ps1` and `generate-codex-handoff.ps1`.
   - Push `main` and annotated tag `v1.0.0` in ascending chronological order to `origin` and `stable`, attaching `Groxy-Desk-Setup-1.0.0.exe`, binaries, `Groxy-Desk-v1.0.0-Guide.pdf`, `groxy-desk-v1.0.0-bundle.zip`, and `SHA256SUMS.txt` with the mandatory 7-Layer Compatibility Matrix release body on both GitHub Releases.
