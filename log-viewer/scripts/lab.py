#!/usr/bin/env python3
"""Disposable native Elastic lab; stdlib only, fixed local endpoint, no live mode.

Run install, up, seed, check, down in that order. down stops only this lab's
supervised service and preserves private data/credentials for the next up.
Never print backend response bodies: they may contain credentials or fixtures.
"""

import argparse
import base64
import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import platform
import resource
import secrets
import shlex
import shutil
import socket
import ssl
import stat
import subprocess
import sys
import tarfile
import time
import urllib.error
import urllib.request
import zipfile


ROOT = Path(__file__).resolve().parents[1]
LAB = ROOT / ".lab"
ENDPOINT = "https://127.0.0.1:19200"
TARGET = "logview-lab-events"
FORBIDDEN = "logview-lab-forbidden"
OWNER = "logview-native-elastic-v1\n"
SERVICE = "logview-elastic-" + hashlib.sha256(str(ROOT).encode()).hexdigest()[:10]
PIN = json.loads((ROOT / "scripts/elastic-release.json").read_text())
HOME = LAB / ("elasticsearch-" + PIN["version"])


class LabError(Exception):
    pass


def private(path):
    info = path.lstat()
    if stat.S_ISLNK(info.st_mode) or info.st_uid != os.getuid() or info.st_mode & 0o077:
        raise LabError("lab state must be private, owned by current uid, and not symlinked")


def owned(create=False):
    if not LAB.exists() and not LAB.is_symlink() and create:
        LAB.mkdir(mode=0o700)
        write(LAB / "owner", OWNER)
    private(LAB)
    marker = LAB / "owner"
    private(marker)
    if marker.read_text() != OWNER:
        raise LabError("unrecognized lab ownership marker")
    # Refuse unsafe runtime state even when a parent is private.
    for base, dirs, files in os.walk(LAB, followlinks=False):
        if Path(base) == HOME or HOME in Path(base).parents:
            dirs[:] = []  # verified distribution has legitimate JDK symlinks
            continue
        for name in dirs + files:
            path = Path(base) / name
            if path == HOME:
                if path.is_symlink() or path.stat().st_uid != os.getuid():
                    raise LabError("unsafe distribution directory")
                continue
            private(path)


def write(path, value):
    tmp = path.with_name(path.name + ".tmp")
    fd = os.open(tmp, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    try:
        with os.fdopen(fd, "w") as f:
            f.write(value)
        os.replace(tmp, path)
    finally:
        tmp.unlink(missing_ok=True)


def run(args, *, env=None, data=None):
    result = subprocess.run(args, cwd=ROOT, env=env, input=data, text=True,
                            stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    if result.returncode:
        raise LabError("local command failed: " + Path(args[0]).name + " (output withheld)")
    return result.stdout


def preflight():
    if os.getuid() == 0 or platform.system() != "Linux" or platform.machine() != "x86_64":
        raise LabError("requires non-root Linux x86_64 with bundled JDK")
    soft, hard = resource.getrlimit(resource.RLIMIT_NOFILE)
    desired = 65535
    if hard != resource.RLIM_INFINITY and hard < desired:
        raise LabError("file descriptor hard limit below 65535")
    resource.setrlimit(resource.RLIMIT_NOFILE, (max(soft, desired), hard))
    available = int(next(line.split()[1] for line in Path("/proc/meminfo").read_text().splitlines()
                         if line.startswith("MemAvailable:"))) * 1024
    # Account for the tightest ancestor cgroup, not just host MemAvailable.
    group = Path("/sys/fs/cgroup") / Path("/proc/self/cgroup").read_text().strip().split("::")[-1].lstrip("/")
    while group != Path("/sys/fs"):
        limit, current = group / "memory.max", group / "memory.current"
        if limit.exists() and current.exists() and limit.read_text().strip() != "max":
            available = min(available, int(limit.read_text()) - int(current.read_text()))
        group = group.parent
    if available < 1024 ** 3:
        raise LabError("requires at least 1024 MiB available memory; heap alone is 512 MiB")
    print(f"preflight: uid={os.getuid()} fd={resource.getrlimit(resource.RLIMIT_NOFILE)[0]} available_memory_mib={available // 1024**2}")


def install():
    preflight()
    owned(create=True)
    if HOME.exists():
        if (LAB / "installed.sha512").read_text().strip() != PIN["sha512"]:
            raise LabError("distribution pin mismatch")
        print("install: verified distribution already installed")
        return
    if shutil.disk_usage(LAB).free < 3 * 1024**3:
        raise LabError("installation requires 3 GiB free disk")
    archive = LAB / "elastic.tar.gz"
    started = time.monotonic()
    if not archive.exists():
        partial = LAB / "elastic.tar.gz.partial"
        try:
            with urllib.request.urlopen(PIN["url"], timeout=120) as source, partial.open("wb") as out:
                shutil.copyfileobj(source, out, 1024 * 1024)
            partial.rename(archive)
        finally:
            partial.unlink(missing_ok=True)
    with archive.open("rb") as stream:
        actual = hashlib.file_digest(stream, "sha512").hexdigest()
    if actual != PIN["sha512"]:
        archive.unlink()
        raise LabError("SHA512 mismatch; archive removed without extraction")
    staging = LAB / "extract"
    staging.mkdir(mode=0o700)
    try:
        with tarfile.open(archive) as tar:
            for member in tar.getmembers():
                parts = PurePosixPath(member.name).parts
                if not parts or parts[0] != HOME.name or ".." in parts or member.name.startswith("/"):
                    raise LabError("unsafe archive path")
                # data_filter rejects external links, devices, and unsafe modes.
                tarfile.data_filter(member, str(staging))
            tar.extractall(staging, filter="data")
        (staging / HOME.name).rename(HOME)
        write(LAB / "installed.sha512", actual + "\n")
    finally:
        shutil.rmtree(staging)
    archive.unlink()
    print(f"install: SHA512 verified before safe extraction; version={PIN['version']} elapsed_seconds={time.monotonic()-started:.2f}")


def environment():
    env = os.environ.copy()
    env["ES_PATH_CONF"] = str(LAB / "config")
    env["ES_JAVA_OPTS"] = "-Xms512m -Xmx512m -XX:ActiveProcessorCount=2"
    return env


def configure():
    if (LAB / "configured").exists():
        return
    config = LAB / "config"
    config.mkdir(mode=0o700, exist_ok=True)
    for name in ("jvm.options", "log4j2.properties"):
        write(config / name, (HOME / "config" / name).read_text())
    (config / "jvm.options.d").mkdir(mode=0o700, exist_ok=True)
    certs = config / "certs"
    certs.mkdir(mode=0o700, exist_ok=True)
    certutil = str(HOME / "bin/elasticsearch-certutil")
    run([certutil, "ca", "--silent", "--pem", "--out", str(certs / "ca.zip")], env=environment())
    # Extract only explicitly expected files, never archive-supplied paths.
    with zipfile.ZipFile(certs / "ca.zip") as archive:
        for name in ("ca.crt", "ca.key"):
            write(certs / name, archive.read("ca/" + name).decode())
    run([certutil, "cert", "--silent", "--pem", "--ca-cert", str(certs / "ca.crt"),
         "--ca-key", str(certs / "ca.key"), "--name", "node", "--ip", "127.0.0.1",
         "--dns", "localhost", "--out", str(certs / "node.zip")], env=environment())
    with zipfile.ZipFile(certs / "node.zip") as archive:
        for name in ("node.crt", "node.key"):
            write(certs / name, archive.read("node/" + name).decode())
    (certs / "ca.zip").unlink()
    (certs / "node.zip").unlink()
    password = secrets.token_urlsafe(36)
    write(LAB / "bootstrap-password", password)
    # add --force creates the keystore too; 9.5.5 create has no --force option.
    run([str(HOME / "bin/elasticsearch-keystore"), "add", "-x", "-f", "bootstrap.password"],
        env=environment(), data=password + "\n")
    (config / "elasticsearch.keystore").chmod(0o600)
    write(config / "elasticsearch.yml", f"""cluster.name: logview-private-lab
node.name: logview-lab
path.data: {LAB / 'data'}
path.logs: {LAB / 'logs'}
network.host: 127.0.0.1
http.port: 19200
transport.port: 19300
discovery.type: single-node
node.store.allow_mmap: false
xpack.license.self_generated.type: basic
xpack.ml.enabled: false
xpack.security.enabled: true
xpack.security.autoconfiguration.enabled: false
xpack.security.http.ssl.enabled: true
xpack.security.http.ssl.key: certs/node.key
xpack.security.http.ssl.certificate: certs/node.crt
xpack.security.http.ssl.certificate_authorities: [certs/ca.crt]
xpack.security.transport.ssl.enabled: true
xpack.security.transport.ssl.verification_mode: certificate
xpack.security.transport.ssl.key: certs/node.key
xpack.security.transport.ssl.certificate: certs/node.crt
xpack.security.transport.ssl.certificate_authorities: [certs/ca.crt]
""")
    write(LAB / "configured", "yes\n")


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None


def request(method, path, body=None, credential="bootstrap", expected=(200,)):
    # All routes are compiled lab operations. No URL/config input is accepted.
    if not path.startswith("/") or path.startswith("//"):
        raise LabError("invalid internal lab route")
    if credential == "password":
        token = base64.b64encode(("elastic:" + (LAB / "bootstrap-password").read_text()).encode()).decode()
        auth = "Basic " + token
    else:
        auth = "ApiKey " + (LAB / (credential + ".key")).read_text().strip()
    data = None if body is None else json.dumps(body).encode()
    context = ssl.create_default_context(cafile=str(LAB / "config/certs/ca.crt"))
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}), NoRedirect(),
                                        urllib.request.HTTPSHandler(context=context))
    req = urllib.request.Request(ENDPOINT + path, data=data, method=method,
                                 headers={"Authorization": auth, "Content-Type": "application/json"})
    try:
        with opener.open(req, timeout=10) as response:
            status, raw = response.status, response.read(8 * 1024**2 + 1)
    except urllib.error.HTTPError as exc:
        status, raw = exc.code, b"{}"
        exc.close()
    if status not in expected:
        raise LabError(f"lab API rejected {method} request (HTTP {status}); response withheld")
    if len(raw) > 8 * 1024**2:
        raise LabError("lab response exceeded 8 MiB")
    return status, json.loads(raw) if raw else {}


def credentials():
    for name, privileges in (("bootstrap", ["all"]), ("read", ["read"])):
        if (LAB / (name + ".key")).exists():
            continue
        targets = [TARGET, FORBIDDEN] if name == "bootstrap" else [TARGET]
        _, result = request("POST", "/_security/api_key", {
            "name": "logview-lab-" + name,
            "role_descriptors": {name: {"cluster": [], "indices": [
                {"names": targets, "privileges": privileges, "allow_restricted_indices": False}]}}},
            credential="password")
        write(LAB / (name + ".key"), result["encoded"])
    write(LAB / "source.json", json.dumps({
        "source": "local-elastic", "endpoint": ENDPOINT, "target": TARGET,
        "ca_file": str(LAB / "config/certs/ca.crt"), "api_key_file": str(LAB / "read.key"),
        "window": {"start": "2026-10-23T14:15:00Z", "end": "2026-10-23T14:30:00Z"}}, indent=2) + "\n")


def rss():
    pidfile = LAB / "node.pid"
    if not pidfile.exists():
        return "unknown"
    try:
        pid = int(pidfile.read_text())
        # Ensure stale pid files never cause reporting of an unrelated process.
        if str(HOME).encode() not in Path(f"/proc/{pid}/cmdline").read_bytes():
            return "unknown"
        for line in Path(f"/proc/{pid}/status").read_text().splitlines():
            if line.startswith("VmRSS:"):
                return f"{int(line.split()[1]) / 1024:.2f}"
    except (OSError, ValueError):
        pass
    return "unknown"


def up():
    preflight()
    owned()
    if not HOME.exists():
        raise LabError("run install first")
    configure()
    try:
        request("GET", "/", credential="password")
        print("up: authenticated node already ready")
    except (OSError, urllib.error.URLError, LabError):
        for port in (19200, 19300):
            with socket.socket() as sock:
                try:
                    sock.bind(("127.0.0.1", port))
                except OSError:
                    raise LabError(f"loopback port {port} occupied; refusing unrelated listener") from None
        started = time.monotonic()
        command = shlex.join([sys.executable, str(Path(__file__).resolve()), "_serve"])
        run(["amp", "orb", "service", "start", SERVICE, "--cwd", str(ROOT),
             "--port", "19200", "--command", command])
        deadline = time.monotonic() + 180
        while True:
            try:
                _, result = request("GET", "/_cluster/health?wait_for_status=yellow&timeout=5s", credential="password")
                if not result.get("timed_out") and result.get("status") in ("yellow", "green"):
                    break
            except (OSError, urllib.error.URLError, LabError):
                pass
            if time.monotonic() > deadline:
                raise LabError("readiness timed out; inspect private .lab/logs; service left for diagnosis")
            time.sleep(1)
        print(f"up: authenticated TLS ready elapsed_seconds={time.monotonic()-started:.2f} rss_mib={rss()}")
    credentials()
    print(f"up: private source config={LAB / 'source.json'}; no portal")


def serve():
    preflight()
    owned()
    os.environ.update(environment())
    os.chdir(ROOT)
    os.execv(str(HOME / "bin/elasticsearch"), [str(HOME / "bin/elasticsearch"), "-p", str(LAB / "node.pid")])


def seed():
    owned()
    mapping = {"settings": {"number_of_shards": 1, "number_of_replicas": 0}, "mappings": {
        "dynamic": "strict", "properties": {
            "@timestamp": {"type": "date"}, "log": {"properties": {"level": {"type": "keyword"}}},
            "service": {"properties": {"name": {"type": "keyword"}, "node": {"properties": {
                "name": {"type": "keyword"}}}}}, "message": {"type": "text"},
            "trace": {"properties": {"id": {"type": "keyword"}}},
            "fields": {"type": "object", "enabled": False}}}}
    # Fixed exact targets and deterministic IDs; reruns overwrite, never delete indices.
    for target in (TARGET, FORBIDDEN):
        status, _ = request("HEAD", "/" + target, expected=(200, 404))
        if status == 404:
            request("PUT", "/" + target, mapping)
    events = json.loads((ROOT / "internal/fixture/events.json").read_text())
    for number, event in enumerate(events):
        native = {"@timestamp": event["Time"], "service": {"name": event["Service"],
                  "node": {"name": event["Instance"]}}, "message": event["Message"],
                  "trace": {"id": event["TraceID"]}, "fields": event["Fields"]}
        if "Level" in event:
            native["log"] = {"level": event["Level"]}
        request("PUT", f"/{TARGET}/_doc/fixture-{number:04d}", native, expected=(200, 201))
    request("PUT", f"/{FORBIDDEN}/_doc/forbidden", {"@timestamp": "2026-10-23T14:26:00Z",
            "message": "Forbidden lab sentinel"}, expected=(200, 201))
    for target in (TARGET, FORBIDDEN):
        request("POST", f"/{target}/_refresh")
    print(f"seed: {len(events)} fixed historical ECS events; forbidden sentinel; refresh complete")


def check():
    owned()
    expected = len(json.loads((ROOT / "internal/fixture/events.json").read_text()))
    _, result = request("POST", f"/{TARGET}/_search", {"size": 100, "track_total_hits": True,
        "query": {"range": {"@timestamp": {"gte": "2026-10-23T14:15:00Z", "lt": "2026-10-23T14:30:00Z"}}}},
        credential="read")
    if result["hits"]["total"]["value"] != expected:
        raise LabError("owned fixture count differs from expected")
    request("PUT", f"/{TARGET}/_doc/permission-probe", {"message": "must not be written"},
            credential="read", expected=(403,))
    request("POST", f"/{FORBIDDEN}/_search", {"size": 1}, credential="read", expected=(403,))
    _, result = request("GET", f"/{TARGET}/_count")
    if result["count"] != expected:
        raise LabError("permission probe changed owned dataset")
    print(f"check: TLS verified; read={expected}; owned_write=403; forbidden_read=403; rss_mib={rss()}")


def down():
    owned()
    run(["amp", "orb", "service", "stop", SERVICE])
    print("down: owned supervised service stopped; private state retained")


def main():
    os.umask(0o077)
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("command", choices=("install", "up", "seed", "check", "down", "_serve"))
    command = parser.parse_args().command
    try:
        {"install": install, "up": up, "seed": seed, "check": check, "down": down, "_serve": serve}[command]()
    except (LabError, OSError, ValueError, KeyError, tarfile.TarError, zipfile.BadZipFile) as exc:
        # Never include arbitrary exception text, which can contain response data.
        message = str(exc) if isinstance(exc, LabError) else type(exc).__name__ + " (details withheld)"
        print("lab: " + message, file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
