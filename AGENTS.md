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
- **PostgreSQL-First Production Rule.** Target native Ubuntu PostgreSQL 16+ (including 18) with Redis 8.2 for multi-worker production. MariaDB remains a supported compatibility target. Use workspace-confined SQLite files only for zero-config, single-node development and test fixtures. SQLite has no native TCP listener.
- **Live Stable Tag Verification Rule.** Before citing a stable tag in any `HANDOFF.md` or specification, verify the relevant repository’s `stable/main` against `git ls-remote stable refs/heads/main`. Refresh the tracking ref and tags if needed, then run `git describe --tags --abbrev=0 stable/main` in that repository. If live verification fails, leave the stable version unconfirmed.
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

### Mandatory Milestone Snapshot & Codex Handoff Rule
- **Ecosystem Snapshot & Codex Handoff Sync:** At the end of every milestone (before closing the milestone, handing off between agents, or pushing release promotions), every agent MUST run:
  ```powershell
  powershell -ExecutionPolicy Bypass -File C:\Development\GRAB\scripts\sync-notebook-sources.ps1
  powershell -ExecutionPolicy Bypass -File C:\Development\GRAB\scripts\generate-codex-handoff.ps1
  ```
  so `C:\Development\ECOSYSTEM-SNAPSHOT.md` and `C:\Development\CODEX-HANDOFF.md` always reflect the latest Git branches, commits, tags, `AGENTS.md`, `*-EXPERT.md`, `HANDOFF.md`, `docs\ROADMAP.md`, and `specifications\*.md` across all discovered `C:\Development\*` repositories.

---

## 4. Codex and Antigravity model and effort routing

| Work | OpenAI Codex | Google Antigravity |
|---|---|---|
| Stage 1 specifications, roadmaps and governance | GPT-6.2 Sol (Medium), alternate GPT-6.1 Sol (Medium) | Claude Sonnet (Medium) or Hydrogen (Medium), backup GPT-OSS 120B (Medium) |
| Stage 2 cross-layer security and contract audit | GPT-6.2 Astra (High) | Claude Opus (High) or Orchestrator (Argon) |
| Stage 3 multi-file implementation and unit or DOM tests | GPT-6.2 Sol (High), alternate GPT-6.1 Terra (High) | Rotate Claude Sonnet (High), GPT-OSS 120B (High) and Hydrogen (High) across milestones |
| Stage 4 independent security audit and release sign-off | GPT-6.2 Astra (High) | Claude Opus (High) or Orchestrator (Argon) for cross-layer review |
| Quick UI edits, snapshot sync, packaging and walkthroughs | GPT-6 Luna (Medium), alternate GPT-6.1 Sol (Medium) | Gemini 3.8 Flash (single package) or Hydrogen (High), including final E2E gate execution |

At milestone boundaries and before high-risk audits, pause and identify the next review or implementation step. Close every checkpoint with **Recommended Next Model & Effort Tier**, naming the model, effort and reason. Reserve Hydrogen (Extra) or Claude Opus (High) for stubborn cross-layer failures.

### Package scope and handoff

- Use the stage matrix for multi-package work. Gemini 3.8 Flash, OpenCode, Cline and Kilo Code must read `GROXY-DESK-EXPERT.md` first, edit one Go package or JS module at a time and immediately verify it with `go test -count=1 ./<package>` or `node --test`.
- After two unsuccessful attempts on Hydrogen (Extra) or Orchestrator (Argon), record the attempts in `HANDOFF.md` and hand off to OpenAI Codex. Revert only the failed attempt’s edits and ask the owner first if that would affect pre-existing work.
- Record the incoming agent in `HANDOFF.md` under **Current Owner**, with the next task and pending checks. On pickup, read `GROXY-DESK-EXPERT.md`, `AGENTS.md` and `HANDOFF.md`, verify the applicable baseline and continue.
