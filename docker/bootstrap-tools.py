#!/usr/bin/env python3
import json
import os
import platform
import shutil
import subprocess
import sys
import tarfile
import tempfile
import urllib.request
from typing import Iterable, List, Tuple


def is_tty() -> bool:
    try:
        return sys.stdout.isatty()
    except Exception:
        return False


def use_color() -> bool:
    if os.environ.get("NO_COLOR"):
        return False
    return is_tty()


def c(code: str, s: str) -> str:
    if not use_color():
        return s
    return f"\x1b[{code}m{s}\x1b[0m"


def info(msg: str) -> None:
    print(c("36", "==>"), msg, file=sys.stderr)


def ok(msg: str) -> None:
    print(c("32", "OK"), msg, file=sys.stderr)


def warn(msg: str) -> None:
    print(c("33", "WARN"), msg, file=sys.stderr)


def err(msg: str) -> None:
    print(c("31", "ERROR"), msg, file=sys.stderr)


def have(cmd: str) -> bool:
    return shutil.which(cmd) is not None


def run(argv: List[str], *, check: bool = True) -> subprocess.CompletedProcess:
    # Stream output; keep installs user-visible.
    return subprocess.run(argv, check=check)


def run_capture(argv: List[str], *, check: bool = True) -> str:
    cp = subprocess.run(argv, check=check, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True)
    return cp.stdout


def detect_pm() -> str:
    if have("apt-get"):
        return "apt"
    if have("dnf"):
        return "dnf"
    if have("yum"):
        return "yum"
    if have("zypper"):
        return "zypper"
    if have("pacman"):
        return "pacman"
    if have("apk"):
        return "apk"
    raise RuntimeError("no supported package manager found")


class PackageSpec(object):
    __slots__ = ("name", "optional")

    def __init__(self, name, optional=False):
        self.name = name
        self.optional = optional


def install_packages(pm: str, packages: Iterable[PackageSpec]) -> None:
    pkgs = [p for p in packages if not have(p.name.split(":")[0])]
    # Note: PackageSpec.name here is *logical*; per manager mapping is below.
    # We still do idempotence by checking binary presence in callers where needed.
    del pkgs

    def apt_install(names: List[str]) -> None:
        run(["apt-get", "update", "-y"])
        env = os.environ.copy()
        env["DEBIAN_FRONTEND"] = "noninteractive"
        subprocess.run(["apt-get", "install", "-y"] + names, check=True, env=env)

    def dnf_install(names: List[str]) -> None:
        run(["dnf", "-y", "install"] + names)

    def yum_install(names: List[str]) -> None:
        run(["yum", "-y", "install"] + names)

    def zypper_install(names: List[str]) -> None:
        # Use non-interactive + auto-import keys to avoid prompts.
        subprocess.run(["zypper", "-n", "--gpg-auto-import-keys", "refresh"], check=False)
        run(["zypper", "-n", "--gpg-auto-import-keys", "install", "-y"] + names)

    def pacman_install(names: List[str]) -> None:
        run(["pacman", "-Sy", "--noconfirm"] + names)

    def apk_install(names: List[str]) -> None:
        run(["apk", "add", "--no-cache"] + names)

    installers = {
        "apt": apt_install,
        "dnf": dnf_install,
        "yum": yum_install,
        "zypper": zypper_install,
        "pacman": pacman_install,
        "apk": apk_install,
    }
    install = installers.get(pm)
    if install is None:
        raise RuntimeError(f"unsupported package manager: {pm}")

    # Package name mapping by manager.
    def pm_names(specs: Iterable[PackageSpec]) -> List[Tuple[str, bool]]:
        out: List[Tuple[str, bool]] = []
        for s in specs:
            key = s.name
            optional = s.optional
            # canonical keys used below
            out.append((key, optional))
        return out

    # Install baseline required tools (best-effort optional ones are allowed to fail)
    mapping = {
        "python3": {
            "apt": ["python3"],
            "dnf": ["python3"],
            "yum": ["python3"],
            "zypper": ["python3"],
            "pacman": ["python"],
            "apk": ["python3"],
        },
        "bash": {
            "apt": ["bash"],
            "dnf": ["bash"],
            "yum": ["bash"],
            "zypper": ["bash"],
            "pacman": ["bash"],
            "apk": ["bash"],
        },
        "curl": {
            "apt": ["curl"],
            "dnf": ["curl"],
            "yum": ["curl"],
            "zypper": ["curl"],
            "pacman": ["curl"],
            "apk": ["curl"],
        },
        "ca-certificates": {
            "apt": ["ca-certificates"],
            "dnf": ["ca-certificates"],
            "yum": ["ca-certificates"],
            "zypper": ["ca-certificates"],
            "pacman": ["ca-certificates"],
            "apk": ["ca-certificates"],
        },
        "tar": {
            "apt": ["tar"],
            "dnf": ["tar"],
            "yum": ["tar"],
            "zypper": ["tar"],
            "pacman": ["tar"],
            "apk": ["tar"],
        },
        "gzip": {
            "apt": ["gzip"],
            "dnf": ["gzip"],
            "yum": ["gzip"],
            "zypper": ["gzip"],
            "pacman": ["gzip"],
            "apk": ["gzip"],
        },
        "xz": {
            "apt": ["xz-utils"],
            "dnf": ["xz"],
            "yum": ["xz"],
            "zypper": ["xz"],
            "pacman": ["xz"],
            "apk": ["xz"],
        },
        "openssh-client": {
            "apt": ["openssh-client"],
            "dnf": ["openssh-clients"],
            "yum": ["openssh-clients"],
            "zypper": ["openssh"],
            "pacman": ["openssh"],
            "apk": ["openssh-client"],
        },
        "git": {
            "apt": ["git"],
            "dnf": ["git"],
            "yum": ["git"],
            "zypper": ["git"],
            "pacman": ["git"],
            "apk": ["git"],
        },
        "node": {
            "apt": ["nodejs", "npm"],
            "dnf": ["nodejs", "npm"],
            "yum": ["nodejs", "npm"],
            "zypper": ["nodejs", "npm"],
            "pacman": ["nodejs", "npm"],
            "apk": ["nodejs", "npm"],
        },
        "gh": {
            "apt": ["gh"],
            "dnf": ["gh"],
            "yum": ["gh"],
            "zypper": ["gh"],
            "pacman": ["github-cli"],
            "apk": ["github-cli"],
        },
    }

    # (name, optional)
    desired = pm_names(packages)
    for name, optional in desired:
        pm_pkg_names = mapping.get(name, {}).get(pm)
        if not pm_pkg_names:
            if optional:
                warn(f"Skipping {name}: no package mapping for {pm}")
                continue
            raise RuntimeError(f"no package mapping for {name} on {pm}")
        info(f"Installing {name} via {pm}: {' '.join(pm_pkg_names)}")
        try:
            install(pm_pkg_names)
            ok(f"{name} installed")
        except subprocess.CalledProcessError as e:
            if optional:
                warn(f"Failed to install optional {name} via {pm} (exit {e.returncode})")
                continue
            raise


def gh_release_asset() -> Tuple[str, str]:
    arch = platform.machine().lower()
    if arch in ("x86_64", "amd64"):
        a = "amd64"
    elif arch in ("aarch64", "arm64"):
        a = "arm64"
    else:
        raise RuntimeError(f"unsupported architecture for gh release install: {arch}")
    return "linux", a


def install_gh_from_release() -> None:
    os_name, arch = gh_release_asset()
    info("Installing gh from GitHub release (fallback)")
    # Use GitHub API to find latest release tag.
    req = urllib.request.Request(
        "https://api.github.com/repos/cli/cli/releases/latest",
        headers={"Accept": "application/vnd.github+json", "User-Agent": "ai-shell-bootstrap"},
    )
    with urllib.request.urlopen(req, timeout=30) as resp:
        data = json.loads(resp.read().decode("utf-8"))
    tag = data.get("tag_name")
    if not tag:
        raise RuntimeError("could not determine latest gh release tag")
    version = tag.lstrip("v")
    filename = f"gh_{version}_{os_name}_{arch}.tar.gz"
    url = f"https://github.com/cli/cli/releases/download/{tag}/{filename}"
    info(f"Downloading {url}")
    with tempfile.TemporaryDirectory() as td:
        tgz = os.path.join(td, filename)
        urllib.request.urlretrieve(url, tgz)
        with tarfile.open(tgz, "r:gz") as tf:
            # gh_<version>_linux_<arch>/bin/gh
            members = [m for m in tf.getmembers() if m.name.endswith("/bin/gh")]
            if not members:
                raise RuntimeError("gh tarball did not contain expected bin/gh")
            m = members[0]
            tf.extract(m, td)
        extracted = os.path.join(td, m.name)
        target = "/usr/local/bin/gh"
        os.makedirs("/usr/local/bin", exist_ok=True)
        shutil.copy2(extracted, target)
        os.chmod(target, 0o755)
    ok("gh installed from release")


def ensure_gh(pm: str) -> None:
    if have("gh"):
        ok("gh already present")
        return
    # Try package manager first (may fail if repo doesn't provide gh).
    try:
        install_packages(pm, [PackageSpec("gh")])
    except Exception as e:
        warn(f"Package-manager install of gh failed: {e}")
        # Fallback to release install (requires python SSL + tar).
        install_gh_from_release()


def main(argv: List[str]) -> int:
    pm = detect_pm()
    info(f"bootstrap-tools: package manager={pm}")

    # Baseline: enough to run scripts reliably.
    baseline = [
        PackageSpec("bash"),
        PackageSpec("curl"),
        PackageSpec("ca-certificates"),
        PackageSpec("tar"),
        PackageSpec("gzip"),
        PackageSpec("xz", optional=True),
        PackageSpec("openssh-client"),
        PackageSpec("git"),
        PackageSpec("node", optional=True),
    ]
    install_packages(pm, baseline)
    ensure_gh(pm)

    # Quick sanity
    for cmd in ("python3", "git", "gh", "ssh"):
        if have(cmd):
            ok(f"{cmd}: OK")
        else:
            warn(f"{cmd}: missing")

    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))

