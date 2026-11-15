# GROXY-DESK-SPEC-001: Groxy Desk `v0.1.0` — Native Desktop Shell, Local `Grisp` Supervisor, Workspace Manager & Unified `Groxy AI` Host Bridge

- **Status:** Specification in Review (Awaiting Project-Owner Approval — **Zero Application Code Written**)
- **Target Release:** `v0.1.0` (`PRE-1.0` — Step 6 Native Desktop IDE Foundation)
- **Foundational Baselines:**
  - **GRAB:** `v1.1.0` (`urbanspress/grab` — `Grisp`, `grab serve`, `grab check`, `grab format`, `grab mcp`, `ADR-005`, `ADR-017`, `ADR-025c`)
  - **Grex JS:** `v1.0.0` (`urbanspress/grex-js` — 20 semantic elements, reactive signals, `createTheme`, `computeContrastRatio`, `auditAccessibility`, zero wrapper-`<div>` schema)
  - **GrabLab CMS:** `v1.0.1` (`urbanspress/grablab-cms`)
  - **Grex Theme Builder:** `v1.0.0` (`urbanspress/grex-builder`)
  - **Groxy AI:** `v1.0.0` (`urbanspress/groxy-ai`)
  - **`grabman`, Grabtaculous & Gashboard:** `v1.0.0` Step 5 Freeze (`urbanspress/grabman`, `urbanspress/grabtaculous`, `urbanspress/gashboard`)
- **Governance:** Read alongside [`GROXY-DESK-EXPERT.md`](../GROXY-DESK-EXPERT.md), [`AGENTS.md`](../AGENTS.md), and [`docs/ROADMAP.md`](../docs/ROADMAP.md).

---

## 1. Executive Summary (Plain English)

**Groxy Desk** brings the entire **GRAB Ecosystem** into one fast, lightweight Windows desktop app (~15–25 MB RAM overhead via native Windows WebView2 instead of heavy Electron/Chromium bundles).

In **`v0.1.0`**, Groxy Desk establishes the secure desktop foundation:
1. **One Unified AI Partner (`Groxy AI`) — Zero Model-Switching Dropdowns:** You never have to guess which AI model to pick from a dropdown. **Groxy AI** is the single built-in assistant, automatically choosing fast instant execution for quick checks/drafts and deep multi-step reasoning for complex code/layout/fleet tasks.
2. **1-Click Local `Grisp` (`grab.exe`) Server Manager:** Start, stop, reload, and monitor local GRAB / GrabLab / Grex sites on private loopback ports (`127.0.0.1:8765–8772`) with live `/_health` and `/_ready` status lights and clean logs.
3. **Built-In `grabman v1.0.0` Project Scaffolder & Security Doctor:** Create new GRAB projects (`grabman init`), run security/compatibility checks (`grabman doctor`), and verify `grabman.lock` (`grabman solve --verify`) without touching a terminal.
4. **Pure `Grex JS v1.0.0` Zero-`<div>` Light/Dark IDE Shell:** A clean, accessible, keyboard-friendly desktop workbench built strictly from Grex's 20 semantic elements.

> **Roadmap Note:** Full-parity `grab mcp` CMS/Builder tools + 22-rule SEO scorecard + live split-preview canvas arrive in `v0.2.0`; integrated Gashboard/Grabtaculous SSH DB tunnels, Visual DB Explorer, and Namecheap DNS + Auto-HTTPS arrive in `v0.3.0`; and the Inno Setup Windows installer (`Groxy-Desk-Setup-1.0.0.exe`) + `Orchestrator (Argon)` audit freeze arrive in `v1.0.0`.

---

## 2. `v0.1.0` Architecture & Repository Directory Layout

```text
groxy-desk/
├── cmd/
│   ├── groxy-desk-host/        # Go (CGO_ENABLED=0) loopback host daemon (127.0.0.1:9490)
│   └── groxy-desk/             # Lightweight native Windows WebView2 launcher
├── internal/
│   ├── host/                   # Loopback HTTP server, bearer auth, X-CSRF-Token, approval_token & snapshots
│   ├── safepath/               # GRAB-E401 workspace path confinement, symlink & secret-segment guards
│   ├── supervisor/             # Grisp (grab.exe) multi-worker supervisor (127.0.0.1:8765–8772)
│   ├── workspace/              # Structured argv wrappers for grabman init/doctor/solve & grab check/format
│   └── groxy/                  # Unified Groxy AI Default Agent & Automatic Adaptive Reasoning Router
├── web/
│   ├── index.html              # Zero-FOUC Grex JS v1.0.0 workbench shell (divCount === 0)
│   ├── grex/                   # Vendored Grex JS v1.0.0 (SHA-256 verified against grex-js/dist/)
│   └── desk/                   # Pure Grex JS v1.0.0 modules (workbench, supervisor, grabman, groxy-bar, theme)
├── tests/                      # Go unit/security/integration tests + Node zero-<div>/WCAG AA/contract tests
├── docs/
│   ├── ROADMAP.md
│   └── badges/
├── specifications/
│   └── GROXY-DESK-SPEC-001.md
├── GROXY-DESK-EXPERT.md
├── AGENTS.md
├── HANDOFF.md
└── README.md
```

---

## 3. Loopback Host Daemon (`127.0.0.1:9490`) & Native WebView2 Bridge

### 3.1 Strict Loopback & Process Isolation (`ADR-005`, `ADR-017`)
- **Bind Enforcement (`DESK-E501`):** `groxy-desk-host` MUST bind strictly to `127.0.0.1:9490` (or an explicit `127.0.0.1:<port>` override). `ValidateLoopbackBind` fails closed on `0.0.0.0`, `::`, `*`, or any non-loopback IP.
- **Session Bearer + `X-CSRF-Token` (`DESK-E403`):** On startup, `groxy-desk-host` generates an in-memory 32-byte random bearer token and CSRF token, passing them to the native Windows WebView2 shell over an authenticated local handshake. All `/api/*` requests verify `Authorization: Bearer <token>` via `subtle.ConstantTimeCompare`, and all `POST` routes require `X-CSRF-Token`.
- **Strict Browser CSP:** All HTML responses from `127.0.0.1:9490` emit:
  - `Content-Security-Policy: default-src 'self'; script-src 'self'; style-src 'self'; object-src 'none'; frame-ancestors 'none'`
  - `X-Content-Type-Options: nosniff`
- **Structured `argv` Process Execution:** Every invocation of `grab.exe`, `grabman.exe`, or `ollama` uses `exec.CommandContext` with explicit `argv` slices—never `sh -c`, `cmd /c`, or shell-string interpolation.

### 3.2 `v0.1.0` Host Daemon HTTP API Surface (`127.0.0.1:9490`)

| Method & Path | Auth | Contract |
|---|---|---|
| `GET /_health` | Loopback local | Returns `200 OK` `{ok: true, status: "ok", service: "groxy-desk-host", version: "0.1.0"}`. |
| `GET /_ready` | Loopback local | Returns `200 OK` `{ok: true, ready: true, grab_bin_ok: bool, grabman_bin_ok: bool}`. |
| `GET /api/workspace/tree` | Bearer | Lists confined workspace files/directories (`GRAB-E401` enforced). |
| `POST /api/workspace/read` | Bearer + CSRF | Reads a workspace file (`<= 10 MiB`, `GRAB-E401` enforced). |
| `POST /api/workspace/check` | Bearer + CSRF | Runs `grab check` and `grab format --check` on target `.grab`/`.grab.html` files. |
| `POST /api/grabman/init` | Bearer + CSRF + `approval_token` | Scaffolds a project via `grabman init --dir <confined> --profile <app\|cms-stack\|ai-stack> --json`. |
| `POST /api/grabman/doctor` | Bearer + CSRF | Runs read-only `grabman doctor --dir <confined> --json` and returns plain-English findings. |
| `POST /api/grabman/solve-verify` | Bearer + CSRF | Runs read-only `grabman solve --verify --dir <confined> --json`. |
| `GET /api/grisp/workers` | Bearer | Lists active local `Grisp` workers, ports (`127.0.0.1:8765–8772`), `/_health`, and `/_ready` states. |
| `POST /api/grisp/start` | Bearer + CSRF | Starts a local `Grisp` (`grab serve`) worker on an available port in `127.0.0.1:8765–8772`. |
| `POST /api/grisp/reload` | Bearer + CSRF | Triggers atomic reload / `SIGHUP` on a running worker and verifies `/_ready === 200`. |
| `POST /api/grisp/stop` | Bearer + CSRF | Gracefully drains (`30s` max) and stops the target `Grisp` worker. |
| `GET /api/grisp/logs` | Bearer | Returns bounded, secret-redacted stdout/stderr ring-buffer entries for a worker. |
| `POST /api/groxy/ask` | Bearer + CSRF | Submits a prompt to the Unified **Groxy AI** Default Agent & Adaptive Reasoning Router. |
| `POST /api/groxy/approvals/apply` | Bearer + CSRF + `approval_token` | Applies an approved file/scaffold plan after capturing a SHA-256 Pre-Mutation Snapshot (`403`/`409` on mismatch/replay). |
| `GET /api/preferences/theme` | Bearer | Returns active theme (`"light"` \| `"dark"`). |
| `POST /api/preferences/theme` | Bearer + CSRF | Persists Light/Dark theme preference cleanly. |

---

## 4. Unified `Groxy AI` Default Agent & Automatic Adaptive Reasoning Router

### 4.1 Zero Model-Switching Dropdowns
- **Single Permanent Identity:** The IDE sidebar and command bar present **Groxy AI** as the sole, unified assistant (`"Ask Groxy AI..."`). No model-picker `<select>` or provider dropdown is ever rendered in the workbench UI.
- **Local-First Execution:** By default, **Groxy AI** routes to local **Ollama** / **OpenCode** endpoints on `127.0.0.1` (zero mandatory cloud API fees). Optional user-supplied API keys live strictly in host environment variables or AES-256-GCM encrypted local config (`DESK-E503` on provider timeout/unavailability).

### 4.2 Deterministic Adaptive Reasoning Router (`internal/groxy/router.go`)
Every request to `POST /api/groxy/ask` is classified deterministically into one of two internal execution tiers without user intervention:

| Internal Tier | Triggering Task Classes | Execution Behavior |
|---|---|---|
| **Instant / Fast Tier** | `grab check`, `grab format`, route/schema lookups, `grabman doctor` summaries, `Grisp` start/stop/reload commands, single-field copy/alt-text drafts | Low-latency single-pass execution with direct tool output and plain-English summary. |
| **Deep / High-Reasoning Tier** | Multi-file `.grab` / `.grab.html` code generation or refactoring, `GRAB-DESIGN-SYSTEM.md` layout synthesis, 22-rule SEO/WCAG/DOM remediation plans, cross-file bug diagnosis | Multi-step planning + structured diff generation + `GROXY-SAFE-001` approval preview. |

### 4.3 Human-in-the-Loop Safety (`GROXY-SAFE-001`)
- Read-only diagnostics (`grab check`, `grabman doctor`, route inspection) execute immediately.
- Any disk-mutating proposal returns a structured `approval_plan` with:
  - `plan_id`, `target_path` (confined via `GRAB-E401`), plain-English summary, unified diff preview,
  - Automatic SHA-256 **Pre-Mutation Snapshot** of existing target files,
  - A single-use, target-bound `approval_token` (`403 DESK-E403` on missing/target mismatch, `409 DESK-E409` on replay).

---

## 5. Local `Grisp` (`grab.exe`) Multi-Worker Supervisor

- **Port Pool (`127.0.0.1:8765–8772`):** Manages up to `8` concurrent local `Grisp` sites bound exclusively to `127.0.0.1`.
- **Health & Readiness Gating (`DESK-E502`):** Polls `GET /_health` and `GET /_ready` on managed workers; marks a worker `ready` only when both return `200 OK`.
- **Atomic Reload & Graceful Drain:** Supports zero-downtime `--reload` / `SIGHUP` (retaining last-known-good app on validation failure) and bounded `30s` shutdown draining.
- **Secret Redaction:** Filters `Authorization`, `Bearer`, `MASTER_KEY`, and `v1:...` AEAD strings from the bounded log ring buffer before streaming to the UI.

---

## 6. Pure `Grex JS v1.0.0` Zero-`<div>` Light/Dark IDE Workbench

- **Semantic 20-Element DOM (`GREX-SCHEMA-001`):** Built strictly with `<section>`, `<header>`, `<nav>`, `<main>`, `<aside>`, `<footer>`, `<article>`, `<form>`, `<table>`, `<dialog>`, `<details>` and Grex's 20 semantic elements (`grex.elements.json`). Enforces `divCount === 0`, `innerHTMLWriteCount === 0`, zero `eval()` / `new Function()`, and zero unsafe URLs.
- **Workbench Panes (`v0.1.0`):**
  1. **Top Header (`<header>`):** Active workspace badge, `Grisp` live status pill (`/_health` + `/_ready`), Light/Dark theme toggle (`#desk-theme-toggle`), and `grabman doctor` quick indicator.
  2. **Left Workspace & Supervisor Rail (`<aside>`):** Confined file explorer, `grabman` Scaffolder/Doctor controls, and Local `Grisp` Worker Manager (`Start` / `Reload` / `Stop` + port `127.0.0.1:8765–8772`).
  3. **Center Editor & Log Console (`<main>`):** Code/template viewer with `grab check` / `grab format` diagnostics and redacted `Grisp` log viewer.
  4. **Right Unified `Groxy AI` Sidebar (`<aside>`):** Single-identity `"Ask Groxy AI..."` prompt bar (zero model-switching dropdowns), adaptive tier indicator, plain-English responses, and diff approval cards.
- **Zero-FOUC Light/Dark WCAG AA Parity:** Both `[data-grex-theme="light"]` and `[data-grex-theme="dark"]` pass `computeContrastRatio() >= 4.5:1` (`>= 3.0:1` large text) and `100%` `auditAccessibility()` checks including visible `:focus-visible` rings.
- **Exact Numeric Fidelity (`ADR-025c`):** All worker ports, file sizes, line/column numbers, and JSON inspector values preserve exact base-10 wire strings without float coercion.

---

## 7. Error Code Registry (`v0.1.0`)

| Code | HTTP Status | Plain-English Meaning |
|---|---|---|
| `GRAB-E401` | `403` | Path confinement or sensitive-path (`.git`, `.env`, `.ssh`, `*.key`, `*.pem`, `*.p12`, `*.pfx`) rejection. |
| `DESK-E101` | `400` | Malformed request payload, unsupported profile, or invalid port/workspace parameter. |
| `DESK-E201` | `403` | Escaping symlink or `..` traversal blocked (`GRAB-E401` parity). |
| `DESK-E301` | `422` | `grab.exe` or `grabman.exe` subprocess execution or `grab check` validation failure. |
| `DESK-E403` | `403` | Missing/invalid bearer token, `X-CSRF-Token`, or target-bound `approval_token` mismatch. |
| `DESK-E409` | `409` | Replayed single-use `approval_token` or conflicting worker/port state. |
| `DESK-E501` | `500` | Non-loopback bind address rejected (`ADR-005`). |
| `DESK-E502` | `502` | Managed `Grisp` worker `/_health` or `/_ready` startup/reload failure. |
| `DESK-E503` | `503` | Unified `Groxy AI` local/upstream reasoning provider unavailable or timed out. |

---

## 8. Mandatory `v0.1.0` Verification Gates

Implementation of `v0.1.0` is NOT complete until all **four** verification gates pass at `100%`:

1. **Gate 1 — Go Host Daemon & Supervisor Suite:**
   - `go build ./...`, `go vet ./...`, `go test -count=1 ./...`, and `govulncheck ./...` (`0` errors, `0` vulnerabilities) covering `internal/host`, `internal/safepath`, `internal/supervisor`, `internal/workspace`, and `internal/groxy`.
2. **Gate 2 — Pure `Grex JS v1.0.0` Zero-`<div>` & WCAG AA DOM Suite:**
   - `node --test tests/*.test.js` verifying `divCount === 0`, `innerHTMLWriteCount === 0`, zero `eval()` / unsafe URLs, zero model-switching dropdowns in the Groxy AI sidebar, Light & Dark `computeContrastRatio() >= 4.5:1`, `100%` `auditAccessibility()`, and `ADR-025c` base-10 wire preservation.
3. **Gate 3 — Security, Confinement & Approval Gate Suite:**
   - Automated tests proving non-loopback bind rejection (`DESK-E501`), `GRAB-E401` path/symlink/sensitive-file blocking, `403 DESK-E403` on missing/mismatched CSRF or `approval_token`, `409 DESK-E409` on replay, and zero shell-string interpolation (`sh -c` / `cmd /c`).
4. **Gate 4 — Cross-Layer Integration Suite:**
   - End-to-end test launching `groxy-desk-host` on `127.0.0.1:9490`, scaffolding a workspace via `grabman v1.0.0` (`init` -> `doctor` -> `solve --verify`), starting a live `GRAB v1.1.0` `Grisp` worker (`/_health` + `/_ready === 200`), executing a Unified `Groxy AI` Adaptive Router request + `approval_token` write + Pre-Mutation Snapshot verification, and cleanly shutting down all workers.
