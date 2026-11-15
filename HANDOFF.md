# HANDOFF.md — Groxy Desk

## Current Status
- **Current Milestone:** `v0.3.0` — **Complete & Verified Across Gates 1–4** ([`specifications/GROXY-DESK-SPEC-003.md`](specifications/GROXY-DESK-SPEC-003.md) | [Walkthrough PDF](docs/releases/pdf/Groxy-Desk-v0.3.0-Guide.pdf))
- **Next Milestone Target:** **Pre-`v1.0.0` Cross-Layer Audit -> `v1.0.0` GA & Branded Inno Setup Installer (`Groxy-Desk-Setup-1.0.0.exe`)**
- **Branch / Repositories:** `main` (`origin` `https://github.com/urbanspress/groxy-desk.progress.git` & `stable` `https://github.com/urbanspress/groxy-desk.stable.git`, synchronized on tag `v0.3.0`)
- **Current Owner:** Project Owner (for Model Switch) -> **`Orchestrator (Argon)`**

## Completed in `v0.3.0`
- Implemented Step 5 Fleet & `Grabtaculous` Bridge (`internal/fleet/`, `public/desk/fleet-panel.js`), `known_hosts` pinned `SHA256:...` SSH (`403 DESK-E405`), encrypted node registry (`v1:...`), `grabman doctor` / `solve --verify` / `Site DNA` (`422 DESK-E302`), `--read-only` non-root `@sha256` deployments, single-use `approval_token`, and SHA-256 Pre-Deploy Snapshot (`snap_deploy_*`) rollback.
- Implemented Encrypted `127.0.0.1:15432–15439` SSH DB Tunnels (`409 DESK-E409` max 8, clean `Shutdown()` teardown) & Built-In Visual DB Explorer (`internal/dbstudio/`, `public/desk/db-studio-panel.js`) supporting workspace-confined SQLite (`GRAB-E401`) and tunneled PostgreSQL/MariaDB with schema inspection, bounded `<= 100` row pagination preserving `ADR-025c` base-10 wire strings, strict read-only SQL default (`403 DESK-E406`), and Pre-Mutation Snapshot (`snap_db_*`) gated writes/rollbacks.
- Implemented Encrypted Namecheap DNS Vault (`use crypto` `AES-256-GCM` `v1:...`), email-preserving DNS diff (`422 DESK-E303`), DNS propagation preflight (`422 DESK-E304`), and Caddy / Nginx + Certbot / Cloudflare Tunnel Auto-HTTPS configuration (`127.0.0.1:8765–8772`) with 1-click snapshot rollback (`internal/dns/`, `public/desk/dns-panel.js`).
- Implemented 1-Click Local "Author Mode" (`Book`, `Chapter`, `CharacterBible`, `WorldLore`, `status_enforced: "draft"`, `SITE-CONTEXT.md`, XSS-safe `schema.org/Book` JSON-LD with exact `ADR-025c` strings) & Loopback Ollama (`127.0.0.1:11434`) status/launcher (`internal/author/`, `public/desk/author-panel.js`) with zero cloud fee requirements and zero model-switching dropdowns.
- Verified Gates 1–4 at `100%` pass (`go build ./...`, `go vet ./...`, `go test -count=1 ./...` across all 11 test packages, `govulncheck ./...` `0` vulnerabilities, `grab check` + canonical `grab format` byte parity, `node --test tests/*.test.js` `10/10` pass, `groxy-desk-windows-amd64.exe --smoke-test`, and `python tests/integration_e2e.py` live E2E) while keeping all 8 peer `v1.x` repositories strictly untouched.
- Generated `docs/releases/pdf/Groxy-Desk-v0.3.0-Guide.pdf`, `dist/` static `CGO_ENABLED=0` binaries + `groxy-desk-v0.3.0-bundle.zip` + `SHA256SUMS.txt`, and published synchronized `v0.3.0` releases on `origin` and `stable`.

## Pending Next Steps
- **Pre-`v1.0.0` Cross-Layer Audit & `v1.0.0` GA Packaging:** Perform final cross-layer security, SSH pinning, SQLite/tunnel isolation, Pure `Grex JS v1.0.0` DOM, `ADR-025c`, and `GROXY-SAFE-001` audit prior to drafting `specifications/GROXY-DESK-SPEC-004.md` (if needed for GA installer) and tagging `Groxy Desk v1.0.0` + building `Groxy-Desk-Setup-1.0.0.exe`.

## Recommended Next Model (`AGENTS.md` 4-Tier Routing)
- **Switch to `Orchestrator (Argon)`:** Recommended next to run our final independent **Pre-`v1.0.0` Cross-Layer Audit** across all `Groxy Desk v0.3.0` security, SSH pinning, DB isolation, Pure `Grex JS v1.0.0` DOM, and upstream `v1.x` contracts before tagging **`Groxy Desk v1.0.0`**.
