#!/usr/bin/env python3
"""
Build static CGO_ENABLED=0 release binaries, Walkthrough PDF, release ZIP bundle, and SHA256SUMS.txt in dist/.
Usage: python scripts/package_release.py --version 0.1.0
"""
import argparse
import hashlib
import os
import shutil
import subprocess
import sys
import zipfile
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
DIST_DIR = ROOT / "dist"


def build_binary(goos: str, goarch: str, pkg: str, out_path: Path):
    env = dict(os.environ)
    env["CGO_ENABLED"] = "0"
    env["GOOS"] = goos
    env["GOARCH"] = goarch
    cmd = [
        "go",
        "build",
        "-trimpath",
        "-ldflags=-s -w",
        "-o",
        str(out_path),
        pkg,
    ]
    print(f"Building {out_path.name} ({goos}/{goarch})...")
    subprocess.run(cmd, cwd=ROOT, env=env, check=True)


def sha256_file(path: Path) -> str:
    h = hashlib.sha256()
    with path.open("rb") as f:
        while True:
            chunk = f.read(65536)
            if not chunk:
                break
            h.update(chunk)
    return h.hexdigest().lower()


def main():
    parser = argparse.ArgumentParser(description="Package Groxy Desk release artifacts into dist/")
    parser.add_argument("--version", default="0.1.0", help="SemVer version string without leading v")
    args = parser.parse_args()
    version = args.version.lstrip("v")

    if DIST_DIR.exists():
        shutil.rmtree(DIST_DIR)
    DIST_DIR.mkdir(parents=True, exist_ok=True)

    desk_win = DIST_DIR / "groxy-desk-windows-amd64.exe"
    deskd_win = DIST_DIR / "groxy-deskd-windows-amd64.exe"
    deskd_linux = DIST_DIR / "groxy-deskd-linux-amd64"

    build_binary("windows", "amd64", "./cmd/groxy-desk", desk_win)
    build_binary("windows", "amd64", "./cmd/groxy-deskd", deskd_win)
    build_binary("linux", "amd64", "./cmd/groxy-deskd", deskd_linux)

    subprocess.run(
        [sys.executable, str(ROOT / "docs" / "releases" / "generate_release_guides.py"), version],
        cwd=ROOT,
        check=True,
    )
    pdf_src = ROOT / "docs" / "releases" / "pdf" / f"Groxy-Desk-v{version}-Guide.pdf"
    pdf_dist = DIST_DIR / f"Groxy-Desk-v{version}-Guide.pdf"
    shutil.copy2(pdf_src, pdf_dist)

    bundle_zip = DIST_DIR / f"groxy-desk-v{version}-bundle.zip"
    include_rel = [
        "README.md",
        "CHANGELOG.md",
        "AGENTS.md",
        "GROXY-DESK-EXPERT.md",
        "HANDOFF.md",
        "src/desk.grab",
        "views/workbench.grab.html",
    ]

    with zipfile.ZipFile(bundle_zip, "w", compression=zipfile.ZIP_DEFLATED) as zf:
        zf.write(desk_win, desk_win.name)
        zf.write(deskd_win, deskd_win.name)
        zf.write(deskd_linux, deskd_linux.name)
        zf.write(pdf_dist, pdf_dist.name)
        for rel in include_rel:
            p = ROOT / rel
            if p.exists():
                zf.write(p, rel)
        for folder in ("public/desk", "public/grex", "docs/badges"):
            for f in sorted((ROOT / folder).rglob("*")):
                if f.is_file():
                    zf.write(f, f.relative_to(ROOT).as_posix())

    artifact_paths = sorted(
        [desk_win, deskd_win, deskd_linux, pdf_dist, bundle_zip],
        key=lambda p: p.name,
    )
    sums_lines = [f"{sha256_file(p)}  {p.name}" for p in artifact_paths]
    sums_file = DIST_DIR / "SHA256SUMS.txt"
    sums_file.write_text("\n".join(sums_lines) + "\n", encoding="utf-8")

    print("\nPackaged release artifacts in dist/:")
    for p in artifact_paths + [sums_file]:
        print(f"  - {p.name} ({p.stat().st_size} bytes)")


if __name__ == "__main__":
    main()
