# GROXY-DESK-SPEC-101: Groxy Desk `v1.0.1` — NGINX `1.30.6` Edge Generator Alignment & Trusted-Proxy Header Completeness (`PATCH 1.0.x`)

- **Status:** Approved & Frozen — Complete (`v1.0.1` Non-Breaking Patch Release)
- **Target Release:** `v1.0.1` (`PATCH 1.0.x — Non-Breaking Bug/Security/Audit Patch`, `100%` drop-in compatible with `Groxy Desk v1.0.0`, `GRAB v1.1.0`, `Gashboard v1.0.0`, and `Grabtaculous v1.1.0`)
- **Foundational Baselines:**
  - **GRAB:** `v1.1.0` (`urbanspress/grab` — `Grisp`, `--trusted-proxy`, `metadata()`, `ADR-005`, `ADR-017`, `ADR-027c`)
  - **Grex JS:** `v1.0.0` (`urbanspress/grex-js` — Pure 20 semantic elements, `GREX-SCHEMA-001`)
  - **GrabLab CMS:** `v1.0.1`–`v1.4.0` (`urbanspress/grablab-cms`)
  - **Grex Theme Builder:** `v1.0.0` (`urbanspress/grex-builder`)
  - **Groxy AI:** `v1.0.0`–`v1.8.0` (`urbanspress/groxy-ai`)
  - **`grabman`, Grabtaculous & Gashboard:** `grabman v1.0.0`, `Grabtaculous v1.1.0`, `Gashboard v1.0.0`
- **Governance:** Read alongside [`GROXY-DESK-EXPERT.md`](../GROXY-DESK-EXPERT.md), [`AGENTS.md`](../AGENTS.md), [`docs/ROADMAP.md`](../docs/ROADMAP.md), and [`GROXY-DESK-SPEC-100.md`](GROXY-DESK-SPEC-100.md).

---

## 1. Executive Summary & Root Cause Fix (Plain English)

When deploying apps with **official `nginx.org` NGINX `1.30.6` (Stable)** (orchestrated by **Grabtaculous `v1.1.0`**), the older Ubuntu `1.24.x` `listen 443 ssl http2;` syntax is deprecated and emits parser warnings or directive inconsistencies, while omitting `X-Real-IP`, `X-Forwarded-For`, `X-Forwarded-Proto`, and `X-Forwarded-Host` prevents `GRAB v1.1.0` `--trusted-proxy` handlers from accurately identifying client IP addresses and HTTPS request origins.

**Groxy Desk `v1.0.1`** fixes `renderEdgeHTTPSConfig` in [`internal/dns/dns.go`](../internal/dns/dns.go) for `edgeProxy == "nginx"`:
1. Replaces deprecated `listen 443 ssl http2;` with `listen 443 ssl;`, `listen [::]:443 ssl;`, and `http2 on;` (matching official `nginx.org` NGINX `1.30.6` Stable).
2. Adds the complete `GRAB v1.1.0` `--trusted-proxy` header tuple inside `location /`:
   - `proxy_set_header Host $host;`
   - `proxy_set_header X-Real-IP $remote_addr;`
   - `proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;`
   - `proxy_set_header X-Forwarded-Proto $scheme;`
   - `proxy_set_header X-Forwarded-Host $host;`
3. Adds dedicated regression assertions in [`internal/dns/dns_test.go`](../internal/dns/dns_test.go) (`TestDNSPlanEmailPreservationPreflightAndHTTPSApply`) verifying `http2 on;`, absence of `listen 443 ssl http2;`, `X-Forwarded-Proto $scheme;`, and `X-Forwarded-Host $host;`.

---

## 2. Frozen Contract Preservation (`100%` Drop-In Compatible)

- **Zero Breaking API, MCP, or UI Changes:** All **55 HTTP paths (57 route handlers)** on `127.0.0.1:9490`, all **13 unified `grab mcp` tools**, all `DESK-*` / `GRAB-E401` security gates, all `GROXY-SAFE-001` approval-token + Pre-Mutation Snapshot contracts, and Pure `Grex JS v1.0.0` (`divCount === 0`, `innerHTMLWriteCount === 0`, Light & Dark WCAG AA `>= 4.5:1`, `ADR-025c`) remain **100% intact and backwards-compatible** with `v1.0.0`.
- **Packaging & Release Parity:** Rebuilds static `CGO_ENABLED=0` binaries (`groxy-desk-windows-amd64.exe`, `groxy-deskd-windows-amd64.exe`, `groxy-deskd-linux-amd64`), branded Inno Setup 6 installer (`dist/Groxy-Desk-Setup-1.0.1.exe` — `.exe` only), plain-English walkthrough guide (`docs/releases/pdf/Groxy-Desk-v1.0.1-Guide.pdf`), `groxy-desk-v1.0.1-bundle.zip`, and `SHA256SUMS.txt`.
