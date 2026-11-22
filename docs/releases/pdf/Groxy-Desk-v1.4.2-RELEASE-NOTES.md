# Groxy Desk v1.4.2

### Release Type & Affected Layers

- **Release Version & SemVer Tier:** `v1.4.2` (`PATCH 1.x.y` for Windows launch and UI fixes)
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
| **Native Desktop Vibe-Coding IDE & Studio** | `urbanspress/groxy-desk` | `>= v1.4.2 (< v2.0.0)` | `Yes` | `No` |

### Windows launch and workbench icons

- Open Groxy Desk from the Start menu or desktop shortcut directly in its native WebView2 window. The standalone daemon still runs as a console executable.
- The 32px menu bar now starts with an emerald Groxy Desk mark. Database, search, rollback, and settings actions use inline monochrome icons.
- In Database Studio, the circular plus opens the centered connection dialog. Use the magnifying glass in the sidebar to search tables and views, or the one on the activity rail to search commands.
- The compact grid, exact database values, and explicit approval and snapshot flow carry forward from v1.4.1.
