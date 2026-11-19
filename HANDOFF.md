# HANDOFF.md — Groxy Desk

## Current Status
- **Verified stable release:** `v1.0.2`, the Windows WebView2 launch-crash fix. Verified `stable/main` on 2026-11-19 (`3c36988`).
- **Current milestone:** `v1.1.0` Browser Co-Pilot, Bestseller Radar and Visual Database Workbench. `Approved; implementation in progress`. The owner approved [SPEC-110](specifications/GROXY-DESK-SPEC-110.md) as written on 2026-11-19.
- **Branch:** `origin/main` (`urbanspress/groxy-desk.progress`) for the documentation checkpoint.
- **Current Owner:** OpenAI Codex for Stage 3 implementation.

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

## Backwards-Compatible Minor Track (`v1.1.0`)

- **Browser Co-Pilot and Bestseller Radar:** Isolated WebView2 split pane, Groxy AI `v1.8.0` research and draft-only receipts, and approved GrabLab CMS `v1.4.0`/`v1.5.0` book draft handoff.
- **Visual Database Workbench:** Pinned SSH loopback connections, multi-tab SQL, exact-decimal inline edits, visual schema diagrams, import/export, and cross-environment schema comparison with separately approved writes and verified rollback.

## Stage 2 Pre-Implementation Cross-Layer Security & Contract Audit (PASS)

The frozen Radar and CMS contracts support a Desk-bound receipt followed by a separately approved book draft. WebView2 isolation, exact-origin checks, pinned SSH and exact decimal handling can all be enforced in Desk. No upstream code changes are required. [Binding decisions and evidence in SPEC-110 §10](specifications/GROXY-DESK-SPEC-110.md#10-stage-2-pre-implementation-cross-layer-security--contract-audit-pass)

## Pending next steps

Implement Stage 3 and execute all four §9 gates on Windows. Include independent browser profiles, receipt replay and duplicate draft checks, SSH pin rejection, exact numeric round trips, and recovery of real rows and schema on each engine. The current network database snapshot contains only operation metadata. Keep writes gated until verified backups and separately approved restores work.

## Recommended Next Model & Effort Tier

**GPT-6.2 Sol (High)** is cleared for Stage 3 multi-file implementation and integration tests. Use **GPT-6.1 Terra (High)** as the alternate.
