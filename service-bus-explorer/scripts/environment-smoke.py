#!/usr/bin/env python3
"""Real offline PTY environment workflow; run after make build (requires pyte)."""
import codecs
import fcntl
import json
import os
import pty
import select
import signal
import struct
import subprocess
import tempfile
import termios
import time

import pyte


class Terminal:
    def __init__(self, config, cols=160, rows=48):
        self.master, slave = pty.openpty()
        self.screen = pyte.Screen(cols, rows)
        self.stream = pyte.Stream(self.screen)
        self.decoder = codecs.getincrementaldecoder("utf-8")("replace")
        fcntl.ioctl(self.master, termios.TIOCSWINSZ, struct.pack("HHHH", rows, cols, 0, 0))
        self.proc = subprocess.Popen(
            ["./bin/sbx", "--fixture", "--config", config],
            stdin=slave, stdout=slave, stderr=slave, start_new_session=True,
            env=dict(os.environ, TERM="xterm-256color"))
        os.close(slave)

    def wait(self, want, absent=None):
        deadline = time.monotonic() + 8
        while time.monotonic() < deadline:
            if select.select([self.master], [], [], 0.05)[0]:
                try:
                    self.stream.feed(self.decoder.decode(os.read(self.master, 65536)))
                except OSError:
                    break
            text = "\n".join(self.screen.display)
            if want in text and (absent is None or absent not in text):
                return text
        raise AssertionError(f"Expected {want!r}, absent {absent!r}\n" + "\n".join(self.screen.display))

    def send(self, keys, want, absent=None):
        os.write(self.master, keys.encode())
        return self.wait(want, absent)

    def action(self, query, want):
        self.send("\x10", "Ctrl+P / ACTIONS")
        self.send(query, "> " + query + "_")
        return self.send("\r", want, "Ctrl+P / ACTIONS")

    def resize(self, cols, rows, want):
        self.screen.resize(rows, cols)
        fcntl.ioctl(self.master, termios.TIOCSWINSZ, struct.pack("HHHH", rows, cols, 0, 0))
        os.kill(self.proc.pid, signal.SIGWINCH)
        return self.wait(want)

    def exit(self, keys="q"):
        os.write(self.master, keys.encode())
        deadline = time.monotonic() + 5
        while self.proc.poll() is None and time.monotonic() < deadline:
            if select.select([self.master], [], [], 0.05)[0]:
                try:
                    os.read(self.master, 65536)
                except OSError:
                    break
        assert self.proc.wait(timeout=2) == 0

    def close(self):
        if self.proc.poll() is None:
            self.proc.terminate()
            try:
                self.proc.wait(timeout=3)
            except subprocess.TimeoutExpired:
                self.proc.kill()
                self.proc.wait(timeout=2)
        os.close(self.master)


def saved(path):
    with open(path, encoding="utf-8") as file:
        return json.load(file)


def environment(data, name):
    return next(e for e in data["environments"] if e["name"] == name)


def main():
    with tempfile.TemporaryDirectory(prefix="sbx-environments-") as directory:
        path = os.path.join(directory, "preferences.json")
        t = Terminal(path)
        try:
            t.wait("Offline sample data")
            t.action("Save preferences", "Preferences saved")
            baseline = saved(path)
            t.action("New environment", "Name: [ _ ]")
            t.send("Production", "Name: [ Production_ ]")
            t.send("\t\r", "Choose Azure subscription")
            t.send("production", "Sample production / sample-production")
            t.send("\r", "rg-payments / prod-payments")
            t.send("rg-core", "Preview 2 listed namespaces in rg-core")
            t.send("\x1b[B\x1b[B", "› [ Preview 2 listed namespaces")
            t.send("\r", "Add only these 2 listed namespaces")
            t.wait("prod-core / rg-core / sample-production")
            t.wait("prod-archive / rg-core / sample-production")
            t.send("\r", "2 selected across opened subscriptions")
            t.send("\x1b", "Search: _")  # clear filter, then return to form
            t.send("\x1b", "Review 2 selected namespaces")
            t.send("\t\t\r", "Add known namespace / no broad enumeration")
            t.send("prod-notifications", "prod-notifications_")
            t.send("\r", "Review 3 selected namespaces")
            t.send("\t\r", "Review environment / Production")
            t.wait("prod-notifications / rg-shared / sample-shared")
            t.send("a", "Environment applied to this session")
            assert saved(path) == baseline, "Apply unexpectedly saved preferences"
            t.send("\x1b", "14 watched entities")
            t.action("Switch environment", "Search: _")
            t.send("Production", "› Production / 3 namespaces")
            t.send("\r", "0 watched entities")
            t.action("Dashboard inclusion", "Mode: Selected entities")
            t.wait("[ ] prod-core / orders")
            t.send("/", "Search: _")
            for key, query in (("q", "q"), ("j", "qj"), ("k", "qjk")):
                t.send(key, "Search: " + query + "_")
            assert t.proc.poll() is None, "search q quit the process"
            t.send("\x15prod-core\r", "3 matching results")
            t.send("a", "[x] prod-core / orders")
            t.send(" ", "[ ] prod-core / orders")
            t.send("s", "Preferences saved")
            production = environment(saved(path), "Production")
            assert not production["all"] and len(production["watchlist"]) == 2
            assert {r["Namespace"] for r in production["watchlist"]} == {"prod-core"}
            assert {n["Address"] for n in production["namespaces"]} == {
                "prod-core", "prod-archive", "prod-notifications"}
            assert {n["AzureSubscriptionID"] for n in production["namespaces"]} == {
                "sample-production", "sample-shared"}
            t.send("\x1b", "Search:  ·")
            t.send("\x1b", "2 watched entities")
            t.action("New environment", "Name: [ _ ]")
            # j/k/q are text in a name field, not global navigation or quit.
            for key, name in (("q", "q"), ("j", "qj"), ("k", "qjk")):
                t.send(key, "Name: [ " + name + "_ ]")
            t.send("\x15Staging", "Name: [ Staging_ ]")
            t.send("\t\t\t\r", "Add known namespace / no broad enumeration")
            t.send("stage-core\r", "Review 1 selected namespaces")
            t.send("\t\r", "Review environment / Staging")
            t.send("\r", "0 watched entities")
            t.action("Dashboard inclusion", "Mode: Selected entities")
            t.send("a", "[x] stage-core / orders")
            t.send("s", "Preferences saved")
            staging = environment(saved(path), "Staging")
            assert len(staging["watchlist"]) == 3 and not staging["all"]
            assert {r["Namespace"] for r in staging["watchlist"]} == {"stage-core"}
            assert environment(saved(path), "Production") == production
            t.exit()
        finally:
            t.close()

        # Restart on Staging, then switch to Production: disk-backed isolation.
        t = Terminal(path, 80, 24)
        try:
            t.wait("3 watched entities")
            t.action("Dashboard inclusion", "[x] stage-core / orders")
            t.wait("Mode: Selected entities", "prod-core")
            t.send("\x1b", "3 watched entities")
            t.action("Switch environment", "Search: _")
            t.send("Production\r", "2 watched entities")
            t.action("Dashboard inclusion", "[ ] prod-core / orders")
            t.wait("[x] prod-core / payments", "stage-core")
            t.send("s", "Preferences saved")
            t.send("\x1b", "2 watched entities")
            t.resize(60, 18, "q quit")
            t.send("t", "TABLE")
            t.resize(50, 10, "Resize terminal")
            t.send("q", "Unsaved changes")
            t.exit("d")
            assert environment(saved(path), "Production") == production
            assert environment(saved(path), "Staging") == staging
        finally:
            t.close()
        t = Terminal(path, 50, 10)
        try:
            t.wait("Resize terminal")
            t.exit()  # clean undersized startup must also remain safely quittable
        finally:
            t.close()
    print("PASS: real fixture PTY — group snapshot, cross-subscription known namespace, Apply != Save, empty Selected, filtered bulk inclusion, input ownership, isolated environments/save/restart, 80x24/60x18/undersized quit")


if __name__ == "__main__":
    main()
