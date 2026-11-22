# Groxy Desk v1.3.1

### Release Type & Affected Layers

- **Release Version & SemVer Tier:** `v1.3.1` (`PATCH 1.x.y` — Backwards-Compatible Bug Fix)
- **Affected Layer(s):** `Groxy Desk` (`urbanspress/groxy-desk`)
- **Neighboring Layer Upgrade Required?** `No — 100% drop-in compatible` (existing workflows use frozen `v1.x` baselines; optional Bestseller Radar and GrabLab CMS book draft handoffs use `Groxy AI v1.8.0` and `GrabLab CMS v1.4.0`/`v1.5.0`)

| Ecosystem Layer | Repository | Required Version Range | Touched in This Release? | Neighbor Upgrade Required? |
|---|---|---|---|---|
| **Core Language, Bytecode VM & Grisp Server** | `urbanspress/grab` | `>= v1.1.0 (< v2.0.0)` | `No` | `No` |
| **Reactive Frontend Runtime & 20-Element Library** | `urbanspress/grex-js` | `>= v1.0.0 (< v2.0.0)` | `No` | `No` |
| **Flagship Zero-Plugin CMS** | `urbanspress/grablab-cms` | `>= v1.0.1 (< v2.0.0)` (`v1.4.0`/`v1.5.0` for optional book drafts) | `No` | `No` |
| **Exclusive `mode: "visual"` Theme & Studio** | `urbanspress/grex-builder` | `>= v1.0.0 (< v2.0.0)` | `No` | `No` |
| **Autonomous Design, Content & SEO Agent** | `urbanspress/groxy-ai` | `>= v1.0.0 (< v2.0.0)` (`v1.8.0` for optional Bestseller Radar) | `No` | `No` |
| **Scaffolder, Orchestrator & Discreet VPS Panel** | `urbanspress/grabman`, `urbanspress/grabtaculous`, `urbanspress/gashboard` | `>= v1.0.0 (< v2.0.0)` (bundle pins `grabman v1.0.0`) | `No` | `No` |
| **Native Desktop Vibe-Coding IDE & Studio** | `urbanspress/groxy-desk` | `>= v1.3.1 (< v2.0.0)` | `Yes` | `No` |

### Remote SSH schema inspection

- Database Studio reads PostgreSQL and MariaDB/MySQL metadata in one query and fetches exact row counts together in a second query. The number of queries stays the same as the schema grows.
- The pinned SSH inspection timeout is 20 seconds. Views and triggers appear in the existing schema tree when visible to the connecting account.
- Existing table response fields, `ADR-025c` numeric strings, and `GROXY-SAFE-001` write approval and recovery rules are preserved.

### Verification

- The delayed pinned-SSH regression passed on PostgreSQL 18.6, MariaDB 11.8.9, and MySQL 8.4.13 with 40 added tables.
- Windows and Linux amd64 build and vet, Windows Go tests, vulnerability scanning, GRAB check and formatting, all 14 Node tests, cross-layer integration, native WebView2 isolation, live Radar/CMS, and database dump/recovery passed.
- Includes the Windows installer, binaries, walkthrough, and ZIP bundle. Compare each download against `SHA256SUMS.txt`.
