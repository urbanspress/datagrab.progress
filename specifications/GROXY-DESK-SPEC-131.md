# GROXY-DESK-SPEC-131 — Remote SSH schema inspection

**Release:** `v1.3.1` patch
**Scope:** Owner-approved implementation and release request dated 2026-11-21.

- Fetch PostgreSQL and MariaDB/MySQL schema metadata with one set-based catalog query. Fetch exact table counts with one additional statement within the same read-only transaction.
- Allow up to 20 seconds for tunneled schema inspection. Preserve existing table fields, exact base-10 row counts, SSH host pinning, and the `GROXY-SAFE-001` write controls.
- Return view and trigger names to the existing Database Studio schema tree.
- Prove the patch with a 40-table, delayed pinned-SSH fixture, the standard release gates, and checksummed artifacts on both release repositories.
