#!/usr/bin/env python3
"""Disposable loopback-only lab. Run .agents/setup, then make lab-up lab-seed.

No reset/delete command is provided. Seeding only adds missing fixture files;
existing content (including edited fixtures) is preserved. No real credentials
or ambient rclone configuration are used.
"""

import argparse
import base64
import configparser
import hashlib
import json
import os
from pathlib import Path
import shlex
import shutil
import socket
import subprocess
import sys
import tempfile
import time
import zipfile


ROOT = Path(__file__).resolve().parent.parent
LAB = ROOT / ".lab"
CONFIG = LAB / "rclone.conf"
MARKER = LAB / ".lazyftp-lab-owner"
RCLONE_VERSION = "1.70.3"
RCLONE_SHA256 = "7d69057e69385f6514a9684c7eaa424d972096b130284bb34dd967c4ed4f9dad"
RCLONE = Path.home() / ".local/bin/rclone"
PYTHON = ROOT / ".venv/bin/python"
AZURITE = ROOT / "lab/node_modules/.bin/azurite-blob"
FTP_SERVICE = "lazyftp-lab-ftp"
BLOB_SERVICE = "lazyftp-lab-blob"
FTP_PORT = 2121
BLOB_PORT = 10000
FTP_USER = "lab"
FTP_PASSWORD = "public-lazyftp-development-only"
ACCOUNT = "lazyftplab"
# Public emulator-only key, not an Azure account credential.
KEY = base64.b64encode(b"lazyftp-public-development-key!!!").decode()
ENDPOINT = f"http://127.0.0.1:{BLOB_PORT}/{ACCOUNT}"
REMOTES = {"local": str(LAB / "local"), "ftp": "lab-ftp:/incoming", "blob": "lab-blob:/files/assets"}
FIXTURES = {
    "local": {
        "local only.txt": b"Only on the local side.\n",
        "shared.txt": b"Local version of shared.txt.\n",
        "nested dir/local notes.txt": b"Nested local notes.\n",
        "Unicode/café.txt": "Local café notes.\n".encode(),
        "zero bytes.bin": b"",
    },
    "ftp": {
        "ftp only.txt": b"Only on FTP.\n",
        "shared.txt": b"FTP version of shared.txt, different size.\n",
        "nested dir/deeper/ftp notes.txt": b"Nested FTP notes.\n",
        "Unicode/日本語.txt": "FTP 日本語 notes.\n".encode(),
        "zero bytes.bin": b"",
    },
    "blob": {
        "blob only.txt": b"Only on Blob.\n",
        "shared.txt": b"Blob version of shared.txt.\n",
        "nested dir/deeper/blob notes.txt": b"Nested Blob notes.\n",
        "Unicode/naïve résumé.txt": "Blob naïve résumé notes.\n".encode(),
        "zero bytes.bin": b"",
    },
}


def fail(message):
    raise RuntimeError(message)


def run(args, **kwargs):
    return subprocess.run([str(arg) for arg in args], check=True, **kwargs)


def lab_path(relative):
    """Refuse symlinks anywhere in a designated lab path."""
    path = LAB
    for part in (".", *Path(relative).parts):
        path = path / part
        if path.is_symlink():
            fail(f"Refusing symlink in owned lab path: {path}")
    return path


def owned(create=False):
    lab_path(".")
    expected = f"lazyftp disposable lab v1\n{ROOT}\n"
    if not LAB.exists():
        if not create:
            fail("Lab is absent. Run make lab-up first.")
        LAB.mkdir(mode=0o700)
    lab_path(MARKER.name)
    if not MARKER.exists() and create and not any(LAB.iterdir()):
        MARKER.write_text(expected)
    if not MARKER.is_file() or MARKER.read_text() != expected:
        fail(f"Refusing unowned {LAB}; expected its repository-specific ownership marker.")


def prereqs(supervisor=False):
    if not RCLONE.is_file():
        fail("Pinned rclone is missing. Run .agents/setup.")
    version = run([RCLONE, "version"], capture_output=True, text=True).stdout.splitlines()[0]
    if version != f"rclone v{RCLONE_VERSION}":
        fail(f"Expected rclone v{RCLONE_VERSION}, got {version}. Run .agents/setup.")
    if not PYTHON.is_file() or not AZURITE.is_file():
        fail("Lab dependencies are missing. Run .agents/setup.")
    result = subprocess.run([str(PYTHON), "-c",
                             "import importlib.metadata; print(importlib.metadata.version('pyftpdlib'))"],
                            capture_output=True, text=True)
    if result.returncode != 0 or result.stdout.strip() != "2.1.0":
        fail("Expected pyftpdlib 2.1.0 in .venv. Run .agents/setup.")
    package = json.loads((ROOT / "lab/node_modules/azurite/package.json").read_text())
    if package["version"] != "3.35.0":
        fail("Expected Azurite 3.35.0. Run .agents/setup.")
    if supervisor and not shutil.which("amp"):
        fail("amp orb service is required; run the lab inside an Amp orb.")


def rclone(*args, **kwargs):
    # Ignore personal remotes, environment authentication, and option overrides.
    env = {key: value for key, value in os.environ.items() if not key.startswith("RCLONE_")}
    env["RCLONE_CONFIG"] = str(CONFIG)
    return run([RCLONE, "--contimeout", "3s", "--timeout", "10s", "--retries", "1", *args], env=env, **kwargs)


def write_config():
    path = lab_path("rclone.conf")
    password = run([RCLONE, "obscure", FTP_PASSWORD], capture_output=True, text=True).stdout.strip()
    # Use an atomic replacement so an interrupted setup never leaves partial config.
    temporary = lab_path("rclone.conf.tmp")
    with open(temporary, "w", opener=lambda p, flags: os.open(p, flags, 0o600)) as config:
        os.chmod(temporary, 0o600)
        config.write(f"""# Public development credentials. Never use for production.
[lab-ftp]
type = ftp
host = 127.0.0.1
port = {FTP_PORT}
user = {FTP_USER}
pass = {password}
tls = false

[lab-blob]
type = azureblob
account = {ACCOUNT}
key = {KEY}
endpoint = {ENDPOINT}
""")
    temporary.replace(path)


def check_config():
    path = lab_path("rclone.conf")
    if not path.is_file() or path.stat().st_mode & 0o077:
        fail("Missing/private config permissions incorrect; run make lab-up (requires mode 600).")
    config = configparser.ConfigParser(interpolation=None)
    config.read(path)
    expected = {
        "lab-ftp": {"type": "ftp", "host": "127.0.0.1", "port": str(FTP_PORT), "user": FTP_USER},
        "lab-blob": {"type": "azureblob", "account": ACCOUNT, "key": KEY, "endpoint": ENDPOINT},
    }
    for remote, values in expected.items():
        if remote not in config or any(config[remote].get(key) != value for key, value in values.items()):
            fail(f"Unexpected lab configuration for {remote}; run make lab-up.")


def listening(port):
    try:
        with socket.create_connection(("127.0.0.1", port), timeout=0.25):
            return True
    except OSError:
        return False


def active(service):
    result = subprocess.run(["amp", "orb", "service", "status", service], capture_output=True, text=True)
    return result.returncode == 0 and "active (running)" in result.stdout


def ready():
    for port, name in ((FTP_PORT, "FTP"), (BLOB_PORT, "Blob")):
        if not listening(port):
            fail(f"{name} service unavailable on loopback port {port}; run make lab-up.")
    rclone("lsd", "lab-ftp:/", stdout=subprocess.DEVNULL)
    rclone("lsd", "lab-blob:/", stdout=subprocess.DEVNULL)


def up():
    prereqs(supervisor=True)
    owned(create=True)
    for directory in ("local", "ftp/incoming", "azurite"):
        lab_path(directory).mkdir(parents=True, exist_ok=True, mode=0o700)
    write_config()
    for service, port, action in ((FTP_SERVICE, FTP_PORT, "_ftp"), (BLOB_SERVICE, BLOB_PORT, "_blob")):
        if listening(port) and not active(service):
            fail(f"Loopback port {port} is already occupied by something other than {service}.")
        command = shlex.join([str(PYTHON), str(ROOT / "scripts/lab.py"), action])
        run(["amp", "orb", "service", "start", service, "--cwd", ROOT,
             "--port", str(port), "--command", command])
    deadline = time.monotonic() + 20
    while not all(listening(port) for port in (FTP_PORT, BLOB_PORT)):
        if time.monotonic() >= deadline:
            fail("Lab startup timed out. Inspect amp orb service logs lazyftp-lab-{ftp,blob}.")
        time.sleep(0.2)
    ready()
    print(f"Lab ready; storage is loopback-only (no portals).\nRCLONE_CONFIG={CONFIG}")
    print(f"--rclone-config {CONFIG} --left {REMOTES['local']} --right {REMOTES['ftp']}")
    print(f"Blob alternative: --right {REMOTES['blob']}\nRun make lab-seed to add missing fixtures.")


def seed():
    prereqs()
    owned()
    check_config()
    ready()
    for backend, files in FIXTURES.items():
        directory = "local" if backend == "local" else f"seed/{backend}"
        for name, data in files.items():
            path = lab_path(f"{directory}/{name}")
            path.parent.mkdir(parents=True, exist_ok=True)
            try:
                with path.open("xb") as output:
                    output.write(data)
            except FileExistsError:
                pass
        if backend != "local":
            rclone("copy", lab_path(directory), REMOTES[backend], "--ignore-existing")
    for backend, remote in REMOTES.items():
        print(f"\n{backend}: {remote}", flush=True)
        rclone("lsf", remote, "--recursive", "--format", "ps")


def down():
    if not shutil.which("amp"):
        fail("amp orb service is required to stop the named lab services.")
    for service in (FTP_SERVICE, BLOB_SERVICE):
        # Stop only our exact names; never touch unrelated orb services or data.
        result = subprocess.run(["amp", "orb", "service", "stop", service], capture_output=True, text=True)
        if result.returncode == 0:
            print(result.stdout.strip())
        elif f"Service {service} is not running." in result.stderr:
            print(f"{service}: already stopped.")
        else:
            fail(f"Could not stop {service}: {result.stderr.strip() or result.stdout.strip()}")
    print("Lab services stopped. All .lab data has been preserved.")


def serve_ftp():
    from pyftpdlib.authorizers import DummyAuthorizer
    from pyftpdlib.handlers import FTPHandler
    from pyftpdlib.servers import FTPServer

    owned()
    authorizer = DummyAuthorizer()
    authorizer.add_user(FTP_USER, FTP_PASSWORD, str(lab_path("ftp")), perm="elradfmwMT")
    handler = FTPHandler
    handler.authorizer = authorizer
    handler.passive_ports = range(30000, 30010)
    FTPServer(("127.0.0.1", FTP_PORT), handler).serve_forever()


def serve_blob():
    owned()
    os.environ["AZURITE_ACCOUNTS"] = f"{ACCOUNT}:{KEY}"
    os.execv(str(AZURITE), [str(AZURITE), "--blobHost", "127.0.0.1", "--blobPort", str(BLOB_PORT),
                          "--location", str(lab_path("azurite")), "--disableTelemetry", "--silent"])


def install_rclone():
    """Install one exact official binary, checked against the official SHA256SUMS."""
    if sys.platform != "linux" or os.uname().machine != "x86_64":
        fail("Pinned lab rclone installer supports linux/amd64 only.")
    target = Path.home() / f".local/toolchains/rclone-v{RCLONE_VERSION}/rclone"
    if target.is_file():
        version = run([target, "version"], capture_output=True, text=True).stdout.splitlines()[0]
        if version != f"rclone v{RCLONE_VERSION}":
            fail(f"Unexpected existing toolchain at {target}.")
    else:
        filename = f"rclone-v{RCLONE_VERSION}-linux-amd64.zip"
        base = f"https://downloads.rclone.org/v{RCLONE_VERSION}"
        checksums = run(["curl", "--fail", "--silent", "--show-error", "--location", "--retry", "3",
                         f"{base}/SHA256SUMS"], capture_output=True, text=True).stdout
        if f"{RCLONE_SHA256}  {filename}" not in checksums.splitlines():
            fail("Official rclone checksum manifest does not match the pinned checksum.")
        with tempfile.TemporaryDirectory(prefix="lazyftp-rclone-") as temporary:
            archive = Path(temporary) / filename
            run(["curl", "--fail", "--silent", "--show-error", "--location", "--retry", "3",
                 f"{base}/{filename}", "--output", archive])
            if hashlib.sha256(archive.read_bytes()).hexdigest() != RCLONE_SHA256:
                fail("Official rclone archive SHA256 verification failed.")
            with zipfile.ZipFile(archive) as bundle:
                binary = bundle.read(f"rclone-v{RCLONE_VERSION}-linux-amd64/rclone")
            target.parent.mkdir(parents=True, exist_ok=True)
            target.write_bytes(binary)
            target.chmod(0o755)
        print(f"Verified official SHA256: {RCLONE_SHA256}")
    RCLONE.parent.mkdir(parents=True, exist_ok=True)
    if RCLONE.is_symlink():
        RCLONE.unlink()
    elif RCLONE.exists():
        fail(f"Refusing to replace unrelated executable: {RCLONE}")
    RCLONE.symlink_to(target)
    run([RCLONE, "version"])
    print(f"Pinned rclone: {RCLONE} -> {target}")


def main():
    actions = {"up": up, "seed": seed, "down": down, "install-rclone": install_rclone,
               "check": lambda: (prereqs(), owned(), check_config(), ready()),
               "_ftp": serve_ftp, "_blob": serve_blob}
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("command", choices=actions)
    actions[parser.parse_args().command]()


if __name__ == "__main__":
    try:
        main()
    except (RuntimeError, OSError, subprocess.CalledProcessError) as error:
        print(f"lab: {error}", file=sys.stderr)
        sys.exit(1)
