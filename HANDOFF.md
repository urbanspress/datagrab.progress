# HANDOFF.md — Groxy Desk

## Current Status
- **Current Milestone:** **`v1.0.1` GA (`PATCH 1.0.x`) — Approved & Frozen — Complete** ([`specifications/GROXY-DESK-SPEC-101.md`](specifications/GROXY-DESK-SPEC-101.md) | [Walkthrough PDF](docs/releases/pdf/Groxy-Desk-v1.0.1-Guide.pdf))
- **Next Milestone Target:** **`v1.1.0 — Interactive WebView2 Research & Account-Registration Co-Pilot`** (`Status: Planned — Backwards-Compatible Minor Feature` alongside ecosystem Post-1.0 Feature Unlocks across `Groxy AI v1.1.0–v1.8.0`, `GrabLab CMS v1.2.0–v1.4.0`, `Grex Theme Builder v1.1.0–v1.2.0`, and `Grabtaculous v1.1.0`)
- **Branch / Repositories:** `main` (`origin` `https://github.com/urbanspress/groxy-desk.progress.git` and `stable` `https://github.com/urbanspress/groxy-desk.stable.git` synchronized at `v1.0.1`)
- **Current Owner:** `Google Antigravity` (`v1.0.1` Patch Complete — Zero Pending Tasks)

## Completed in `v1.0.1` (`PATCH 1.0.x` — NGINX `1.30.6` Edge Generator Alignment)
- Updated `renderEdgeHTTPSConfig` in `internal/dns/dns.go` (`edgeProxy == "nginx"`) to emit NGINX `1.30.6` (`listen 443 ssl;`, `listen [::]:443 ssl;`, `http2 on;`) instead of deprecated `listen 443 ssl http2;`, plus the full `Host`, `X-Real-IP`, `X-Forwarded-For`, `X-Forwarded-Proto $scheme;`, and `X-Forwarded-Host $host;` header tuple required by `GRAB v1.1.0` `--trusted-proxy`.
- Added regression checks in `internal/dns/dns_test.go`, bumped runtime/UI/installer version constants to `1.0.1` (`GROXY-DESK-SPEC-101.md`), verified Gates 1–4 (`go build`, `go vet`, `go test -count=1 ./...`, `govulncheck ./...`, `go mod verify`, `grab check`, `grab format`, `node --test tests/*.test.js`, `--smoke-test`, and `python tests/integration_e2e.py`) at `100%` pass, packaged `dist/Groxy-Desk-Setup-1.0.1.exe` + `docs/releases/pdf/Groxy-Desk-v1.0.1-Guide.pdf` + `dist/groxy-desk-v1.0.1-bundle.zip` + `dist/SHA256SUMS.txt`, and synchronized `v1.0.1` across `origin` and `stable`.

## Completed in `v1.0.0` (Production Freeze & Branded Inno Setup Installer)
- Marked [`specifications/GROXY-DESK-SPEC-100.md`](specifications/GROXY-DESK-SPEC-100.md) and `v1.0.0` in [`docs/ROADMAP.md`](docs/ROADMAP.md) as `Approved & Frozen — Complete`.
- Bumped all runtime, MCP handshake, template, workbench, badge, test, and package versions to `1.0.0` (`internal/server/server.go`, `internal/mcpbridge/mcpbridge.go`, `cmd/groxy-desk/webview_windows.go`, `package.json`, `src/desk.grab`, `views/workbench.grab.html`, `public/desk/workbench.js`, `public/desk/api-client.js`, `public/desk/theme.js`, `docs/badges/release-v1.0.0.svg`, `README.md`, `CHANGELOG.md`, `internal/server/v020_server_test.go`, `internal/server/v030_server_test.go`, and `tests/integration_e2e.py`).
- Added side-by-side `{app}\bin\grab.exe` and `{app}\bin\grabman.exe` auto-discovery in `internal/workspace/workspace.go` (`bundledBinFallbacks`) and authored `installer/groxy-desk.iss` + `scripts/package_release.py` compiling **`dist/Groxy-Desk-Setup-1.0.0.exe`** via Inno Setup 6 (`ISCC.exe`, `.exe` only, never `.msi`), `docs/releases/pdf/Groxy-Desk-v1.0.0-Guide.pdf`, `dist/groxy-desk-v1.0.0-bundle.zip`, and `dist/SHA256SUMS.txt`.
- Verified Gates 1–4 at `100%` pass (`go build ./...`, `go vet ./...`, `go test -count=1 ./...` across all 11 tested packages, `govulncheck ./...` `0` vulnerabilities, `grab check` + canonical `grab format` byte parity, `node --test tests/*.test.js` `10/10` pass, `.\dist\groxy-desk-windows-amd64.exe --smoke-test`, and `python tests/integration_e2e.py`) while keeping all 8 frozen `v1.x` peer repositories untouched.
- Published synchronized `v1.0.0` GitHub Releases with the 7-Layer Ecosystem Compatibility Matrix and all release assets on both `urbanspress/groxy-desk.progress` (`origin`) and `urbanspress/groxy-desk.stable` (`stable`).

## Completed in Pre-`v1.0.0` Audit & `v0.1.0`–`v0.3.0`
- Completed Pre-`v1.0.0` 7-layer security/IPC/SSH/DB/DNS/Author/DOM audit, wiring `(*author.Manager).Shutdown()` into `(*server.Server).Shutdown()` and enforcing pre-token `DESK-E303` DNS email re-verification + explicit `SPF`/`DKIM`/`DMARC` protection in `ApplyDNS`.
- Delivered `v0.1.0` (WebView2 shell, `127.0.0.1:9490` daemon, `Grisp` multi-worker supervisor, `grabman` manager, Unified `Groxy AI`), `v0.2.0` (13-tool `grab mcp` studio, Split-Preview Canvas, 8x5 Style Packs, 22-Rule Scorecard & 🚀 Boost), and `v0.3.0` (Step 5 Fleet, `127.0.0.1:15432–15439` SSH DB Tunnels + Visual DB Explorer, Namecheap DNS + Auto-HTTPS, and 1-Click Local Author Mode + Ollama launcher).

## Planned Backwards-Compatible Minor Track (`v1.1.0`)
- **Milestone `v1.1.0` — Interactive WebView2 Research & Account-Registration Co-Pilot:** `Status: Planned — Backwards-Compatible Minor Feature` (zero `v1.0.0` implementation code modified):
  - Opens third-party research/registration pages (e.g., SEMrush, AnswerThePublic) inside **Groxy Desk**'s WebView2 pane, offers 1-click `BRAND-DNA.md` form prefill behind human confirmation (`GROXY-SAFE-001`), pauses for user CAPTCHA/password completion, and extracts authorized page insights cleanly.

## Pending Next Steps
- None in `groxy-desk v1.0.1` (`v1.0.1` Patch Complete, `0` pending tasks). Next ecosystem work begins on `Groxy Desk v1.1.0` per `docs/ROADMAP.md` and `C:\Development\CODEX-HANDOFF.md`.

## Recommended Next Model (`AGENTS.md` 4-Tier Routing)
- **Switch to `Google Hydrogen (Medium)` (or hand off to `OpenAI Codex` via `C:\Development\CODEX-HANDOFF.md`):** Recommended next for drafting pre-implementation specifications (`*-SPEC-*.md`) for the Post-1.0 Feature Unlocks now that all 6 core ecosystem layers are frozen at `v1.0.0+`.
