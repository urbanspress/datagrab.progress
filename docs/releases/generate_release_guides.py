#!/usr/bin/env python3
"""
Generate printable HTML and Edge-rendered Walkthrough PDF guides for Groxy Desk releases.
Usage: python docs/releases/generate_release_guides.py [0.2.0]
"""
import html
import os
import shutil
import subprocess
import sys
import tempfile
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent.parent
HTML_DIR = ROOT / "docs" / "releases" / "html"
PDF_DIR = ROOT / "docs" / "releases" / "pdf"

EDGE_CANDIDATES = [
    Path(r"C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe"),
    Path(r"C:\Program Files\Microsoft\Edge\Application\msedge.exe"),
    Path(shutil.which("msedge") or "") if shutil.which("msedge") else None,
    Path(shutil.which("chrome") or "") if shutil.which("chrome") else None,
]


def find_browser() -> Path:
    for cand in EDGE_CANDIDATES:
        if cand and cand.exists():
            return cand
    raise FileNotFoundError("Could not locate msedge.exe or chrome to render PDF.")


def render_guide_html(version: str) -> str:
    v = html.escape(version)
    body_class = "candidate-guide" if version in ("1.1.0", "1.2.0", "1.3.0", "1.3.1", "1.4.0") else ""
    footer_text = f"Groxy Desk v{v} — Verified across Gate 1 (Go + govulncheck + grab check/format), Gate 2 (Pure Grex JS DOM/WCAG/ADR-025c), Gate 3 (GRAB-E401 + GROXY-SAFE-001 + DESK-E403/E409), and Gate 4 (Live E2E)."
    if version == "1.4.0":
        body_sections = """
  <header>
    <h1>Groxy Desk v1.4.0</h1>
    <p>True DbGate-Parity Connection Manager, Collapsible Rail &amp; Full-Screen Spreadsheet Grid</p>
    <p><strong>Complete &middot; All 4 Acceptance Gates Verified</strong></p>
  </header>
  <main>
    <section>
      <h2>1. Saved Connections &amp; Tabbed Connection Window</h2>
      <p>The Database Studio left sidebar now shows a compact <strong>CONNECTIONS</strong> list at the top with a <strong>+ New Connection</strong> button instead of raw form boxes. Click <strong>+</strong> to open the tabbed Connection dialog (<strong>General</strong> and <strong>SSH Tunnel</strong> tabs), then choose <strong>Save</strong> or <strong>Connect</strong>. Database and SSH passwords are encrypted on disk with AES-256-GCM (<code>v1:...</code>) and are never stored in browser storage.</p>
      <p>Once saved, <strong>one click</strong> on any connection opens its pinned SSH tunnel and loads your database schema automatically. Use <strong>Edit</strong>, <strong>Disconnect</strong>, or <strong>Delete</strong> right on the connection card.</p>
    </section>
    <section>
      <h2>2. Collapsible Left Sidebar &amp; Focused Schema Tree</h2>
      <p>Click the active left-rail icon (such as <strong>Database Studio</strong> or <strong>Code &amp; Grex Studio</strong>) anytime to collapse or reopen the sidebar and give your data grid or code editor extra screen width. The lower 75% of the Database sidebar is dedicated to filtering and browsing <code>Tables</code>, <code>Views</code>, and <code>Triggers</code> with row-count badges; clicking any table loads its rows immediately.</p>
    </section>
    <section>
      <h2>3. Full-Screen Spreadsheet Grid &amp; Dedicated Sub-Tabs</h2>
      <p>The center workspace uses a clean sub-tab bar (<strong>Data</strong>, <strong>Structure</strong>, <strong>SQL Query</strong>, <strong>ER Diagram</strong>, and <strong>Export &amp; Diff</strong>) with a single-line action toolbar. <strong>Data</strong> opens by default as an edge-to-edge spreadsheet grid with sticky row numbers (<code>1, 2, 3...</code>), column names with data types, live per-column filter boxes, <code>28px</code> rows, alternating row shading, and exact <code>ADR-025c</code> numbers.</p>
    </section>
    <section>
      <h2>4. Collapsed-by-Default Safety Inspector</h2>
      <p>The right-hand <strong>Row Inspector &amp; Safety Gate</strong> stays tucked away by default so the spreadsheet grid fills the screen. Click or double-click any editable cell (or click <strong>Write SQL / Snapshot Guard</strong>) to slide the inspector open, review your change with a verified SHA-256 pre-mutation snapshot, and approve or roll back safely.</p>
    </section>
  </main>"""
        footer_text = "Groxy Desk v1.4.0 — Verified across Acceptance Gates 1–4 (Go + govulncheck + grab check/format, Pure Grex JS v1.0.0 DOM/WCAG/ADR-025c, Native WebView2, Radar/CMS E2E, and Database Recovery)."
    elif version == "1.3.1":
        body_sections = """
  <header>
    <h1>Groxy Desk v1.3.1</h1>
    <p>Faster remote database schema inspection</p>
  </header>
  <main>
    <section>
      <h2>Install the patch</h2>
      <p>Download <code>Groxy-Desk-Setup-1.3.1.exe</code> and <code>SHA256SUMS.txt</code> from the same release. Compare the installer hash with the entry in the checksum file, then run the installer.</p>
    </section>
    <section>
      <h2>Open a remote schema</h2>
      <p>In Database Studio, open your saved PostgreSQL or MariaDB connection through its pinned SSH tunnel and refresh the schema. Expand Tables, Views, or Triggers in the schema tree.</p>
    </section>
    <section>
      <h2>What changed</h2>
      <p>Desk sends one catalog query and one query for exact table row counts, even when a database has dozens of tables. The inspection has a 20-second deadline. Views and triggers are listed according to the account's catalog visibility.</p>
      <p>Row counts and database numeric values retain their exact decimal text. Database writes still require explicit approval and a pre-mutation snapshot.</p>
    </section>
  </main>"""
        footer_text = "Groxy Desk v1.3.1 | Database Studio patch"
    elif version == "1.3.0":
        body_sections = """
  <header>
    <h1>Groxy Desk v1.3.0</h1>
    <p>VS Code / DbGate / Coolify High-Density 100vh Desktop IDE Shell</p>
    <p><strong>Complete &middot; All 4 Acceptance Gates Verified</strong></p>
  </header>
  <main>
    <section>
      <h2>1. Locked 100vh Desktop IDE Shell &amp; Dark Obsidian / Clean Light Palettes</h2>
      <p>Groxy Desk v1.3.0 locks <code>html, body, #desk-root</code> to <code>height: 100vh; max-height: 100vh; overflow: hidden;</code> so outer viewports never scroll and every operational tool remains visible above the fold. Both the <strong>Dark Obsidian</strong> (<code>#0f1115</code> / <code>#181b20</code>) and <strong>Clean Light</strong> (<code>#f8fafc</code> / <code>#ffffff</code>) palettes enforce WCAG AA contrast (&ge; 4.5:1 text, &ge; 3.0:1 UI components), <code>1px solid var(--grex-border)</code> hairline pane dividers, and compact <code>26px&ndash;28px</code> semantic monospace form controls.</p>
    </section>
    <section>
      <h2>2. 32px Top IDE Menu &amp; Command Bar + 48px Vertical Activity Icon Rail</h2>
      <p>The <code>32px</code> top bar (<code>#desk-header</code>) provides an application menu bar (<code>File</code>, <code>Edit</code>, <code>View</code>, <code>Database</code>, <code>Browser</code>, <code>Tools</code>, <code>Help</code>), a centered <code>Ctrl+K</code> command search bar (<code>#desk-command-search</code>), active workspace badge, authenticated loopback badge (<code>#desk-host-status-badge</code> / <code>#desk-connection-badge</code>), and theme toggle (<code>#desk-theme-toggle</code>).</p>
      <p>The <code>48px</code> far-left vertical activity icon rail (<code>#desk-activity-rail</code>, <code>role="tablist"</code>) renders crisp <code>grab-ide-icons</code> SVG vectors via Pure Grex JS <code>Icon</code> (<code>createElementNS</code>) for <strong>Explorer &amp; Code</strong> (<code>#desk-tab-code</code>), <strong>Database Studio</strong> (<code>#desk-tab-database</code>), <strong>Browser &amp; Radar</strong> (<code>#desk-tab-browser</code>), <strong>Grex Visual Builder</strong> (<code>#desk-tab-grex</code>), <strong>Server &amp; Fleet</strong> (<code>#desk-tab-fleet</code>), plus bottom-anchored <strong>Settings</strong> (<code>#desk-action-settings</code>) and <strong>Shortcuts</strong> (<code>#desk-action-shortcuts</code>).</p>
    </section>
    <section>
      <h2>3. Locked Edge-to-Edge Studios &amp; 28px Tabular Spreadsheet Grid</h2>
      <p><strong>Database Studio</strong> (<code>#desk-panel-database</code>) pairs a <code>240px</code> collapsible connection/schema tree with live object filtering (<code>#desk-db-object-filter</code>), multi-tab autocomplete SQL, a <code>28px</code>-row-height sticky-header zebra-striped spreadsheet grid (<code>.desk-grid-table</code>, <code>tabular-nums</code>, exact <code>ADR-025c</code> wire strings), and a <code>280px</code> <code>GROXY-SAFE-001</code> inspector drawer (<code>#desk-db-inspector</code>). <strong>Browser &amp; Radar</strong> (<code>#desk-panel-browser</code>) provides an edge-to-edge 50/50 split pane, and <strong>Code &amp; Grex Studio</strong> (<code>#desk-panel-code</code>) combines workspace exploration, Coolify-density Server &amp; Fleet pills, live Grex split-preview canvas, and Unified <strong>Groxy AI</strong> (zero model-switching dropdowns).</p>
    </section>
    <section>
      <h2>4. 24px Pinned Bottom IDE Status Bar</h2>
      <p>The <code>24px</code> status bar (<code>#desk-status-bar</code>) surfaces five live indicators at all times: <code>#desk-status-db</code> (1-click database connection pill), <code>#desk-status-ssh</code> (pinned <code>SHA256:...</code> host fingerprint), <code>#desk-status-safety</code> (<code>GROXY-SAFE-001</code> snapshot guard), <code>#desk-status-grisp</code> (active local worker pool), and <code>#desk-status-adr025c</code> (exact-wire runtime badge).</p>
    </section>
  </main>"""
        footer_text = "Groxy Desk v1.3.0 — Verified across Acceptance Gates 1–4 (Go + govulncheck + grab check/format, Pure Grex JS v1.0.0 DOM/WCAG/ADR-025c, Native WebView2, Radar/CMS E2E, and Database Recovery)."
    elif version == "1.2.0":
        body_sections = """
  <header>
    <h1>Groxy Desk v1.2.0</h1>
    <p>DbGate-Parity 3-Tab Dedicated Workspace Studio &amp; Visual Polish</p>
    <p><strong>Complete &middot; Section 9 Acceptance Gates Verified</strong></p>
  </header>
  <main>
    <section>
      <h2>Sticky Top 3-Tab Studio Switcher &amp; Offline Urbanist Typography</h2>
      <p>Groxy Desk v1.2.0 introduces a sticky top <code>role="tablist"</code> switcher across three dedicated full-viewport studio workspaces: <strong>Database Studio</strong>, <strong>Browser &amp; Radar</strong>, and <strong>Code &amp; Grex Studio</strong>. Keyboard navigation supports wrapping <code>ArrowLeft</code>/<code>ArrowRight</code> plus <code>Home</code>/<code>End</code>, with strictly allowlisted URL hash (<code>#studio=database|browser|code</code>) and <code>sessionStorage</code> persistence while keeping shared reactive state, active SSH tunnels, and local Grisp workers alive.</p>
      <p>The entire desktop IDE renders with locally bundled <code>Urbanist</code> (<code>.woff2</code>) geometric UI typography and monospace code/data fonts with zero external CDN requests and full Light/Dark WCAG AA (&ge; 4.5:1) parity.</p>
    </section>
    <section>
      <h2>Tab 1 &mdash; Database Studio (DbGate 3-Pane Workspace)</h2>
      <p>Database Studio provides a full-height 3-pane layout: saved/active connections (PostgreSQL, MariaDB, SQLite) and an expandable <code>Tables</code> / <code>Views</code> / <code>Triggers</code> schema tree on the left; a focused action toolbar (<code>Run Query</code>, <code>Browse table rows</code>, <code>Inspect schema</code>, <code>Export CSV/XLSX/JSON/SQL</code>, <code>ER Diagram</code>, <code>Schema Diff</code>, <code>Undo Snapshot</code>) above a multi-tab autocomplete SQL editor and spreadsheet-style data grid in the center; and an <code>ADR-025c</code> exact-decimal row/cell inspector plus <code>GROXY-SAFE-001</code> pre-mutation snapshot guard &amp; approval drawer on the right.</p>
    </section>
    <section>
      <h2>Tab 2 &mdash; Browser &amp; Radar (50/50 Split Pane)</h2>
      <p>Browser &amp; Radar pairs the isolated WebView2 allowlisted browser pane and live <code>GRAB-E402</code> origin-lock badge on the left with the Bestseller Radar niche explorer, <code>BRAND-DNA.md</code> organization prefill preview, and separately approved <code>GrabLab CMS</code> draft handoff (<code>status_enforced: "draft"</code>) on the right.</p>
    </section>
    <section>
      <h2>Tab 3 &mdash; Code &amp; Grex Studio (3-Pane Vibe-Coding Studio)</h2>
      <p>Code &amp; Grex Studio organizes workspace file exploration, <code>grabman v1.0.0</code> diagnostics, and collapsible Local <code>Grisp</code> Supervisor / Step 5 Fleet / Namecheap DNS / Author Mode accordions on the left; the active <code>.grab</code>/<code>.html</code> workbench and live responsive <code>Grex</code> split-preview canvas in the center; and Unified <code>Groxy AI</code> (Adaptive Reasoning, zero model-switching dropdowns), the 22-rule audit scorecard, and Pre-Mutation Snapshots on the right.</p>
    </section>
  </main>"""
        footer_text = "Groxy Desk v1.2.0 — Verified across Section 9 Acceptance Gates 1–4 (Go + govulncheck + grab check/format, Pure Grex JS v1.0.0 DOM/WCAG/ADR-025c, Native WebView2, Radar/CMS E2E, and Database Recovery)."
    elif version == "1.1.0":
        body_sections = """
  <header>
    <h1>Groxy Desk v1.1.0</h1>
    <p>Browser Co-Pilot, Bestseller Radar &amp; Visual Database Workbench</p>
    <p><strong>Complete &middot; Section 9 Acceptance Gates Verified (Stage 4 Sign-Off Pending)</strong></p>
  </header>
  <main>
    <section>
      <h2>Open the research pane</h2>
      <p>In Browser Co-Pilot, enter a URL from the configured HTTPS origins and choose Open. The active origin appears in the window title and panel. Resize or close the pane from the workbench.</p>
      <p>The browser has a separate profile. The workbench keeps service and database credentials in its own session. A blocked browser origin reports <code>GRAB-E402</code>.</p>
      <p>Capture selected text and add it to the editable Groxy AI prompt for review. On registration pages with a compatible organization field, preview and approve sharing the business name from <code>storage/BRAND-DNA.md</code>. Review and submit the form in the browser.</p>
    </section>
    <section>
      <h2>Research and review a book draft</h2>
      <ol>
        <li>Configure the Groxy AI v1.8.0 and GrabLab CMS origins on the host. Open Bestseller Radar, choose a genre and set any price or live-source preferences.</li>
        <li>Review the ranked results and source freshness. Confirm the research plan to obtain a Desk-bound draft receipt.</li>
        <li>Review the proposed fields and CMS workspace. Confirm the separate Create Book Draft operation. In CMS, verify the new entry has draft status.</li>
      </ol>
      <p>Author Mode can also open the configured CMS Radar page. That page uses its own CMS session.</p>
    </section>
    <section>
      <h2>Inspect a database</h2>
      <p>Choose a workspace SQLite file, or open a loopback SSH tunnel with a verified <code>known_hosts</code> key and its <code>SHA256:</code> fingerprint. For remote SQLite, choose the Gashboard bridge and enter its bearer and bridge-relative SQLite path.</p>
      <p>Load the schema, browse rows, open SQL tabs, or compare two environments. Choose Export relationship diagram to save the current catalog as workspace HTML. Enter decimals and large integers as exact text, such as <code>0.00</code>.</p>
    </section>
    <section>
      <h2>Review an edit and restore</h2>
      <p>Review the primary key, SQL, DDL, or import plan alongside its verified Pre-Mutation SHA-256 Snapshot before asking for approval. Use <strong>Undo last database change</strong> to review a restore, or <strong>Review forced restore</strong> when a reviewed conflict requires overwriting newer rows.</p>
      <p>Network recovery acquires an exclusive per-database lock over pinned SSH. Compliant least-privilege PostgreSQL 18 targets use a separate maintenance login to close target connections during recovery; compliant least-privilege MariaDB 11.8 targets use a database-scoped administrator with <code>GRANT OPTION</code> on the target database plus <code>CONNECTION ADMIN</code> and <code>PROCESS</code> (zero global <code>*.*</code> write privileges). If any non-Desk MariaDB account holds global write privileges, Desk fails closed with HTTP 409 (<code>write_fence_unavailable</code>) before approval. Partial MariaDB DDL failures retain the verified snapshot so one-click rollback restores exact tables, views, triggers, and data.</p>
    </section>
  </main>"""
        footer_text = "Groxy Desk v1.1.0 — Verified across Section 9 Acceptance Gates 1–4 (Go + govulncheck + grab check/format, Pure Grex JS DOM/WCAG/ADR-025c, Native WebView2, Radar/CMS E2E, and PostgreSQL 18 + Least-Privilege MariaDB 11.8 recovery)."
    elif version.startswith("1.0."):
        patch_note = ""
        tier_desc = f"GA {v} — Production Step 6 Desktop IDE Freeze &amp; Branded Windows Installer"
        if version == "1.0.2":
            tier_desc = f"PATCH {v} — Windows WebView2 Launch Crash Fix &amp; Branded Installer"
            patch_note = f"""
    <section>
      <h2>What&rsquo;s New in v{v} (PATCH 1.0.x — Windows WebView2 Launch Fix)</h2>
      <p><strong>Groxy Desk v{v}</strong> is a critical 100% drop-in compatible patch release resolving a Windows startup crash that caused Groxy Desk to flash black and exit immediately:</p>
      <ul>
        <li><strong>WebView2 Handshake Serialization Fix (<code>public/desk/workbench.js</code>):</strong> Safely serializes the initial <code>groxy-desk-ready</code> handshake message via <code>JSON.stringify</code> instead of passing a bare JavaScript object, preventing <code>go-webview2</code>'s internal <code>TryGetWebMessageAsString()</code> COM method from throwing <code>E_INVALIDARG</code> (<code>0x80070057</code>: &ldquo;The parameter is incorrect&rdquo;) and terminating the application process via <code>os.Exit(1)</code>.</li>
        <li><strong>Window Class Registration Safety (<code>cmd/groxy-desk/webview_windows.go</code>):</strong> Tolerates <code>ERROR_CLASS_ALREADY_EXISTS</code> during repeated desktop window initializations without returning an error.</li>
        <li><strong>Updated Installer:</strong> Refreshes <code>Groxy-Desk-Setup-1.0.2.exe</code>, static binaries, SHA-256 signatures, and walkthrough PDF guide.</li>
      </ul>
    </section>"""
        elif version != "1.0.0":
            tier_desc = f"PATCH {v} — Non-Breaking NGINX 1.30.6 Edge Alignment &amp; Branded Windows Installer"
            patch_note = f"""
    <section>
      <h2>What&rsquo;s New in v{v} (PATCH 1.0.x — Non-Breaking Alignment)</h2>
      <p><strong>Groxy Desk v{v}</strong> is a 100% drop-in compatible patch release aligning the built-in Namecheap DNS &amp; Auto-HTTPS edge generator (<code>renderEdgeHTTPSConfig</code> in <code>internal/dns/dns.go</code>) with official <code>nginx.org</code> <strong>NGINX 1.30.6 (Stable)</strong> and <strong>Grabtaculous v1.1.0</strong>:</p>
      <ul>
        <li>Replaces deprecated NGINX <code>1.24.x</code> <code>listen 443 ssl http2;</code> syntax with <code>listen 443 ssl;</code>, <code>listen [::]:443 ssl;</code>, and <code>http2 on;</code>.</li>
        <li>Emits the complete <code>GRAB v1.1.0</code> <code>--trusted-proxy</code> forwarding header tuple (<code>Host</code>, <code>X-Real-IP</code>, <code>X-Forwarded-For</code>, <code>X-Forwarded-Proto $scheme;</code>, and <code>X-Forwarded-Host $host;</code>) so downstream Grisp apps always see accurate client IPs and HTTPS origins.</li>
      </ul>
    </section>"""
        body_sections = f"""
  <header>
    <h1>Groxy Desk v{v} — Production GA Walkthrough, 7-Layer Audit &amp; Architecture Guide</h1>
    <p><strong>Release Tier:</strong> {tier_desc} | <strong>Default Bind:</strong> <code>127.0.0.1:9490</code> | <strong>Built With:</strong> Go CGO-Free WebView2 + GRAB v1.1.0 + Pure Grex JS v1.0.0</p>
  </header>

  <main>{patch_note}
    <section>
      <h2>1. Plain-English Overview of Groxy Desk v{v} (Production GA)</h2>
      <p><strong>Groxy Desk v{v}</strong> is the Production General Availability release for <strong>Step 6 of the Post-1.0 GRAB Ecosystem</strong>. Double-clicking <code>Groxy-Desk-Setup-{v}.exe</code> installs a lightweight native Windows WebView2 desktop IDE (~15–25 MB UI RAM overhead, zero Electron/Chromium bundle bloat) paired with the static <code>CGO_ENABLED=0</code> Go loopback daemon (<code>groxy-deskd.exe</code> on <code>127.0.0.1:9490</code>) and side-by-side <code>grab.exe</code> (<code>v1.1.0</code>) + <code>grabman.exe</code> (<code>v1.0.0</code>) CLIs—without modifying any frozen <code>v1.x</code> upstream repository.</p>
      <ul>
        <li><strong>Branded Flicker-Free Inno Setup Installer (<code>Groxy-Desk-Setup-{v}.exe</code>):</strong> Installs cleanly under <code>{{localappdata}}\\Groxy Desk</code> (<code>PrivilegesRequired=lowest</code>, zero mandatory UAC elevation) with Start Menu &amp; optional Desktop shortcuts, and preserves user workspaces, encrypted vaults (<code>v1:...</code>), and <code>GROXY-SAFE-001</code> Pre-Mutation Snapshots on upgrade or uninstall.</li>
        <li><strong>One Unified AI Partner (<code>Groxy AI</code> — Zero Model-Switching Dropdowns):</strong> The <strong>Automatic Adaptive Reasoning Router</strong> seamlessly routes fast daily checks vs. deep multi-step architecture with local open-weights <strong>Ollama (<code>127.0.0.1:11434</code>)</strong> support and zero mandatory cloud API fees.</li>
        <li><strong>Complete Post-1.0 Studio &amp; Step 5 Operations:</strong> Supervises up to 8 local <code>Grisp</code> workers (<code>127.0.0.1:8765–8772</code>), bridges all <strong>13</strong> unified <code>grab mcp</code> tools across Core GRAB + GrabLab CMS + Grex Theme Builder, embeds a live split-screen preview canvas with 1-click <code>Ctrl+Z</code> (<code>applyAiLayout</code>) undo, runs the 22-rule 4-pillar SEO/WCAG/DOM/Link scorecard + 🚀 <strong>&ldquo;Let Groxy Boost This&rdquo;</strong> remediation diffs, manages <code>known_hosts</code>-pinned Step 5 Fleet deploys, opens up to 8 <code>127.0.0.1:15432–15439</code> SSH DB tunnels + read-only-default Visual DB Explorer, automates Namecheap DNS + Auto-HTTPS, and scaffolds 1-Click Local <strong>Author Mode</strong>.</li>
      </ul>
    </section>

    <section>
      <h2>2. Pre-v1.0.0 7-Layer Audit Sign-Off &amp; Frozen Security Taxonomy</h2>
      <table>
        <thead>
          <tr><th>Code</th><th>HTTP</th><th>Frozen v{v} Protection Guarantee</th></tr>
        </thead>
        <tbody>
          <tr><td><code>DESK-E501</code></td><td>403 / 500</td><td>Fails closed on <code>0.0.0.0</code>, <code>::</code>, wildcards, or non-loopback binds/URLs (strictly <code>127.0.0.1</code> only).</td></tr>
          <tr><td><code>GRAB-E401</code></td><td>403</td><td>Confines all workspace &amp; SQLite paths; blocks <code>..</code>, UNC shares, escaping symlinks/junctions, NUL bytes, <code>.env*</code>, <code>.git</code>, <code>.ssh</code>, private keys, and non-loopback <code>GrexPreviewBridge</code> origins.</td></tr>
          <tr><td><code>DESK-E403</code></td><td>403</td><td>Enforces constant-time <code>Authorization: Bearer</code>, <code>X-CSRF-Token</code>, and target-bound <code>approval_token</code> scope matching.</td></tr>
          <tr><td><code>DESK-E405</code></td><td>403</td><td>Rejects missing or mismatched <code>known_hosts</code> <code>SHA256:...</code> SSH host-key fingerprints (zero <code>InsecureIgnoreHostKey</code>).</td></tr>
          <tr><td><code>DESK-E406</code></td><td>403</td><td>Blocks multi-statement SQL, DDL/DML writes, <code>ATTACH DATABASE</code>, or write <code>PRAGMA</code> inside Read-Only Visual DB Query mode.</td></tr>
          <tr><td><code>DESK-E409</code></td><td>409</td><td>Rejects single-use <code>approval_token</code> replays and enforces &le; 8 <code>Grisp</code> workers and &le; 8 SSH DB tunnels.</td></tr>
          <tr><td><code>DESK-E302</code></td><td>422</td><td>Enforces <code>groxy-site-dna-v0.4.0</code> zero-<code>&lt;div&gt;</code>, WCAG AA (&ge; 4.5:1), <code>ADR-025c</code> wire strings, and <code>--read-only</code> non-root fleet deploys.</td></tr>
          <tr><td><code>DESK-E303</code></td><td>422</td><td>Verifies (both at Plan and pre-token Apply) that Namecheap DNS updates preserve <code>MX</code>, <code>TXT</code>, <code>SPF</code>, <code>DKIM</code>, <code>DMARC</code>, <code>CAA</code>, <code>SRV</code>, <code>NS</code>, and email <code>CNAME</code> records byte-for-byte.</td></tr>
          <tr><td><code>DESK-E304</code></td><td>422</td><td>Verifies live DNS propagation before ACME / Let's Encrypt Auto-HTTPS provisioning.</td></tr>
          <tr><td><code>DESK-E503</code></td><td>503</td><td>Fails closed when local Ollama (<code>127.0.0.1:11434</code>) is unreachable, with clean subprocess teardown on daemon shutdown.</td></tr>
        </tbody>
      </table>
    </section>

    <section>
      <h2>3. Frozen v{v} Contract Inventory (55 Paths / 57 Handlers &amp; 13 MCP Tools)</h2>
      <ul>
        <li><strong>Health, Shell &amp; Workspace (10 routes):</strong> <code>GET /_health</code> (<code>"version": "{v}"</code>), <code>GET /_ready</code>, <code>GET /</code>, <code>GET /public/{{rest...}}</code>, <code>GET/POST /api/preferences/theme</code>, <code>GET /api/workspace/tree</code>, <code>GET /api/workspace/read</code>, <code>POST /api/workspace/check</code>, and <code>grabman init/plan</code>, <code>init</code>, <code>doctor</code>, <code>solve-verify</code>.</li>
        <li><strong>Local Grisp Supervisor &amp; Unified Groxy AI (12 routes):</strong> <code>GET /api/grisp/workers</code>, <code>start</code>, <code>reload</code>, <code>stop</code>, <code>logs</code>; <code>POST /api/groxy/ask</code>, <code>approvals/apply</code>, <code>GET /api/groxy/snapshots</code>, <code>snapshots/restore</code>, <code>audit</code>, <code>boost</code>, and <code>boost/approve</code>.</li>
        <li><strong>13-Tool Unified <code>grab mcp</code> &amp; Visual Studio (5 routes):</strong> <code>GET /api/mcp/tools</code>, <code>POST /api/mcp/call</code> (<code>grab_check</code>, <code>grab_format</code>, <code>grab_inspect_routes</code>, <code>grab_inspect_schema</code>, <code>grab_scaffold</code>, <code>list_content_types</code>, <code>query_entries</code>, <code>create_entry_draft</code> with <code>status_enforced: "draft"</code>, <code>update_entry_layout</code>, <code>list_media</code>, <code>list_menus</code>, <code>studio_design_system</code>, <code>studio_apply_layout</code>), <code>GET /api/studio/design-system</code>, <code>POST /api/studio/synthesize</code>, and <code>POST /api/studio/layout/apply</code>.</li>
        <li><strong>Step 5 Fleet, DB Studio, Namecheap DNS/HTTPS &amp; Author Mode (26 routes / 27 handlers):</strong> Fleet node verification, <code>Site DNA</code> validation, deploy plan/apply &amp; rollback; SSH tunnel open/close/list, schema, bounded &le; 100 row pagination (<code>ADR-025c</code>), read-only query, mutate plan/apply &amp; rollback; Namecheap <code>AES-256-GCM</code> vault, DNS plan/apply, HTTPS preflight/apply &amp; rollback; and Author Mode plan/apply + Ollama status/start.</li>
      </ul>
    </section>

    <section>
      <h2>4. Installer &amp; Quickstart Commands</h2>
      <pre># Install via Branded Inno Setup Windows Installer (.exe only, never .msi)
dist\\Groxy-Desk-Setup-{v}.exe

# Or run static CGO_ENABLED=0 release binaries directly
dist\\groxy-desk-windows-amd64.exe --workspace C:\\path\\to\\project
dist\\groxy-desk-windows-amd64.exe --smoke-test --addr 127.0.0.1:0
dist\\groxy-deskd-windows-amd64.exe --addr 127.0.0.1:9490 --workspace C:\\path\\to\\project</pre>
    </section>

    <section>
      <h2>5. 7-Layer Ecosystem Compatibility Matrix (GA v{v})</h2>
      <table>
        <thead>
          <tr><th>Ecosystem Layer</th><th>Repository</th><th>Required Version Range</th><th>Touched in v{v}?</th><th>Neighbor Upgrade Required?</th></tr>
        </thead>
        <tbody>
          <tr><td>Core Language, Bytecode VM &amp; Grisp Server</td><td><code>urbanspress/grab</code></td><td><code>&gt;= v1.1.0 (&lt; v2.0.0)</code></td><td>No</td><td>No</td></tr>
          <tr><td>Reactive Frontend Runtime &amp; 20-Element Library</td><td><code>urbanspress/grex-js</code></td><td><code>&gt;= v1.0.0 (&lt; v2.0.0)</code></td><td>No</td><td>No</td></tr>
          <tr><td>Flagship Zero-Plugin CMS</td><td><code>urbanspress/grablab-cms</code></td><td><code>&gt;= v1.0.1 (&lt; v2.0.0)</code></td><td>No</td><td>No</td></tr>
          <tr><td>Exclusive <code>mode: "visual"</code> Theme &amp; Studio</td><td><code>urbanspress/grex-builder</code></td><td><code>&gt;= v1.0.0 (&lt; v2.0.0)</code></td><td>No</td><td>No</td></tr>
          <tr><td>Autonomous Design, Content &amp; SEO Agent</td><td><code>urbanspress/groxy-ai</code></td><td><code>&gt;= v1.0.0 (&lt; v2.0.0)</code></td><td>No</td><td>No</td></tr>
          <tr><td>Scaffolder, Orchestrator &amp; Discreet VPS Panel</td><td><code>grabman</code>, <code>grabtaculous</code>, <code>gashboard</code></td><td><code>&gt;= v1.0.0 (&lt; v2.0.0)</code> (Grabtaculous <code>v1.1.0</code>)</td><td>No</td><td>No</td></tr>
          <tr><td>Native Desktop Vibe-Coding IDE &amp; Studio</td><td><code>urbanspress/groxy-desk</code></td><td><code>&gt;= v1.0.0 (&lt; v2.0.0)</code></td><td>Yes (v{v})</td><td>No</td></tr>
        </tbody>
      </table>
    </section>
  </main>"""
    elif version.startswith("0.3."):
        body_sections = f"""
  <header>
    <h1>Groxy Desk v{v} — Step 5 Fleet, Visual DB Studio, Namecheap DNS/HTTPS &amp; Author Mode Guide</h1>
    <p><strong>Release Tier:</strong> PRE-1.0 Step 6 Integrated Infrastructure &amp; Author Studio | <strong>Default Bind:</strong> <code>127.0.0.1:9490</code> | <strong>Built With:</strong> Go CGO-Free WebView2 + GRAB v1.1.0 + Pure Grex JS v1.0.0</p>
  </header>

  <main>
    <section>
      <h2>1. Plain-English Overview of What's New in v{v}</h2>
      <p><strong>Groxy Desk v{v}</strong> adds full Step 5 production orchestration, encrypted database management, domain-to-HTTPS automation, and 1-click Local Author Mode directly inside the native desktop workbench—with zero cloud gatekeeping and zero modifications to any frozen <code>v1.x</code> peer repository.</p>
      <ul>
        <li><strong>Step 5 Fleet &amp; Grabtaculous Bridge (<code>internal/fleet/</code>):</strong> Connects over <code>known_hosts</code>-pinned SSH (<code>SHA256:...</code> fingerprint check — <code>403 DESK-E405</code> on mismatch) to loopback <code>grabtaculousd</code> (<code>127.0.0.1:9440</code>), validates <code>groxy-site-dna-v0.4.0</code> (<code>422 DESK-E302</code>), enforces <code>--read-only</code> non-root containers (pinned <code>@sha256</code> digests), and requires single-use <code>approval_token</code> + SHA-256 Pre-Deploy Snapshots (<code>snap_deploy_*</code>) for rollouts and 1-click rollbacks.</li>
        <li><strong>Encrypted 127.0.0.1 SSH DB Tunnels &amp; Visual DB Explorer (<code>internal/dbstudio/</code>):</strong> Opens up to 8 concurrent <code>127.0.0.1:15432–15439</code> SSH tunnels for PostgreSQL/MariaDB or confined SQLite (<code>GRAB-E401</code>), browses bounded pages (&le; 100 rows) with exact <code>ADR-025c</code> base-10 wire strings, runs strict read-only <code>SELECT</code>/<code>EXPLAIN</code> queries (<code>403 DESK-E406</code> on writes), and gates writes behind Pre-Mutation Snapshots (<code>snap_db_*</code>) + 1-click restore.</li>
        <li><strong>Namecheap AES-256-GCM DNS Vault &amp; Auto-HTTPS Wizard (<code>internal/dns/</code>):</strong> Encrypts API keys at rest (<code>v1:...</code>), updates web records while preserving <code>MX</code>/<code>TXT</code>/<code>SPF</code>/<code>DKIM</code>/<code>DMARC</code> records 100% byte-identical (<code>422 DESK-E303</code>), blocks ACME issuance until DNS propagation converges (<code>422 DESK-E304</code>), and generates Caddy, Nginx+Certbot, or Cloudflare Tunnel configs to <code>127.0.0.1:8765–8772</code>.</li>
        <li><strong>1-Click Local Author Mode &amp; Zero-Fee Ollama Runtime (<code>internal/author/</code>):</strong> Scaffolds <code>Book</code>, <code>Chapter</code>, <code>CharacterBible</code>, and <code>WorldLore</code> schemas (<code>status_enforced: "draft"</code>), <code>SITE-CONTEXT.md</code> continuity, zero-<code>&lt;div&gt;</code> landing markup, and XSS-safe <code>schema.org/Book</code> JSON-LD alongside a local loopback Ollama launcher (<code>127.0.0.1:11434</code>, zero model-switching dropdowns).</li>
      </ul>
    </section>

    <section>
      <h2>2. Security Error Codes Added / Enforced in v{v}</h2>
      <table>
        <thead>
          <tr><th>Code</th><th>HTTP</th><th>Protection Guarantee</th></tr>
        </thead>
        <tbody>
          <tr><td><code>DESK-E405</code></td><td>403</td><td>Rejects missing or mismatched SSH host key <code>SHA256:...</code> pins (zero <code>InsecureIgnoreHostKey</code>).</td></tr>
          <tr><td><code>DESK-E406</code></td><td>403</td><td>Blocks write/DDL or multi-statement SQL inside Read-Only DB Query mode.</td></tr>
          <tr><td><code>DESK-E302</code></td><td>422</td><td>Rejects wrapper <code>&lt;div&gt;</code>, XSS payloads, WCAG AA contrast &lt; 4.5:1, or IEEE-754 float drift in <code>Site DNA</code>.</td></tr>
          <tr><td><code>DESK-E303</code></td><td>422</td><td>Aborts Namecheap DNS plans if any <code>MX</code>/<code>TXT</code>/<code>SPF</code>/<code>DKIM</code>/<code>CAA</code>/<code>SRV</code> record would be dropped or altered.</td></tr>
          <tr><td><code>DESK-E304</code></td><td>422</td><td>Blocks Auto-HTTPS / Let's Encrypt issuance until DNS resolves to the target VPS IP.</td></tr>
          <tr><td><code>GRAB-E401</code> / <code>DESK-E501</code></td><td>403 / 422</td><td>Confines workspace SQLite/migration/Author files and blocks non-loopback control planes or SSRF endpoints.</td></tr>
        </tbody>
      </table>
    </section>

    <section>
      <h2>3. Quickstart &amp; Verification</h2>
      <pre>groxy-desk-windows-amd64.exe --workspace C:\\path\\to\\project
groxy-desk-windows-amd64.exe --smoke-test --addr 127.0.0.1:0
groxy-deskd-windows-amd64.exe --addr 127.0.0.1:9490 --workspace C:\\path\\to\\project</pre>
    </section>

    <section>
      <h2>4. 7-Layer Ecosystem Compatibility Matrix</h2>
      <table>
        <thead>
          <tr><th>Ecosystem Layer</th><th>Repository</th><th>Required Range</th><th>Repo Modified?</th></tr>
        </thead>
        <tbody>
          <tr><td>Core Language &amp; Grisp Server</td><td><code>urbanspress/grab</code></td><td><code>&gt;= v1.1.0 (&lt; v2.0.0)</code></td><td>No (Consumable CLI &amp; <code>grab mcp</code>)</td></tr>
          <tr><td>Reactive Frontend Runtime</td><td><code>urbanspress/grex-js</code></td><td><code>&gt;= v1.0.0 (&lt; v2.0.0)</code></td><td>No (Vendored SHA-256 Parity)</td></tr>
          <tr><td>Flagship Zero-Plugin CMS</td><td><code>urbanspress/grablab-cms</code></td><td><code>&gt;= v1.0.1 (&lt; v2.0.0)</code></td><td>No (Author &amp; Draft Bridge)</td></tr>
          <tr><td>Exclusive Visual Theme &amp; Studio</td><td><code>urbanspress/grex-builder</code></td><td><code>&gt;= v1.0.0 (&lt; v2.0.0)</code></td><td>No (Loopback Studio Bridge)</td></tr>
          <tr><td>Autonomous Design &amp; SEO Agent</td><td><code>urbanspress/groxy-ai</code></td><td><code>&gt;= v1.0.0 (&lt; v2.0.0)</code></td><td>No (Adaptive Router &amp; Boost)</td></tr>
          <tr><td>Scaffolder, Orchestrator &amp; VPS Panel</td><td><code>grabman</code> / <code>grabtaculous</code> / <code>gashboard</code></td><td><code>&gt;= v1.0.0 (&lt; v2.0.0)</code></td><td>No (Bridged via Step 5 Contracts)</td></tr>
          <tr><td>Native Desktop Vibe-Coding IDE</td><td><code>urbanspress/groxy-desk</code></td><td><code>v{v}</code></td><td>Yes (Step 6 v{v} Release)</td></tr>
        </tbody>
      </table>
    </section>
  </main>"""
    elif version.startswith("0.2."):
        body_sections = f"""
  <header>
    <h1>Groxy Desk v{v} — Walkthrough &amp; Architecture Guide</h1>
    <p><strong>Release Tier:</strong> PRE-1.0 Step 6 Native Desktop IDE | <strong>Default Bind:</strong> <code>127.0.0.1:9490</code> | <strong>Built With:</strong> Go CGO-Free WebView2 + GRAB v1.1.0 + Pure Grex JS v1.0.0</p>
  </header>

  <main>
    <section>
      <h2>1. Plain-English Overview</h2>
      <p><strong>Groxy Desk</strong> is the native Windows desktop IDE built for Step 6 of the GRAB Post-1.0 Ecosystem. Opening <code>groxy-desk.exe</code> launches an authenticated loopback host daemon (<code>127.0.0.1:9490</code>) and a native Microsoft Edge WebView2 desktop window—without requiring Electron or external cloud logins.</p>
      <ul>
        <li><strong>Zero Model-Switching Dropdowns:</strong> <strong>Groxy AI</strong> is your sole permanent assistant across the entire IDE. Its Adaptive Reasoning Router chooses instant execution for fast checks and gated planning with rollback safety for file changes.</li>
        <li><strong>Local Grisp Worker Control:</strong> Start, health-check, reload, inspect redacted logs for, and cleanly shut down up to 8 local <code>grab.exe</code> workers on <code>127.0.0.1:8765–8772</code>.</li>
        <li><strong>Built-In grabman v1.0.0 Manager:</strong> Initialize projects, audit deployments with <code>grabman doctor</code>, and verify lockfiles with <code>grabman solve --verify</code> in one click.</li>
      </ul>
    </section>

    <section>
      <h2>2. Security &amp; Confinement Boundaries</h2>
      <table>
        <thead>
          <tr><th>Code</th><th>HTTP</th><th>Guarantee</th></tr>
        </thead>
        <tbody>
          <tr><td><code>DESK-E501</code></td><td>500</td><td>Fails closed unless bound strictly to <code>127.0.0.1</code> / loopback (blocks <code>0.0.0.0</code>, <code>::</code>, LAN IPs).</td></tr>
          <tr><td><code>DESK-E403</code></td><td>403</td><td>Enforces constant-time <code>Authorization: Bearer</code>, <code>X-CSRF-Token</code>, and target-bound <code>approval_token</code> matching.</td></tr>
          <tr><td><code>DESK-E409</code></td><td>409</td><td>Rejects any replay attempt of a single-use <code>approval_token</code>.</td></tr>
          <tr><td><code>GRAB-E401</code></td><td>403</td><td>Rejects traversal (<code>..</code>), symlink/NTFS junction escapes, and sensitive files (<code>.env</code>, <code>.git</code>, <code>*.pem</code>, <code>*.key</code>, <code>id_rsa</code>, <code>id_ed25519</code>).</td></tr>
          <tr><td><code>DESK-E503</code></td><td>503</td><td>Fails closed when an explicitly configured local Ollama endpoint is unreachable.</td></tr>
        </tbody>
      </table>
    </section>

    <section>
      <h2>3. Adaptive Reasoning Router &amp; GROXY-SAFE-001 Rollback</h2>
      <ol>
        <li><strong>Fast Instant Path:</strong> Deterministic 22-rule SEO/A11Y/DOM/Link audits, <code>grab check</code> + canonical <code>grab format</code> checks, <code>grabman doctor</code>, and <code>Grisp</code> worker status checks execute immediately.</li>
        <li><strong>Deep Gated Path:</strong> Code edits or layout fixes draft a step-by-step Plan, capture a pre-mutation <strong>SHA-256 Snapshot</strong> outside the workspace, and issue a single-use target-bound <code>approval_token</code>.</li>
        <li><strong>1-Click Snapshot Rollback:</strong> Every approved edit remains reversible to exact byte-for-byte original SHA-256 state in one click (or removes newly created files cleanly).</li>
      </ol>
    </section>

    <section>
      <h2>4. Pure Grex JS v1.0.0 Light &amp; Dark Workbench</h2>
      <p>The entire browser workbench renders exclusively with the 20 semantic elements defined in <code>grex.elements.json</code>—guaranteeing <code>divCount === 0</code>, <code>innerHTMLWriteCount === 0</code>, zero <code>eval()</code>/<code>new Function()</code>, zero-FOUC theme loading, WCAG AA contrast (&ge; 4.5:1) in both Light &amp; Dark themes, and exact <code>ADR-025c</code> base-10 wire strings.</p>
    </section>

    <section>
      <h2>5. Quickstart Commands</h2>
      <pre>groxy-desk-windows-amd64.exe --workspace C:\\path\\to\\project
groxy-desk-windows-amd64.exe --smoke-test --addr 127.0.0.1:0
groxy-deskd-windows-amd64.exe --addr 127.0.0.1:9490 --workspace C:\\path\\to\\project</pre>
    </section>

    <section>
      <h2>6. 7-Layer Ecosystem Compatibility Matrix</h2>
      <table>
        <thead>
          <tr><th>Ecosystem Layer</th><th>Repository</th><th>Required Range</th><th>Touched in v{v}?</th></tr>
        </thead>
        <tbody>
          <tr><td>Core Language &amp; Grisp Server</td><td><code>urbanspress/grab</code></td><td><code>&gt;= v1.1.0 (&lt; v2.0.0)</code></td><td>No (Consumable CLI)</td></tr>
          <tr><td>Reactive Frontend Runtime</td><td><code>urbanspress/grex-js</code></td><td><code>&gt;= v1.0.0 (&lt; v2.0.0)</code></td><td>No (Vendored v1.0.0)</td></tr>
          <tr><td>Flagship Zero-Plugin CMS</td><td><code>urbanspress/grablab-cms</code></td><td><code>&gt;= v1.0.1 (&lt; v2.0.0)</code></td><td>No</td></tr>
          <tr><td>Exclusive Visual Theme &amp; Studio</td><td><code>urbanspress/grex-builder</code></td><td><code>&gt;= v1.0.0 (&lt; v2.0.0)</code></td><td>No</td></tr>
          <tr><td>Autonomous Design &amp; SEO Agent</td><td><code>urbanspress/groxy-ai</code></td><td><code>&gt;= v1.0.0 (&lt; v2.0.0)</code></td><td>No</td></tr>
          <tr><td>Scaffolder, Fleet &amp; VPS Panel</td><td><code>grabman</code> / <code>grabtaculous</code> / <code>gashboard</code></td><td><code>&gt;= v1.0.0 (&lt; v2.0.0)</code></td><td>No (<code>grabman v1.0.0</code> Bridge)</td></tr>
          <tr><td>Native Desktop Vibe-Coding IDE</td><td><code>urbanspress/groxy-desk</code></td><td><code>v{v}</code></td><td>Yes</td></tr>
        </tbody>
      </table>
    </section>
  </main>"""
    else:
        body_sections = f"""
  <header>
    <h1>Groxy Desk v{v} — Unified grab mcp Studio &amp; Walkthrough Guide</h1>
    <p><strong>Release Tier:</strong> PRE-1.0 Step 6 Native Desktop IDE | <strong>Default Bind:</strong> <code>127.0.0.1:9490</code> | <strong>Built With:</strong> Go CGO-Free WebView2 + GRAB v1.1.0 + Pure Grex JS v1.0.0</p>
  </header>

  <main>
    <section>
      <h2>1. What's New in Groxy Desk v{v} (Plain-English Overview)</h2>
      <p><strong>Groxy Desk v{v}</strong> connects the native desktop workbench to the full Post-1.0 ecosystem over authenticated loopback IPC—giving developers an all-in-one visual studio without leaving the IDE or modifying any frozen <code>v1.x</code> upstream repository.</p>
      <ul>
        <li><strong>Unified 13-Tool <code>grab mcp</code> Bridge:</strong> Execute Core GRAB <code>v1.1.0</code> CLI tools (<code>grab_check</code>, <code>grab_format</code>, <code>grab_inspect_routes</code>, <code>grab_inspect_schema</code>, <code>grab_scaffold</code>), GrabLab CMS <code>v1.0.1</code> draft/content tools (<code>list_content_types</code>, <code>query_entries</code>, <code>create_entry_draft</code>, <code>update_entry_layout</code>, <code>list_media</code>, <code>list_menus</code>), and Grex Theme Builder <code>v1.0.0</code> endpoints (<code>GET /api/design-system.md</code>, <code>POST /api/mcp/layout</code>) under one permanent <strong>Groxy AI</strong> experience with zero model-switching dropdowns.</li>
        <li><strong>Live Split-Preview Workbench Canvas:</strong> Switch instantly between <strong>Code</strong>, <strong>Split</strong>, and <strong>Visual &amp; Scorecard</strong> views connected to local <code>Grisp</code> workers (<code>127.0.0.1:8765–8772</code>) via <code>GrexPreviewBridge</code> (<code>grex-preview-v1</code>) across <strong>Base</strong>, <strong>Tablet (768px)</strong>, and <strong>Mobile (375px)</strong> viewports.</li>
        <li><strong>8 Style Packs &times; 5 Blueprints &amp; 1-Click Ctrl+Z Undo:</strong> Generate WCAG AA Light &amp; Dark zero-<code>&lt;div&gt;</code> layouts across all 40 Style Pack combinations enriched with <code>GRAB-DESIGN-SYSTEM.md</code>, <code>SITE-CONTEXT.md</code>, and <code>grex.elements.json</code>, dispatched with <code>actor: "groxy-ai"</code> and full <code>applyAiLayout</code> <code>Ctrl+Z</code> undo parity.</li>
        <li><strong>22-Rule Scorecard &amp; 🚀 &ldquo;Let Groxy Boost This&rdquo;:</strong> Inspect 4 equal-weight pillars (<code>on_page_seo</code>, <code>content_a11y</code>, <code>performance_dom</code>, <code>linking_readiness</code> — weight <code>"25"</code> each), review side-by-side before/after diffs with <code>schema.org</code> JSON-LD + semantic landmark fixes, and apply with Pre-Mutation SHA-256 Snapshot rollback.</li>
      </ul>
    </section>

    <section>
      <h2>2. Security, Loopback Confinement &amp; Validation Guarantees</h2>
      <table>
        <thead>
          <tr><th>Code / Status</th><th>HTTP / RPC</th><th>Guarantee</th></tr>
        </thead>
        <tbody>
          <tr><td><code>DESK-E501</code></td><td>500</td><td>Rejects all non-loopback host bindings or external MCP/CMS URLs (<code>127.0.0.1</code> / loopback only).</td></tr>
          <tr><td><code>GRAB-E401</code></td><td>403</td><td>Workspace confinement across stdio <code>grab mcp</code> arguments, workspace reads/writes, symlinks/junctions, and secrets (<code>.env</code>, <code>.git</code>, <code>*.pem</code>, <code>*.key</code>, keys).</td></tr>
          <tr><td><code>DESK-E403</code></td><td>403</td><td>Enforces constant-time Bearer auth, <code>X-CSRF-Token</code>, and single-use target-bound <code>approval_token</code> matching.</td></tr>
          <tr><td><code>DESK-E409</code></td><td>409</td><td>Rejects any replay of a consumed <code>approval_token</code> on Boost, file writes, or MCP <code>--allow-write</code> calls.</td></tr>
          <tr><td><code>ValidationError</code></td><td>422 / -32602</td><td>Rejects invalid semantic/WCAG trees (<code>&lt;div&gt;</code>, contrast &lt; 4.5:1, missing <code>alt</code>) in CMS layout updates and enforces <code>status_enforced: "draft"</code> on draft creation.</td></tr>
        </tbody>
      </table>
    </section>

    <section>
      <h2>3. Loopback HTTP Endpoints Added in v{v}</h2>
      <table>
        <thead>
          <tr><th>Method &amp; Route</th><th>Router Path</th><th>Purpose</th></tr>
        </thead>
        <tbody>
          <tr><td><code>GET /api/mcp/tools</code></td><td>Fast</td><td>Discover all 13 unified Core GRAB, GrabLab CMS, and Grex Builder MCP tools.</td></tr>
          <tr><td><code>POST /api/mcp/call</code></td><td>Fast / Deep</td><td>Execute confined MCP tools via structured <code>argv</code> slices; write-capable tools require consumed <code>approval_token</code>.</td></tr>
          <tr><td><code>GET /api/studio/design-system</code></td><td>Fast</td><td>Load <code>GRAB-DESIGN-SYSTEM.md</code>, <code>SITE-CONTEXT.md</code>, and 8 Style Packs &times; 5 Blueprints catalog.</td></tr>
          <tr><td><code>POST /api/studio/synthesize</code></td><td>Fast</td><td>Synthesize zero-<code>&lt;div&gt;</code> semantic HTML + inverse undo state with <code>ADR-025c</code> numeric wire fidelity.</td></tr>
          <tr><td><code>POST /api/studio/layout/apply</code></td><td>Deep</td><td>Capture Pre-Mutation Snapshot and apply layout to local CMS/Builder with <code>actor: "groxy-ai"</code> &amp; <code>Ctrl+Z</code> stack.</td></tr>
          <tr><td><code>POST /api/groxy/audit</code></td><td>Fast</td><td>Run deterministic 22-rule 4-pillar SEO/A11Y/DOM/Link audit (hard <code>F</code> on <code>&lt;div&gt;</code> or inline script/event handlers).</td></tr>
          <tr><td><code>POST /api/groxy/boost</code></td><td>Deep</td><td>Generate side-by-side remediation diff + projected score delta (<code>F &rarr; A/A+</code>) + single-use approval token.</td></tr>
          <tr><td><code>POST /api/groxy/boost/approve</code></td><td>Deep</td><td>Validate <code>X-CSRF-Token</code> + single-use target-bound <code>approval_token</code>, apply fix cleanly, or reject replay (<code>409</code>).</td></tr>
        </tbody>
      </table>
    </section>

    <section>
      <h2>4. Pure Grex JS v1.0.0 Light &amp; Dark UI &amp; Origin-Checked Bridge</h2>
      <p>All workbench views render strictly with the 20 elements in <code>grex.elements.json</code> (<code>divCount === 0</code>, <code>innerHTMLWriteCount === 0</code>, zero <code>eval()</code>/<code>new Function()</code>, zero unsafe URLs, WCAG AA contrast &ge; 4.5:1 Light &amp; Dark, exact <code>ADR-025c</code> wire strings). Cross-iframe studio synchronization uses <code>GrexPreviewBridge</code> (<code>PREVIEW_CHANNEL = "grex-preview-v1"</code>) with strict loopback <code>allowedOrigins</code> matching and rejects wildcard <code>"*"</code>.</p>
    </section>

    <section>
      <h2>5. Quickstart Commands</h2>
      <pre>groxy-desk-windows-amd64.exe --workspace C:\\path\\to\\project
groxy-desk-windows-amd64.exe --smoke-test --addr 127.0.0.1:0
groxy-deskd-windows-amd64.exe --addr 127.0.0.1:9490 --workspace C:\\path\\to\\project</pre>
    </section>

    <section>
      <h2>6. 7-Layer Ecosystem Compatibility Matrix</h2>
      <table>
        <thead>
          <tr><th>Ecosystem Layer</th><th>Repository</th><th>Required Range</th><th>Repo Modified?</th></tr>
        </thead>
        <tbody>
          <tr><td>Core Language &amp; Grisp Server</td><td><code>urbanspress/grab</code></td><td><code>&gt;= v1.1.0 (&lt; v2.0.0)</code></td><td>No (Bridged via <code>grab mcp</code> &amp; CLI)</td></tr>
          <tr><td>Reactive Frontend Runtime</td><td><code>urbanspress/grex-js</code></td><td><code>&gt;= v1.0.0 (&lt; v2.0.0)</code></td><td>No (Vendored SHA-256 Parity)</td></tr>
          <tr><td>Flagship Zero-Plugin CMS</td><td><code>urbanspress/grablab-cms</code></td><td><code>&gt;= v1.0.1 (&lt; v2.0.0)</code></td><td>No (Loopback MCP &amp; Draft Bridge)</td></tr>
          <tr><td>Exclusive Visual Theme &amp; Studio</td><td><code>urbanspress/grex-builder</code></td><td><code>&gt;= v1.0.0 (&lt; v2.0.0)</code></td><td>No (Loopback Studio &amp; Undo Parity)</td></tr>
          <tr><td>Autonomous Design &amp; SEO Agent</td><td><code>urbanspress/groxy-ai</code></td><td><code>&gt;= v1.0.0 (&lt; v2.0.0)</code></td><td>No (22-Rule &amp; 8&times;5 Parity Engine)</td></tr>
          <tr><td>Scaffolder, Fleet &amp; VPS Panel</td><td><code>grabman</code> / <code>grabtaculous</code> / <code>gashboard</code></td><td><code>&gt;= v1.0.0 (&lt; v2.0.0)</code></td><td>No (<code>grabman v1.0.0</code> Bridge)</td></tr>
          <tr><td>Native Desktop Vibe-Coding IDE</td><td><code>urbanspress/groxy-desk</code></td><td><code>v{v}</code></td><td>Yes (Step 6 Studio Release)</td></tr>
        </tbody>
      </table>
    </section>
  </main>"""

    return f"""<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>Groxy Desk v{v} — Walkthrough & Architecture Guide</title>
  <style>
    @page {{
      size: Letter;
      margin: 0.7in 0.75in;
    }}
    * {{
      box-sizing: border-box;
    }}
    body {{
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
      color: #0f172a;
      background: #ffffff;
      line-height: 1.55;
      font-size: 10.5pt;
      margin: 0;
    }}
    header {{
      border-bottom: 3px solid #2563eb;
      padding-bottom: 12px;
      margin-bottom: 18px;
    }}
    h1 {{
      margin: 0 0 6px 0;
      font-size: 20pt;
      color: #0f172a;
    }}
    h2 {{
      font-size: 13pt;
      color: #1d4ed8;
      margin: 18px 0 8px 0;
      border-bottom: 1px solid #e2e8f0;
      padding-bottom: 4px;
    }}
    h3 {{
      font-size: 11pt;
      color: #1e293b;
      margin: 12px 0 4px 0;
    }}
    p, li {{
      color: #334155;
    }}
    code, pre {{
      font-family: Consolas, "Cascadia Mono", monospace;
      background: #f1f5f9;
      border: 1px solid #e2e8f0;
      border-radius: 4px;
      color: #0f172a;
    }}
    code {{
      padding: 1px 5px;
      font-size: 9.2pt;
    }}
    pre {{
      padding: 10px 12px;
      font-size: 8.8pt;
      overflow-x: auto;
    }}
    table {{
      width: 100%;
      border-collapse: collapse;
      margin: 10px 0 14px 0;
      font-size: 9.2pt;
    }}
    th, td {{
      border: 1px solid #cbd5e1;
      padding: 6px 8px;
      text-align: left;
      vertical-align: top;
    }}
    th {{
      background: #f8fafc;
      color: #0f172a;
    }}
    section {{
      page-break-inside: avoid;
    }}
    footer {{
      margin-top: 22px;
      border-top: 1px solid #cbd5e1;
      padding-top: 8px;
      font-size: 9pt;
      color: #475569;
    }}
    .candidate-guide {{ line-height: 1.4; }}
    .candidate-guide header {{ padding-bottom: 8px; margin-bottom: 12px; }}
    .candidate-guide h2 {{ margin: 12px 0 6px; }}
    .candidate-guide p, .candidate-guide ol {{ margin: 7px 0; }}
    .candidate-guide code {{ white-space: nowrap; }}
    .candidate-guide footer {{ margin-top: 14px; }}
  </style>
</head>
<body class="{body_class}">
{body_sections}

  <footer>
    <p>{footer_text}</p>
  </footer>
</body>
</html>
"""


def main():
    version = sys.argv[1] if len(sys.argv) > 1 else "1.0.1"
    PDF_DIR.mkdir(parents=True, exist_ok=True)
    pdf_path = PDF_DIR / f"Groxy-Desk-v{version}-Guide.pdf"
    browser = find_browser()
    with tempfile.TemporaryDirectory(prefix="groxy-desk-guide-") as temp:
        staging = Path(temp)
        html_path = (staging if version == "1.1.0" else HTML_DIR) / f"Groxy-Desk-v{version}-Guide.html"
        html_path.parent.mkdir(parents=True, exist_ok=True)
        html_path.write_text(render_guide_html(version), encoding="utf-8")
        staged_pdf = staging / pdf_path.name
        cmd = [
            str(browser),
            "--headless",
            "--disable-gpu",
            "--no-pdf-header-footer",
            f"--user-data-dir={staging / 'profile'}",
            f"--print-to-pdf={staged_pdf}",
            html_path.resolve().as_uri(),
        ]
        res = subprocess.run(cmd, capture_output=True, text=True, timeout=30, check=False)
        if res.returncode != 0 or not staged_pdf.exists() or staged_pdf.stat().st_size < 5000:
            raise RuntimeError(f"Failed rendering PDF ({res.returncode}): {res.stderr}\n{res.stdout}")
        shutil.copy2(staged_pdf, pdf_path)
    print(f"Generated {pdf_path} ({pdf_path.stat().st_size} bytes)")


if __name__ == "__main__":
    main()
