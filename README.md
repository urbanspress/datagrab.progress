# Groxy Desk

![Built with GRAB v1.1.0](docs/badges/built-with-grab.svg)
![Powered by Grex JS v1.0.0](docs/badges/powered-by-grex-js.svg)
![GRAB Ecosystem Step 6 — Groxy Desk](docs/badges/ecosystem-step-6.svg)
![v0.1.0 Spec — In Review](docs/badges/spec-in-review.svg)

**Groxy Desk** is the Native Windows WebView2 + **`Grex JS v1.0.0`** Desktop Vibe-Coding IDE, Local **`Grisp`** (`grab.exe`) Multi-Worker Supervisor, Unified **`Groxy AI`** + `grab mcp` Studio, and **Step 5** (`grabman` + `Grabtaculous` + `Gashboard`) Bridge for **Step 6 of the Post-1.0 GRAB Ecosystem**.

- **Current Status:** `v0.1.0` — **Specification in Review** ([`specifications/GROXY-DESK-SPEC-001.md`](specifications/GROXY-DESK-SPEC-001.md) drafted for project-owner approval; strictly zero application code written yet).
- **Development Repo (`origin`):** `https://github.com/urbanspress/groxy-desk.progress`
- **Production Release Repo (`stable`):** `https://github.com/urbanspress/groxy-desk.stable`

---

## Core Architecture Pillars

1. **Single Unified Default Agent (`Groxy AI` Only — Zero Model-Switching Dropdowns):**
   - **Groxy AI** is the sole, permanent built-in agent identity. Its **Automatic Adaptive Reasoning Router** handles both instant daily checks (`grab check`, `grab format`, route/schema lookups, content drafts, server toggles) and deep multi-step vibe coding, visual layout synthesis, and fleet orchestration seamlessly—with zero model-picker dropdowns.
2. **100% Full-Stack `grab mcp` Capability Parity:**
   - Unifies Core **GRAB `v1.1.0`** (`grab_check`, `grab_format`, `grab_inspect_routes`, `grab_inspect_schema`, `grab_scaffold`), **GrabLab CMS `v1.0.1`** (`create_entry_draft`, 22-rule SEO/WCAG/DOM/Link scorecard, 🚀 `"Let Groxy Boost This"`, `SITE-CONTEXT.md`), **Grex Theme Builder `v1.0.0`** (`GRAB-DESIGN-SYSTEM.md`, 8 Design Style Packs, `POST /api/mcp/layout` with 1-click `Ctrl+Z` `applyAiLayout` undo), and **Step 5** (`grabman`, `Grabtaculous`, `Gashboard`).
3. **Loopback-Only Control Plane & `GRAB-E401` Confinement (`ADR-005`, `ADR-017`):**
   - Host daemon binds strictly to `127.0.0.1:9490` with constant-time bearer + `X-CSRF-Token` auth, structured `argv` process spawning (zero `sh -c` / `cmd /c`), and strict workspace path confinement.
4. **Pure `Grex JS v1.0.0` Zero-`<div>` IDE Shell & WCAG AA Light/Dark Themes:**
   - 100% semantic elements from `grex.elements.json` (`divCount === 0`, `innerHTMLWriteCount === 0`, zero `eval()`), zero-FOUC Light/Dark contrast `>= 4.5:1`, and `ADR-025c` exact base-10 numeric wire preservation.
5. **Human-in-the-Loop Safety (`GROXY-SAFE-001`) & Native Windows Packaging:**
   - Target-bound single-use `approval_token` + automatic Pre-Mutation Snapshots before any AI file write, layout apply, DB mutation, or deployment; packaged as a lean native Windows WebView2 app via Inno Setup (`Groxy-Desk-Setup-<X.Y.Z>.exe`).

---

## 7-Layer Ecosystem Compatibility Matrix

| Ecosystem Layer | Repository | Required Version Range | Role in Groxy Desk |
|---|---|---|---|
| **Core Language, Bytecode VM & Grisp Server** | `urbanspress/grab` | `>= v1.1.0 (< v2.0.0)` | Local `Grisp` worker runtime, `grab check`/`format`/`mcp` |
| **Reactive Frontend Runtime & 20-Element Library** | `urbanspress/grex-js` | `>= v1.0.0 (< v2.0.0)` | Zero-`<div>` IDE workbench shell & WCAG AA theme engine |
| **Flagship Zero-Plugin CMS** | `urbanspress/grablab-cms` | `>= v1.0.1 (< v2.0.0)` | Content types, drafts (`status_enforced: "draft"`), menus, media |
| **Exclusive `mode: "visual"` Theme & Studio** | `urbanspress/grex-builder` | `>= v1.0.0 (< v2.0.0)` | `GRAB-DESIGN-SYSTEM.md`, 8 Style Packs, `Ctrl+Z` layout canvas |
| **Autonomous Design, Content & SEO Agent** | `urbanspress/groxy-ai` | `>= v1.0.0 (< v2.0.0)` | 22-rule scorecard, 🚀 Boost diffs, `Site DNA`, Adaptive Router |
| **Scaffolder, Orchestrator & Discreet VPS Panel** | `urbanspress/grabman`, `urbanspress/grabtaculous`, `urbanspress/gashboard` | `>= v1.0.0 (< v2.0.0)` | `grabman init/doctor/solve`, SSH DB tunnels, DNS/HTTPS, fleet rollouts |
| **Native Desktop Vibe-Coding IDE & Studio** | `urbanspress/groxy-desk` | `>= v0.1.0` (`Spec in Review`) | Unified desktop IDE, `Grisp` supervisor & full-stack MCP studio |

---

## Governance & Specifications

- [Agent Rules (`AGENTS.md`)](AGENTS.md)
- [Engineering Invariants (`GROXY-DESK-EXPERT.md`)](GROXY-DESK-EXPERT.md)
- [`v0.1.0` Specification (`specifications/GROXY-DESK-SPEC-001.md`)](specifications/GROXY-DESK-SPEC-001.md) — **In Review**
- [Roadmap (`docs/ROADMAP.md`)](docs/ROADMAP.md)
- [Handoff Log (`HANDOFF.md`)](HANDOFF.md)
- [License (`LICENSE` — MIT)](LICENSE)
