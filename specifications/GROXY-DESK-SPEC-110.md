# GROXY-DESK-SPEC-110 — Groxy Desk v1.1.0

**Browser Co-Pilot, Bestseller Radar & DbGate-Parity Visual Database Workbench**

- **Status:** Approved target; specification in review
- **Release tier:** Backwards-compatible minor feature (`ADR-027c`)

## 1. Stage 1 approval and compatibility

The project owner has approved the milestone target. Stage 1 requires separate, explicit approval of this specification before any Go, GRAB, C/Win32, WebView2, or JavaScript implementation begins. This draft defines additive desktop capabilities. The frozen Groxy Desk `v1.0.0` release, subsequent `1.0.x` compatibility, and frozen peer contracts remain protected. Existing workspaces, routes, MCP tools, and read-only DB workflows retain their behavior.

Continue using GRAB `v1.1.0`, Grex JS `v1.0.0`, and the existing `grabman v1.0.0` baseline. Radar requires Groxy AI `v1.8.0`. CMS book-draft handoff requires Author Mode and the relevant capabilities of GrabLab CMS `v1.4.0` or `v1.5.0`. Detect availability per configured service and show a setup message when a capability is unavailable. Core IDE use remains local-first, with Groxy AI as the single built-in agent and automatic reasoning selection.

## 2. Split-pane WebView2 Browser Co-Pilot

Open an independently navigable WebView2 browser beside the workbench. Keep the active origin visible and let users resize or close the split without losing workbench state. Users can select authorized research material and explicitly choose what to share with Groxy AI. Account-registration assistance may preview an allowlisted set of `BRAND-DNA.md` fields for approved, one-click prefill. Users enter passwords, solve CAPTCHAs, and submit forms themselves.

Isolate browsing content from the trusted workbench profile and host bridge. Keep bridge credentials, CMS credentials, database secrets, and privileged WebView2 objects out of page reach. Disable privileged web messaging in the browser pane. If the trusted workbench uses native messaging, validate the exact sender origin, frame, and message schema for every call. Treat page text as untrusted data and require explicit user action for capture, prefill, or handoff.

## 3. Bestseller Radar side panel and CMS handoff

1. The user chooses a genre and optional price tier and live-feed settings. Through the trusted host, Desk calls Groxy AI `v1.8.0` directly at `POST /api/groxy/research/bestseller-radar` with `target_consumer: "groxy_desk"` and the upstream target slot. Render ranked research, source links, source freshness, and exact decimal unit economics. Distinguish live results from offline fallback and show partial or unavailable sources.
2. Stage the returned `radar_plan_id`, `payload_sha256`, and single-use `rtok_res_<hex>` in the trusted host. Show a brief preview. On explicit confirmation, use the existing `POST /api/groxy/research/bestseller-radar/apply` contract with its CSRF protection and the original bindings. Validate the returned receipt against the staged plan, slot, consumer, and payload hash. Require `status_enforced: "draft"` and `human_confirmation_required: true`. Accept receipts from the authenticated, allowlisted Groxy AI response. A receipt hash alone does not authenticate a receipt supplied by browser content.
3. Let the user review the proposed book fields and selected GrabLab workspace before a separate, single-use Desk approval. Invoke the existing `create_book_draft` MCP tool for that workspace and verify the returned draft status. Filter fields against the live CMS capabilities. Link to the allowlisted CMS `/admin/author/radar` for users who prefer its existing query and import workflow. That workflow uses its own CMS session, CSRF protection, consumer binding, and import ledger. A Desk receipt stays bound to Desk. Prevent duplicate submissions per plan and CMS target, and reconcile an ambiguous response before offering any retry.

The workbench never publishes an entry, writes product pricing or stock, or sends service credentials into browser content. Research staging and receipt creation leave CMS data untouched until draft creation is separately confirmed.

## 4. Origin, host and workspace boundaries

- Apply `GRAB-E402` to every Groxy AI and GrabLab integration origin and redirect. Match the configured scheme, host, and port exactly. Require HTTPS for remote integrations. Allow HTTP only for explicitly configured local workers on `127.0.0.1`. Validate destinations at save and request time, reject private, link-local, and metadata destinations outside that exception, and check DNS results at connection time. Reject unsafe schemes and embedded URL credentials. Keep third-party navigation separate from service discovery and privileged requests.
- Bind `groxy-deskd` to `127.0.0.1:9490` under `ADR-005`. Check the session bearer in constant time and require `X-CSRF-Token` on state-changing bridge requests. Keep service sessions and tokens scoped to their intended destinations. Omit credentials and raw tokens from URLs, page content, and logs.
- Enforce `GRAB-E401` for workspace paths and file actions, including database selection, imports, and exports. Use structured subprocess arguments under `ADR-017`.

## 5. Database connections and deployment coverage

- Open workspace-confined SQLite files locally. For remote SQLite, use a capability-checked remote database bridge over pinned SSH with remote workspace confinement. SQLite has no native TCP listener. Keep the remote option disabled when the bridge is unavailable.
- Connect to PostgreSQL and MariaDB/MySQL through the existing loopback SSH tunnel model. Bind each local forward to `127.0.0.1` and verify the configured `known_hosts` entry and `SHA256:...` host-key fingerprint before forwarding. Reject missing or changed keys and require explicit re-verification for rotation. Scope each forward to its approved target and close it when the last consumer disconnects or the host shuts down.
- Support native Ubuntu `systemd` PostgreSQL and MariaDB services on private loopback endpoints, and containerized PostgreSQL and MariaDB/MySQL exposed to the SSH host through explicitly configured private loopback endpoints. Neither requires a public DB port. Native connections use the database's normal protocol and do not depend on Grabtaculous `v1.2.0` shipping.
- Use read-only connections and least-privilege credentials by default. Keep connection identities explicit in every tab, preview, export, and confirmation.

## 6. Query editor, row editing, and schema diagrams

- Provide a multi-tab SQL editor with dialect-aware, schema-aware autocomplete for SQLite, PostgreSQL, and MariaDB/MySQL. Keep result sets and schema metadata tied to the tab's connection. Use bounded, paginated results and support cancellation. Read-only execution must enforce read-only permissions at the DB layer where supported.
- Provide spreadsheet-style inline editing with keyboard navigation, typed validation, and a before/after preview. Use stable row identity and a freshness check before applying a parameterized update. Tables without a reliable row identity stay read-only for inline editing. Preserve null and empty values distinctly.
- Carry exact `GrabDecimal` and `GrabInteger` values as base-10 wire strings through editing, previews, approvals, and readback under `ADR-025c`. Preserve scale where the field supports it, including `"0.00"`, and round-trip integers above JavaScript's safe range. Surface engine precision or scale limits before mutation.
- Render interactive entity-relationship diagrams for tables, columns, primary keys, and foreign keys, with a keyboard-accessible relationship view that exposes equivalent information.

## 7. Import, export, and cross-environment schema diff

- Offer one-click entry points for CSV, Excel (`.xlsx`), JSON, and engine-aware SQL dump import and export. Show the selected scope and target. Imports preview mappings, types, and expected effects before approval. Validate format, size, and dialect. Treat spreadsheet formulas as data and protect spreadsheet exports against formula interpretation. Encode exact numerics as text in Excel and strings in JSON where needed to preserve fidelity. Preserve numeric lexemes in CSV and use an engine-compatible exact numeric representation in SQL dumps.
- Compare local, staging, and production schemas side by side across supported engines. Show additions, removals, type changes, keys, and index changes with a proposed target-dialect plan. Flag unsupported or lossy translations for review. Keep comparison read-only until the user approves a specific plan against a named target.

## 8. Mutation approval, snapshots, and rollback

The host must gate inline edits, write SQL, DDL, imports, schema sync, and snapshot restore with `GROXY-SAFE-001`. Show the exact target and effect. Require the session bearer, `X-CSRF-Token`, and a short-lived, single-use `approval_token` bound to the exact operation or plan digest, target, and approved baseline. Atomically claim the token and reject replay. Reject target drift or approval mismatch and require fresh review.

Before writing, take and verify a restorable SHA-256 Pre-Mutation Snapshot of the affected scope while maintaining a consistent target baseline. Refuse the change if a recoverable snapshot cannot be taken. Use engine transactions where supported. For nontransactional DDL, validate the restore path before execution and retain the snapshot and operation status on partial failure. Record the target, plan, outcome, and snapshot reference in the audit trail without exposing secrets.

Provide one-click rollback from the operation record to a target-bound restore confirmation. Verify the snapshot hash and check for newer changes that restoration would overwrite before restoring. Use fresh approval for restore and verify schema or row state afterward. Surface partial or ambiguous outcomes without silently reapplying mutations.

## 9. Acceptance gates

All of the following gates are required for a future implementation milestone. This Stage 1 draft records the checks for review.

| Gate | Required evidence |
|---|---|
| **1. Go host and launcher** | On Windows, run `go build ./...`, `go vet ./...`, `go test -count=1 ./...`, and `govulncheck ./...` with zero failures and zero vulnerabilities. Verify dependency integrity with `go mod verify` and run the Windows WebView2 launcher smoke test. Assert loopback binding, bearer and CSRF enforcement, origin rejection, authenticated pinning, approval binding, and single-use behavior. |
| **2. GRAB and Grex conformance** | Run `grab check` and `grab format` parity for all relevant `.grab` and `.grab.html` files. Run `node --test tests/*.test.js`. Every owned workbench view and dynamic state uses pure Grex JS `v1.0.0` and semantic elements: `divCount === 0`, `innerHTMLWriteCount === 0`, zero `eval()` or `new Function()`, zero unsafe URLs, and 100% `auditAccessibility()` pass. Verify Light and Dark normal-text contrast `>= 4.5:1`, large-text WCAG AA contrast `>= 3.0:1`, visible focus, keyboard-only operation, and startup without a theme flash. External page DOM stays isolated from the owned workbench. |
| **3. Data fidelity and recovery** | Verify exact `"0.00"` and large-integer round trips across UI, bridge, imports, exports, and supported database types. Cover nulls, stale rows, all four formats, cross-dialect compare, failed snapshots, token races, failed DB operations, and verified rollback on each supported engine. Prove read-only sessions and schema comparison never mutate an environment. |
| **4. Cross-layer Windows E2E** | Pass the full suite, including `python tests/integration_e2e.py` and milestone scenarios, with Groxy Desk, GRAB `v1.1.0`, Grex JS `v1.0.0`, `grabman v1.0.0`, Groxy AI `v1.8.0`, and both GrabLab CMS target versions. Exercise WebView2 origin isolation, allowlisted Radar research and receipt apply, CMS MCP draft creation and CMS Radar navigation, duplicate and mismatch rejection, and disabled or unreachable services. Exercise local SQLite, remote SQLite over the approved pinned bridge, and pinned SSH connections on native Ubuntu `systemd` and containerized PostgreSQL/MariaDB/MySQL targets. Verify existing workbench and Step 5 workflows against the frozen baseline. Require 100% passing results. |

Before implementation approval, confirm the supported remote SQLite bridge and test fixture, Groxy AI and CMS authentication details against the packaged versions, and the supported snapshot and restore scope for each engine. Record the resulting compatibility decisions in this specification.
