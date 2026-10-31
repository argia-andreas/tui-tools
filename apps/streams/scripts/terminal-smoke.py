#!/usr/bin/env python3
"""Real-binary PTY acceptance; fictional data and private disposable preferences."""
import codecs
import fcntl
import json
import os
from pathlib import Path
import pty
import re
import select
import struct
import subprocess
import tempfile
import termios
import time

import pyte

ROOT = Path(__file__).resolve().parents[1]


class Terminal:
    def __init__(self, directory, mouse=True, color=True):
        self.master, slave = pty.openpty()
        fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack("HHHH", 40, 160, 0, 0))
        self.screen = pyte.Screen(160, 40)
        self.stream = pyte.Stream(self.screen)
        self.decoder = codecs.getincrementaldecoder("utf-8")("replace")
        self.raw = b""
        env = {**os.environ, "TERM": "xterm-256color", "COLORTERM": "truecolor"}
        # The runner may disable colors globally; each PTY owns its color mode.
        for name in ("NO_COLOR", "CLICOLOR", "CLICOLOR_FORCE"):
            env.pop(name, None)
        if not color:
            env["NO_COLOR"] = "1"
        self.process = subprocess.Popen([str(ROOT / "bin/streams"), "--fixture",
            "--settings", str(Path(directory) / "settings.json"),
            f"--mouse={str(mouse).lower()}"], stdin=slave, stdout=slave, stderr=slave,
            env=env, close_fds=True)
        os.close(slave)

    def read(self, seconds=0.15):
        end = time.monotonic() + seconds
        while time.monotonic() < end:
            if select.select([self.master], [], [], 0.02)[0]:
                try:
                    data = os.read(self.master, 65536)
                except OSError:
                    break
                if not data:
                    break
                self.raw += data
                # Bubble Tea asks for cursor location during startup.
                if b"\x1b[6n" in data:
                    os.write(self.master, b"\x1b[1;1R")
                self.stream.feed(self.decoder.decode(data))
        return "\n".join(self.screen.display)

    def wait(self, text, regex=False):
        for _ in range(80):
            view = self.read()
            if (re.search(text, view) if regex else text in view):
                return view
            if self.process.poll() is not None:
                break
        raise AssertionError(f"Missing {text!r}:\n{view}")

    def send(self, data):
        os.write(self.master, data.encode())
        self.read()

    def close(self):
        if self.process.poll() is None:
            self.send("\x03")
        try:
            self.process.wait(timeout=5)
        finally:
            if self.process.poll() is None:
                self.process.terminate()
                self.process.wait(timeout=5)
            os.close(self.master)


def preview(t, offset):
    view = t.wait(rf"orders\.created\s*·\s*p0\s*/\s*offset\s+{offset}\b", regex=True)
    assert "INSPECTOR" in view and "PAYLOAD /" not in view, view
    t.wait(rf'"orderId"\s*:\s*"ord-{offset}"', regex=True)
    return view


def palette(t, label):
    t.send("\x10")
    t.wait("ACTIONS")
    t.send(label)
    t.wait(label)
    t.send("\r")


def density(t, label):
    return t.wait(rf"Committed only\s*·\s*arrival order\s*·\s*{label}\b", regex=True)


def json_colors(t):
    # Inspect rendered token cells, not just ANSI anywhere in the chrome.
    colors = []
    for token in ('"orderId"', '"ord-101"', '249.50'):
        for y, line in enumerate(t.screen.display):
            x = line.find(token)
            if x >= 0:
                foregrounds = {t.screen.buffer[y][i].fg for i in range(x, x + len(token))}
                assert len(foregrounds) == 1 and "default" not in foregrounds, (token, foregrounds)
                # Sample blank pane padding before the modal's right border.
                # Nested SGR resets must not expose the terminal background.
                padding = line.index("│", x + len(token)) - 1
                background = t.screen.buffer[y][padding].bg
                assert background != "default", (token, "pane background is default")
                backgrounds = {t.screen.buffer[y][i].bg for i in range(x, x + len(token))}
                assert backgrounds == {background}, (token, backgrounds, background)
                if token == '"orderId"':
                    separator = line.index(":", x + len(token)) + 1
                    assert line[separator] == " "
                    assert t.screen.buffer[y][separator].bg == background, "JSON separator exposes terminal background"
                colors.append(next(iter(foregrounds)))
                break
        else:
            raise AssertionError(f"Missing JSON token {token!r}")
    assert len(set(colors)) == 3, f"JSON keys, strings and numbers need distinct colors: {colors}"


def output_modes(t, mouse, color):
    if not mouse:
        assert not re.search(rb"\x1b\[\?(?:1000|1002|1003|1006|1015)h", t.raw)
    if not color:
        for sgr in re.findall(rb"\x1b\[([0-9;]*)m", t.raw):
            codes = {int(code) for code in sgr.split(b";") if code}
            color_codes = set(range(30, 39)) | set(range(40, 49)) | set(range(90, 98)) | set(range(100, 108))
            assert not codes.intersection(color_codes), sgr


def scenario(mouse, color):
    with tempfile.TemporaryDirectory(prefix="streams-pty-") as directory:
        t = Terminal(directory, mouse, color)
        try:
            t.wait("3 topics")
            if mouse:
                # Data row selects, never opens. Press/release pair.
                y, line = next((y, line) for y, line in enumerate(t.screen.display) if "orders.failed" in line)
                x = line.index("orders.failed") + 1
                t.send(f"\x1b[<0;{x};{y + 1}M\x1b[<0;{x};{y + 1}m")
                assert "EVENTS /" not in t.read()
                t.send("k")  # Return to orders.created after selecting orders.failed.
            t.send("\r")
            t.wait("18 loaded")
            compact = density(t, "Compact")
            preview(t, 100)
            t.send("j")
            preview(t, 101)
            t.send("k")
            preview(t, 100)
            t.send("j")
            preview(t, 101)
            t.send("\r")
            title = r"PAYLOAD / orders\.created\s*·\s*p0\s*/\s*offset\s+101\b"
            t.wait(title, regex=True)
            t.wait(r'"orderId"\s*:\s*"ord-101"', regex=True)
            if color:
                json_colors(t)
            # Payload navigation must scroll the modal, not change the event.
            for key in ("j", "j", "k", "k", "\x1b[6~", "\x1b[5~"):
                t.send(key)
                t.wait(title, regex=True)
            t.send("v")
            t.wait("raw · v changes mode")
            t.send("\x1b")
            t.wait("INSPECTOR")
            assert "PAYLOAD /" not in t.read()
            t.send("vv")  # raw -> hex -> JSON
            preview(t, 101)
            t.send("k")
            preview(t, 100)

            palette(t, "Toggle Compact / Comfortable event rows")
            comfortable = density(t, "Comfortable")
            # Comfortable rows visibly trade loaded-window capacity for space.
            keys = lambda view: set(re.findall(r"ord-\d+", view))
            assert 1 < len(keys(comfortable)) < len(keys(compact)), comfortable
            assert not (Path(directory) / "settings.json").exists(), "Density should save only on request"
            palette(t, "Save appearance preferences")
            t.wait("Appearance saved")
            assert json.loads((Path(directory) / "settings.json").read_text())["comfortable"] is True

            t.send("s\x15offset 2 303")
            t.wait("offset 2 303")
            t.send("\x1bOP")  # F1
            t.wait("HELP")
            t.send("\x1b")
            t.wait("offset 2 303")
            t.send("\r")
            t.wait("p2:306")
            t.send("F")
            t.wait("FOLLOWING")
            t.send(" ")
            t.wait("PAUSED")
            t.send("c")
            t.wait("FOLLOWING")
            t.send(" ")
            t.send("g")
            t.wait("checkout-workers")
            t.send("\r")
            t.wait("lag 84 (partial: false)")
            palette(t, "Toggle ASCII borders")
            palette(t, "Save appearance preferences")
            t.wait("Appearance saved")
            settings = json.loads((Path(directory) / "settings.json").read_text())
            assert settings["comfortable"] is True and settings["ascii"] is True, settings
            t.send("q")
            assert t.process.wait(timeout=5) == 0
            output_modes(t, mouse, color)
        finally:
            t.close()

        # A fresh process must read the actual saved preferences, not session state.
        t = Terminal(directory, mouse, color)
        try:
            t.wait("3 topics")
            t.send("\r")
            t.wait("18 loaded")
            density(t, "Comfortable")
            preview(t, 100)
            palette(t, "Toggle Compact / Comfortable event rows")
            density(t, "Compact")
            palette(t, "Save appearance preferences")
            t.wait("Appearance saved")
            assert json.loads((Path(directory) / "settings.json").read_text())["comfortable"] is False
            t.send("q")
            assert t.process.wait(timeout=5) == 0
            output_modes(t, mouse, color)
            print(f"PASS real PTY mouse={mouse} color={color}: compact default, j/k preview, payload/modal navigation/raw/Esc, JSON token colors/backgrounds={color}, density/save/restart, seek/help, follow/pause/resume, group lag, ASCII")
        finally:
            t.close()


if __name__ == "__main__":
    scenario(True, True)
    scenario(False, False)
