# Groxy Desk

![Built with GRAB v1.1.0](docs/badges/built-with-grab.svg)
![Powered by Grex JS v1.0.0](docs/badges/powered-by-grex-js.svg)
![GRAB Ecosystem Step 6 — Groxy Desk](docs/badges/ecosystem-step-6.svg)
![Release v0.1.0](docs/badges/release-v0.1.0.svg)

**Groxy Desk** is the native desktop vibe-coding IDE, local `Grisp` (`grab.exe`) multi-worker supervisor, `grabman v1.0.0` workspace manager, and Unified **`Groxy AI`** Adaptive Reasoning studio for **Step 6 of the Post-1.0 GRAB Ecosystem**.

- **Current Status:** `v0.1.0` — **Complete** (See [`CHANGELOG.md`](CHANGELOG.md), [`docs/ROADMAP.md`](docs/ROADMAP.md), and the [Walkthrough PDF](docs/releases/pdf/Groxy-Desk-v0.1.0-Guide.pdf))
- **Built Strictly Upon Frozen Baselines:** `GRAB v1.1.0`, `Grex JS v1.0.0`, `GrabLab CMS v1.0.1`, `Grex Theme Builder v1.0.0`, `Groxy AI v1.0.0`, `grabman v1.0.0`, `Grabtaculous v1.0.0`, and `Gashboard v1.0.0` (all 8 peer `v1.x` repositories remain untouched).

---

## Key Features (`v0.1.0`)

1. **Native Windows WebView2 Desktop App + Loopback Host Daemon (`127.0.0.1:9490`):**
   - Zero-CGo (`CGO_ENABLED=0`) native desktop shell embedding Microsoft Edge WebView2 (`go-webview2`) paired with an authenticated `127.0.0.1`-only daemon (`DESK-E501` fail-closed on `0.0.0.0`, `::`, or LAN binds).
   - Constant-time `Authorization: Bearer` + `X-CSRF-Token` validation (`403 DESK-E403`), memory-only session delivery over `window.chrome.webview`, strict CSP (`script-src 'self'`), and `GRAB-E401` path/symlink/junction/sensitive-file confinement.
2. **Local `Grisp` (`grab.exe`) Multi-Worker Supervisor (`127.0.0.1:8765–8772`):**
   - Run, monitor (`/_health` & `/_ready`), `grab check`-preflight atomic-reload, inspect secret-redacted logs for, and cleanly shut down up to 8 concurrent GRAB projects from one screen.
3. **Built-In `grabman v1.0.0` Workspace Manager:**
   - Scaffold apps (`app`, `cms-stack`, `ai-stack`) behind single-use approval tokens, run `grabman doctor`, verify lockfiles (`grabman solve --verify`), and check canonical `.grab` + `.grab.html` parity in one click.
4. **Unified `Groxy AI` + Adaptive Reasoning Router (`GROXY-SAFE-001`):**
   - Single permanent assistant identity (`Groxy AI`) with **zero model-switching dropdowns**.
   - Routes quick inspections and 22-rule WCAG/SEO/DOM/Link scorecard audits to the `fast` instant path, and gates code/template mutations on the `deep` path behind Plan -> Pre-Mutation SHA-256 Snapshot -> single-use target-bound `approval_token` (`403`/`409`) -> 1-click Snapshot Rollback.
5. **Pure `Grex JS v1.0.0` Zero-`<div>` Light & Dark UI:**
   - Vendored byte-identical `grex.js` + `grex.elements.json` (`divCount === 0`, `innerHTMLWriteCount === 0`, zero `eval()`/`new Function()`, 100% WCAG AA Light/Dark `>= 4.5:1` contrast, exact `ADR-025c` base-10 wire strings).

---

## Quickstart

```powershell
# Launch headless smoke verification
go run ./cmd/groxy-desk --smoke-test --addr 127.0.0.1:0

# Launch Native Windows Desktop IDE against any GRAB project directory
go run ./cmd/groxy-desk --workspace C:\path\to\project

# Or launch pre-built static release binaries (dist/)
.\dist\groxy-desk-windows-amd64.exe --workspace C:\path\to\project
.\dist\groxy-deskd-windows-amd64.exe --addr 127.0.0.1:9490 --workspace C:\path\to\project
```

---

## Verification & Packaging Commands

```powershell
# Gate 1: Go host, supervisor, confinement & vulnerability audit + GRAB check/format
go build ./...
go vet ./...
go test -count=1 ./...
govulncheck ./...
C:\Development\GRAB\dist\grab-windows-amd64.exe check src/desk.grab views/workbench.grab.html
C:\Development\GRAB\dist\grab-windows-amd64.exe format src/desk.grab

# Gate 2: Pure Grex JS v1.0.0 DOM, WCAG AA, ADR-025c & Unified Groxy AI checks
node --test tests/*.test.js

# Gate 4 (and Gate 3 live probes): End-to-end multi-layer integration suite
python tests/integration_e2e.py

# Build dist binaries, PDF walkthrough guide, bundle zip, and SHA256SUMS.txt
python scripts/package_release.py --version 0.1.0
```
