#!/usr/bin/env python3
"""
Build static CGO_ENABLED=0 release binaries, Walkthrough PDF, Branded Inno Setup .exe installer
(Groxy-Desk-Setup-<version>.exe), release ZIP bundle, and SHA256SUMS.txt in dist/.
Usage: python scripts/package_release.py --version 1.0.0
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
ISS_SCRIPT = ROOT / "installer" / "groxy-desk.iss"
GRAB_BIN_SRC = Path(r"C:\Development\GRAB\dist\grab-windows-amd64.exe")
GRABMAN_BIN_SRC = Path(r"C:\Development\grabman\dist\grabman-windows-amd64.exe")


def find_iscc() -> Path:
    local_appdata = os.environ.get("LOCALAPPDATA", "")
    candidates = [
        Path(local_appdata) / "Programs" / "Inno Setup 6" / "ISCC.exe" if local_appdata else None,
        Path(r"C:\Program Files (x86)\Inno Setup 6\ISCC.exe"),
        Path(r"C:\Program Files\Inno Setup 6\ISCC.exe"),
        Path(shutil.which("ISCC") or "") if shutil.which("ISCC") else None,
        Path(shutil.which("ISCC.exe") or "") if shutil.which("ISCC.exe") else None,
    ]
    for cand in candidates:
        if cand and cand.is_file():
            return cand
    raise FileNotFoundError(
        "Inno Setup 6 compiler (ISCC.exe) not found. Install via: "
        "winget install --id JRSoftware.InnoSetup -e --accept-package-agreements --accept-source-agreements"
    )


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
    parser.add_argument("--version", default="1.0.1", help="SemVer version string without leading v")
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

    if not GRAB_BIN_SRC.is_file():
        raise FileNotFoundError(f"Missing frozen GRAB v1.1.0 binary: {GRAB_BIN_SRC}")
    if not GRABMAN_BIN_SRC.is_file():
        raise FileNotFoundError(f"Missing frozen grabman v1.0.0 binary: {GRABMAN_BIN_SRC}")

    iscc = find_iscc()
    setup_exe = DIST_DIR / f"Groxy-Desk-Setup-{version}.exe"
    print(f"Compiling {setup_exe.name} via Inno Setup 6 ({iscc})...")
    subprocess.run(
        [
            str(iscc),
            f"/DMyAppVersion={version}",
            f"/DGrabBinSource={GRAB_BIN_SRC}",
            f"/DGrabmanBinSource={GRABMAN_BIN_SRC}",
            str(ISS_SCRIPT),
        ],
        cwd=ISS_SCRIPT.parent,
        check=True,
    )
    if not setup_exe.is_file() or setup_exe.stat().st_size == 0:
        raise RuntimeError(f"Expected Inno Setup output {setup_exe} was not created.")

    bundle_zip = DIST_DIR / f"groxy-desk-v{version}-bundle.zip"
    include_rel = [
        "README.md",
        "CHANGELOG.md",
        "AGENTS.md",
        "GROXY-DESK-EXPERT.md",
        "HANDOFF.md",
        "specifications/GROXY-DESK-SPEC-100.md",
        "specifications/GROXY-DESK-SPEC-101.md",
        "src/desk.grab",
        "views/workbench.grab.html",
        "installer/groxy-desk.iss",
    ]

    with zipfile.ZipFile(bundle_zip, "w", compression=zipfile.ZIP_DEFLATED) as zf:
        zf.write(desk_win, desk_win.name)
        zf.write(deskd_win, deskd_win.name)
        zf.write(deskd_linux, deskd_linux.name)
        zf.write(GRAB_BIN_SRC, "bin/grab.exe")
        zf.write(GRABMAN_BIN_SRC, "bin/grabman.exe")
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
        [setup_exe, desk_win, deskd_win, deskd_linux, pdf_dist, bundle_zip],
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
