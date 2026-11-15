#!/usr/bin/env python3
"""
Gate 4 Cross-Layer End-to-End Integration Suite for Groxy Desk v0.1.0.
Verifies compiled binaries + GRAB v1.1.0 + Grex JS v1.0.0 + grabman v1.0.0 + Unified Groxy AI.
"""
import hashlib
import json
import os
import shutil
import socket
import subprocess
import sys
import tempfile
import time
import urllib.error
import urllib.parse
import urllib.request
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
    print("[Gate 4] Verifying 8 peer v1.x repositories are strictly clean...")
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
            assert st == 200, f"/_health failed: {st} {raw_health}"
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

            for asset in ("/public/grex/grex.js", "/public/grex/grex.elements.json", "/public/desk/workbench.js"):
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
