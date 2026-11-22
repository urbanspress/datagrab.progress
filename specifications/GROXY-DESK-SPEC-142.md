# Groxy Desk v1.4.2 - Console-free launcher and workbench icons

The project owner supplied the requirements and approved v1.4.2 as the follow-up to the published v1.4.1 layout patch on 2026-11-22.

- Build the Windows launcher with `-s -w -H=windowsgui`. Both installer shortcuts launch this WebView2 app. Keep the standalone Windows daemon on the console subsystem.
- Add an 18px geometric emerald brand mark directly before the File menu in the existing 32px header. Add monochrome Grex SVG icons for the database rail and connection tree, New Connection, sidebar and command search, snapshot rollback and history, and the two settings variants.
- Open the existing native connection dialog directly from New Connection. Keep the v1.4.1 mock layout, frozen control IDs, keyboard navigation, Light and Dark palettes, exact numeric values, and write approval and snapshot behavior.
- Run the four verification gates, check both Windows executable subsystems and all package checksums, and verify shortcut launch on Windows. Publish the installer, bundle, binaries, guide, and hashes on both remotes under the same annotated tag. Refresh the ecosystem snapshot and handoff.
