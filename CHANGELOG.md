# Changelog — Groxy Desk

All notable releases of **Groxy Desk** (Step 6 of the GRAB Post-1.0 Ecosystem) are documented in this file.

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
