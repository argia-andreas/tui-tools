#!/usr/bin/env python3
"""Real-PTY independent-source acceptance; needs the seeded disposable lab.

Use --binary /tmp/lazyftp-before for the same regression scenario on a baseline.
Only temporary local files and a uniquely named remote file are mutated.
"""
import argparse
import codecs
import errno
import fcntl
import json
import os
from pathlib import Path
import pty
import select
import signal
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
    raise SystemExit("Pane PTY FAIL: pyte required; use .venv/bin/python")

ROOT = Path(__file__).resolve().parent.parent
CONFIG = ROOT / ".lab/rclone.conf"
REMOTE = "lab-ftp:/incoming"
SIDES = ("LEFT", "RIGHT")
BUTTONS = ("[ Local files ]", "[ Saved connection ]", "[ New connection ]")
WAIT = 20


def rclone(command, remote):
    result = subprocess.run(
        ["rclone", command, "--config", str(CONFIG), "--retries", "1",
         "--low-level-retries", "1", "--contimeout", "5s", "--timeout", "10s",
         "--", remote], stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=25)
    # Never print backend stderr: it may contain credential-bearing URLs.
    assert result.returncode == 0, f"independent rclone {command} failed ({result.returncode})"
    return result.stdout


class Terminal:
    def __init__(self, binary, left, right, bookmarks):
        self.master, slave = pty.openpty()
        self.screen = pyte.Screen(200, 40)
        self.stream = pyte.Stream(self.screen)
        self.decoder = codecs.getincrementaldecoder("utf-8")("replace")
        fcntl.ioctl(self.master, termios.TIOCSWINSZ, struct.pack("HHHH", 40, 200, 0, 0))
        env = dict(os.environ, TERM="xterm-256color", COLORTERM="truecolor", CLICOLOR="1")
        env.pop("NO_COLOR", None)
        try:
            self.proc = subprocess.Popen(
                [str(binary), "--left", str(left), "--right", str(right),
                 "--rclone-config", str(CONFIG), "--connections", str(bookmarks)],
                stdin=slave, stdout=slave, stderr=slave, env=env, cwd=ROOT,
                start_new_session=True)
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

    def text(self):
        return "\n".join(self.screen.display)

    def wait(self, *wanted, absent=()):
        deadline = time.monotonic() + WAIT
        while time.monotonic() < deadline:
            alive = self.read()
            current = self.text()
            if all(s in current for s in wanted) and not any(s in current for s in absent):
                return current
            if not alive or self.proc.poll() is not None:
                break
        raise AssertionError(f"expected {wanted!r}, absent {absent!r}\n{self.text()}")

    def send(self, keys, *wanted, absent=()):
        os.write(self.master, keys.encode())
        return self.wait(*wanted, absent=absent)

    def locate(self, label, pane=None):
        lo, hi = (0, self.screen.columns)
        if pane is not None:
            lo, hi = pane * self.screen.columns // 2, (pane + 1) * self.screen.columns // 2
        for y, row in enumerate(self.screen.display):
            x = row.find(label, lo, hi)
            if x >= 0:
                return x, y
        raise AssertionError(f"missing pointer target {label!r} in pane {pane}\n{self.text()}")

    def click(self, label, pane, *wanted):
        x, y = self.locate(label, pane)
        return self.send(f"\x1b[<0;{x+3};{y+1}M\x1b[<0;{x+3};{y+1}m", *wanted)

    def focus(self, pane):
        # Clicking a pane title focuses it without moving the file cursor.
        self.click(SIDES[pane] + " ·", pane, SIDES[pane] + " ·")

    def palette(self, query, *wanted):
        self.send("\x10", "ACTIONS · Ctrl+P")
        self.send(query, "ACTIONS · Ctrl+P", query)
        return self.send("\r", *wanted, absent=("ACTIONS · Ctrl+P",))

    def detach(self, pane, pointer=True):
        label = SIDES[pane] + " · No source"
        if pointer:
            self.click("[ Detach ]", pane, label, *BUTTONS)
        else:
            self.focus(pane)
            self.palette("detach", label, *BUTTONS)

    def folder(self, pane, path, keyboard=False):
        title = "Open local folder · " + SIDES[pane] + " pane"
        if keyboard:
            self.send("\r", title)
        else:
            self.focus(pane)
            self.palette("open local folder", title)
        self.send("\x15" + str(path), str(path), title)
        self.send("\r", SIDES[pane] + " · LOCAL " + str(path), absent=(title, "Loading"))

    def saved(self, pane, keyboard=False):
        title = "Choose connection · " + SIDES[pane] + " pane"
        if keyboard:
            self.send("j", *BUTTONS)
            self.send("\r", title, "Pane FTP")
        else:
            self.focus(pane)
            self.palette("saved connection", title, "Pane FTP")
        self.send("\r", "Connection details · " + SIDES[pane] + " pane", "Remote root")
        assert "Local folder" not in self.text(), "in-workspace saved flow demands companion local folder"
        # Details focus root -> name -> Open. Use pointer so startup-only
        # fields cannot accidentally alter the target action's tab order.
        self.wait("Open in " + SIDES[pane].lower() + " pane")
        self.click("Open in " + SIDES[pane].lower() + " pane", None,
                   SIDES[pane] + " · REMOTE " + REMOTE)
        self.wait(SIDES[pane] + " · REMOTE " + REMOTE, absent=("Loading", "Connection details"))

    def resize(self, cols, rows):
        self.screen.resize(rows, cols)
        fcntl.ioctl(self.master, termios.TIOCSWINSZ, struct.pack("HHHH", rows, cols, 0, 0))
        os.kill(self.proc.pid, signal.SIGWINCH)

    def quit(self):
        os.write(self.master, b"q")
        deadline = time.monotonic() + 5
        while self.proc.poll() is None and time.monotonic() < deadline:
            if not self.read():
                break
        self.proc.wait(timeout=1)
        assert self.proc.returncode == 0, f"unclean terminal exit: {self.proc.returncode}"

    def close(self):
        if self.proc.poll() is None:
            os.killpg(self.proc.pid, signal.SIGKILL)
            self.proc.wait(timeout=5)
        os.close(self.master)


def preservation(t, pane):
    """Observe actual file rows, selection and filter in the untouched pane."""
    lo, hi = pane * t.screen.columns // 2, (pane + 1) * t.screen.columns // 2
    rows = [row[lo:hi] for row in t.screen.display]
    for token in ("keep-a.txt", "keep-b.txt", "1 selected", "/keep"):
        assert any(token in row for row in rows), f"opposite pane lost {token!r}\n{t.text()}"
    # Cursor marker changes when focus moves; checkbox/selection marker must not.
    return tuple(row.strip().replace("›", " ").strip() for row in rows
                 if "keep-" in row or "entries ·" in row)


def matrix(binary, local, replacement, opposite, bookmarks):
    for pane in (0, 1):
        for initial in ("local", "remote"):
            for target in ("local", "remote"):
                # CLI intentionally retains local-left/remote-right startup.
                # Establish other arrangements through the actual pane flows.
                t = Terminal(binary, local, REMOTE, bookmarks)
                try:
                    t.wait("[ Detach ]", absent=("Loading",))
                    t.folder(1 - pane, opposite)
                    if initial == "local":
                        t.folder(pane, local)
                    else:
                        t.saved(pane)
                    t.wait("keep-a.txt", absent=("Loading",))
                    t.focus(1 - pane)
                    t.send("/", "Filter:")
                    t.send("keep", "Filter: keep")
                    t.send("\r", "/keep", absent=("Filter:",))
                    t.send(" ", "1 selected")
                    before = preservation(t, 1 - pane)
                    t.detach(pane)
                    assert preservation(t, 1 - pane) == before, "detach changed opposite pane"
                    if target == "local":
                        t.folder(pane, replacement, keyboard=True)
                        t.wait("replacement.txt")
                    else:
                        t.saved(pane, keyboard=True)
                    assert preservation(t, 1 - pane) == before, "replacement changed opposite pane"
                    t.quit()
                    print(f"Pane PTY matrix PASS: {SIDES[pane]} {initial} -> detached -> {target}; opposite rows/selection/filter retained", flush=True)
                finally:
                    t.close()


def boundaries(binary, local, bookmarks):
    t = Terminal(binary, local, REMOTE, bookmarks)
    try:
        t.wait("[ Detach ]", absent=("Loading",))
        for pane in (0, 1):
            t.focus(pane)
            t.palette("saved connection", "Choose connection · " + SIDES[pane] + " pane")
            t.send("\x1b", SIDES[pane] + " ·", absent=("Choose connection",))
            t.palette("new connection", "New connection · " + SIDES[pane] + " pane")
            # Wizard Esc returns to the chooser; a second Esc restores files.
            t.send("\x1b", "Choose connection · " + SIDES[pane] + " pane", absent=("New connection ·",))
            t.send("\x1b", SIDES[pane] + " ·", absent=("Choose connection ·",))
            t.palette("open local folder", "Open local folder · " + SIDES[pane] + " pane")
            t.send("\x15/does-not-exist-pane-" + uuid.uuid4().hex, "Open local folder")
            t.send("\r", "Local folder could not be opened")
            t.send("\x15" + str(local), str(local))
            t.send("\r", SIDES[pane] + " · LOCAL " + str(local), absent=("Open local folder", "Loading"))
            t.palette("open local folder", "Open local folder · " + SIDES[pane] + " pane")
            t.send("\x1b", SIDES[pane] + " · LOCAL", absent=("Open local folder",))
        t.detach(0, pointer=False)
        t.detach(1)
        for key, reason in (("c", "copying"), ("p", "previewing"), ("e", "editing"), ("\x12", "refreshing")):
            t.send(key, "Choose a source before " + reason)
            assert "CONFIRM" not in t.text(), "detached pane opened file action"
        t.resize(60, 12)
        t.wait(*BUTTONS)
        for pane in (0, 1):
            for button in BUTTONS:
                t.locate(button, pane)
        t.resize(200, 40)
        t.wait("LEFT · No source", "RIGHT · No source", *BUTTONS)
        # Exercise third keyboard choice and k to move back, without writes.
        t.focus(0)
        t.send("j", *BUTTONS)
        t.send("j", *BUTTONS)
        t.send("\r", "New connection · LEFT pane")
        t.send("\x1b", "Choose connection · LEFT pane", absent=("New connection ·",))
        t.send("\x1b", "LEFT · No source", absent=("Choose connection ·",))
        t.focus(0)
        t.send("k", *BUTTONS)
        t.send("\r", "Choose connection · LEFT pane")
        t.send("\x1b", "LEFT · No source", absent=("Choose connection ·",))
        t.saved(0)
        t.saved(1)
        t.focus(0)
        t.send("c", "Remote-to-remote copies are not supported yet")
        assert "CONFIRM" not in t.text() and "Running" not in t.text(), "remote/remote attempted write"
        t.quit()
        print("Pane PTY boundaries PASS: both targets/cancel, invalid-folder recovery, both detached guards, 60x12 choices, j/k choices, remote/remote refusal", flush=True)
    finally:
        t.close()


def reversed_transfer(binary, local, bookmarks):
    name = "pane-" + uuid.uuid4().hex + ".bin"
    destination = REMOTE + "/" + name
    path = local / name
    original = bytes(range(256)) * 17 + b"\x00\xffreversed-panes\r\n"
    path.write_bytes(original)
    t = None
    upload_attempted = False
    try:
        t = Terminal(binary, local, REMOTE, bookmarks)
        t.wait("[ Detach ]", absent=("Loading",))
        t.saved(0)
        t.folder(1, local)
        t.wait("LEFT · REMOTE " + REMOTE, name, absent=("Loading",))
        t.focus(1)
        t.send("/", "Filter:")
        t.send(name, "Filter: " + name)
        t.send("\r", name, absent=("Filter:",))
        t.send("c", "Destination is new", "[Copy]")
        upload_attempted = True
        t.send("\t\r", "Running", "To:   " + destination, absent=("Completed",))
        t.wait("Completed: " + destination, absent=("Running",))
        assert rclone("cat", destination) == original, "right-local -> left-remote upload bytes differ"
        path.unlink()
        t.focus(0)
        t.send("/", "Filter:")
        t.send(name, "Filter: " + name)
        t.send("\r", name, absent=("Filter:",))
        t.send("c", "Destination is new", "[Copy]")
        t.send("\t\r", "Running", "To:   " + str(path), absent=("Completed",))
        t.wait("Completed: " + str(path), absent=("Running",))
        assert path.read_bytes() == original, "left-remote -> right-local download bytes differ"
        assert rclone("cat", destination) == original, "download changed remote source"
        t.quit()
        print(f"Pane PTY reversed transfer PASS: right-local upload / left-remote download, independent byte equality ({len(original)} bytes)", flush=True)
    finally:
        if t is not None:
            t.close()
        if upload_attempted:
            names = rclone("lsf", REMOTE).decode().splitlines()
            if name in names:
                rclone("deletefile", destination)
            assert name not in rclone("lsf", REMOTE).decode().splitlines(), "owned remote cleanup failed"
        path.unlink(missing_ok=True)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--binary", type=Path, default=ROOT / "bin/lazyftp")
    args = parser.parse_args()
    binary = args.binary.resolve()
    assert binary.is_file(), f"binary missing: {binary}; build first"
    assert CONFIG.is_file(), "seeded .lab/rclone.conf required"
    rclone("lsf", REMOTE)
    started = time.monotonic()
    with tempfile.TemporaryDirectory(prefix="panes-", dir="/tmp") as temp:
        root = Path(temp)
        local, replacement, opposite = (root / name for name in ("a", "b", "keep"))
        for directory in (local, replacement, opposite):
            directory.mkdir()
        (local / "initial.txt").write_text("initial\n")
        (replacement / "replacement.txt").write_text("replacement\n")
        for name in ("keep-a.txt", "keep-b.txt", "hidden.txt"):
            (opposite / name).write_text(name + "\n")
        bookmarks = root / "connections.json"
        # Intentionally no companion folder: workspace flows must not require it.
        bookmarks.write_text(json.dumps([dict(name="Pane FTP", remote="lab-ftp", root="/incoming", local="")]))
        before = bookmarks.read_bytes()
        matrix(binary, local, replacement, opposite, bookmarks)
        boundaries(binary, local, bookmarks)
        reversed_transfer(binary, local, bookmarks)
        assert bookmarks.read_bytes() == before, "source selection/cancel implicitly saved bookmarks"
    print(f"Pane PTY PASS: eight transitions + boundaries + reversed byte transfers; owned files cleaned ({time.monotonic() - started:.2f}s)")


if __name__ == "__main__":
    try:
        main()
    except Exception as exc:
        print(f"Pane PTY FAIL: {exc}", file=sys.stderr)
        sys.exit(1)
