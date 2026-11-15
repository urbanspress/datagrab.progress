# GROXY-DESK-SPEC-003: Groxy Desk `v0.3.0` — Step 5 Fleet Orchestration, Encrypted SSH DB Tunnels, Built-In Visual DB Explorer, Namecheap DNS/Auto-HTTPS & 1-Click Local Author Mode

- **Status:** Approved target; specification in review (Awaiting explicit project-owner approval before writing `v0.3.0` application code)
- **Target Release:** `v0.3.0` (`PRE-1.0` — Step 6 Integrated Infrastructure, Database, DNS & Author Studio)
- **Foundational Baselines:**
  - **Groxy Desk:** `v0.2.0` ([`GROXY-DESK-SPEC-001.md`](GROXY-DESK-SPEC-001.md), [`GROXY-DESK-SPEC-002.md`](GROXY-DESK-SPEC-002.md) — `groxy-deskd` `127.0.0.1:9490`, WebView2 shell, `Grisp` multi-worker supervisor, `grab mcp` studio, Split-Preview canvas, 22-rule auditor & 🚀 `"Let Groxy Boost This"`)
  - **GRAB:** `v1.1.0` (`urbanspress/grab` — `Grisp`, `grab migrate`, `use crypto` `AES-256-GCM` `v1:...`, `use env`, `ADR-005`, `ADR-017`, `ADR-024f`, `ADR-025c`)
  - **Grex JS:** `v1.0.0` (`urbanspress/grex-js` — `grex.elements.json` 20 semantic elements, `auditAccessibility()`, `computeContrastRatio()`, zero wrapper-`<div>` DOM)
  - **GrabLab CMS:** `v1.0.1` (`urbanspress/grablab-cms` — custom content types, `status_enforced: "draft"`, `SITE-CONTEXT.md`)
  - **Grex Theme Builder:** `v1.0.0` (`urbanspress/grex-builder` — `GRAB-DESIGN-SYSTEM.md`, 8 Design Style Packs, `applyAiLayout` `Ctrl+Z` undo)
  - **Groxy AI:** `v1.0.0` (`urbanspress/groxy-ai` — `groxy-site-dna-v0.4.0`, `schema.org` JSON-LD generator, Automatic Adaptive Reasoning Router — zero model-switching dropdowns)
  - **Step 5 Freeze (`grabman v1.0.0`, `Grabtaculous v1.0.0`, `Gashboard v1.0.0`):** `grabman doctor` / `solve --verify` / `dna validate|diff`, `grabtaculousd` (`127.0.0.1:9440`) `--read-only` container fleet rollouts & Pre-Deploy Snapshots, `known_hosts`-pinned `127.0.0.1` SSH DB tunnels, Visual DB Manager, and Namecheap DNS + Auto-HTTPS Wizard
- **Governance:** Read alongside [`GROXY-DESK-EXPERT.md`](../GROXY-DESK-EXPERT.md), [`AGENTS.md`](../AGENTS.md), and [`docs/ROADMAP.md`](../docs/ROADMAP.md).

---

## 1. Executive Summary (Plain English)

In **`v0.1.0`** and **`v0.2.0`**, **Groxy Desk** gave you a local Windows desktop studio to scaffold GRAB apps, supervise local `Grisp` workers, design accessible `Grex` pages in a live Split-Preview canvas, and boost SEO/WCAG scores to `A`/`A+` with **Groxy AI**.

In **`v0.3.0`**, **Groxy Desk** brings the entire **Step 5 (`grabman` + `Grabtaculous` + `Gashboard`)** infrastructure, database, and domain workflow—plus a **1-Click Local "Author Mode"**—directly into the desktop workbench:

1. **Step 5 Fleet & `Grabtaculous` Bridge (`internal/fleet/`):** Connect from your desktop over `known_hosts`-pinned SSH (`SHA256:...` host-key verification—zero public admin portals on your VPS) to remote `grabtaculousd` (`127.0.0.1:9440`). Validate `grabman.lock` (`grabman solve --verify`) and `Site DNA` (`groxy-site-dna-v0.4.0`), deploy hardened `--read-only` Marketplace stacks or `Site DNA` updates after explicit approval (`GROXY-SAFE-001`), and roll back to an immutable SHA-256 **Pre-Deploy Snapshot** in 1 click.
2. **Encrypted `127.0.0.1` SSH Database Tunnels & Built-In Visual DB Explorer (`internal/dbstudio/` + `public/desk/db-studio-panel.js`):** Open up to `8` concurrent `127.0.0.1:<local_port>` SSH tunnels to remote PostgreSQL/MariaDB databases (or open confined local SQLite files). Browse schemas, tables, indexes, and paginated rows (`<= 100` rows/page) with exact `ADR-025c` base-10 numbers (never float-corrupted), run safe read-only `SELECT`/`EXPLAIN` queries by default, and require a single-use `approval_token` + automatic **Pre-Mutation Snapshot** before any row edit, `INSERT`/`UPDATE`/`DELETE`, DDL, or `.sql` import.
3. **Encrypted Namecheap DNS Vault & 1-Click Domain -> DNS -> Auto-HTTPS Wizard (`internal/dns/` + `public/desk/dns-panel.js`):** Store Namecheap API credentials encrypted at rest (`use crypto` `AES-256-GCM` `v1:...`), point `A`/`AAAA`/`CNAME` records to your VPS while keeping existing `MX`/`TXT`/`SPF`/`DKIM` email records `100%` byte-identical, block certificate provisioning with a clear `422` preflight if DNS has not propagated yet (protecting Let's Encrypt rate limits), and configure Caddy, Nginx + Certbot, or Cloudflare Tunnel Auto-HTTPS with 1-click rollback.
4. **1-Click Local "Author Mode" & Optional Open-Weights Model Launcher:** Click one card in **Groxy Desk** to initialize a local **GrabLab CMS + Groxy AI** workspace pre-configured for **Author Mode** (`Book`, `Chapter`, `CharacterBible`, and `WorldLore` content schemas + automatic `schema.org/Book` JSON-LD preview), paired with a 1-click local **Ollama** open-weights status check and launcher (zero required external API fees and zero model-switching dropdowns).

---

## 2. `v0.3.0` Architecture & Package Additions

```text
groxy-desk/
├── internal/
│   ├── fleet/                  # NEW (v0.3.0): Step 5 SSH known_hosts bridge to grabtaculousd (127.0.0.1:9440),
│   │                           #               grabman dna validate/diff, --read-only stack rollout & Pre-Deploy Snapshots
│   ├── dbstudio/               # NEW (v0.3.0): <= 8 loopback SSH DB tunnels (127.0.0.1:15432–15439), confined SQLite,
│   │                           #               Visual DB Explorer, ADR-025c wire strings & Pre-Mutation DB Snapshots
│   ├── dns/                    # NEW (v0.3.0): AES-256-GCM (v1:...) Namecheap vault, MX/TXT/SPF/DKIM-preserving DNS diff,
│   │                           #               422 DNS propagation preflight & Caddy/Nginx/Cloudflare Auto-HTTPS wizard
│   ├── author/                 # NEW (v0.3.0): 1-Click Local Author Mode initializer (Book/Chapter/CharacterBible/WorldLore
│   │                           #               + schema.org/Book JSON-LD) & local Ollama open-weights status/launcher
│   ├── server/                 # UPDATED: Registers v0.3.0 /api/fleet/*, /api/db/*, /api/dns/*, and /api/author/* routes
│   ├── groxybridge/            # UPDATED: Routes fleet/DB/DNS/Author Mode synthesis through Adaptive Reasoning Router
│   ├── mcpbridge/              # Existing (v0.2.0): Core GRAB + GrabLab CMS + Grex Builder MCP bridge
│   ├── auditor/                # Existing (v0.2.0): 22-rule 4-pillar SEO/WCAG/DOM/Link auditor (extended with Book JSON-LD)
│   ├── supervisor/             # Existing (v0.1.0): Local Grisp multi-worker supervisor (127.0.0.1:8765–8772)
│   ├── workspace/              # Existing (v0.1.0): grabman init/doctor/solve-verify + grab check/format
│   └── security/               # Existing (v0.1.0): DESK-E501 loopback bind, GRAB-E401 confinement, CSRF & token auth
├── public/
│   └── desk/
│       ├── fleet-panel.js      # NEW (v0.3.0): Pure Grex JS v1.0.0 Fleet, Marketplace, Site DNA & Rollback UI (divCount === 0)
│       ├── db-studio-panel.js  # NEW (v0.3.0): Pure Grex JS v1.0.0 SSH Tunnel Manager & Visual DB Explorer (divCount === 0)
│       ├── dns-panel.js        # NEW (v0.3.0): Pure Grex JS v1.0.0 Namecheap DNS Vault & Auto-HTTPS Wizard (divCount === 0)
│       └── author-panel.js     # NEW (v0.3.0): Pure Grex JS v1.0.0 1-Click Local Author Mode & Ollama Status Card (divCount === 0)
```

---

## 3. Step 5 Fleet & `Grabtaculous` Bridge (`internal/fleet/` + `public/desk/fleet-panel.js`)

### 3.1 `known_hosts`-Pinned SSH Control Plane (`ADR-005`, `ADR-017`)
- **Zero Public Admin Ports:** Remote VPS nodes run `grabtaculousd` bound exclusively to `127.0.0.1:9440` (or rootless Unix socket). `groxy-deskd` connects to remote nodes over SSH using structured argument vectors / native Go SSH (`golang.org/x/crypto/ssh`) with mandatory **`known_hosts` `SHA256:...` host-key fingerprint verification**.
- **Hard Rejection on Unpinned/Mismatched Host Keys (`DESK-E405`):** `InsecureIgnoreHostKey` is strictly forbidden. Any missing or mismatched `SHA256:...` host-key fingerprint fails closed with `403 DESK-E405` before opening any tunnel or control channel.
- **Encrypted SSH Key Metadata (`use crypto` `v1:...`):** Any saved node profile stores credential descriptors encrypted via `AES-256-GCM` (`v1:<nonce_b64>:<ciphertext_b64>`) with keys derived from host environment variables (`GROXY_DESK_MASTER_KEY`); private key contents are never exposed in DOM, API responses, or logs.

### 3.2 `grabman` Lockfile, `Site DNA` (`groxy-site-dna-v0.4.0`) & Hardened `--read-only` Rollouts
- **Pre-Rollout Lockfile & Doctor Gate:** Before any Marketplace deployment or `Site DNA` fleet sync, `internal/fleet/` runs `grabman solve --verify` and `grabman doctor` via structured `argv` (`DESK-E301` on lockfile drift or SemVer incompatibility).
- **Portable `Site DNA` (`groxy-site-dna-v0.4.0`) Sync & Diff:**
  - Validates `schema_version === "groxy-site-dna-v0.4.0"`, `compatible_consumers`, `design_system`, `site_context`, and `decimal_integrity_verified === true` (`grabman dna validate` / `grabman dna diff`).
  - Rejects any IEEE-754 float drift (`"0.30000000000000004"`), wrapper `<div>`, `<script>`, or unsafe URL inside `Site DNA` (`422 DESK-E302`).
- **Curated `--read-only` Container Enforcement:** Every stack manifest dispatched to `grabtaculousd` (`grab-core-app`, `grablab-cms-stack`, `groxy-ai-seo-companion`, `postgres-16-private`, `mariadb-11-private`, `redis-7.2-private`) enforces:
  - `read_only: true`, `cap_drop: ["ALL"]`, `security_opt: ["no-new-privileges:true"]`, non-root `65532:65532`, pinned image digests (`:latest` rejected), and private `127.0.0.1` data-store bindings.
- **Explicit Pre-Rollout Migrations (`ADR-024f`) & `/_ready` Gating (`ADR-025g`):** Database migrations (`grab migrate`) execute explicitly before worker traffic switchover—never inside live HTTP reloads. Rolling worker replacements verify `GET /_health === 200` and `GET /_ready === 200` before draining prior workers (`30s` bounded drain).
- **Mandatory Pre-Deploy Snapshot & 1-Click Rollback (`GROXY-SAFE-001`):**
  - Every rollout requires a single-use, target-bound `approval_token` + `X-CSRF-Token` and records an immutable SHA-256 **Pre-Deploy Snapshot** (`snap_deploy_*` capturing `grabman.lock`, stack manifest, `Site DNA`, and edge config).
  - If `/_ready` fails or the user clicks **"Rollback to Pre-Deploy Snapshot"**, `groxy-deskd` atomically restores the prior snapshot and verifies `/_ready === 200`.

---

## 4. Encrypted `127.0.0.1` SSH Database Tunnels & Built-In Visual DB Explorer (`internal/dbstudio/` + `public/desk/db-studio-panel.js`)

### 4.1 Loopback-Only SSH Database Tunnels (`<= 8` Concurrent)
- **Strict `127.0.0.1` Tunnel Binding:** `internal/dbstudio/` manages up to `8` concurrent encrypted SSH port-forward tunnels bound strictly to `127.0.0.1:15432–15439` (`DESK-E501` on any non-loopback bind attempt; `409 DESK-E409` if `8` tunnels are already active).
- **Supported Engines:**
  - Remote **PostgreSQL (`16.x`)** and **MariaDB (`11.x`)** over `known_hosts`-pinned `127.0.0.1` SSH tunnels.
  - Local workspace-confined **SQLite** databases (`GRAB-E401` enforced; rollback-journal mode).
- **Bounded Idle Timeouts & Zero Secret Leaks:** Tunnels auto-close on configurable idle timeout (default `15m`) or clean daemon shutdown; connection DSNs and passwords are redacted from all responses and logs.

### 4.2 Built-In Visual DB Explorer (Read-Only Default + `ADR-025c` Fidelity + `GROXY-SAFE-001`)
- **Schema & Table Inspector:** Lists tables, columns (`name`, `type`, `nullable`, `default`, `primary_key`), indexes, foreign keys, and estimated row counts without executing mutating SQL.
- **Paginated Row Browser (`<= 100` Rows/Page) & Exact `ADR-025c` Wire Strings:**
  - Browses table rows with deterministic sorting, column filtering, and a hard cap of `100` rows per page (`DESK-E101` if `per_page > 100`).
  - **Exact Base-10 Wire Preservation (`ADR-025c`):** Every `NUMERIC`, `DECIMAL`, `BIGINT`, `INT64`, currency price, and `GrabDecimal` / `GrabInteger` column is serialized and rendered as an exact base-10 wire string (e.g., `"1999.95"`, `"9007199254740993"`) so JavaScript `Number` coercion never silently rounds financial or ID values.
- **Read-Only Default Query Runner (`SELECT` / `EXPLAIN` Only):**
  - The SQL query bar executes in strict read-only transaction mode by default, permitting only single-statement `SELECT`, `WITH ... SELECT`, `EXPLAIN`, or `PRAGMA table_info` queries (`403 DESK-E406` if a write/DDL statement is submitted to the read-only endpoint).
- **Gated Write / DDL / `.sql` Import (`GROXY-SAFE-001`):**
  - Any cell edit, row `INSERT`/`UPDATE`/`DELETE`, schema migration (`ALTER`/`CREATE`/`DROP`), or `.sql` batch import MUST:
    1. First call `POST /api/db/mutate/plan` to preview the exact parameterized SQL, affected table/row count estimate, and receive a single-use, target-bound `approval_token`.
    2. On `POST /api/db/mutate/apply` (with `Authorization: Bearer`, `X-CSRF-Token`, and `approval_token`), capture an automatic SHA-256 **Pre-Mutation DB Snapshot** (`snap_db_*` containing the prior row/table state or SQLite rollback copy) before committing inside a bounded transaction (`403 DESK-E403` on mismatch, `409 DESK-E409` on replay).
    3. Support 1-click restoration via `POST /api/db/snapshots/restore`.

---

## 5. Encrypted Namecheap DNS Vault & 1-Click Domain -> DNS -> Auto-HTTPS Wizard (`internal/dns/` + `public/desk/dns-panel.js`)

### 5.1 `use crypto` `AES-256-GCM` (`v1:...`) Namecheap Credential Vault
- **Encrypted At Rest:** Namecheap API credentials (`api_user`, `api_key`, `client_ip`) are encrypted using GRAB `v1.1.0`-compatible `AES-256-GCM` (`v1:<nonce_b64>:<ciphertext_b64>`) keyed by the host master key (`use env`).
- **Zero Plaintext Exposure:** API keys are masked in UI responses (`"nc_****...****"`) and never written to workspace files, Git commits, DOM attributes, or logs.

### 5.2 Email-Safe DNS Record Planner (`MX` / `TXT` / `SPF` / `DKIM` Preservation)
- **Byte-Identical Email Record Preservation:** Because Namecheap's `domains.dns.setHosts` replaces the host record set for a domain, `internal/dns/` ALWAYS fetches the complete existing record set first, computes a deterministic diff for the target `A`, `AAAA`, and `CNAME` web records, and **preserves every existing `MX`, `TXT`, `SPF`, `DKIM`, `DMARC`, `CAA`, and `SRV` record byte-for-byte**.
- **Safety Invariant Verification:** If a proposed DNS update would drop or alter any existing `MX`, `SPF`, `DKIM`, or `DMARC` record without explicit `allow_email_record_change: true` confirmation, `internal/dns/` aborts with `422 DESK-E303` (`"Email DNS preservation guard tripped"`).

### 5.3 Mandatory `422` DNS Propagation Preflight & Edge Auto-HTTPS Wiring
- **Live Propagation Preflight (`422 DESK-E304` Before Convergence):**
  - Before triggering Let's Encrypt / ZeroSSL ACME certificate issuance on the target VPS, `POST /api/dns/https/preflight` resolves the target domain against authoritative and public DNS resolvers to verify that `A`/`AAAA` records match the target VPS IP.
  - If DNS has not converged yet, `groxy-deskd` returns **`422 Unprocessable Entity` (`DESK-E304`)** with a plain-English countdown and blocks ACME provisioning so Let's Encrypt rate limits are never tripped.
- **Unprivileged Edge Auto-HTTPS (`Caddy` / `Nginx + Certbot` / `Cloudflare Tunnel`):**
  - Generates and deploys the edge proxy configuration (**Caddy** automatic TLS & HTTP/3 default, **Nginx + Certbot**, or **Cloudflare Tunnel**) forwarding to private loopback `Grisp` workers (`127.0.0.1:8765–8772 --trusted-proxy 127.0.0.1`).
  - Captures a **Pre-DNS / Pre-TLS Snapshot** (`snap_dns_*`) before applying changes, enabling 1-click DNS + edge config rollback (`GROXY-SAFE-001`).

---

## 6. 1-Click Local "Author Mode" & Optional Open-Weights Model Launcher (`internal/author/` + `public/desk/author-panel.js`)

### 6.1 1-Click Local "Author Mode" Workspace Initializer
- **Plain-English Fiction & Non-Fiction Publishing Studio:** Adds a 1-click **"Initialize Author Mode"** setup card inside **Groxy Desk** that configures a local **GrabLab CMS (`v1.0.1`) + Groxy AI (`v1.0.0`)** workspace (`GRAB-E401` confined, `GROXY-SAFE-001` approved) with four ready-to-use content schemas:
  1. **`Book` (`book`):** `title`, `subtitle`, `author_name`, `isbn_13` (`ADR-025c` exact string), `synopsis`, `genre`, `publication_status` (`status_enforced: "draft"`), `cover_media_id`, and `price_usd` (`GrabDecimal` base-10 wire string, e.g., `"24.99"`).
  2. **`Chapter` (`chapter`):** `book_id`, `chapter_number` (`GrabInteger` base-10 string), `title`, `pov_character`, `word_count` (`GrabInteger`), `manuscript_richtext` (pure semantic Grex `RichText` / `Section` layout, `divCount === 0`), and `continuity_notes`.
  3. **`CharacterBible` (`character_bible`):** `character_name`, `role`, `archetype`, `physical_traits`, `voice_profile`, `arc_summary`, and `first_appearance_chapter`.
  4. **`WorldLore` (`world_lore`):** `entry_title`, `category` (`location`, `timeline_event`, `magic_or_tech_rule`, `faction`), `canonical_facts`, and `related_chapters`.
- **Automatic `schema.org/Book` JSON-LD & `SITE-CONTEXT.md` Continuity Graph:**
  - Generates deterministic, XSS-escaped `schema.org/Book` JSON-LD (`@type: "Book"`, `name`, `author`, `isbn`, `workExample`, `offers.price` preserving exact `ADR-025c` base-10 strings) and updates `SITE-CONTEXT.md` so **Groxy AI** checks chapter-to-character/lore internal links and continuity automatically.

### 6.2 Optional Local Ollama Open-Weights Status Check & 1-Click Launcher
- **Zero Required External API Fees:** `internal/author/` probes `http://127.0.0.1:11434/api/tags` on loopback to report whether a local **Ollama** open-weights runtime is running and ready.
- **Structured `argv` Local Launch:** If Ollama is installed locally but stopped, the user can click **"Start Local Open-Weights Engine"** (`POST /api/author/ollama/start`), which spawns `ollama serve` via structured OS `argv` (`exec.CommandContext`, bound to `127.0.0.1:11434`, never `sh -c` / `cmd /c`).
- **Zero Model-Switching Dropdowns Invariant Preserved:** Consistent with [`GROXY-DESK-EXPERT.md`](../GROXY-DESK-EXPERT.md) Pillar 1, the UI shows only a clean readiness pill (`"Groxy AI Local Engine: Ready"` / `"Start Local Engine"`) and **never renders any model-picker `<select>` or model-switching dropdown**. `Groxy AI`'s **Automatic Adaptive Reasoning Router** selects the instant vs. deep reasoning tier automatically under the hood.

---

## 7. `v0.3.0` Host Daemon HTTP API Extensions (`127.0.0.1:9490`)

All routes require constant-time `Authorization: Bearer <token>` (`subtle.ConstantTimeCompare`), and all `POST` routes require `X-CSRF-Token` (`403 DESK-E403`):

| Method & Path | Auth | Contract |
|---|---|---|
| `GET /api/fleet/nodes` | Bearer | Lists registered Step 5 VPS nodes, `known_hosts` `SHA256:...` pin status, and `grabtaculousd` (`127.0.0.1:9440`) health. |
| `POST /api/fleet/nodes/verify` | Bearer + CSRF | Verifies SSH `known_hosts` `SHA256:...` fingerprint (`403 DESK-E405` on mismatch) and checks remote `grabtaculousd /_health`. |
| `POST /api/fleet/dna/validate` | Bearer + CSRF | Runs `grabman dna validate` / `diff` on a `groxy-site-dna-v0.4.0` bundle (`422 DESK-E302` on float drift or wrapper `<div>`). |
| `POST /api/fleet/deploy/plan` | Bearer + CSRF | Runs `grabman solve --verify` preflight, validates `--read-only` stack manifest, and issues a single-use `approval_token`. |
| `POST /api/fleet/deploy/apply` | Bearer + CSRF + `approval_token` | Captures SHA-256 Pre-Deploy Snapshot (`snap_deploy_*`), runs pre-rollout migrations (`ADR-024f`), and executes `/_ready`-gated rollout. |
| `POST /api/fleet/snapshots/restore` | Bearer + CSRF + `approval_token` | Atomically rolls back a fleet deployment to its Pre-Deploy Snapshot in 1 click. |
| `GET /api/db/tunnels` | Bearer | Lists active `127.0.0.1:15432–15439` SSH DB tunnels (`<= 8`) and confined local SQLite connections. |
| `POST /api/db/tunnels/open` | Bearer + CSRF | Opens a `known_hosts`-pinned `127.0.0.1:<local_port>` SSH tunnel (`409 DESK-E409` if `> 8` tunnels; `403 DESK-E405` on bad host key). |
| `POST /api/db/tunnels/close` | Bearer + CSRF | Cleanly drains and closes an active loopback SSH DB tunnel. |
| `GET /api/db/schema` | Bearer | Inspects tables, columns, indexes, and row counts for an active tunnel or confined SQLite DB (`GRAB-E401`). |
| `POST /api/db/rows` | Bearer + CSRF | Paginated read-only row browser (`<= 100` rows/page) returning exact `ADR-025c` base-10 wire strings. |
| `POST /api/db/query` | Bearer + CSRF | Executes read-only `SELECT` / `EXPLAIN` queries (`403 DESK-E406` on any write/DDL statement). |
| `POST /api/db/mutate/plan` | Bearer + CSRF | Previews a parameterized row edit / DDL / `.sql` import and issues a target-bound `approval_token`. |
| `POST /api/db/mutate/apply` | Bearer + CSRF + `approval_token` | Captures a SHA-256 Pre-Mutation DB Snapshot (`snap_db_*`) and commits the approved mutation (`403`/`409` on mismatch/replay). |
| `POST /api/db/snapshots/restore` | Bearer + CSRF + `approval_token` | Restores a database table/row or SQLite file from its Pre-Mutation DB Snapshot. |
| `POST /api/dns/vault/save` | Bearer + CSRF | Encrypts Namecheap API credentials in `AES-256-GCM` (`v1:...`) and returns masked metadata only. |
| `POST /api/dns/plan` | Bearer + CSRF | Computes `A`/`AAAA`/`CNAME` diff while verifying byte-identical preservation of `MX`/`TXT`/`SPF`/`DKIM` (`422 DESK-E303` on violation). |
| `POST /api/dns/apply` | Bearer + CSRF + `approval_token` | Captures Pre-DNS Snapshot (`snap_dns_*`) and applies the approved email-safe DNS update. |
| `POST /api/dns/https/preflight` | Bearer + CSRF | Verifies live DNS propagation to target VPS IP; returns **`422 DESK-E304`** if not yet converged (protecting Let's Encrypt rate limits). |
| `POST /api/dns/https/apply` | Bearer + CSRF + `approval_token` | Requires passed `preflight`; wires Caddy / Nginx+Certbot / Cloudflare Tunnel Auto-HTTPS to `127.0.0.1:8765–8772`. |
| `POST /api/author/init/plan` | Bearer + CSRF | Previews 1-Click Local Author Mode (`Book`, `Chapter`, `CharacterBible`, `WorldLore` + `schema.org/Book` JSON-LD) and issues `approval_token`. |
| `POST /api/author/init/apply` | Bearer + CSRF + `approval_token` | Captures Pre-Mutation Snapshot and scaffolds the Author Mode schemas, starter draft entries (`status_enforced: "draft"`), and `SITE-CONTEXT.md`. |
| `GET /api/author/ollama/status` | Bearer | Checks `127.0.0.1:11434` local open-weights readiness without exposing model-picker dropdowns. |
| `POST /api/author/ollama/start` | Bearer + CSRF | Launches local `ollama serve` on `127.0.0.1:11434` via structured `argv` (`exec.CommandContext`). |

---

## 8. Additional `v0.3.0` Error Codes

| Code | HTTP Status | Plain-English Meaning |
|---|---|---|
| `DESK-E302` | `422` | `Site DNA` (`groxy-site-dna-v0.4.0`) validation failure, `ADR-025c` float drift, or wrapper-`<div>` / XSS violation. |
| `DESK-E303` | `422` | Email DNS preservation guard tripped (`MX` / `TXT` / `SPF` / `DKIM` / `DMARC` record would be modified or dropped). |
| `DESK-E304` | `422` | DNS propagation preflight not yet converged to target VPS IP; ACME / Let's Encrypt issuance blocked to prevent rate-limit lockout. |
| `DESK-E405` | `403` | SSH `known_hosts` `SHA256:...` fingerprint missing or mismatched (`InsecureIgnoreHostKey` is forbidden). |
| `DESK-E406` | `403` | Mutating SQL/DDL statement rejected on read-only query endpoint (`POST /api/db/query`); use `POST /api/db/mutate/plan` + `approval_token`. |

---

## 9. Pure `Grex JS v1.0.0` UI & Non-Negotiable Invariants

1. **Zero Wrapper-`<div>` Semantic DOM (`GREX-SCHEMA-001`):** All four new workbench panels (`fleet-panel.js`, `db-studio-panel.js`, `dns-panel.js`, `author-panel.js`) and updated `views/workbench.grab.html` MUST use strictly the 20 semantic elements in `grex.elements.json` (`Section`, `Container`, `Grid`, `Stack`, `Card`, `Heading`, `Text`, `RichText`, `Link`, `Button`, `Image`, `Icon`, `Form`, `Input`, `Select`, `Tabs`, `Accordion`, `Modal`, `Nav`, `ForEach`), passing `divCount === 0`, `innerHTMLWriteCount === 0`, zero `eval()` / `new Function()`, and zero unsafe URLs.
2. **Zero Model-Switching Dropdowns:** Neither the Author Mode card, the Ollama status pill, nor any Fleet/DB/DNS panel may render a model-picker `<select>` or model-switching dropdown. **Groxy AI** remains the single permanent built-in agent identity.
3. **100% Light & Dark WCAG AA Contrast Parity:** Every status badge, SQL table cell, DNS diff row, tunnel indicator, and modal in both `[data-grex-theme="light"]` and `[data-grex-theme="dark"]` MUST pass `computeContrastRatio() >= 4.5:1` (`>= 3.0:1` large text) and `100%` `auditAccessibility()` checks with visible `:focus-visible` outlines.
4. **Exact Base-10 Wire Preservation (`ADR-025c`):** All Visual DB Explorer cells, `Book` `price_usd` / `isbn_13`, `Chapter` numbers/word counts, `Site DNA` values, and tunnel ports (`15432–15439`) MUST remain exact base-10 strings end-to-end.

---

## 10. Mandatory `v0.3.0` Verification Gates (4-Gate Acceptance Criteria)

Implementation of `v0.3.0` is NOT complete until all **four** verification gates pass at `100%`:

1. **Gate 1 — Go Host Daemon, Fleet, DB Studio, DNS & Author Mode Suite:**
   - `go build ./...`, `go vet ./...`, `go test -count=1 ./...`, and `govulncheck ./...` (`0` errors, `0` vulnerabilities) across `internal/fleet`, `internal/dbstudio`, `internal/dns`, `internal/author`, `internal/mcpbridge`, `internal/auditor`, `internal/groxybridge`, `internal/supervisor`, `internal/workspace`, `internal/security`, and `internal/server`.
2. **Gate 2 — Pure `Grex JS v1.0.0` Zero-`<div>`, WCAG AA & `ADR-025c` DOM Suite:**
   - `grab check src/desk.grab views/workbench.grab.html` & `grab format` byte parity.
   - `node --test tests/*.test.js` verifying `divCount === 0`, `innerHTMLWriteCount === 0`, zero `eval()` / unsafe URLs, zero model-switching dropdowns across all panels (including `fleet-panel.js`, `db-studio-panel.js`, `dns-panel.js`, and `author-panel.js`), Light & Dark `computeContrastRatio() >= 4.5:1`, `100%` `auditAccessibility()`, and `ADR-025c` base-10 wire string fidelity in Visual DB Explorer rows and Author Mode `schema.org/Book` JSON-LD.
3. **Gate 3 — Security, Confinement, SSH Pinning & Approval Gate Suite:**
   - Automated tests proving:
     - `known_hosts` `SHA256:...` verification (`403 DESK-E405` on unpinned/mismatched host key; zero `InsecureIgnoreHostKey`).
     - Loopback-only SSH tunnel binding (`127.0.0.1:15432–15439`, `<= 8` cap `409 DESK-E409`, `DESK-E501` on `0.0.0.0`).
     - Read-only SQL enforcement on `/api/db/query` (`403 DESK-E406` on `DROP`/`UPDATE`/`INSERT`) and `<= 100` rows/page cap (`400 DESK-E101`).
     - Namecheap `AES-256-GCM` (`v1:...`) vault encryption, `MX`/`TXT`/`SPF`/`DKIM` byte-identical preservation (`422 DESK-E303` on email record drop), and `422 DESK-E304` ACME preflight block prior to DNS convergence.
     - Single-use `approval_token` + `X-CSRF-Token` (`403 DESK-E403` on mismatch, `409 DESK-E409` on replay) + Pre-Mutation/Pre-Deploy Snapshot rollback across Fleet, DB Studio, DNS, and Author Mode.
4. **Gate 4 — Live 7-Layer Cross-Layer Python E2E Integration Suite:**
   - `python tests/integration_e2e.py` exercising `groxy-deskd` (`127.0.0.1:9490`) + `GRAB v1.1.0` (`Grisp` + `grab mcp`) + `Grex JS v1.0.0` + `GrabLab CMS v1.0.1` + `Grex Theme Builder v1.0.0` + `Groxy AI v1.0.0` + Step 5 (`grabman v1.0.0` + loopback `grabtaculousd` fleet deploy & snapshot rollback + `127.0.0.1` SSH DB tunnel & Visual DB Explorer read/write/rollback + Namecheap DNS `MX`-preserving plan & `422`->`200` Auto-HTTPS flow + 1-Click Local Author Mode initialization), while confirming all 8 peer `v1.x` repositories remain completely untouched.
