#!/usr/bin/env python3
"""Exercise the actual terminal app, decoding its screen rather than grepping ANSI."""
import codecs
import fcntl
import os
import pty
import select
import signal
import struct
import subprocess
import termios
import time

import pyte


def main():
    started = time.monotonic()
    master, slave = pty.openpty()
    screen = pyte.Screen(160, 40)
    stream = pyte.Stream(screen)
    decoder = codecs.getincrementaldecoder("utf-8")("replace")

    def resize(cols, rows):
        fcntl.ioctl(master, termios.TIOCSWINSZ, struct.pack("HHHH", rows, cols, 0, 0))
        screen.resize(rows, cols)

    resize(160, 40)
    env = dict(os.environ, TERM="xterm-256color", COLORTERM="truecolor", CLICOLOR="1")
    env.pop("NO_COLOR", None)
    proc = subprocess.Popen(["./bin/lazyftp", "--demo"], stdin=slave, stdout=slave,
                            stderr=slave, env=env, start_new_session=True)
    os.close(slave)

    def wait(want, absent=None):
        deadline = time.monotonic() + 5
        while time.monotonic() < deadline:
            ready, _, _ = select.select([master], [], [], 0.05)
            if ready:
                try:
                    data = os.read(master, 65536)
                except OSError:
                    break
                stream.feed(decoder.decode(data))
            current = "\n".join(screen.display)
            if want in current and (absent is None or absent not in current):
                return current
        raise AssertionError(f"Expected {want!r}, absent {absent!r}\n" + "\n".join(screen.display))

    def send(keys, want, absent=None):
        os.write(master, keys.encode())
        return wait(want, absent)

    def expect_background(row, col, expected):
        actual = screen.buffer[row][col].bg
        # RGB conversion in the terminal color library can round a channel by 1.
        assert len(actual) == 6 and all(
            abs(int(actual[i:i+2], 16) - int(expected[i:i+2], 16)) <= 1
            for i in (0, 2, 4)
        ), f"background at {row},{col}: expected {expected}, got {actual}"

    try:
        wait("README.md")
        wait("no network or disk writes")
        # Styled labels must not reset their trailing fill to the terminal's
        # default background (visible as dark rectangles in real terminals).
        expect_background(0, 40, "1e1e2e")
        expect_background(2, 50, "1e1e2e")
        notice_row = next(i for i, row in enumerate(screen.display) if "FTP · unencrypted" in row)
        expect_background(notice_row, 20, "313244")
        send("j", "› ▸ src/")
        send("\r", "browser.go")
        send(" ", "1 selected")
        send("h", "README.md")
        send("]", "INSPECTOR")
        send("t", "TRANSFERS")
        send("\x10", "ACTIONS · Ctrl+P")
        send("dracula", "Theme: dracula")
        send("\r", "dracula", "ACTIONS · Ctrl+P")
        send("\x10", "ACTIONS · Ctrl+P")
        send("copy", "Copy to right pane")
        send("\r", "demo is read-only")
        send("\x1b", "README.md", "ACTIONS · Ctrl+P")
        send("[", "LOCAL /project", "WORKSPACE")
        send("]", "LOCAL /project", "INSPECTOR")
        send("[", "WORKSPACE")
        send("\x1b[Z", "› demo-ftp /incoming")
        send("j", "› demo-ftp /archive")
        send("j", "› demo-blob /assets")
        send("\r", "demo-blob:/assets")
        wait("hero image.png")
        resize(80, 24)
        os.kill(proc.pid, signal.SIGWINCH)
        wait("hero image.png", "WORKSPACE")
        resize(50, 10)
        os.kill(proc.pid, signal.SIGWINCH)
        wait("Resize terminal")
        resize(160, 40)
        os.kill(proc.pid, signal.SIGWINCH)
        wait("WORKSPACE")
        os.write(master, b"q")
        # Keep consuming PTY output while Bubble Tea flushes its final frame and
        # restores the terminal; waiting without reading can fill the PTY buffer.
        deadline = time.monotonic() + 5
        while proc.poll() is None and time.monotonic() < deadline:
            if select.select([master], [], [], 0.05)[0]:
                try:
                    os.read(master, 65536)
                except OSError:
                    break
        proc.wait(timeout=0.2)
        assert proc.returncode == 0, proc.returncode
        print(f"PTY smoke PASS: navigation, selection, palette, themes, disabled actions, "
              f"sidebars, Blob fixture, resize, clean exit ({time.monotonic()-started:.2f}s)")
    finally:
        if proc.poll() is None:
            proc.kill()
            proc.wait(timeout=5)
        os.close(master)


if __name__ == "__main__":
    main()
