#!/usr/bin/env python3
"""
Gate 4 Cross-Layer End-to-End Integration Suite for Groxy Desk v1.1.0.
Verifies compiled binaries + GRAB v1.1.0 + Grex JS v1.0.0 + GrabLab CMS v1.0.1 + Grex Builder v1.0.0 + grabman v1.0.0 + Unified Groxy AI.
"""
import hashlib
import json
import os
import shutil
import socket
import subprocess
import sys
import tempfile
import threading
import time
import urllib.error
import urllib.parse
import urllib.request
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parent.parent

PEER_REPOS = [
    Path(r"C:\Development\GRAB"),
    Path(r"C:\Development\grex-js"),
    Path(r"C:\Development\grablab-cms"),
    Path(r"C:\Development\grex-builder"),
    Path(r"C:\Development\groxy-ai"),
    Path(r"C:\Development\grabman"),
    Path(r"C:\Development\grabtaculous"),
    Path(r"C:\Development\gashboard"),
    Path(r"C:\Development\grab-ide-icons"),
    Path(r"C:\Development\plotific-press"),
]


def free_loopback_port() -> int:
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
        s.bind(("127.0.0.1", 0))
        return int(s.getsockname()[1])


def assert_peer_repos_clean():
    for repo in PEER_REPOS:
        if not repo.exists():
            continue
        res = subprocess.run(
            ["git", "-C", str(repo), "status", "--short"],
            capture_output=True,
            text=True,
            check=False,
        )
        assert res.returncode == 0, f"git status failed in {repo}: {res.stderr}"
        assert res.stdout.strip() == "", f"Peer repo {repo} has modified files:\n{res.stdout}"


def http_req(method: str, url: str, headers=None, body=None, timeout=15.0):
    req_headers = dict(headers or {})
    data = None
    if body is not None:
        data = json.dumps(body).encode("utf-8")
        req_headers.setdefault("Content-Type", "application/json")
    req = urllib.request.Request(url, data=data, headers=req_headers, method=method)
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            raw = resp.read().decode("utf-8", errors="replace")
            return resp.status, dict(resp.headers), raw
    except urllib.error.HTTPError as e:
        raw = e.read().decode("utf-8", errors="replace")
        return e.code, dict(e.headers), raw


def sha256_hex(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def main():
    print("[Gate 4] Verifying 10 peer v1.x repositories are strictly clean...")
    assert_peer_repos_clean()

    with tempfile.TemporaryDirectory(prefix="groxy-desk-e2e-") as tmp:
        tmp_path = Path(tmp)
        bin_dir = tmp_path / "bin"
        ws_dir = tmp_path / "workspace"
        state_dir = tmp_path / "state"
        bin_dir.mkdir(parents=True, exist_ok=True)
        (ws_dir / "src").mkdir(parents=True, exist_ok=True)
        (ws_dir / "views").mkdir(parents=True, exist_ok=True)
        state_dir.mkdir(parents=True, exist_ok=True)

        deskd_bin = bin_dir / ("groxy-deskd.exe" if os.name == "nt" else "groxy-deskd")
        desk_bin = bin_dir / ("groxy-desk.exe" if os.name == "nt" else "groxy-desk")

        print("[Gate 4] Building CGO_ENABLED=0 groxy-deskd & groxy-desk binaries...")
        env_cgo0 = dict(os.environ)
        env_cgo0["CGO_ENABLED"] = "0"
        subprocess.run(["go", "build", "-o", str(deskd_bin), "./cmd/groxy-deskd"], cwd=REPO_ROOT, env=env_cgo0, check=True)
        subprocess.run(["go", "build", "-o", str(desk_bin), "./cmd/groxy-desk"], cwd=REPO_ROOT, env=env_cgo0, check=True)

        # 1. Smoke test desktop launcher binary
        print("[Gate 4] Running groxy-desk --smoke-test...")
        smoke = subprocess.run(
            [str(desk_bin), "--smoke-test", "--addr", "127.0.0.1:0", "--workspace", str(ws_dir), "--state-dir", str(state_dir)],
            capture_output=True,
            text=True,
            timeout=20,
            check=False,
        )
        assert smoke.returncode == 0, f"desk smoke-test failed: {smoke.stderr}\n{smoke.stdout}"
        assert "groxy-desk smoke-test ok" in smoke.stdout, smoke.stdout

        # 2. Fail-closed non-loopback bind check (DESK-E501)
        print("[Gate 4] Verifying non-loopback bind fail-closed (DESK-E501)...")
        for bad_addr in ("0.0.0.0:9490", "[::]:9490"):
            bad_proc = subprocess.run(
                [str(deskd_bin), "--addr", bad_addr, "--workspace", str(ws_dir), "--state-dir", str(state_dir)],
                capture_output=True,
                text=True,
                timeout=10,
                check=False,
            )
            assert bad_proc.returncode != 0, f"Expected fail-closed exit for {bad_addr}"
            assert "DESK-E501" in (bad_proc.stderr + bad_proc.stdout), f"Missing DESK-E501 for {bad_addr}: {bad_proc.stderr}"

        # Seed workspace + confinement traps
        shutil.copyfile(REPO_ROOT / "src" / "desk.grab", ws_dir / "src" / "desk.grab")
        shutil.copyfile(REPO_ROOT / "views" / "workbench.grab.html", ws_dir / "views" / "workbench.grab.html")
        (ws_dir / ".env").write_text("API_SECRET=supersecretvalue999\n", encoding="utf-8")
        (ws_dir / ".git").mkdir()
        (ws_dir / ".git" / "config").write_text("[core]\n\trepositoryformatversion = 0\n", encoding="utf-8")
        (ws_dir / "dev.key").write_text("PRIVATE KEY CONTENT\n", encoding="utf-8")
        (ws_dir / "id_rsa").write_text("RSA KEY\n", encoding="utf-8")

        # 3. Start live host daemon on free loopback port
        daemon_port = free_loopback_port()
        daemon_addr = f"127.0.0.1:{daemon_port}"
        print(f"[Gate 4] Starting live groxy-deskd on {daemon_addr}...")
        popen_kwargs = {}
        if os.name == "nt":
            popen_kwargs["creationflags"] = subprocess.CREATE_NEW_PROCESS_GROUP
        proc = subprocess.Popen(
            [
                str(deskd_bin),
                "--addr",
                daemon_addr,
                "--workspace",
                str(ws_dir),
                "--state-dir",
                str(state_dir),
                "--emit-session",
            ],
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
            **popen_kwargs,
        )

        try:
            session_line = proc.stdout.readline().strip()
            assert session_line, f"Daemon did not emit session line; stderr={proc.stderr.read()}"
            session = json.loads(session_line)
            base_url = session["url"]
            auth_headers = {
                "Authorization": f"Bearer {session['bearer_token']}",
                "X-CSRF-Token": session["csrf_token"],
            }

            # Verify /_health, /_ready, /, and static bundle
            st, hdrs, raw_health = http_req("GET", f"{base_url}/_health")
            assert st == 200 and json.loads(raw_health)["version"] == "1.1.0", f"/_health failed: {st} {raw_health}"
            assert hdrs.get("X-Frame-Options") == "DENY"
            assert hdrs.get("X-Content-Type-Options") == "nosniff"
            assert "script-src 'self'" in hdrs.get("Content-Security-Policy", "")

            st, _, raw_ready = http_req("GET", f"{base_url}/_ready")
            assert st == 200
            ready_obj = json.loads(raw_ready)
            assert ready_obj["ready"] is True
            assert ready_obj["grab_ready"] is True
            assert ready_obj["grabman_ready"] is True

            st, _, root_html = http_req("GET", f"{base_url}/")
            assert st == 200
            assert session["bearer_token"] not in root_html
            assert session["csrf_token"] not in root_html
            assert "<div" not in root_html.lower()

            for asset in (
                "/public/grex/grex.js",
                "/public/grex/grex.elements.json",
                "/public/desk/workbench.js",
                "/public/desk/studio-panel.js",
                "/public/desk/scorecard-panel.js",
                "/public/desk/fleet-panel.js",
                "/public/desk/db-studio-panel.js",
                "/public/desk/dns-panel.js",
                "/public/desk/author-panel.js",
            ):
                st, _, _ = http_req("GET", f"{base_url}{asset}")
                assert st == 200, f"Missing embedded asset {asset}: {st}"

            # 4. Auth & CSRF enforcement (403 DESK-E403)
            print("[Gate 4] Checking Bearer + CSRF 403 enforcement...")
            for bad_hdr in ({}, {"Authorization": f"Bearer {session['bearer_token']}"}, {"X-CSRF-Token": session["csrf_token"]}):
                st, _, raw = http_req("GET", f"{base_url}/api/workspace/tree?dir=.", headers=bad_hdr)
                assert st == 403 and json.loads(raw)["error"]["code"] == "DESK-E403", f"Expected 403 DESK-E403, got {st} {raw}"

            # 5. GRAB-E401 Confinement & sensitive file exclusion
            print("[Gate 4] Verifying GRAB-E401 confinement and sensitive file exclusions...")
            for sensitive in ("../outside.txt", ".env", ".git/config", "dev.key", "id_rsa"):
                st, _, raw = http_req(
                    "GET",
                    f"{base_url}/api/workspace/read?path={urllib.parse.quote(sensitive)}",
                    headers=auth_headers,
                )
                assert st == 403 and json.loads(raw)["error"]["code"] == "GRAB-E401", f"Expected GRAB-E401 for {sensitive}, got {st} {raw}"

            st, _, raw_tree = http_req("GET", f"{base_url}/api/workspace/tree?dir=.", headers=auth_headers)
            assert st == 200
            names = {e["name"] for e in json.loads(raw_tree)["entries"]}
            for forbidden in (".env", ".git", "dev.key", "id_rsa"):
                assert forbidden not in names, f"Sensitive item {forbidden} exposed in tree"

            # 6. grabman v1.0.0 Init Plan -> Target Mismatch -> Execute -> Replay 409 -> Doctor -> Solve Verify
            print("[Gate 4] Testing live grabman v1.0.0 workflow...")
            st, _, raw_plan = http_req(
                "POST",
                f"{base_url}/api/grabman/init/plan",
                headers=auth_headers,
                body={"target_dir": "demo-app", "profile": "app", "project_name": "Demo App", "edge_proxy": "caddy"},
            )
            assert st == 200, raw_plan
            init_tok = json.loads(raw_plan)["approval_token"]

            # Target mismatch must 403
            st, _, raw_mismatch = http_req(
                "POST",
                f"{base_url}/api/grabman/init",
                headers=auth_headers,
                body={"approval_token": init_tok, "target_dir": "wrong-app", "profile": "app"},
            )
            assert st == 403 and json.loads(raw_mismatch)["error"]["code"] == "DESK-E403", raw_mismatch

            # Rightful init must 200
            st, _, raw_init = http_req(
                "POST",
                f"{base_url}/api/grabman/init",
                headers=auth_headers,
                body={"approval_token": init_tok, "target_dir": "demo-app", "profile": "app", "project_name": "Demo App", "edge_proxy": "caddy"},
            )
            assert st == 200 and json.loads(raw_init)["ok"] is True, raw_init

            # Replay must 409 DESK-E409
            st, _, raw_replay = http_req(
                "POST",
                f"{base_url}/api/grabman/init",
                headers=auth_headers,
                body={"approval_token": init_tok, "target_dir": "demo-app", "profile": "app"},
            )
            assert st == 409 and json.loads(raw_replay)["error"]["code"] == "DESK-E409", raw_replay

            st, _, raw_doc = http_req("POST", f"{base_url}/api/grabman/doctor", headers=auth_headers, body={"target_dir": "demo-app"})
            assert st == 200 and "schema_version" in json.loads(raw_doc)["report"], raw_doc

            st, _, raw_solve = http_req("POST", f"{base_url}/api/grabman/solve-verify", headers=auth_headers, body={"target_dir": "demo-app"})
            assert st == 200 and json.loads(raw_solve)["ok"] is True, raw_solve

            # 7. grab check & grab format parity
            print("[Gate 4] Testing live grab check & grab format parity...")
            st, _, raw_chk = http_req(
                "POST",
                f"{base_url}/api/workspace/check",
                headers=auth_headers,
                body={"grab_file": "src/desk.grab", "template_file": "views/workbench.grab.html"},
            )
            chk_obj = json.loads(raw_chk)
            assert st == 200 and chk_obj["ok"] is True and chk_obj["format_clean"] is True, raw_chk

            # 8. Live Grisp (grab.exe v1.1.0) multi-worker lifecycle
            print("[Gate 4] Testing live Grisp worker start, /_health, reload, redacted logs, and stop...")
            st, _, raw_start = http_req(
                "POST",
                f"{base_url}/api/grisp/workers/start",
                headers=auth_headers,
                body={"id": "e2e-worker", "name": "E2E Worker", "subdir": ".", "grab_file": "src/desk.grab", "template_file": "views/workbench.grab.html"},
            )
            assert st == 200, f"startWorker failed: {st} {raw_start}"
            w = json.loads(raw_start)
            assert isinstance(w["port"], str) and 8765 <= int(w["port"]) <= 8772, w
            assert isinstance(w["pid"], str) and int(w["pid"]) > 0, w
            assert w["health_ok"] is True and w["ready_ok"] is True, w

            wst, _, _ = http_req("GET", f"{w['url']}/_health")
            assert wst == 200

            st, _, raw_reload = http_req(
                "POST",
                f"{base_url}/api/grisp/workers/reload",
                headers=auth_headers,
                body={"id": "e2e-worker"},
            )
            assert st == 200 and json.loads(raw_reload)["reload_count"] == "1", raw_reload
            wst2, _, _ = http_req("GET", f"{w['url']}/_health")
            assert wst2 == 200

            st, _, raw_logs = http_req("GET", f"{base_url}/api/grisp/workers/logs?id=e2e-worker", headers=auth_headers)
            assert st == 200 and session["bearer_token"] not in raw_logs and session["csrf_token"] not in raw_logs

            st, _, _ = http_req("POST", f"{base_url}/api/grisp/workers/stop", headers=auth_headers, body={"id": "e2e-worker"})
            assert st == 200
            time.sleep(0.3)

            # 9. Unified Groxy AI Fast Audit + Deep Gated Mutation + Mismatch 403 + Apply + Replay 409 + 1-Click Rollback
            print("[Gate 4] Testing Unified Groxy AI Adaptive Router and GROXY-SAFE-001 rollback...")
            st, _, raw_fast = http_req(
                "POST",
                f"{base_url}/api/groxy/ask",
                headers=auth_headers,
                body={"prompt": "Run deterministic 22-rule audit", "target_path": "views/workbench.grab.html"},
            )
            fast_obj = json.loads(raw_fast)
            assert st == 200 and fast_obj["agent"] == "groxy-ai" and fast_obj["model_switching_allowed"] is False
            assert fast_obj["reasoning_mode"] == "fast" and fast_obj["approval_required"] is False
            assert fast_obj["audit"]["score"] == "100" and fast_obj["audit"]["div_count"] == "0", fast_obj

            target_disk = ws_dir / "views" / "workbench.grab.html"
            before_sha = sha256_hex(target_disk)
            mutated_html = target_disk.read_text(encoding="utf-8").replace(
                "Native Desktop Vibe-Coding IDE", "Verified Updated Title"
            )

            st, _, raw_deep = http_req(
                "POST",
                f"{base_url}/api/groxy/ask",
                headers=auth_headers,
                body={
                    "prompt": "Update page title safely",
                    "target_path": "views/workbench.grab.html",
                    "proposed_content": mutated_html,
                },
            )
            deep_obj = json.loads(raw_deep)
            assert st == 200 and deep_obj["reasoning_mode"] == "deep" and deep_obj["approval_required"] is True
            appr_tok = deep_obj["approval_token"]
            snap_id = deep_obj["snapshot"]["snapshot_id"]

            # Target mismatch -> 403
            st, _, raw_mm = http_req(
                "POST",
                f"{base_url}/api/groxy/approvals/apply",
                headers=auth_headers,
                body={"approval_token": appr_tok, "target_path": "src/desk.grab"},
            )
            assert st == 403 and json.loads(raw_mm)["error"]["code"] == "DESK-E403"

            # Valid apply -> 200
            st, _, raw_ap = http_req(
                "POST",
                f"{base_url}/api/groxy/approvals/apply",
                headers=auth_headers,
                body={"approval_token": appr_tok, "target_path": "views/workbench.grab.html"},
            )
            assert st == 200 and json.loads(raw_ap)["ok"] is True
            assert sha256_hex(target_disk) != before_sha

            # Replay -> 409 DESK-E409
            st, _, raw_rp = http_req(
                "POST",
                f"{base_url}/api/groxy/approvals/apply",
                headers=auth_headers,
                body={"approval_token": appr_tok, "target_path": "views/workbench.grab.html"},
            )
            assert st == 409 and json.loads(raw_rp)["error"]["code"] == "DESK-E409"

            # 1-Click Snapshot Restore -> exact original SHA-256
            st, _, raw_rst = http_req(
                "POST",
                f"{base_url}/api/groxy/snapshots/restore",
                headers=auth_headers,
                body={"snapshot_id": snap_id},
            )
            assert st == 200 and json.loads(raw_rst)["restored_sha256"] == before_sha
            assert sha256_hex(target_disk) == before_sha

            # 10. Theme persistence roundtrip
            st, _, _ = http_req("POST", f"{base_url}/api/preferences/theme", headers=auth_headers, body={"theme": "dark"})
            assert st == 200
            st, _, dark_html = http_req("GET", f"{base_url}/")
            assert st == 200 and 'data-grex-theme="dark"' in dark_html

            # 11. v0.2.0 Core grab mcp, Loopback GrabLab CMS Draft + 422, Grex Builder applyAiLayout, 22-Rule Scorecard & Boost
            print("[Gate 4] Testing v0.2.0 grab mcp, CMS draft, Theme Builder, and Let Groxy Boost This lifecycle...")
            st, _, raw_tools = http_req("GET", f"{base_url}/api/mcp/tools", headers=auth_headers)
            tools_obj = json.loads(raw_tools)
            assert st == 200 and len(tools_obj["tools"]) == 13 and tools_obj["model_switching_allowed"] is False

            # Core GRAB v1.1.0 MCP read-only calls
            for tname, targs in (
                ("grab_check", {"file": "src/desk.grab"}),
                ("grab_format", {"file": "src/desk.grab"}),
                ("grab_inspect_routes", {"file": "src/desk.grab"}),
                ("grab_inspect_schema", {}),
            ):
                st, _, raw_mcp = http_req("POST", f"{base_url}/api/mcp/call", headers=auth_headers, body={"tool": tname, "arguments": targs})
                mcp_obj = json.loads(raw_mcp)
                assert st == 200 and mcp_obj["read_only"] is True and mcp_obj["allow_write_flag"] is False, raw_mcp

            # Start local loopback simulated GrabLab CMS v1.0.1 + Grex Builder v1.0.0 worker on 127.0.0.1:0
            cms_records = []

            class LoopbackWorkerHandler(BaseHTTPRequestHandler):
                def log_message(self, fmt, *args):
                    return

                def do_GET(self):
                    if self.path == "/api/design-system.md":
                        md = "# GRAB-DESIGN-SYSTEM.md\n- --grex-bg: #ffffff\n- --grex-text: #0f172a\n- --grex-primary: #047857\n- Classes: .hero, .card, .btn-primary\n"
                        self.send_response(200)
                        self.send_header("Content-Type", "text/markdown; charset=utf-8")
                        self.end_headers()
                        self.wfile.write(md.encode("utf-8"))
                        return
                    self.send_response(404)
                    self.end_headers()

                def do_POST(self):
                    length = int(self.headers.get("Content-Length", "0"))
                    payload = json.loads(self.rfile.read(length).decode("utf-8") or "{}")
                    if self.path == "/api/mcp/tools/create_entry_draft":
                        args = payload.get("arguments", {})
                        cms_records.append(args)
                        self.send_response(201)
                        self.send_header("Content-Type", "application/json")
                        self.end_headers()
                        self.wfile.write(json.dumps({
                            "status_enforced": "draft",
                            "entry": {"id": "77", "slug": args.get("slug"), "status": args.get("status")},
                        }).encode("utf-8"))
                        return
                    if self.path == "/api/mcp/tools/list_content_types":
                        self.send_response(200)
                        self.send_header("Content-Type", "application/json")
                        self.end_headers()
                        self.wfile.write(json.dumps({"content_types": [{"slug": "post"}, {"slug": "page"}]}).encode("utf-8"))
                        return
                    if self.path == "/api/mcp/layout":
                        assert payload.get("actor") == "groxy-ai"
                        self.send_response(200)
                        self.send_header("Content-Type", "application/json")
                        self.end_headers()
                        self.wfile.write(json.dumps({
                            "ok": True,
                            "actor": "groxy-ai",
                            "slot": payload.get("slot", "main"),
                            "command": {
                                "type": "applyAiLayout",
                                "actor": "groxy-ai",
                                "slot": payload.get("slot", "main"),
                                "inverse": {"_restore": True, "slot": payload.get("slot", "main")},
                            },
                        }).encode("utf-8"))
                        return
                    self.send_response(404)
                    self.end_headers()

            peer_srv = ThreadingHTTPServer(("127.0.0.1", 0), LoopbackWorkerHandler)
            peer_port = peer_srv.server_address[1]
            peer_url = f"http://127.0.0.1:{peer_port}"
            peer_thread = threading.Thread(target=peer_srv.serve_forever, daemon=True)
            peer_thread.start()
            try:
                # Non-loopback host rejected DESK-E501
                st, _, raw_ssrf = http_req(
                    "POST",
                    f"{base_url}/api/mcp/call",
                    headers=auth_headers,
                    body={"tool": "list_content_types", "cms_url": "http://example.com:8080"},
                )
                assert st in (403, 500) and json.loads(raw_ssrf)["error"]["code"] == "DESK-E501", raw_ssrf

                # Studio Design System + Synthesize (exact ADR-025c price="49.95")
                st, _, raw_ds = http_req("GET", f"{base_url}/api/studio/design-system?studio_url={urllib.parse.quote(peer_url)}", headers=auth_headers)
                assert st == 200 and json.loads(raw_ds)["combination_count"] == "40"

                st, _, raw_syn = http_req(
                    "POST",
                    f"{base_url}/api/studio/synthesize",
                    headers=auth_headers,
                    body={
                        "style_pack": "emerald-editorial",
                        "blueprint": "pricing_grid",
                        "title": "E2E Semantic Pricing Layout",
                        "custom_fields": {"price": "49.95", "sku": "10055"},
                    },
                )
                syn_obj = json.loads(raw_syn)
                assert st == 200 and syn_obj["contrast"]["meets_wcag_aa"] is True
                assert syn_obj["scorecard"]["overall_grade"] in ("A", "A+")
                assert syn_obj["custom_fields"]["price"] == "49.95"

                # Invalid div tree on update_entry_layout -> 422 / -32602
                st, _, raw_bad_tree = http_req(
                    "POST",
                    f"{base_url}/api/mcp/call",
                    headers=auth_headers,
                    body={
                        "tool": "update_entry_layout",
                        "cms_url": peer_url,
                        "arguments": {"entry_id": "77", "layout": {"element": "Section", "tag": "div"}},
                    },
                )
                assert st == 422 and json.loads(raw_bad_tree)["mcp_error"]["code"] == -32602

                # GrabLab CMS create_entry_draft even when requesting published -> forces draft
                draft_args = {
                    "content_type": "post",
                    "slug": "e2e-semantic-draft",
                    "title": "E2E Semantic Draft",
                    "status": "published",
                    "grex_layout": syn_obj["grex_layout"],
                }
                st, _, raw_dp = http_req(
                    "POST",
                    f"{base_url}/api/mcp/call",
                    headers=auth_headers,
                    body={"tool": "create_entry_draft", "cms_url": peer_url, "arguments": draft_args, "prepare": True},
                )
                draft_tok = json.loads(raw_dp)["approval_token"]
                st, _, raw_da = http_req(
                    "POST",
                    f"{base_url}/api/mcp/call",
                    headers=auth_headers,
                    body={"tool": "create_entry_draft", "cms_url": peer_url, "arguments": draft_args, "approval_token": draft_tok},
                )
                assert st == 200 and json.loads(raw_da)["status_enforced"] == "draft" and cms_records[0]["status"] == "draft"

                # Studio Layout Apply -> applyAiLayout + Ctrl+Z undo inverse + 409 replay rejection
                st, _, raw_sp = http_req(
                    "POST",
                    f"{base_url}/api/studio/layout/apply",
                    headers=auth_headers,
                    body={"slot": "main", "studio_url": peer_url, "grex_layout": syn_obj["grex_layout"], "prepare": True},
                )
                studio_tok = json.loads(raw_sp)["approval_token"]
                st, _, raw_sa = http_req(
                    "POST",
                    f"{base_url}/api/studio/layout/apply",
                    headers=auth_headers,
                    body={"slot": "main", "studio_url": peer_url, "grex_layout": syn_obj["grex_layout"], "approval_token": studio_tok},
                )
                sa_obj = json.loads(raw_sa)
                assert st == 200 and sa_obj["command"]["type"] == "applyAiLayout" and sa_obj["command"]["actor"] == "groxy-ai"
                st, _, raw_sr = http_req(
                    "POST",
                    f"{base_url}/api/studio/layout/apply",
                    headers=auth_headers,
                    body={"slot": "main", "studio_url": peer_url, "grex_layout": syn_obj["grex_layout"], "approval_token": studio_tok},
                )
                assert st == 409 and json.loads(raw_sr)["error"]["code"] == "DESK-E409"

                # 22-Rule Audit (F) -> Let Groxy Boost This -> Approve (A/A+) -> Replay 409 -> Snapshot Rollback (exact SHA)
                boost_file = ws_dir / "views" / "boost_e2e.grab.html"
                boost_file.write_text(
                    '<html><head><title>Bad</title></head><body><div onclick="x()"><a href="javascript:void(0)">click here</a><img src="/a.png" /><h1>A</h1><h4>Skip</h4></div></body></html>',
                    encoding="utf-8",
                )
                boost_orig_sha = sha256_hex(boost_file)

                st, _, raw_au = http_req("POST", f"{base_url}/api/groxy/audit", headers=auth_headers, body={"target_path": "views/boost_e2e.grab.html"})
                assert st == 200 and json.loads(raw_au)["overall_grade"] == "F" and json.loads(raw_au)["publish_safe"] is False

                st, _, raw_bp = http_req(
                    "POST",
                    f"{base_url}/api/groxy/boost",
                    headers=auth_headers,
                    body={"target_path": "views/boost_e2e.grab.html", "style_pack": "emerald-editorial"},
                )
                bp_obj = json.loads(raw_bp)
                assert st == 200 and bp_obj["proposal"]["projected_delta"]["before_grade"] == "F"
                assert bp_obj["proposal"]["projected_delta"]["after_grade"] in ("A", "A+")
                boost_tok = bp_obj["approval_token"]
                boost_snap_id = bp_obj["snapshot"]["snapshot_id"]

                st, _, raw_ba = http_req(
                    "POST",
                    f"{base_url}/api/groxy/boost/approve",
                    headers=auth_headers,
                    body={"approval_token": boost_tok, "target_path": "views/boost_e2e.grab.html"},
                )
                assert st == 200 and json.loads(raw_ba)["ok"] is True
                assert "<div" not in boost_file.read_text(encoding="utf-8").lower()

                st, _, raw_br = http_req(
                    "POST",
                    f"{base_url}/api/groxy/boost/approve",
                    headers=auth_headers,
                    body={"approval_token": boost_tok, "target_path": "views/boost_e2e.grab.html"},
                )
                assert st == 409 and json.loads(raw_br)["error"]["code"] == "DESK-E409"

                st, _, raw_brst = http_req(
                    "POST",
                    f"{base_url}/api/groxy/snapshots/restore",
                    headers=auth_headers,
                    body={"snapshot_id": boost_snap_id},
                )
                assert st == 200 and json.loads(raw_brst)["restored_sha256"] == boost_orig_sha
                assert sha256_hex(boost_file) == boost_orig_sha
            finally:
                peer_srv.shutdown()
                peer_srv.server_close()

            # 8. v0.3.0 End-to-End Integration: Fleet, SSH Tunnel + SQLite DB Explorer, Namecheap DNS/HTTPS, Author Mode & Ollama
            print("[Gate 4] Testing v0.3.0 Fleet, SSH Tunnel + DB Explorer, Namecheap DNS/HTTPS & Author Mode...")
            import sqlite3
            import socket
            from urllib.parse import parse_qs

            daemon_calls = {"deploy": 0, "rollout": 0, "rollback": 0, "set_hosts": 0}

            class MockInfraHandler(BaseHTTPRequestHandler):
                def do_GET(self):
                    if self.path in ("/_health", "/_ready"):
                        self.send_response(200)
                        self.send_header("Content-Type", "application/json")
                        self.end_headers()
                        self.wfile.write(b'{"ok":true,"service":"grabtaculousd"}')
                        return
                    if self.path == "/api/tags":
                        self.send_response(200)
                        self.send_header("Content-Type", "application/json")
                        self.end_headers()
                        self.wfile.write(b'{"models":[{"name":"qwen2.5-coder:7b"}]}')
                        return
                    self.send_response(404)
                    self.end_headers()

                def do_POST(self):
                    length = int(self.headers.get("Content-Length", "0"))
                    body = self.rfile.read(length).decode("utf-8", errors="replace")
                    if self.path in ("/v1/deploy", "/v1/rollout", "/v1/rollback"):
                        key = self.path.split("/")[-1]
                        daemon_calls[key] += 1
                        self.send_response(200)
                        self.send_header("Content-Type", "application/json")
                        self.end_headers()
                        self.wfile.write(b'{"ok":true,"status":"complete"}')
                        return
                    if self.path == "/namecheap":
                        form = parse_qs(body)
                        cmd = form.get("Command", [""])[0]
                        if cmd == "namecheap.domains.dns.getHosts":
                            xml_resp = (
                                '<?xml version="1.0"?><ApiResponse Status="OK"><Errors/>'
                                '<CommandResponse Type="namecheap.domains.dns.getHosts">'
                                '<DomainDNSGetHostsResult Domain="example.org">'
                                '<host HostId="1" Name="@" Type="A" Address="192.0.2.10" TTL="300"/>'
                                '<host HostId="2" Name="@" Type="MX" Address="mx1.fastmail.com." MXPref="10" TTL="1800"/>'
                                '<host HostId="3" Name="@" Type="TXT" Address="v=spf1 include:spf.messagingengine.com ~all" TTL="1800"/>'
                                '</DomainDNSGetHostsResult></CommandResponse></ApiResponse>'
                            ).encode("utf-8")
                        else:
                            daemon_calls["set_hosts"] += 1
                            assert "mx1.fastmail.com." in body
                            xml_resp = b'<?xml version="1.0"?><ApiResponse Status="OK"><Errors/><CommandResponse Type="namecheap.domains.dns.setHosts"><DomainDNSSetHostsResult Domain="example.org" IsSuccess="true"/></CommandResponse></ApiResponse>'
                        self.send_response(200)
                        self.send_header("Content-Type", "application/xml")
                        self.end_headers()
                        self.wfile.write(xml_resp)
                        return
                    self.send_response(404)
                    self.end_headers()

                def log_message(self, format, *args):
                    pass

            infra_srv = ThreadingHTTPServer(("127.0.0.1", 0), MockInfraHandler)
            infra_url = f"http://127.0.0.1:{infra_srv.server_address[1]}"
            threading.Thread(target=infra_srv.serve_forever, daemon=True).start()

            echo_sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
            echo_sock.bind(("127.0.0.1", 0))
            echo_sock.listen(4)
            echo_stop = threading.Event()

            def run_echo():
                echo_sock.settimeout(0.2)
                while not echo_stop.is_set():
                    try:
                        conn, _ = echo_sock.accept()
                    except OSError:
                        continue
                    with conn:
                        data = conn.recv(64)
                        if data:
                            conn.sendall(data)

            threading.Thread(target=run_echo, daemon=True).start()

            try:
                fp_good = "SHA256:u4wF3L1N8n0vM9kP2xR5tQ7yW8zB3cV6eD1aG4jK9sM"
                fp_bad = "SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"

                # Fleet SSH pin enforcement + Site DNA + Plan/Apply/Replay/Rollback
                st, _, raw = http_req(
                    "POST", f"{base_url}/api/fleet/nodes/verify", headers=auth_headers,
                    body={"node_id": "prod-1", "host": "127.0.0.1", "port": "22", "pinned_fingerprint": fp_good, "presented_fingerprint": fp_bad},
                )
                assert st == 403 and json.loads(raw)["error"]["code"] == "DESK-E405"

                st, _, raw = http_req(
                    "POST", f"{base_url}/api/fleet/nodes/verify", headers=auth_headers,
                    body={"node_id": "prod-1", "host": "127.0.0.1", "port": "22", "pinned_fingerprint": fp_good, "presented_fingerprint": fp_good, "control_url": infra_url, "control_token": "tok-e2e-1", "require_daemon_health": True},
                )
                assert st == 200 and json.loads(raw)["ssh_pin_verified"] is True

                st, _, raw = http_req(
                    "POST", f"{base_url}/api/fleet/dna/validate", headers=auth_headers,
                    body={"raw_json": '{"schema_version":"groxy-site-dna-v0.4.0","html":"<div>bad</div>"}'},
                )
                assert st == 422 and json.loads(raw)["error"]["code"] == "DESK-E302"

                st, _, raw_fp = http_req(
                    "POST", f"{base_url}/api/fleet/deploy/plan", headers=auth_headers,
                    body={"node_id": "prod-1", "app_name": "cms-e2e", "blueprint_id": "grablab-cms-sqlite", "site_dna_raw": '{"schema_version":"groxy-site-dna-v0.4.0","site_name":"OK"}'},
                )
                fp_obj = json.loads(raw_fp)
                assert st == 200 and fp_obj["snapshot"]["snapshot_id"].startswith("snap_deploy_")
                fl_tok = fp_obj["approval_token"]

                st, _, raw_fa = http_req(
                    "POST", f"{base_url}/api/fleet/deploy/apply", headers=auth_headers,
                    body={"node_id": "prod-1", "app_name": "cms-e2e", "approval_token": fl_tok, "require_daemon": True},
                )
                assert st == 200 and json.loads(raw_fa)["daemon_dispatched"] is True
                assert daemon_calls["deploy"] == 1 and daemon_calls["rollout"] == 1

                st, _, raw_frp = http_req(
                    "POST", f"{base_url}/api/fleet/deploy/apply", headers=auth_headers,
                    body={"node_id": "prod-1", "app_name": "cms-e2e", "approval_token": fl_tok},
                )
                assert st == 409 and json.loads(raw_frp)["error"]["code"] == "DESK-E409"

                st, _, raw_frb = http_req(
                    "POST", f"{base_url}/api/fleet/snapshots/restore", headers=auth_headers,
                    body={"snapshot_id": fp_obj["snapshot"]["snapshot_id"], "control_url": infra_url, "control_token": "tok-e2e-1"},
                )
                assert st == 200 and daemon_calls["rollback"] == 1

                # Production HTTP must reject the internal loopback test transport.
                echo_port = str(echo_sock.getsockname()[1])
                st, _, raw_to = http_req(
                    "POST", f"{base_url}/api/db/tunnels/open", headers=auth_headers,
                    body={"engine": "postgres", "pinned_fingerprint": fp_good, "presented_fingerprint": fp_good, "remote_db_host": "127.0.0.1", "remote_db_port": echo_port, "transport": "loopback_test"},
                )
                assert st == 403, raw_to

                # DB Studio SQLite confinement, ADR-025c rows, DESK-E406, mutation + restore
                e2e_db = ws_dir / "data" / "e2e_books.db"
                e2e_db.parent.mkdir(parents=True, exist_ok=True)
                conn = sqlite3.connect(str(e2e_db))
                try:
                    conn.execute("CREATE TABLE books (id INTEGER PRIMARY KEY, isbn TEXT, price TEXT, word_count INTEGER)")
                    conn.execute("INSERT INTO books VALUES (1, '9780143127550', '19.99', 9007199254740993)")
                    conn.commit()
                finally:
                    conn.close()

                db_target = {"engine": "sqlite", "sqlite_path": "data/e2e_books.db"}
                st, _, raw_rows = http_req("POST", f"{base_url}/api/db/rows", headers=auth_headers, body={"target": db_target, "table": "books", "page": "1", "limit": "50"})
                rows_obj = json.loads(raw_rows)
                assert st == 200 and rows_obj["rows"][0]["word_count"] == "9007199254740993" and rows_obj["rows"][0]["price"] == "19.99"

                st, _, raw_ro = http_req("POST", f"{base_url}/api/db/query", headers=auth_headers, body={"target": db_target, "sql": "DELETE FROM books"})
                assert st == 403 and json.loads(raw_ro)["error"]["code"] == "DESK-E406"

                st, _, raw_dp = http_req("POST", f"{base_url}/api/db/mutate/plan", headers=auth_headers, body={"target": db_target, "sql": "UPDATE books SET price='24.00' WHERE id=1;"})
                dp_obj = json.loads(raw_dp)
                assert st == 200 and dp_obj["snapshot"]["snapshot_id"].startswith("snap_db_")
                st, _, raw_apply = http_req("POST", f"{base_url}/api/db/mutate/apply", headers=auth_headers, body={"target": db_target, "sql": "UPDATE books SET price='24.00' WHERE id=1;", "approval_token": dp_obj["approval_token"]})
                assert st == 201, raw_apply
                restore = {"target": db_target, "snapshot_id": dp_obj["snapshot"]["snapshot_id"]}
                st, _, raw_restore_plan = http_req("POST", f"{base_url}/api/db/snapshots/restore/plan", headers=auth_headers, body=restore)
                assert st == 200, raw_restore_plan
                restore["approval_token"] = json.loads(raw_restore_plan)["approval_token"]
                st, _, raw_restore = http_req("POST", f"{base_url}/api/db/snapshots/restore", headers=auth_headers, body=restore)
                assert st == 201, raw_restore
                st, _, raw_rows2 = http_req("POST", f"{base_url}/api/db/rows", headers=auth_headers, body={"target": db_target, "table": "books", "page": "1", "limit": "50"})
                assert json.loads(raw_rows2)["rows"][0]["price"] == "19.99"

                # Namecheap Vault + MX-safe Plan + Apply + DNS Propagation Preflight (422 DESK-E304) + Auto-HTTPS + Rollback
                st, _, raw_vs = http_req("POST", f"{base_url}/api/dns/vault/save", headers=auth_headers, body={"api_user": "e2e", "api_key": "nc-key-e2e-4321", "client_ip": "198.51.100.1", "endpoint_override": f"{infra_url}/namecheap"})
                assert st == 200 and "nc-key-e2e-4321" not in raw_vs and json.loads(raw_vs)["api_key_masked"] == "nc_****...4321"

                st, _, raw_drop = http_req("POST", f"{base_url}/api/dns/plan", headers=auth_headers, body={"domain": "example.org", "target_ip": "203.0.113.55", "simulated_drop_mx": True})
                assert st == 422 and json.loads(raw_drop)["error"]["code"] == "DESK-E303"

                st, _, raw_dnsp = http_req("POST", f"{base_url}/api/dns/plan", headers=auth_headers, body={"domain": "example.org", "target_ip": "203.0.113.55", "configure_www": True})
                dnsp_obj = json.loads(raw_dnsp)
                assert st == 200 and dnsp_obj["preserved_email_count"] == "2"

                st, _, _ = http_req("POST", f"{base_url}/api/dns/apply", headers=auth_headers, body={"domain": "example.org", "approval_token": dnsp_obj["approval_token"], "proposed_records": dnsp_obj["proposed_records"]})
                assert st == 200 and daemon_calls["set_hosts"] == 1

                st, _, raw_hp_bad = http_req("POST", f"{base_url}/api/dns/https/preflight", headers=auth_headers, body={"domain": "example.org", "expected_ip": "203.0.113.55", "edge_proxy": "caddy", "upstream_port": "8765", "observed_ips": ["192.0.2.10"]})
                assert st == 422 and json.loads(raw_hp_bad)["error"]["code"] == "DESK-E304"

                st, _, raw_hp_ok = http_req("POST", f"{base_url}/api/dns/https/preflight", headers=auth_headers, body={"domain": "example.org", "expected_ip": "203.0.113.55", "edge_proxy": "caddy", "upstream_port": "8765", "observed_ips": ["203.0.113.55"]})
                hp_ok = json.loads(raw_hp_ok)
                assert st == 200 and hp_ok["propagated"] is True
                st, _, raw_ha = http_req("POST", f"{base_url}/api/dns/https/apply", headers=auth_headers, body={"domain": "example.org", "expected_ip": "203.0.113.55", "edge_proxy": "caddy", "upstream_port": "8765", "approval_token": hp_ok["approval_token"], "observed_ips": ["203.0.113.55"]})
                assert st == 200 and "reverse_proxy 127.0.0.1:8765" in json.loads(raw_ha)["rendered_config"]

                # Author Mode Plan + Apply + Loopback Ollama check
                st, _, raw_ap = http_req("POST", f"{base_url}/api/author/init/plan", headers=auth_headers, body={"target_subdir": "e2e-author", "book_title": "Glass Citadel", "author_name": "Ada", "isbn_13": "9780143127550", "price_usd": "19.99", "word_count": "9007199254740993"})
                ap_obj = json.loads(raw_ap)
                assert st == 200 and '"@type":"Book"' in ap_obj["schema_org_jsonld"] and "<div" not in ap_obj["preview_landing_html"].lower()

                st, _, raw_aa = http_req("POST", f"{base_url}/api/author/init/apply", headers=auth_headers, body={"target_subdir": "e2e-author", "book_title": "Glass Citadel", "author_name": "Ada", "isbn_13": "9780143127550", "price_usd": "19.99", "word_count": "9007199254740993", "approval_token": ap_obj["approval_token"]})
                assert st == 200 and json.loads(raw_aa)["status_enforced"] == "draft"
                assert (ws_dir / "e2e-author" / "SITE-CONTEXT.md").exists()

                st, _, raw_ol = http_req("GET", f"{base_url}/api/author/ollama/status?url={infra_url}", headers=auth_headers)
                ol_obj = json.loads(raw_ol)
                assert st == 200 and ol_obj["running"] is True and ol_obj["model_switching_allowed"] is False
            finally:
                echo_stop.set()
                echo_sock.close()
                infra_srv.shutdown()
                infra_srv.server_close()

            print("[Gate 4] All E2E assertions passed; terminating host daemon cleanly...")
        finally:
            if proc.poll() is None:
                proc.terminate()
                try:
                    proc.wait(timeout=5)
                except subprocess.TimeoutExpired:
                    proc.kill()
                    proc.wait(timeout=5)

    assert_peer_repos_clean()
    print("[Gate 4] SUCCESS — 100% pass across all cross-layer integration and isolation checks.")


if __name__ == "__main__":
    main()
