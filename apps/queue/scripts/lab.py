#!/usr/bin/env python3
"""Disposable, loopback-only Service Bus lab. No Azure credentials accepted.

Use --sudo for this orb's Docker daemon and --orb for supervised foreground
Compose. Outside an orb, up uses portable Docker Compose detached mode.
Seed always resets through this wrapper before publishing scenario messages.
SQL Edge is retired; native macOS/Apple Silicon execution remains unverified.
"""
import argparse
import json
import os
from pathlib import Path
import secrets
import shlex
import shutil
import subprocess
import sys
import time
import urllib.request

ROOT = Path(__file__).resolve().parents[1]
PROJECT = "sbx-explorer-lab"
SERVICE = "sbx-explorer-lab"
HEALTH = "http://127.0.0.1:5301/health"
IMAGE = "mcr.microsoft.com/azure-messaging/servicebus-emulator@sha256:5a96d893b245031740f7d46e0fe5ff282d24b78c4b7d761dd57590f3f010a9b3"


def run(args, **kwargs):
    return subprocess.run(args, check=True, **kwargs)


def docker(args):
    return (["sudo"] if args.sudo else []) + ["docker"]


def local_docker(args):
    # Never honor remote Docker or Compose endpoint/project/file overrides.
    for key in os.environ:
        if key.startswith("COMPOSE_") or key in ("DOCKER_HOST", "DOCKER_CONTEXT", "DOCKER_TLS_VERIFY", "DOCKER_CERT_PATH"):
            raise ValueError(f"unset {key}; lab refuses Docker/Compose overrides")
    result = run(docker(args) + ["context", "inspect"], capture_output=True, text=True)
    endpoint = json.loads(result.stdout)[0]["Endpoints"]["docker"]["Host"]
    if not endpoint.startswith(("unix://", "npipe://")):
        raise ValueError("lab requires a local Docker socket")


def credential_file():
    # Deliberately independent of Git, cwd, XDG overrides and connection-string env.
    directory = Path.home() / ".local/share/sbx-explorer-lab"
    directory.mkdir(mode=0o700, parents=True, exist_ok=True)
    directory.chmod(0o700)
    path = directory / "compose.env"
    if path.is_symlink():
        raise ValueError("credential file must not be a symlink")
    if not path.exists():
        fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
        with os.fdopen(fd, "w") as stream:
            stream.write("SBX_ACCEPT_EULA=Y\nSBX_SQL_PASSWORD=Sbx!" + secrets.token_hex(24) + "\n")
    path.chmod(0o600)
    return path


def compose(args):
    return docker(args) + ["compose", "--project-name", PROJECT,
                           "--env-file", str(credential_file()),
                           "-f", str(ROOT / "lab/compose.yaml")]


def health():
    # Disable proxies and redirects: this check must never leave loopback.
    class NoRedirect(urllib.request.HTTPRedirectHandler):
        def redirect_request(self, *args, **kwargs):
            return None
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}), NoRedirect())
    with opener.open(HEALTH, timeout=2) as response:
        body = json.loads(response.read(1024))
        return response.status == 200 and isinstance(body, dict) and body.get("status") == "healthy"


def guarded(args, require_health=True):
    local_docker(args)
    result = run(compose(args) + ["ps", "-q", "emulator"], capture_output=True, text=True,
                 env=compose_env())
    ids = result.stdout.split()
    if len(ids) != 1:
        raise ValueError("expected exactly one running disposable emulator")
    result = run(docker(args) + ["inspect", ids[0]], capture_output=True, text=True)
    container = json.loads(result.stdout)[0]
    validate_container(container)
    if require_health and not health():
        raise ValueError("fixed local emulator is not healthy")


def validate_container(container):
    labels = container["Config"].get("Labels", {})
    if labels.get("com.docker.compose.project") != PROJECT or labels.get("com.docker.compose.service") != "emulator":
        raise ValueError("not the disposable lab project")
    if container["Config"]["Image"] != IMAGE:
        raise ValueError("unexpected emulator image")
    ports = container["HostConfig"]["PortBindings"]
    expected = {"5672/tcp": [{"HostIp": "127.0.0.1", "HostPort": "5673"}],
                "5300/tcp": [{"HostIp": "127.0.0.1", "HostPort": "5301"}]}
    if ports != expected:
        raise ValueError("lab port bindings are not the fixed loopback target")


def compose_env():
    env = dict(os.environ, SBX_ACCEPT_EULA="Y")
    env.pop("SBX_SQL_PASSWORD", None)  # only the generated outside-Git env file
    return env


def up(args):
    if not args.accept_eula:
        raise ValueError("up/reset/seed require --accept-eula (Microsoft emulator and SQL terms)")
    local_docker(args)
    command = compose(args) + ["up", "--remove-orphans"]
    if args.orb:
        # An env file carries the generated SQL secret; never place it in argv.
        shell = "unset SBX_SQL_PASSWORD SBX_ACCEPT_EULA; " + shlex.join(command)
        run(["amp", "orb", "service", "start", SERVICE, "--command", shell])
    else:
        run(command + ["--detach"], env=compose_env())
    deadline = time.monotonic() + 120
    while time.monotonic() < deadline:
        try:
            guarded(args)
            print("Disposable lab ready: AMQP 127.0.0.1:5673; management 127.0.0.1:5301. Counts unavailable.")
            return
        except (ValueError, OSError, subprocess.CalledProcessError):
            time.sleep(2)
    raise ValueError("lab did not become healthy within 120 seconds; inspect Compose/service logs")


def down(args):
    local_docker(args)
    # Fixed project and file; no arbitrary project/volume deletion.
    if args.orb:
        # First-ever reset has no supervisor unit. A successful inventory with
        # no exact unit is known absence, unlike swallowing a failed stop.
        result = run(["amp", "orb", "service", "list"], capture_output=True, text=True)
        units = {line.split()[0] for line in result.stdout.splitlines() if line.split()}
        if f"amp-svc-{SERVICE}.service" in units:
            run(["amp", "orb", "service", "stop", SERVICE])
    run(compose(args) + ["down", "--volumes", "--remove-orphans"], env=compose_env())


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--sudo", action="store_true", help="use sudo docker for local daemon")
    parser.add_argument("--orb", action="store_true", help="supervise foreground Compose via amp orb service")
    parser.add_argument("--accept-eula", action="store_true", help="explicitly accept Microsoft emulator and SQL EULAs")
    parser.add_argument("--confirm-disposable", action="store_true", help="authorize destructive changes to this lab only")
    parser.add_argument("command", choices=["up", "status", "down", "reset", "seed", "send", "peek", "dead-letter", "drain"])
    parser.add_argument("cli_args", nargs=argparse.REMAINDER, help="queue-lab flags; see go run ./cmd/queue-lab --help")
    args = parser.parse_args(argv)
    if args.command in ("down", "reset", "seed", "send", "dead-letter", "drain") and not args.confirm_disposable:
        raise ValueError("mutation requires --confirm-disposable")
    if args.command in ("reset", "seed") and not args.accept_eula:
        raise ValueError("reset/seed require --accept-eula")
    if args.command in ("up", "down", "status", "reset") and args.cli_args:
        raise ValueError("lifecycle commands do not accept endpoint or other extra arguments")
    if args.command == "up":
        up(args)
    elif args.command == "status":
        guarded(args)
        print("Disposable lab healthy; runtime counts unavailable (not zero).")
    elif args.command == "down":
        down(args)
    elif args.command == "reset":
        print("Reset destroys all data in sbx-explorer-lab only.", flush=True)
        down(args)
        up(args)
    else:
        # Seed's Go entry point invokes Python reset before publishing. Other
        # commands require the running, image/label/port-verified local lab.
        if args.command != "seed":
            guarded(args)
        flags = []
        for name in ("confirm_disposable", "accept_eula", "sudo", "orb"):
            if getattr(args, name):
                flags.append("--" + name.replace("_", "-"))
        go = shutil.which("go")
        if not go:
            raise ValueError("Go 1.27.3 must be on PATH, or invoke built queue-lab directly")
        run([go, "run", "./cmd/queue-lab", args.command] + flags + args.cli_args, cwd=ROOT)


if __name__ == "__main__":
    try:
        main()
    except (ValueError, OSError, subprocess.CalledProcessError) as exc:
        # Do not echo Docker output/env/SDK errors containing credentials.
        print(str(exc) if isinstance(exc, ValueError) else "Lab command failed; inspect local service state.", file=sys.stderr)
        sys.exit(1)
