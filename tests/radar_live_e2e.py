#!/usr/bin/env python3
"""Exercise Desk against exact frozen Radar and CMS tags in disposable workspaces."""

import hashlib
import http.cookiejar
import json
import os
from pathlib import Path
import shutil
import socket
import sqlite3
import subprocess
import tempfile
import time
from contextlib import closing, contextmanager
from urllib.error import HTTPError, URLError
from urllib.request import HTTPCookieProcessor, ProxyHandler, Request, build_opener
from zipfile import ZipFile


ROOT = Path(__file__).resolve().parents[1]


def checkout(repo, version, destination):
    archive = destination.with_suffix(".zip")
    subprocess.run(
        ["git", "-C", str(ROOT.parent / repo), "-c", "core.autocrlf=false", "archive", "--format=zip",
         "-o", str(archive), f"refs/tags/{version}"],
        check=True, timeout=60,
    )
    with ZipFile(archive) as source:
        source.extractall(destination)
    archive.unlink()


def loopback_address():
    with socket.socket() as listener:
        listener.bind(("127.0.0.1", 0))
        return f"127.0.0.1:{listener.getsockname()[1]}"


class Client:
    def __init__(self, origin, headers=None):
        self.origin = origin
        self.headers = headers or {}
        self.opener = build_opener(ProxyHandler({}), HTTPCookieProcessor(http.cookiejar.CookieJar()))

    def request(self, path, expected, body=None, omit_csrf=False):
        headers = dict(self.headers)
        if omit_csrf:
            headers.pop("X-CSRF-Token", None)
        data = None
        if body is not None:
            headers["Content-Type"] = "application/json"
            data = json.dumps(body).encode("utf-8")
            # The CSRF guard rejects before reading the JSON. Send an empty
            # POST so Windows cannot reset urllib's closing TCP connection
            # while the client is still sending the rejected body.
            if omit_csrf:
                data = b""
        request = Request(self.origin + path, data=data, headers=headers)
        try:
            response = self.opener.open(request, timeout=20)
        except HTTPError as err:
            response = err
        with response:
            raw = response.read().decode("utf-8")
            assert response.status == expected, f"{path}: wanted {expected}, got {response.status}: {raw[:400]}"
            if response.headers.get_content_type() == "application/json":
                assert "rtok_res_" not in raw or not path.startswith("/api/radar/")
                return json.loads(raw)
            return raw


@contextmanager
def service(args, directory, address, label, env=None):
    client = Client(f"http://{address}")
    log = directory / f"{label}.log"
    with log.open("w", encoding="utf-8") as output:
        process = subprocess.Popen(args, cwd=directory, env=env, stdout=output, stderr=subprocess.STDOUT)
        try:
            deadline = time.monotonic() + 30
            while time.monotonic() < deadline:
                if process.poll() is not None:
                    break
                try:
                    client.request("/_health", 200)
                    break
                except (URLError, AssertionError):
                    time.sleep(0.15)
            else:
                raise RuntimeError(f"{label} did not start: {log.read_text(encoding='utf-8')[-3000:]}")
            if process.poll() is not None:
                raise RuntimeError(f"{label} exited: {log.read_text(encoding='utf-8')[-3000:]}")
            yield client
        finally:
            if process.poll() is None:
                process.terminate()
                try:
                    process.wait(timeout=8)
                except subprocess.TimeoutExpired:
                    process.kill()
                    process.wait(timeout=8)


def start_groxy(grab, root, address):
    (root / "storage").mkdir(exist_ok=True)
    subprocess.run([grab, "migrate", "--db-config", "grab.data.json", "--allow-db", "commerce:migrate",
                    "scripts/migrate_commerce.grab"], cwd=root, check=True, timeout=60)
    variables = {
        "GROXY_OPERATOR_ID": "desk-radar-fixture",
        "GROXY_MASTER_KEY": "0123456789abcdef" * 4,
        "GROXY_WEB_RESEARCH_ENABLED": "false",
    }
    return service([
        grab, "serve", "--addr", address, "--allow-scripts",
        "--db-config", "grab.data.json", "--allow-db", "commerce:read-write",
        "--allow-db", "commerce_inspect:read", "--platform-config", "grab.platform.json",
        "--allow-cache", "groxy:read-write", "--allow-cache", "groxy:limit", "--static", "public",
        "--allow-read", ".", "--allow-write", "storage", "--allow-env", ",".join(variables), "src/groxy.grab",
    ], root, address, "radar", {**os.environ, **variables})


def start_cms(grab, root, address, groxy_address):
    lock = json.loads((root / "deploy" / "grab.connect.lock").read_text(encoding="utf-8"))
    for package, manifest in lock["packages"].items():
        for file, expected in manifest["files"].items():
            assert hashlib.sha256((root / "deploy" / package / file).read_bytes()).hexdigest() == expected, file
    config = root / "desk-fixture-db.json"
    config.write_text(
        json.dumps({"connections": {"main": {"driver": "sqlite", "path": str(root / "desk-fixture.db")}}}),
        encoding="utf-8",
    )
    db = ["--db-config", str(config)]
    platform = ["--platform-config", "deploy/grab.platform.commerce.json"]
    subprocess.run([grab, "migrate", *db, "--allow-db", "main:migrate", "migrations/005_author_schema.grab"],
                   cwd=root, check=True, timeout=60)
    subprocess.run([grab, "migrate", *platform, *db, "--events-store", "commerce",
                    "--allow-db", "main:migrate", "--allow-events", "commerce:work"],
                   cwd=root, check=True, timeout=60)
    for seed in ("seed", "seed_commerce", "seed_author"):
        subprocess.run([grab, "run", *db, "--allow-db", "main:read-write", "--allow-read", ".",
                        f"migrations/{seed}.grab"], cwd=root, check=True, timeout=60)
    uploads = root / "desk-fixture-uploads"
    uploads.mkdir()
    variables = {
        "GRABLAB_UPLOAD_DIR": str(uploads),
        "GRABLAB_MASTER_KEY": "0123456789abcdef" * 4,
        "GRABLAB_PAYMENT_WEBHOOK_SECRET": "desk-fixture-webhook-secret-unused",
    }
    return service([
        grab, "serve", "--addr", address, "--allow-scripts", *db, "--allow-db", "main:read-write",
        *platform, "--allow-cache", "cms:read-write", "--allow-cache", "cms:limit",
        "--allow-env", ",".join(variables), "--allow-net", groxy_address,
        "--allow-read", ".", "--allow-read", str(uploads), "--allow-write", str(uploads),
        "--allow-events", "commerce:accept", "--allow-events", "commerce:work", "--static", "public", "src/app.grab",
    ], root, address, "cms", {**os.environ, **variables})


def enable_cms(cms, radar_origin):
    setup = cms.request("/api/auth/setup", 201, {
        "email": "desk-fixture@example.test", "password": "DeskFixturePassword!0123", "display_name": "Desk Fixture",
    })
    cms.headers["X-CSRF-Token"] = setup["csrf_token"]
    settings = cms.request("/api/admin/author/settings", 200, {
        "author_mode_enabled": True, "groxy_ai_origin": radar_origin,
    })
    assert settings["author_mode_enabled"] is True
    assert "Radar" in cms.request("/admin/author/radar", 200)
    return cms.request("/api/auth/mcp-token", 201, {"label": "Disposable Desk"})["token"]


def exercise_desk(desk, cms_db, radar_db, version):
    def radar(path, expected, body, **kwargs):
        return desk.request("/api/radar/" + path, expected, body, **kwargs)

    research = {"genre_slug": "urban-fiction", "price_tier_usd": "14.99", "include_live_feeds": False}
    radar("research", 422, {**research, "price_tier_usd": 14.99})
    radar("research", 403, research, omit_csrf=True)
    plan = radar("research", 200, research)
    plan_id = plan["radar_plan_id"]
    slug = "desk-acceptance-draft"
    draft = {"radar_plan_id": plan_id, "slug": slug, "title": "Desk Acceptance Draft"}
    radar("draft/plan", 403, draft)
    request = {"radar_plan_id": plan_id, "approval_token": plan["approval_token"]}
    radar("receipt", 403, request, omit_csrf=True)
    receipt = radar("receipt", 201, request)
    assert receipt["status_enforced"] == "draft"
    assert receipt["radar_plan_id"] == plan_id and receipt["human_confirmation_required"] is True
    radar("receipt", 409, request)
    with closing(sqlite3.connect(radar_db)) as ledger:
        state = ledger.execute(
            "SELECT status, target_consumer, target_slot, payload_sha256, outcome_json"
            " FROM groxy_research_ledger WHERE plan_id = ?",
            (plan_id,),
        ).fetchone()
        assert state[:3] == ("applied", "groxy_desk", "desk-copilot"), state[:3]
        digest = hashlib.sha256(f"{plan_id}|{state[3]}".encode()).hexdigest()
        outcome = json.loads(state[4])
        assert outcome["receipt_sha256"] == digest and outcome["status_enforced"] == "draft"
    radar("draft/apply", 403, {**draft, "approval_token": plan["approval_token"]})
    review = radar("draft/plan", 200, draft)
    approved = {**draft, "approval_token": review["approval_token"]}
    radar("draft/apply", 403, approved, omit_csrf=True)
    radar("draft/apply", 403, {**approved, "title": "Unapproved title"})
    result = radar("draft/apply", 201, approved)
    assert result["status_enforced"] == "draft"
    radar("draft/apply", 409, approved)
    with closing(sqlite3.connect(cms_db)) as database:
        entries = database.execute(
            "SELECT e.title, e.status, e.published_at FROM entries e JOIN books b ON b.entry_id = e.id WHERE e.slug = ?",
            (slug,),
        ).fetchall()
        assert entries == [("Desk Acceptance Draft", "draft", None)], entries
    return f"PASS Groxy AI v1.8.0 + GrabLab CMS {version}: Desk-bound receipt and separately approved draft"


@contextmanager
def start_desk(exe, state, variables):
    address = loopback_address()
    state.mkdir()
    session_path = state / "session.json"
    with service([
        str(exe), "--addr", address, "--state-dir", str(state),
        "--workspace", str(state), "--session-file", str(session_path),
    ], state, address, "desk", {**os.environ, **variables}) as desk:
        session = json.loads(session_path.read_text(encoding="utf-8"))
        desk.headers.update({
            "Authorization": "Bearer " + session["bearer_token"],
            "X-CSRF-Token": session["csrf_token"],
        })
        yield desk


def exercise_unavailable_services(exe, root):
    disabled = {
        "GROXY_DESK_ALLOW_LOCAL_SERVICES": "1",
        "GROXY_DESK_RADAR_ORIGIN": "", "GROXY_DESK_RADAR_BEARER": "",
        "GROXY_DESK_CMS_ORIGIN": "", "GROXY_DESK_CMS_MCP_BEARER": "",
    }
    research = {"genre_slug": "urban-fiction", "price_tier_usd": "14.99", "include_live_feeds": False}
    with start_desk(exe, root / "desk-disabled", disabled) as desk:
        result = desk.request("/api/radar/capabilities", 200)
        assert result["research_configured"] is False and result["cms_configured"] is False
        desk.request("/api/radar/research", 503, research)
    with socket.socket() as reserved:
        reserved.bind(("127.0.0.1", 0))
        unreachable = {**disabled, "GROXY_DESK_RADAR_ORIGIN": f"http://127.0.0.1:{reserved.getsockname()[1]}"}
        with start_desk(exe, root / "desk-unreachable", unreachable) as desk:
            desk.request("/api/radar/research", 502, research)
    print("PASS disabled service capabilities and unreachable Radar origin")


def main():
    grab = shutil.which("grab")
    assert grab and subprocess.check_output([grab, "--version"], text=True).split()[:2] == ["GRAB", "1.1.0"]
    with tempfile.TemporaryDirectory(prefix="groxy-desk-live-radar-") as temporary:
        root = Path(temporary)
        exe = root / ("groxy-deskd.exe" if os.name == "nt" else "groxy-deskd")
        subprocess.run(["go", "build", "-o", str(exe), "./cmd/groxy-deskd"], cwd=ROOT, check=True, timeout=90)
        exercise_unavailable_services(exe, root)
        groxy = root / "groxy-ai"
        checkout("groxy-ai", "v1.8.0", groxy)
        groxy_address = loopback_address()
        with start_groxy(grab, groxy, groxy_address) as radar:
            for version in ("v1.4.0", "v1.5.0"):
                cms_root = root / f"cms-{version}"
                checkout("grablab-cms", version, cms_root)
                with start_cms(grab, cms_root, loopback_address(), groxy_address) as cms:
                    token = enable_cms(cms, radar.origin)
                    variables = {
                        "GROXY_DESK_ALLOW_LOCAL_SERVICES": "1",
                        "GROXY_DESK_RADAR_ORIGIN": radar.origin,
                        "GROXY_DESK_RADAR_BEARER": "",
                        "GROXY_DESK_CMS_ORIGIN": cms.origin,
                        "GROXY_DESK_CMS_MCP_BEARER": token,
                    }
                    with start_desk(exe, root / f"desk-{version}", variables) as desk:
                        print(exercise_desk(desk, cms_root / "desk-fixture.db",
                                            groxy / "storage" / "groxy-commerce.db", version))


if __name__ == "__main__":
    main()
