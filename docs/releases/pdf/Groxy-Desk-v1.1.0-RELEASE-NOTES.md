# Groxy Desk v1.1.0

### Release Type & Affected Layers

- **Release Version & SemVer Tier:** `v1.1.0` (`MINOR 1.x.0` — Backwards-Compatible Feature Release)
- **Affected Layer(s):** Groxy Desk (`urbanspress/groxy-desk`)
- **Neighboring Layer Upgrade Required?** Existing workflows use the frozen baselines. The optional Bestseller Radar and book draft integrations require the service versions listed below.

| Ecosystem Layer | Repository | Required Version Range | Touched in This Release? | Neighbor Upgrade Required? |
|---|---|---|---|---|
| **Core Language, Bytecode VM & Grisp Server** | `urbanspress/grab` | `>= v1.1.0 (< v2.0.0)` | `No` | `No` |
| **Reactive Frontend Runtime & 20-Element Library** | `urbanspress/grex-js` | `>= v1.0.0 (< v2.0.0)` | `No` | `No` |
| **Flagship Zero-Plugin CMS** | `urbanspress/grablab-cms` | `>= v1.0.1 (< v2.0.0)` for existing workflows; `v1.4.0` or `v1.5.0` with Author Mode capabilities for book drafts | `No` | `For book drafts only, if needed` |
| **Exclusive `mode: "visual"` Theme & Studio** | `urbanspress/grex-builder` | `>= v1.0.0 (< v2.0.0)` | `No` | `No` |
| **Autonomous Design, Content & SEO Agent** | `urbanspress/groxy-ai` | `>= v1.0.0 (< v2.0.0)` for existing workflows; `v1.8.0` for Bestseller Radar | `No` | `For Bestseller Radar only, if needed` |
| **Scaffolder, Orchestrator & Discreet VPS Panel** | `urbanspress/grabman`, `urbanspress/grabtaculous`, `urbanspress/gashboard` | `>= v1.0.0 (< v2.0.0)` (bundle pins `grabman v1.0.0`) | `No` | `No` |
| **Native Desktop Vibe-Coding IDE & Studio** | `urbanspress/groxy-desk` | `>= v1.1.0 (< v2.0.0)` | `Yes` | `No` |

### Verified `v1.1.0` Capabilities (SPEC-110 Section 9 Complete; Stage 4 Sign-Off Pending)

- **Split-Pane WebView2 Browser Co-Pilot & Bestseller Radar:** Isolated WebView2 browser profile with exact HTTPS origin enforcement (`GRAB-E402`), blocked external frames/popups/images/redirects, workbench-only session tokens, explicit selection capture, approved `BRAND-DNA.md` organization prefill, `Groxy AI v1.8.0` Desk-bound draft receipts, and separately approved unpublished `GrabLab CMS v1.4.0`/`v1.5.0` book drafts.
- **Visual Database Workbench & Exact Numeric Fidelity:** Multi-tab SQL editor with schema-aware autocomplete, inline spreadsheet cell editing preserving `ADR-025c` base-10 wire strings (`"0.00"`, `9007199254740993`), Pure `Grex JS v1.0.0` interactive ER diagrams (`divCount === 0`), HTML ER export, CSV/XLSX/JSON/SQL export and import, cross-dialect schema diffing, local WAL SQLite recovery, and remote SQLite inspection over pinned SSH (`Gashboard v1.0.0`).
- **Full PostgreSQL 18 & Least-Privilege MariaDB 11.8 Recovery:**
  - Acquires an exclusive per-database lock over pinned SSH SFTP before applying network recovery.
  - **PostgreSQL 18:** Uses a separate maintenance login to close target admission, drain backends, verify logical replay on a reserved connection, and reopen the database only after SHA-256 verification.
  - **MariaDB 11.8:** Audits every account via `mysql.global_priv` and `information_schema` and requires target-only isolation (`admission_admin` with `ALL PRIVILEGES ON <target_db>.* WITH GRANT OPTION`, global `CONNECTION ADMIN` + `PROCESS`, `SELECT ON mysql.*`, and zero global `*.*` write privileges). Any server with a global-write account fails closed with HTTP `409 write_fence_unavailable` before approval. During recovery, Desk revokes target writer grants, terminates target writer sessions while leaving unrelated databases connected and undisturbed, verifies byte-for-byte replay of tables, views, and triggers, and restores exact grants afterward. Partial MariaDB DDL failures return HTTP `422 DESK-E406` with the retained snapshot so one-click rollback restores exact pre-mutation SHA-256 state.

### Section 9 Acceptance Evidence

- `go build ./...` and `go vet ./...` passed for both Windows amd64 and Linux amd64 (`CGO_ENABLED=0`), along with `go test -count=1 ./...`, `go mod verify`, `govulncheck ./...` (`0` vulnerabilities), `grab check`, canonical `grab format` parity, and `10/10` Node DOM/WCAG AA tests (`divCount === 0`, `innerHTMLWriteCount === 0`).
- Native Windows `TestNativeWebView2Isolation`, `tests/integration_e2e.py`, `tests/radar_live_e2e.py`, `tests/network_dump_recovery.py`, containerized `TestPinnedDatabaseCatalogs` (`postgres:18`, `mariadb:11.8`, `mysql:8.4`), `TestPinnedNetworkRecoveryAPI/mariadb_least_privilege`, and native Ubuntu `tests/native_ubuntu_db_e2e.sh` (PostgreSQL `18.6` and MariaDB `11.8.6`) passed `100%`, with all 10 frozen sibling `v1.x` repositories verified clean. [Acceptance gates](../../../specifications/GROXY-DESK-SPEC-110.md#9-acceptance-gates)
