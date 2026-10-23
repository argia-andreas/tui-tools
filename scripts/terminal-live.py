#!/usr/bin/env python3
"""Bounded real-PTY FTP/Blob copy smoke; only unique owned files are mutated.

Run with .venv/bin/python scripts/terminal-live.py after building and seeding
the lab. No fixture fallback: missing dependencies or unavailable services fail.
"""
import codecs
import errno
import fcntl
import os
from pathlib import Path
import pty
import select
import shutil
import struct
import subprocess
import sys
import tempfile
import termios
import time
import uuid

try:
    import pyte
except ImportError:
    raise SystemExit("Live PTY smoke FAIL: pyte required; use .venv/bin/python after .agents/setup")


ROOT = Path(__file__).resolve().parent.parent
CONFIG = ROOT / ".lab/rclone.conf"
WAIT = 30


def rclone(command, remote, *flags):
    result = subprocess.run(
        ["rclone", command, "--config", str(CONFIG), "--retries", "1",
         "--low-level-retries", "1", "--contimeout", "5s", "--timeout", "10s",
         *flags, "--", remote],
        stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=25,
    )
    # Backend stderr can contain credential-bearing URLs. Never print it.
    if result.returncode:
        raise AssertionError(f"independent rclone {command} failed (exit {result.returncode})")
    return result.stdout


class Terminal:
    def __init__(self, local, remote):
        self.master, slave = pty.openpty()
        self.screen = pyte.Screen(200, 44)
        self.stream = pyte.Stream(self.screen)
        self.decoder = codecs.getincrementaldecoder("utf-8")("replace")
        fcntl.ioctl(self.master, termios.TIOCSWINSZ, struct.pack("HHHH", 44, 200, 0, 0))
        env = dict(os.environ, TERM="xterm-256color", COLORTERM="truecolor", CLICOLOR="1")
        env.pop("NO_COLOR", None)
        try:
            self.proc = subprocess.Popen(
                [str(ROOT / "bin/lazyftp"), "--live", "--rclone-config", str(CONFIG),
                 "--left", str(local), "--right", remote],
                stdin=slave, stdout=slave, stderr=slave, env=env, cwd=ROOT,
                start_new_session=True,
            )
        except BaseException:
            os.close(self.master)
            raise
        finally:
            os.close(slave)

    def read(self, timeout=0.05):
        if select.select([self.master], [], [], timeout)[0]:
            try:
                data = os.read(self.master, 65536)
            except OSError as exc:
                if exc.errno != errno.EIO:
                    raise
                return False
            if not data:
                return False
            self.stream.feed(self.decoder.decode(data))
        return True

    def wait(self, *wanted, absent=()):
        deadline = time.monotonic() + WAIT
        while time.monotonic() < deadline:
            alive = self.read()
            current = "\n".join(self.screen.display)
            if all(value in current for value in wanted) and not any(value in current for value in absent):
                return current
            if not alive or self.proc.poll() is not None:
                break
        raise AssertionError(f"PTY expected {wanted!r}, absent {absent!r}\n" + current)

    def send(self, keys, *wanted, absent=()):
        os.write(self.master, keys.encode())
        return self.wait(*wanted, absent=absent)

    def transfer(self, keys, destination):
        # Require a fresh Running frame with no stale Completed text, then the
        # new destination's completion. An old upload success cannot pass a download.
        self.send(keys, "TRANSFERS", "Running", "To:   " + destination,
                  absent=("Completed", "DESTINATION EXISTS", "CONFIRM SINGLE-FILE COPY"))
        self.wait("Completed: " + destination, "TRANSFERS", absent=("Running",))

    def quit(self):
        os.write(self.master, b"q")
        deadline = time.monotonic() + 5
        # Drain during shutdown: Bubble Tea can block flushing a full PTY.
        while self.proc.poll() is None and time.monotonic() < deadline:
            if not self.read():
                break
        self.proc.wait(timeout=1)
        if self.proc.returncode != 0:
            raise AssertionError(f"terminal exit {self.proc.returncode}")

    def close(self):
        if self.proc.poll() is None:
            # Kill the owned process group, including any outstanding rclone.
            os.killpg(self.proc.pid, 9)
            self.proc.wait(timeout=5)
        os.close(self.master)


def exercise(remote):
    started = time.monotonic()
    name = "pty-" + uuid.uuid4().hex[:16] + ".bin"
    destination = remote.rstrip("/") + "/" + name
    local = Path(tempfile.mkdtemp(prefix="terminal-live-", dir=ROOT / ".lab/local"))
    filename = local / name
    # Deliberately asymmetric binary data: includes NULs, high bytes and CR/LF.
    original = bytes(range(256)) * 129 + b"\x00\xffremote-original\r\n\x80"
    changed = bytes(value ^ 0xA7 for value in original)
    assert original != changed and len(original) == len(changed)
    terminal = None
    upload_started = False
    try:
        rclone("lsf", remote, "--files-only")  # Fail fast if lab/config is unavailable.
        filename.write_bytes(original)
        terminal = Terminal(local, remote)
        terminal.wait(name, "Live workspace", absent=("Loading",))
        terminal.send("c", "Destination is new", "[Cancel]", "[Copy]", "[Rename]")
        upload_started = True
        terminal.transfer("\t\r", destination)
        assert rclone("cat", destination) == original, "upload bytes differ from local source"
        assert filename.read_bytes() == original, "upload changed its local source"

        filename.write_bytes(changed)
        # After upload focus stays local; the first Tab focuses the remote pane
        # even with the transfers drawer open. Filter to the exact owned file.
        # Send the command separately: a PTY read can group /name into one
        # KeyRunes message, which is text rather than the '/' command.
        terminal.send("\t/", "Filter:")
        terminal.send(name, "Filter: " + name)
        terminal.send("\r", name, absent=("Filter:", "Loading"))
        terminal.send("c", "DESTINATION EXISTS", "[Cancel]", "[Skip]", "[Overwrite]", "[Rename]")
        terminal.send("\r", name, absent=("DESTINATION EXISTS",))
        assert filename.read_bytes() == changed, "default Cancel overwrote local bytes"
        assert rclone("cat", destination) == original, "default Cancel changed remote bytes"

        terminal.send("c", "DESTINATION EXISTS", "[Overwrite]")
        terminal.transfer("\t\t\r", str(filename))
        assert filename.read_bytes() == original, "explicit overwrite download did not restore original bytes"
        assert rclone("cat", destination) == original, "download changed remote source"
        terminal.quit()
        print(f"Live PTY {remote} PASS: upload + independent cat, default Cancel, "
              f"same-length overwrite download + byte equality, fresh Running→Completed, "
              f"clean exit ({len(original)} bytes; {time.monotonic() - started:.2f}s)", flush=True)
    finally:
        try:
            if terminal is not None:
                terminal.close()
        finally:
            try:
                if upload_started:
                    # Probe exact ownership before deleting; never remove an endpoint,
                    # a directory, a prefix, or any seeded fixture.
                    names = rclone("lsf", remote, "--files-only").decode().splitlines()
                    if name in names:
                        rclone("deletefile", destination)
                    assert name not in rclone("lsf", remote, "--files-only").decode().splitlines(), "remote cleanup failed"
            finally:
                shutil.rmtree(local)


def main():
    started = time.monotonic()
    if not shutil.which("rclone"):
        raise AssertionError("rclone missing from PATH; run .agents/setup")
    if not (ROOT / "bin/lazyftp").is_file() or not CONFIG.is_file():
        raise AssertionError("bin/lazyftp and .lab/rclone.conf required; build and start/seed lab first")
    if not (ROOT / ".lab/local").is_dir():
        raise AssertionError(".lab/local missing; start/seed the lab first")
    for remote in ("lab-ftp:/incoming", "lab-blob:/files/assets"):
        exercise(remote)
    print(f"Live PTY smoke PASS: both real backends; owned local/remote files cleaned ({time.monotonic() - started:.2f}s)")


if __name__ == "__main__":
    try:
        main()
    except Exception as exc:
        print(f"Live PTY smoke FAIL: {exc}", file=sys.stderr)
        sys.exit(1)
