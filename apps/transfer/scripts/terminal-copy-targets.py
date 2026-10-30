#!/usr/bin/env python3
"""Real PTY regression using rclone's local backend in a disposable subtree.

Run after make build with .venv/bin/python scripts/terminal-copy-targets.py.
No lab, ambient rclone configuration, network service, or shared fixture needed.
"""
import fcntl
import importlib.util
from pathlib import Path
import os
import signal
import struct
import tempfile
import termios


ROOT = Path(__file__).resolve().parent.parent
spec = importlib.util.spec_from_file_location("terminal_live", ROOT / "scripts/terminal-live.py")
live = importlib.util.module_from_spec(spec)
spec.loader.exec_module(live)


def main():
    with tempfile.TemporaryDirectory(prefix=".pty-copy-targets-", dir=ROOT) as scratch:
        root = Path(scratch)
        source = root / ("long-source-directory-" * 5)
        destination = root / ("long-destination-directory-" * 5)
        source.mkdir()
        destination.mkdir()
        originals = {"a-checksums.txt": b"checksums\n", "b-asset.tar.gz": b"asset\x00\xff",
                     "release-notes.txt": b"release notes\n"}
        for name, data in originals.items():
            (source / name).write_bytes(data)
        live.CONFIG = root / "rclone.conf"
        live.CONFIG.write_text("[owned-local]\ntype = local\n")
        terminal = live.Terminal(source, "owned-local:" + str(destination))
        try:
            terminal.wait("a-checksums.txt", "release-notes.txt", absent=("Loading",))
            terminal.send(" j ", "2 selected (marked)", "Ctrl+G clear marks")
            terminal.send("c", "Copy: 2 marked; highlight included", "2 captured file jobs",
                          "From:", "To:", "[ Cancel ]")
            terminal.send("\t\r", "2 completed", "c Copy 2 marked", absent=("Loading",))
            for name in ("a-checksums.txt", "b-asset.tar.gz"):
                assert (destination / name).read_bytes() == originals[name]
            assert not (destination / "release-notes.txt").exists()
            terminal.send("j", "› \uf15c release-notes.txt")
            terminal.send("c", "Copy: 2 marked; highlight excluded", "2 captured file jobs")
            print("RETAINED MARKS CONFIRMATION (200 columns):\n" + "\n".join(terminal.screen.display))
            terminal.send("\r", "Ctrl+G clear marks", absent=("CONFIRM TRANSFER QUEUE",))
            # Ctrl+G must clear retained marks; Copy then captures the highlight.
            terminal.send("\x07c", "Copy: highlighted entry", "release-notes.txt", "Destination is new")
            terminal.send("\t\r", "Completed:", absent=("CONFIRM SINGLE-FILE COPY", "Loading"))
            terminal.wait("0 selected (marked)")
            assert (destination / "release-notes.txt").read_bytes() == originals["release-notes.txt"]
            # Verify the same captured filename survives narrow conflict review.
            fcntl.ioctl(terminal.master, termios.TIOCSWINSZ, struct.pack("HHHH", 30, 80, 0, 0))
            terminal.screen.resize(30, 80)
            os.kill(terminal.proc.pid, signal.SIGWINCH)
            terminal.send("c", "DESTINATION EXISTS", "From:", "To:", "[ Cancel ]")
            rows = "\n".join(terminal.screen.display)
            for label in ("From:", "To:"):
                row = next(row for row in terminal.screen.display if label in row)
                assert "release-notes.txt" in row, row
            print("NARROW CONFLICT CONFIRMATION (80 columns):\n" + rows)
            terminal.send("\r", "Completed:", absent=("DESTINATION EXISTS",))
            assert (destination / "release-notes.txt").read_bytes() == originals["release-notes.txt"]
            terminal.quit()
        finally:
            terminal.close()
    print("Copy targets PTY PASS: retained batch, excluded highlight, default Cancel, Ctrl+G, exact bytes, narrow conflict filenames")


if __name__ == "__main__":
    main()
