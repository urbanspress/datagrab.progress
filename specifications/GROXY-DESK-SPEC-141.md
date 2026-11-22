# Groxy Desk v1.4.1 - Database Studio layout patch

The project owner supplied the WebView2 DevTools HTML and CSS as the visual specification on 2026-11-22 and directed their implementation and synchronized release.

- Apply the mock's light shell colors, menu spacing, and icon rail. Preserve the dark theme's contrast.
- Place the database connection status beside the studio heading. Put the schema filter and collapsed object groups above saved connections.
- Use a single toolbar row for Data, SQL, ER Diagram, Export, Save, and the two Refresh actions. Give row and schema refresh distinct IDs and accessible names. Save opens the existing review flow.
- Preserve frozen DOM IDs and handlers in hidden or collapsed controls. Keep zero wrapper elements, zero unsafe DOM writes, exact ADR-025c values, and the GROXY-SAFE-001 approval and snapshot flow.
- Verify Gates 1–4, generate the installer, guide, and checksums, then synchronize the release on both remotes.
