# Groxy Desk Roadmap

**Repositories:** `urbanspress/groxy-desk.progress` (active development) / `urbanspress/groxy-desk.stable` (verified releases)  
**Foundational Baselines:** **GRAB `v1.1.0`**, **Grex JS `v1.0.0`**, **GrabLab CMS `v1.0.1`**, **Grex Theme Builder `v1.0.0`**, **Groxy AI `v1.0.0`**, **`grabman v1.0.0`**, **Grabtaculous `v1.0.0`**, and **Gashboard `v1.0.0`**

**Status Labels:**
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

**Status:** Specification in Review ([`specifications/GROXY-DESK-SPEC-100.md`](../specifications/GROXY-DESK-SPEC-100.md)) — **Pre-`v1.0.0` 7-Layer Audit Complete**

Target Scope:
- Pre-`v1.0.0` cross-layer security, IPC (`GrexPreviewBridge`), SSH pinning/tunnels, DB isolation, Namecheap DNS/HTTPS, Author Mode, Pure `Grex JS v1.0.0` DOM/WCAG AA, `ADR-025c`, and `GROXY-SAFE-001` audit (**Completed**, including `author.Manager.Shutdown()` teardown wiring in `server.Shutdown` and pre-token `DESK-E303` DNS email re-verification + explicit `SPF`/`DKIM`/`DMARC` protection; Gates 1–4 `100%` passing).
- Branded flicker-free Inno Setup Windows installer (`installer/groxy-desk.iss` -> `dist/Groxy-Desk-Setup-1.0.0.exe`, Inno Setup `.exe` only), `SHA256SUMS.txt`, plain-English walkthrough PDF (`docs/releases/pdf/Groxy-Desk-v1.0.0-Guide.pdf`), frozen `55` path / `57` route handler + `13` `grab mcp` tool contracts, and synchronized `v1.0.0` promotion to `origin` and `stable` (pending `GROXY-DESK-SPEC-100.md` approval).
