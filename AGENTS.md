# AGENTS.md — Groxy Desk

**Groxy Desk** (`urbanspress/groxy-desk.progress` / `urbanspress/groxy-desk.stable`, local directory `C:\Development\groxy-desk`) is the Native Windows WebView2 + **`Grex JS v1.0.0`** Desktop Vibe-Coding IDE, Local `Grisp` (`grab.exe`) Multi-Worker Supervisor, Unified **`Groxy AI`** + `grab mcp` Studio, and **Step 5** (`grabman` + `Grabtaculous` + `Gashboard`) Bridge for **Step 6 of the Post-1.0 GRAB Ecosystem**.

---

## 1. Core Invariants & Upstream Scope
1. **Read [`GROXY-DESK-EXPERT.md`](GROXY-DESK-EXPERT.md) first** before drafting any specification, editing any file, or reviewing any pull request.
2. **Respect Frozen Upstream Contracts:** Build **Groxy Desk** strictly on top of the frozen `v1.x` ecosystem baselines:
   - **GRAB `v1.1.0`** (`grab serve`, `grab check`, `grab format`, `grab migrate`, `grab mcp`, `use crypto`, `use env`)
   - **Grex JS `v1.0.0`** (`grex.elements.json`, `auditAccessibility()`, `computeContrastRatio()`, zero wrapper-`<div>` reactive signals)
   - **GrabLab CMS `v1.0.1`** (`grab mcp` CMS tools, `status_enforced: "draft"`, `SITE-CONTEXT.md`)
   - **Grex Theme Builder `v1.0.0`** (`GET /api/design-system.md`, `POST /api/mcp/layout`, `applyAiLayout` 1-click `Ctrl+Z` undo, 8 Design Style Packs)
   - **Groxy AI `v1.0.0`** (22-rule SEO/WCAG/DOM/Link scorecard, 🚀 `"Let Groxy Boost This"`, `groxy-site-dna-v0.4.0`, Automatic Adaptive Reasoning Router — zero model-switching dropdowns)
   - **`grabman v1.0.0`**, **Grabtaculous `v1.0.0`**, and **Gashboard `v1.0.0`** (Step 5 scaffolder, doctor, lockfile, `--read-only` fleet rollout, SSH DB tunnels, Visual DB Manager, Namecheap DNS + Auto-HTTPS)
   Never duplicate upstream compiler internals or fork frozen upstream contracts inside `groxy-desk`.
3. **Loopback-Only & Zero Shell Interpolation (`ADR-005`, `ADR-017`):** Bind strictly to `127.0.0.1` (`127.0.0.1:9490`), authenticate local bridge calls via constant-time bearer token + `X-CSRF-Token`, spawn all subprocesses via structured `argv` slices only, and enforce `GRAB-E401` workspace confinement.
4. **Pure `Grex JS v1.0.0` Zero-`<div>` IDE UI:** All Groxy Desk templates and UI modules MUST satisfy `divCount === 0`, `innerHTMLWriteCount === 0`, zero `eval()` / `new Function()`, zero unsafe URLs, `100%` WCAG AA (`>= 4.5:1` Light & Dark), and exact `ADR-025c` base-10 numeric wire preservation.

---

## 2. Workflow, Spec Gate & Verification Gates
- **Spec-First Approval Gate:** Do NOT write `groxy-desk` application code (Go, GRAB, C/Win32/WebView2, or JS) until the active milestone specification (starting with `specifications/GROXY-DESK-SPEC-001.md`) has been reviewed and explicitly approved by the project owner.
- **Plain-English User Communication:** The project owner directs development in plain English. Always explain IDE workflows, local `Grisp` workers, `Groxy AI` actions, and fleet/database bridges simply without unnecessary jargon. Handle all file edits, git commands, and terminal verification directly.
- **No Unapproved Commits/Pushes:** Do not commit, push, or publish releases without explicit project-owner approval (explicitly requested repository bootstraps excepted).
- **Mandatory Test & Lint Gate:** Before marking any implementation milestone complete or requesting release promotion:
  1. Run `go build ./...`, `go vet ./...`, `go test -count=1 ./...`, and `govulncheck ./...` on the Go loopback host daemon and desktop launcher (`0` errors, `0` vulnerabilities).
  2. Run `grab check` and `grab format` parity checks on all `.grab` / `.grab.html` files.
  3. Verify `divCount === 0`, `innerHTMLWriteCount === 0`, zero unsafe URLs, `100%` `auditAccessibility()` pass (Light & Dark `>= 4.5:1`), and `ADR-025c` numeric fidelity across all UI views (`node --test tests/*.test.js`).
  4. Run the full cross-layer integration suite verifying `Groxy Desk` + `GRAB v1.1.0` + `Grex JS v1.0.0` + `grabman v1.0.0` with **100% passing results**.

---

## 3. Two-Repository Workflow & Release Protection
- **Repository Roles:**
  - `origin` (`https://github.com/urbanspress/groxy-desk.progress.git`): Active development repository for specifications, drafts, work-in-progress checkpoints, and milestone commits.
  - `stable` (`https://github.com/urbanspress/groxy-desk.stable.git`): Production release repository for 100% verified milestone releases only.
- **Strict `stable` Protection:** All AI agents and models MUST push work-in-progress commits, unapproved specs, and partial checkpoints ONLY to `origin` (`urbanspress/groxy-desk.progress`). Pushing to `stable` (`urbanspress/groxy-desk.stable`) is strictly forbidden unless the milestone is 100% `Complete` in `docs/ROADMAP.md`, all verification gates pass 100%, and `HANDOFF.md` records no pending milestone tasks.
- **Synchronized Releases & Inno Setup Installers:** Whenever a completed milestone (`v0.1.0` onward) is promoted to `stable`, push `main` and annotated version tags in strict ascending chronological order to both `origin` and `stable`, build `Groxy-Desk-Setup-<X.Y.Z>.exe` (Inno Setup `.exe` only, never `.msi`) + `SHA256SUMS.txt`, generate `docs/releases/pdf/Groxy-Desk-v<X.Y.Z>-Guide.pdf`, and publish matching GitHub Releases + assets on **both** repositories (`urbanspress/groxy-desk.progress` and `urbanspress/groxy-desk.stable`).

### SemVer (MAJOR.MINOR.PATCH) & 7-Layer Ecosystem Compatibility Matrix Standard
1. **Strict `MAJOR.MINOR.PATCH` Semantic Versioning (`ADR-027c`):**
   - **`0.x.y` (`PRE-1.0`):** Pre-`1.0.0` Step 6 desktop IDE milestones (`v0.1.0` supervisor/bridge, `v0.2.0` full MCP/studio parity, `v0.3.0` Step 5 fleet/DB/DNS bridge).
   - **`1.0.x` (`PATCH`):** Strictly for non-breaking bug fixes, security hardening, documentation polish, and cross-layer audit patches (`100%` drop-in compatible).
   - **`1.x.0` (`MINOR`):** For backwards-compatible new features or additive IDE/MCP panels.
   - **`2.0.0` (`MAJOR`):** Reserved for breaking architectural or contract changes—**never used without explicit project-owner approval (`ADR-027c`)**.
2. **Mandatory Release Compatibility Badge & 7-Layer Matrix Block:**
   - Every GitHub Release across both `.progress` and `.stable` repositories MUST display at the top of its release notes:
     1. A **Release Type & Affected Layers** summary callout (`PRE-1.0 (v0.x.y)` | `PATCH (v1.0.x)` | `MINOR (v1.x.0)`).
     2. A **7-Layer Ecosystem Compatibility Matrix** Markdown table covering all seven ecosystem layers:

````markdown
### Release Type & Affected Layers
- **Release Version & SemVer Tier:** `v<X.Y.Z>` (`PRE-1.0 v0.x.y` | `PATCH 1.0.x — Non-Breaking Bug/Security/Audit Patch` | `MINOR 1.x.0 — Backwards-Compatible Feature Release`)
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
````

### Mandatory Milestone Snapshot Rule
- **Ecosystem Snapshot Sync:** At the end of every milestone (before closing the milestone or pushing release promotions), every agent MUST run:
  ```powershell
  powershell -ExecutionPolicy Bypass -File C:\Development\GRAB\scripts\sync-notebook-sources.ps1
  ```
  so `C:\Development\ECOSYSTEM-SNAPSHOT.md` always reflects the latest Git branches, commits, tags, `AGENTS.md`, `*-EXPERT.md`, `HANDOFF.md`, `docs\ROADMAP.md`, and `specifications\*.md` across all discovered `C:\Development\*` repositories.

---

## 4. 4-Tier Model Routing & Tandem Handoff Protocol

### 4-Tier Antigravity Model Routing
Before ending a turn or starting a new phase, advise the project owner which Antigravity model and effort level to select for the next step so AI Pro quota is used efficiently:
1. **`Google Hydrogen (Medium)`:** Pre-implementation specifications (`*-SPEC-*.md`), documentation, governance updates, and single-module edits.
2. **`Google Hydrogen (High)`:** Standard feature milestone implementations (`v0.1.0` WebView2 shell, `Grisp` supervisor, `grabman` integration, unit/DOM tests).
3. **`Orchestrator (Argon)`:** Second-opinion pre-`v1.0.0` security/DOM/IPC/contract audits, multi-repo cross-layer integrations (`groxy-desk` + `Groxy AI` + `Grex Theme Builder` + `GrabLab CMS` + `Gashboard`), and complex refactors.
4. **`Google Hydrogen (Extra)`:** Reserved for multi-server orchestration, full `grab mcp` + split-preview canvas integration, and stubborn cross-layer bugs.
- **Mandatory Pause & Swap Prompt Rule:** Every active AI agent MUST pause at milestone boundaries or before high-risk cross-layer/pre-`v1.0.0` audits and explicitly tell the user when to switch to `Orchestrator (Argon)`, including a clear 1-sentence reason why `Orchestrator (Argon)` is recommended next.

### Tier 1 vs. Tier 2 Strict Single-Module/Package Routing
- **Tier 1 — Frontier Models (`Google Hydrogen`, `Gemini 4 Argon`, `GPT-6.1 Sol`, `Claude Opus/Sonnet`):** Authorized to draft specifications, implement multi-layer Go host daemon + WebView2 shell + `Grex JS` workbench modules, and execute cross-layer verification suites.
- **Tier 2 — Fast Workhorse & Backup Models (`Gemini 3.8 Flash`, `OpenCode`, `Cline`, `Kilo Code`):** Must operate in **Strict Single-Module / Single-Package Mode** (`GROXY-DESK-EXPERT.md` first, one Go package or JS UI module at a time, immediate `go test -count=1 ./<package>` or `node --test` verification).

### Tandem Handoff Protocol (`Google Antigravity <-> OpenAI Codex`)
- **Google Antigravity -> Codex:** If an issue remains unresolved after 2 attempts on `Google Hydrogen (Extra)` or `Orchestrator (Argon)`, stop editing, revert only the specific files edited during the failed attempt (asking the owner first if pre-existing uncommitted work would be affected), record what was tried in `HANDOFF.md`, update `Current Owner: OpenAI Codex`, and say: `"Ready to hand off to Codex. Type use-codex to continue."`
- **Codex -> Google Antigravity:** When Codex completes an architectural or verification task, update `HANDOFF.md`, set `Current Owner: Google Antigravity`, and say: `"Core task complete—ready to hand off to Google Antigravity. Type use-antigravity to continue."`
- **Automatic Pickup:** On pickup (`use-antigravity` or `use-codex`), immediately read `GROXY-DESK-EXPERT.md`, `AGENTS.md`, and `HANDOFF.md`, verify baseline checks, and continue the next pending task.
