#!/usr/bin/env python3
"""Real PTY rename acceptance on disposable local/FTP/Blob/S3 paths."""
import importlib.util
import subprocess
import sys
import tempfile
import uuid
from pathlib import Path

sys.dont_write_bytecode = True
spec = importlib.util.spec_from_file_location("terminal_live", Path(__file__).with_name("terminal-live.py"))
live = importlib.util.module_from_spec(spec)
spec.loader.exec_module(live)


def command(action, location, data=None):
    result = subprocess.run(
        ["rclone", action, "--config", str(live.CONFIG), "--retries", "1",
         "--low-level-retries", "1", "--", location], input=data,
        stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=30,
    )
    if result.returncode:
        raise AssertionError(f"rclone {action} failed (exit {result.returncode})")
    return result.stdout


def rename(terminal, name, expected, inline=False):
    if not inline:
        terminal.send("R", "Enter review")
    terminal.send("\x15" + name + "\r", "RENAME FILE?", "From:", "To:", "[Cancel]")
    terminal.send("\t\r", expected, absent=("RENAME FILE?", "Loading"))


def exercise(root):
    with tempfile.TemporaryDirectory(prefix="lazyftp-rename-") as temp:
        local = Path(temp)
        (local / "original.txt").write_bytes(b"local\x00payload\xff")
        prefix = root + "/lazyftp-rename-" + uuid.uuid4().hex
        payload = b"remote\x00payload\xff"
        command("rcat", prefix + "/original.txt", payload)
        command("rcat", prefix + "/existing.txt", b"preserve")
        terminal = None
        try:
            terminal = live.Terminal(local, prefix if ":" in root else "lab-ftp:/incoming")
            terminal.wait("original.txt", absent=("Loading",))
            if ":" not in root:
                terminal.send("\t", "original.txt")
                terminal.send("\x10Open local folder\r", "Open local folder · RIGHT pane")
                terminal.send("\x15" + prefix + "\r", "existing.txt", absent=("Open local folder ·", "Loading"))
                terminal.send("\t", "original.txt")
            terminal.wait("original.txt", "existing.txt", absent=("Loading",))
            terminal.send("r", "No failed or canceled transfers to retry")
            terminal.send("R", "Enter review", absent=("RENAME FILE?",))
            terminal.send("\x15canceled.txt\r", "RENAME FILE?", "[Cancel]")
            terminal.send("\r", "Rename canceled", absent=("RENAME FILE?",))
            assert (local / "original.txt").read_bytes() == b"local\x00payload\xff"
            assert not (local / "canceled.txt").exists()
            rename(terminal, "local final 雪.txt", "Renamed")
            assert (local / "local final 雪.txt").read_bytes() == b"local\x00payload\xff"
            assert not (local / "original.txt").exists()

            terminal.send("\t", "original.txt")
            terminal.send("/", "Filter:")
            terminal.send("original\r", "/original", "original.txt")
            terminal.send("R", "Enter review")
            terminal.send("\x15remote final 雪.txt\r", "RENAME FILE?", 'From: "original.txt"', 'To:   "remote final 雪.txt"')
            # Merely reviewing must leave both storage names untouched.
            assert command("cat", prefix + "/original.txt") == payload
            assert b"remote final" not in command("lsf", prefix)
            terminal.send("\x1b", "Rename canceled", absent=("RENAME FILE?",))
            rename(terminal, "existing.txt", "Name already exists")
            assert command("cat", prefix + "/original.txt") == payload
            assert command("cat", prefix + "/existing.txt") == b"preserve"
            rename(terminal, "remote final 雪.txt", "Renamed", inline=True)
            assert command("cat", prefix + "/remote final 雪.txt") == payload
            names = command("lsf", prefix).decode()
            assert "original.txt" not in names and "existing.txt" in names
            terminal.quit()
            print(f"Rename PTY PASS: local left + {root} right; cancel/review/collision/bytes/R vs r")
        finally:
            if terminal is not None:
                terminal.close()
            command("purge", prefix)


def main():
    with tempfile.TemporaryDirectory(prefix="lazyftp-rename-right-") as right:
        exercise(right)
    for root in ("lab-ftp:/incoming", "lab-blob:/files/assets", "lab-s3:/files/assets"):
        exercise(root)


if __name__ == "__main__":
    main()
