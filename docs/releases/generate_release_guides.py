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
    if version.startswith("0.3."):
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
  </style>
</head>
<body>
{body_sections}

  <footer>
    <p>Groxy Desk v{v} — Verified across Gate 1 (Go + govulncheck + grab check/format), Gate 2 (Pure Grex JS DOM/WCAG/ADR-025c), Gate 3 (GRAB-E401 + GROXY-SAFE-001 + DESK-E403/E409), and Gate 4 (Live E2E).</p>
  </footer>
</body>
</html>
"""


def main():
    version = sys.argv[1] if len(sys.argv) > 1 else "0.1.0"
    HTML_DIR.mkdir(parents=True, exist_ok=True)
    PDF_DIR.mkdir(parents=True, exist_ok=True)

    html_path = HTML_DIR / f"Groxy-Desk-v{version}-Guide.html"
    pdf_path = PDF_DIR / f"Groxy-Desk-v{version}-Guide.pdf"

    html_path.write_text(render_guide_html(version), encoding="utf-8")

    browser = find_browser()
    file_url = html_path.resolve().as_uri()
    cmd = [
        str(browser),
        "--headless",
        "--disable-gpu",
        "--no-pdf-header-footer",
        f"--print-to-pdf={pdf_path}",
        file_url,
    ]
    res = subprocess.run(cmd, capture_output=True, text=True, timeout=30, check=False)
    if res.returncode != 0 or not pdf_path.exists() or pdf_path.stat().st_size < 5000:
        raise RuntimeError(f"Failed rendering PDF ({res.returncode}): {res.stderr}\n{res.stdout}")

    print(f"Generated {html_path}")
    print(f"Generated {pdf_path} ({pdf_path.stat().st_size} bytes)")


if __name__ == "__main__":
    main()
