# GROXY-DESK-EXPERT.md — Non-Negotiable Engineering Invariants

This checklist defines the **never-break invariants** for **Groxy Desk** (`urbanspress/groxy-desk`), **Step 6 of the Post-1.0 GRAB Ecosystem**. Every specification, desktop host route, MCP bridge call, UI view, supervisor action, and release must satisfy all seven pillars.

---

## 1. Single Unified Default Agent (Groxy AI Only — Zero Model-Switching Dropdowns)
- **One Permanent Built-In Agent Identity:** **Groxy Desk** uses **Groxy AI** as its sole, permanent built-in agent identity across the entire IDE. Users MUST NEVER be shown model-picker or model-switching dropdowns in the chat bar, sidebar, or workflow panels.
- **Automatic Adaptive Reasoning Router:** Under the hood, Groxy Desk's local host bridge runs Groxy AI's **Automatic Adaptive Reasoning Router**, seamlessly routing:
  - **Instant / Fast Tier:** Daily tasks such as `grab check`, `grab format`, route/schema lookups, content drafts, 22-rule SEO/WCAG/DOM audits, media alt-text checks, and local `Grisp` worker start/stop/reload toggles.
  - **Deep / High-Reasoning Tier:** Multi-file vibe-coding refactors, `GRAB-DESIGN-SYSTEM.md`-aware Grex visual layout synthesis, 🚀 **"Let Groxy Boost This"** multi-pillar remediation diffs, `SITE-CONTEXT.md` internal linking & `schema.org` JSON-LD architecture, and Step 5 fleet/database/DNS orchestration.
- **Local-First & Zero Mandatory API Fees:** Supports local models (Ollama / OpenCode) by default with zero mandatory recurring cloud API fees; optional user-configured provider keys stay encrypted at rest (`use crypto` AES-256-GCM / host env) and are never exposed in DOM, logs, or URLs.

---

## 2. 100% Full-Stack `grab mcp` Capability Parity Across All Layers
Everything **Groxy AI** can do via `grab mcp` inside **GrabLab CMS** and **Grex Theme Builder**—plus Core **GRAB** CLI tools and **Step 5** orchestration—MUST work natively inside **Groxy Desk**:
- **Core GRAB `v1.1.0` MCP & Vibe-Coding Tools:**
  - `grab_check`, `grab_format`, `grab_inspect_routes`, `grab_inspect_schema`, `grab_scaffold`, multi-file `.grab` / `.grab.html` code & template diffing, and local `Grisp` multi-worker supervision (`/_health`, `/_ready`, atomic `--reload`).
- **GrabLab CMS `v1.0.1` MCP & Content/SEO Tools:**
  - `list_content_types`, `query_entries`, `create_entry_draft` (`status_enforced: "draft"`), `update_entry_layout`, `list_media`, `list_menus`, the 22-rule 4-pillar SEO/WCAG/DOM/Link scorecard (`A+`–`F` / `0–100`), 🚀 **"Let Groxy Boost This"** side-by-side remediation diffs, `SITE-CONTEXT.md` internal linking & `schema.org` JSON-LD generation, and media `alt`-text enrichment.
- **Grex Theme Builder `v1.0.0` MCP & Visual Studio Tools:**
  - Live `GRAB-DESIGN-SYSTEM.md` (`GET /api/design-system.md`) and `grex.elements.json` ingestion, all 8 Design Style Packs (40 blueprint combinations), targeted section refinement, and `POST /api/mcp/layout` (`actor: "groxy-ai"`) execution with 1-click `Ctrl+Z` (`applyAiLayout`) command-stack undo parity inside an embedded split-screen live preview canvas.
- **Step 5 Orchestration & Fleet Management (`grabman v1.0.0` + `Grabtaculous v1.0.0` + `Gashboard v1.0.0`):**
  - Workspace scaffolding & security doctor (`grabman init`, `grabman solve --verify`, `grabman doctor`), pinned `127.0.0.1` SSH database tunnels & read-only-default Visual DB Manager, Namecheap DNS + Auto-HTTPS Wizard, and 1-click Marketplace / `Site DNA` (`groxy-site-dna-v0.4.0`) fleet deploy & snapshot rollback.

---

## 3. Loopback-Only Control Plane & Process Supervision (`ADR-005`, `ADR-017`)
- **Strict `127.0.0.1` Binding:** The Groxy Desk local host daemon MUST bind strictly to `127.0.0.1` (default `127.0.0.1:9490`) and reject `0.0.0.0`, `::`, wildcards, or non-loopback addresses (`DESK-E501`).
- **Constant-Time Auth & CSRF Protection:** Every host bridge API call MUST validate the per-session bearer token using `subtle.ConstantTimeCompare`, and every mutating `POST` action MUST enforce `X-CSRF-Token` (`403 DESK-E403`).
- **Structured Argv Spawning Only:** All child processes (`grab.exe`, `grabman.exe`, `gashboard-bridge`, `ollama`, `git`) MUST be launched via structured OS `argv` slices (`exec.CommandContext`)—NEVER via `sh -c`, `cmd /c`, or interpolated shell strings.
- **`GRAB-E401` Workspace Confinement:** Reject empty paths, `\x00` NUL bytes, UNC `\\`/`//` shares, `..` traversal, escaping symlinks (`filepath.EvalSymlinks`), and reads/writes touching `.git`, `.env`, `.env.*` (except explicit `.env.example`), `.ssh`, `id_rsa`, `id_ed25519`, `id_ecdsa`, `id_dsa`, `*.pem`, `*.key`, `*.p12`, or `*.pfx`.

---

## 4. Pure `Grex JS v1.0.0` Zero-`<div>` IDE Shell & Light/Dark WCAG AA Parity
- **100% Semantic Elements (`GREX-SCHEMA-001`):** Every Groxy Desk workbench template and dynamically mounted panel MUST use strictly the 20 semantic elements from `grex.elements.json` (`Section`, `Container`, `Grid`, `Stack`, `Card`, `Heading`, `Text`, `RichText`, `Link`, `Button`, `Image`, `Icon`, `Form`, `Input`, `Select`, `Tabs`, `Accordion`, `Modal`, `Nav`, `ForEach`) with `divCount === 0`, `innerHTMLWriteCount === 0`, zero `eval()` / `new Function()`, and zero unsafe URL protocols (`javascript:`, `vbscript:`, `file:`, unsanitized `data:`).
- **Zero-FOUC Light/Dark WCAG AA Parity:** Support Light and Dark workbench themes (`data-grex-theme="light|dark"`) with zero flash on startup, passing `computeContrastRatio() >= 4.5:1` (`>= 3.0:1` large text) and `100%` `auditAccessibility()` checks including `:focus-visible` keyboard navigation.

---

## 5. Exact Numeric Fidelity (`ADR-025c`) & Human-in-the-Loop Safety (`GROXY-SAFE-001`)
- **Base-10 `GrabDecimal` / `GrabInteger` Preservation:** Preserve base-10 wire strings across all inspector, AI diff, `Site DNA`, port/metric, and Visual DB Manager panels without IEEE-754 floating-point drift.
- **Target-Bound Approval Gate & Pre-Mutation Snapshots:** Require an explicit single-use, target-bound `approval_token` + `X-CSRF-Token` confirmation + automatic SHA-256 **Pre-Mutation Snapshot** before any AI file write, `applyAiLayout` commit, DB mutation, or remote deployment (`403 DESK-E403` on mismatch, `409 DESK-E409` on replay). Never auto-publish live GrabLab pages (`status_enforced: "draft"`).

---

## 6. Native Windows Packaging & Dual-Repo SemVer Discipline
- **Lean Native Windows Shell:** Compile the Go loopback host daemon with `CGO_ENABLED=0` and pair it with a lightweight native Windows WebView2 launcher (~15–25 MB RAM overhead, zero Electron/Chromium bundle bloat). Package releases exclusively with an **Inno Setup** Windows installer (`Groxy-Desk-Setup-<X.Y.Z>.exe` — **never `.msi`**) and `SHA256SUMS.txt`.
- **Committed Emerald `#10B981` Vector Badges:** Keep `docs/badges/built-with-grab.svg`, `docs/badges/powered-by-grex-js.svg`, `docs/badges/ecosystem-step-6.svg`, and `docs/badges/spec-in-review.svg` (or release badge) committed and linked in `README.md`.
- **Dual-Repo Protection & 7-Layer Compatibility Matrix:** Never push unverified builds or unapproved specs to `stable` (`urbanspress/groxy-desk.stable`). Every release across both repositories MUST include the **Release Type & Affected Layers** summary block, the **7-Layer Ecosystem Compatibility Matrix**, and the plain-English walkthrough PDF (`docs/releases/pdf/Groxy-Desk-v<X.Y.Z>-Guide.pdf`).
