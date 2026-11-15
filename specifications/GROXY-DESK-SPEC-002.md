# GROXY-DESK-SPEC-002: Groxy Desk `v0.2.0` — Full `grab mcp` Studio, Live Split-Preview Canvas, 22-Rule Scorecard & 🚀 `"Let Groxy Boost This"` Co-Pilot

- **Status:** Complete (Implemented, Verified Across Gates 1–4, and Released in `v0.2.0`)
- **Target Release:** `v0.2.0` (`PRE-1.0` — Step 6 Full-Parity `grab mcp` Studio & Visual Co-Pilot)
- **Foundational Baselines:**
  - **Groxy Desk:** `v0.1.0` ([`GROXY-DESK-SPEC-001.md`](GROXY-DESK-SPEC-001.md) — `groxy-deskd` `127.0.0.1:9490`, WebView2 shell, `Grisp` multi-worker supervisor, `grabman v1.0.0` bridge, `GROXY-SAFE-001`, `GRAB-E401`)
  - **GRAB:** `v1.1.0` (`urbanspress/grab` — `grab mcp` `stdio` JSON-RPC 2.0 server, `grab_check`, `grab_format`, `grab_inspect_routes`, `grab_inspect_schema`, `grab_scaffold`)
  - **Grex JS:** `v1.0.0` (`urbanspress/grex-js` — `grex.elements.json` 20 semantic elements, `auditAccessibility()`, `computeContrastRatio()`, `GrexPreviewBridge` `PREVIEW_CHANNEL = "grex-preview-v1"`)
  - **GrabLab CMS:** `v1.0.1` (`urbanspress/grablab-cms` — 6 capability-gated `grab mcp` CMS tools, `status_enforced: "draft"`, `SITE-CONTEXT.md`)
  - **Grex Theme Builder:** `v1.0.0` (`urbanspress/grex-builder` — `GET /api/design-system.md`, `POST /api/mcp/layout`, `applyAiLayout` `Ctrl+Z` command-stack undo, `GrexPreviewBridge`, 8 Design Style Packs)
  - **Groxy AI:** `v1.0.0` (`urbanspress/groxy-ai` — 22-rule 4-pillar SEO/WCAG/DOM/Link scorecard, 🚀 `"Let Groxy Boost This"`, `8 × 5` Style Pack × Blueprint synthesizer, `SITE-CONTEXT.md` & `schema.org` JSON-LD)
- **Governance:** Read alongside [`GROXY-DESK-EXPERT.md`](../GROXY-DESK-EXPERT.md), [`AGENTS.md`](../AGENTS.md), and [`docs/ROADMAP.md`](../docs/ROADMAP.md).

---

## 1. Executive Summary (Plain English)

**Groxy Desk `v0.2.0`** turns the desktop foundation from `v0.1.0` into a complete, visual **GRAB + Grex + GrabLab + Groxy AI** studio—all inside one lightweight Windows WebView2 app:

1. **One Unified AI Co-Pilot (`Groxy AI`) With Full `grab mcp` Parity (Zero Model-Switching Dropdowns):** Ask **Groxy AI** in plain English to check `.grab` syntax, inspect routes or schemas, draft GrabLab CMS pages (always safely as `"draft"`), or compose accessible Grex visual layouts—with zero model-picker dropdowns.
2. **Interactive Split-Preview Workbench Canvas (`Code` / `Split` / `Visual & Scorecard`):** Edit `.grab` / `.grab.html` code side-by-side with a live, origin-checked `GrexPreviewBridge` (`PREVIEW_CHANNEL = "grex-preview-v1"`) preview and a `Base` / `Tablet` / `Mobile` responsive viewport switcher.
3. **22-Rule Scorecard & 🚀 `"Let Groxy Boost This"` Diff Co-Pilot:** See real-time `0–100` scores and `A+`–`F` grades across all 4 pillars (`On-Page SEO`, `Content & WCAG Accessibility`, `Performance & Clean DOM`, `Off-Page / Internal Linking Readiness`). Click 🚀 **"Let Groxy Boost This"** to review a side-by-side before/after diff + projected score delta, capture an automatic SHA-256 Pre-Mutation Snapshot, and apply changes only after clicking an explicit target-bound approval (`GROXY-SAFE-001`), with 1-click `Ctrl+Z` (`applyAiLayout`) or Snapshot Rollback.

---

## 2. Unified `Groxy AI` + Full `grab mcp` & Ecosystem Tool Parity

### 2.1 Zero Model-Switching Dropdowns & Adaptive Reasoning Routing
- **Single Permanent Identity:** All `v0.2.0` studio panels, MCP drawers, Boost cards, and CMS/Builder actions route through the single **Groxy AI** identity (`"Ask Groxy AI..."`). No model-picker `<select>` or provider dropdown is ever rendered.
- **Deterministic Adaptive Reasoning Router Extensions (`internal/groxybridge/router.go`):**
  - **Instant / Fast Tier:** `grab_check`, `grab_format` (dry-run), `grab_inspect_routes`, `grab_inspect_schema`, `list_content_types`, `query_entries`, `list_media`, `list_menus`, `GET /api/design-system.md`, and running the deterministic 22-rule 4-pillar audit.
  - **Deep / High-Reasoning Tier:** `grab_scaffold` multi-file generation, `create_entry_draft` with composed `grex_layout`, `update_entry_layout`, `GRAB-DESIGN-SYSTEM.md` + `SITE-CONTEXT.md` visual layout synthesis (`8` Style Packs × `5` Blueprints), `POST /api/mcp/layout` dispatch, and 🚀 **"Let Groxy Boost This"** multi-pillar remediation diffs.

### 2.2 Core GRAB `v1.1.0` MCP Tools (Workspace-Confined `GRAB-E401`)
`groxy-deskd` spawns `grab mcp --workspace <confined_root>` via structured OS `argv` (`exec.CommandContext`, never `sh -c` / `cmd /c`), defaulting to strict read-only mode unless an explicit `GROXY-SAFE-001` `approval_token` authorizes `--allow-write`:

| Tool | Mode | Contract & Confinement |
|---|---|---|
| `grab_check` | Read-only | Runs offline lexer/parser/static + `.grab.html` context-aware checks; returns `{ ok, diagnostics: [{ code, category, message, line, column, file }] }`. |
| `grab_format` | Read-only default (`write: true` gated) | Canonical 2-space `parser.Format`. `write: true` requires `GROXY-SAFE-001` `approval_token` + Pre-Mutation Snapshot. |
| `grab_inspect_routes` | Read-only | Returns AST-declared `route` blocks (`method`, `path`, `params`, `has_req_binding`, `line`, `column`) and `use api`/`use sockets` bindings without executing code. |
| `grab_inspect_schema` | Read-only | Reads `grab.data.json` / `grab.api.json` inside workspace; redacts secret env var values (metadata names only). |
| `grab_scaffold` | Write (`GROXY-SAFE-001` gated unless `dry_run: true`) | Generates `grab check`-passing `.grab`, `.grab.html`, or `grab.api.json` starter files; refuses overwrite unless explicitly approved. |

All 5 tools enforce `GRAB-E401` (rejecting `\x00`, UNC `\\`/`//`, `..` traversal, escaping symlinks, and `.git`/`.env`/`.ssh`/`*.key`/`*.pem`/`*.p12`/`*.pfx`).

### 2.3 GrabLab CMS `v1.0.1` MCP & Draft Tools
Groxy Desk connects to the active local GrabLab CMS worker (`127.0.0.1:8765–8772`) over loopback HTTP (`POST /api/mcp` / `POST /api/mcp/tools/<name>`) using the configured capability-gated CMS MCP token (stored encrypted or in host env, never leaked in DOM/logs):

| Tool | Capability / Safety Contract |
|---|---|
| `list_content_types` | Read-only discovery of registered CMS content types (`post`, `page`, custom types). |
| `query_entries` | Read-only filtered query (`content_type_slug`, `status`, `filter_by_term`, `field_*`, `sort_*`, `page`, `per_page`). |
| `create_entry_draft` | **Strictly enforces `status: "draft"` (`status_enforced: "draft"`)** regardless of requested status; validates optional `grex_layout`/`layout` through `validateElementNode` + WCAG AA (`422` / JSON-RPC `-32602` on violation). Requires `GROXY-SAFE-001` approval. |
| `update_entry_layout` | Accepts `entry_id` + `layout`/`grex_layout`, validates against `grex.elements.json` & WCAG AA (`422` / `-32602` on wrapper-`<div>`, `<script>`, unsafe URL, missing `alt`, skipped heading, or `< 4.5:1` contrast). Requires `GROXY-SAFE-001` approval + Pre-Mutation Snapshot. |
| `list_media` | Read-only media asset discovery (`mime_filter`, pagination) for `alt`-text enrichment and image insertion. |
| `list_menus` | Read-only navigation menu discovery (with optional `location` filter) for `SITE-CONTEXT.md` internal linking. |

### 2.4 Grex Theme Builder `v1.0.0` Visual Synthesis & Live Canvas Bridge
- **Design System & Context Ingestion:** Reads live `GRAB-DESIGN-SYSTEM.md` (`GET /api/design-system.md`), `SITE-CONTEXT.md`, and vendored `grex.elements.json` (`20` semantic elements: `Section`, `Container`, `Grid`, `Stack`, `Card`, `Heading`, `Text`, `RichText`, `Link`, `Button`, `Image`, `Icon`, `Form`, `Input`, `Select`, `Tabs`, `Accordion`, `Modal`, `Nav`, `ForEach`).
- **8 Design Style Packs × 5 Blueprints (`40` WCAG AA Combinations):** Supports composing and refining layouts across all 8 curated packs (`emerald-editorial`, `obsidian-tech`, `warm-commerce`, `swiss-minimal`, `local-service-trust`, `creative-studio-bold`, `creator-academy`, `wellness-organic`) and 5 blueprints (`full_landing_page`, `hero_section`, `pricing_grid`, `faq_accordion`, `foreach_showcase`), guaranteeing Light/Dark contrast `>= 4.5:1` and `Grade A` / `A+` (`>= 90`) pre-dispatch.
- **Approved Dispatch & 1-Click `Ctrl+Z` Undo Parity:** Dispatches approved layouts to the active worker's `POST /api/mcp/layout` with `actor: "groxy-ai"`, `X-CSRF-Token`, and `approval_token` (`GROXY-SAFE-001`), recording an `applyAiLayout` command in the studio command stack so `Ctrl+Z` / **Undo AI Layout** restores the prior tree in 1 click without worker restarts.

---

## 3. Interactive Split-Preview Workbench Canvas & Origin-Checked Bridge

### 3.1 Center Workbench Split-Pane Modes
The `<main>` center workbench from `v0.1.0` upgrades to a 3-mode view switcher (built strictly with Grex semantic elements, `divCount === 0`):
1. **`Code`:** `.grab` / `.grab.html` editor with live `grab_check` / `grab_format` diagnostics and redacted `Grisp` logs.
2. **`Split` (Code + Live Preview):** Side-by-side code/JSON layout editor paired with the live preview canvas.
3. **`Visual & Scorecard`:** Full-width interactive `Grex` preview canvas paired with the 22-Rule 4-Pillar Scorecard & Style Pack / Blueprint synthesizer.

### 3.2 Origin-Checked `GrexPreviewBridge` (`PREVIEW_CHANNEL = "grex-preview-v1"`)
- **Strict Origin Allowlist:** The preview canvas connects to the active local `Grisp` worker (`http://127.0.0.1:<8765–8772>`) or the embedded Grex preview renderer (`http://127.0.0.1:9490`) using `GrexPreviewBridge` with an explicit non-empty `allowedOrigins` allowlist. Wildcard `"*"` origins are rejected (`GrexSecurityError`).
- **Channel & Payload Validation:** Every `postMessage` frame MUST carry `channel === "grex-preview-v1"` and originate from an allowlisted `event.origin`. Untrusted origins or mismatched channels throw/reject immediately and log a redacted security diagnostic.
- **Responsive Viewport Switcher:** Supports `Base` (`100%` desktop), `Tablet` (`768px`), and `Mobile` (`375px`) preview widths with 1:1 visual parity (`styles.base`, `styles.tablet`, `styles.mobile`) and zero `Grisp` worker restarts during live editing.

---

## 4. Built-In 22-Rule Deterministic Auditor, Scorecard & 🚀 `"Let Groxy Boost This"` Diff Engine

### 4.1 4-Pillar / 22-Rule Deterministic Auditor
Runs locally inside `groxy-deskd` + `public/desk/` with zero external network calls, mirroring **Groxy AI `v1.0.0`**'s frozen 22-rule playbook:

| Pillar | Weight | Deterministic Checks |
|---|---|---|
| `on_page_seo` (**On-Page SEO**) | `25%` | Title presence & length, meta description, single `H1`, non-skipped heading hierarchy (`H1 -> H2 -> H3`), canonical/slug hygiene. |
| `content_a11y` (**Content & WCAG Accessibility**) | `25%` | Light & Dark `computeContrastRatio() >= 4.5:1` (`>= 3.0:1` large), non-decorative `Image` `alt` text, `:focus-visible` outlines, accessible button/link labels. |
| `performance_dom` (**Performance & Clean DOM**) | `25%` | `divCount === 0`, `innerHTMLWriteCount === 0`, zero `<script>` / `eval()` / unsafe URLs, bounded DOM depth, valid `grex.elements.json` nodes. |
| `linking_readiness` (**Off-Page / Internal Linking Readiness**) | `25%` | `SITE-CONTEXT.md` internal link graph (zero orphan/broken links), `schema.org` JSON-LD (`Article`, `Product` with `ADR-025c` exact price, `FAQPage`) preview. |

- **Scoring & Grades:** Computes exact `0–100` overall and per-pillar scores and grades `A+` (`97–100`), `A` (`90–96`), `B` (`80–89`), `C` (`70–79`), `D` (`60–69`), `F` (`0–59`). Any wrapper-`<div>` or XSS/unsafe-URL violation forces a hard `F` (`publish_safe: false`).

### 4.2 🚀 `"Let Groxy Boost This"` Remediation Workflow (`GROXY-SAFE-001`)
1. **Analyze & Propose (`POST /api/groxy/boost`):** Routes through the Adaptive Reasoning Router's Deep tier to generate deterministic + optional local-LLM (`Ollama`/`OpenCode`) remediations (SEO title/meta, heading fixes, missing `alt` text, contrast-safe Style Pack tokens, `SITE-CONTEXT.md` links, `schema.org` JSON-LD).
2. **Side-by-Side Diff & Projected Score Delta:** Renders a structured before/after diff alongside the projected score/grade delta (e.g., `74 (C) -> 96 (A)`) and issues a single-use, target-bound `approval_token` (`tok_*`).
3. **Pre-Mutation Snapshot & Approved Apply (`POST /api/groxy/boost/approve`):** On explicit user click (**"Approve & Apply Boost"**), `groxy-deskd` verifies `Authorization: Bearer`, `X-CSRF-Token`, and the target-bound `approval_token` (`403 DESK-E403` on missing/mismatched token, `409 DESK-E409` on replay), captures an automatic SHA-256 **Pre-Mutation Snapshot**, applies the change (or dispatches `POST /api/mcp/layout` with `actor: "groxy-ai"`), and enables 1-click **Snapshot Rollback** (`POST /api/groxy/snapshots/restore`) + `Ctrl+Z` undo.

---

## 5. `v0.2.0` Host Daemon HTTP API Extensions (`127.0.0.1:9490`)

All routes require `Authorization: Bearer <token>` (constant-time `subtle.ConstantTimeCompare`), and all `POST` routes require `X-CSRF-Token`:

| Method & Path | Auth | Contract |
|---|---|---|
| `GET /api/mcp/tools` | Bearer | Lists unified available tools across Core GRAB (`5`), GrabLab CMS (`6`), and Grex Builder (`2`), with read/write mode badges. |
| `POST /api/mcp/call` | Bearer + CSRF (+ `approval_token` for writes) | Executes a workspace-confined `grab mcp` or GrabLab CMS tool via structured `argv` / loopback HTTP (`GRAB-E401` enforced). |
| `GET /api/studio/design-system` | Bearer | Proxies/reads live `GRAB-DESIGN-SYSTEM.md`, `SITE-CONTEXT.md`, `grex.elements.json`, and the `8` Style Packs × `5` Blueprints catalog. |
| `POST /api/studio/synthesize` | Bearer + CSRF | Composes a `validateElementNode` + `auditAccessibility` + `Grade A/A+` Grex layout tree for a selected Style Pack & Blueprint. |
| `POST /api/studio/layout/apply` | Bearer + CSRF + `approval_token` | Captures Pre-Mutation Snapshot and dispatches `POST /api/mcp/layout` (`actor: "groxy-ai"`) with `applyAiLayout` undo metadata. |
| `POST /api/groxy/audit` | Bearer + CSRF | Runs the deterministic 22-rule 4-pillar auditor and returns `0–100` / `A+`–`F` scorecard + `publish_safe` flag. |
| `POST /api/groxy/boost` | Bearer + CSRF | Generates 🚀 `"Let Groxy Boost This"` side-by-side before/after diffs, projected score delta, and target-bound `approval_token`. |
| `POST /api/groxy/boost/approve` | Bearer + CSRF + `approval_token` | Validates single-use `approval_token` (`403`/`409`), captures SHA-256 Pre-Mutation Snapshot, and applies approved Boost changes. |

---

## 6. Non-Negotiable Architecture & Security Invariants

1. **Pure `Grex JS v1.0.0` Zero-`<div>` UI (`GREX-SCHEMA-001`):** All new `v0.2.0` panels (Workbench mode switcher, Split-Preview frame, Responsive Viewport bar, 22-Rule Scorecard, Style Pack/Blueprint selector, Boost Diff drawer, and CMS Draft modal) MUST satisfy `divCount === 0`, `innerHTMLWriteCount === 0`, zero `eval()` / `new Function()`, zero unsafe URLs (`javascript:`, `vbscript:`, `file:`, unsanitized `data:`), and Light/Dark WCAG AA contrast `>= 4.5:1` (`>= 3.0:1` large text) with visible `:focus-visible` rings.
2. **Exact `ADR-025c` Numeric Fidelity:** All 22-rule scores, pillar weights (`25`), projected deltas, `GrabDecimal` / `GrabInteger` custom fields, `schema.org` `Product` prices, and worker ports MUST preserve exact base-10 wire strings without IEEE-754 float coercion.
3. **Draft-Only CMS Safety & Origin-Checked Bridge:** `create_entry_draft` MUST enforce `status_enforced: "draft"`, `GrexPreviewBridge` MUST reject wildcard `"*"` or non-allowlisted origins, and all mutating actions MUST enforce `GROXY-SAFE-001` (`403 DESK-E403` / `409 DESK-E409`) + Pre-Mutation Snapshots.

---

## 7. Mandatory `v0.2.0` Verification Gates

Implementation of `v0.2.0` is NOT complete until all **four** verification gates pass at `100%`:

1. **Gate 1 — Go Host Daemon & MCP Bridge Suite:**
   - `go build ./...`, `go vet ./...`, `go test -count=1 ./...`, and `govulncheck ./...` (`0` errors, `0` vulnerabilities) covering `internal/mcpbridge`, `internal/auditor`, `internal/groxybridge`, `internal/server`, and `internal/security`.
2. **Gate 2 — Pure `Grex JS v1.0.0` Zero-`<div>`, WCAG AA & Preview Bridge Suite:**
   - `node --test tests/*.test.js` verifying `divCount === 0`, `innerHTMLWriteCount === 0`, zero `eval()` / unsafe URLs, zero model-switching dropdowns, Light & Dark `computeContrastRatio() >= 4.5:1`, `100%` `auditAccessibility()`, `GrexPreviewBridge` (`PREVIEW_CHANNEL = "grex-preview-v1"`) origin rejection, all `8 × 5` Style Pack × Blueprint combinations (`Grade A/A+`), and `ADR-025c` base-10 wire preservation.
3. **Gate 3 — Security, Confinement & Approval Gate Suite:**
   - Automated tests proving `GRAB-E401` confinement across all 5 `grab mcp` tools, `status_enforced: "draft"` on `create_entry_draft`, `403 DESK-E403` on missing/mismatched CSRF or `approval_token`, `409 DESK-E409` on `approval_token` replay, and zero shell-string interpolation (`sh -c` / `cmd /c`).
4. **Gate 4 — Live Cross-Layer Python E2E Integration Suite:**
   - `python tests/integration_e2e.py` launching `groxy-deskd` (`127.0.0.1:9490`) + live `GRAB v1.1.0` `Grisp` worker (`127.0.0.1:8765–8772`), executing `grab mcp` (`grab_check`, `grab_format`, `grab_inspect_routes`, `grab_inspect_schema`), GrabLab CMS draft + `update_entry_layout` (`422` on bad tree, `200`/`201` on valid draft), Grex Theme Builder `GET /api/design-system.md` + `POST /api/mcp/layout` (`actor: "groxy-ai"`) + `Ctrl+Z` undo, 22-rule audit + 🚀 `"Let Groxy Boost This"` diff -> `approval_token` apply -> `409` replay rejection -> 1-click Snapshot Rollback.
