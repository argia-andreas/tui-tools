#!/usr/bin/env python3
"""Exercise the real offline logview executable through a terminal, no mocks."""
import codecs
import base64
import fcntl
import copy
import json
import os
import pty
import re
import select
import signal
import struct
import subprocess
import sys
import termios
import tempfile
import time

import pyte


def main():
    lab = "--lab" in sys.argv
    search = "--search" in sys.argv
    private = tempfile.TemporaryDirectory(prefix="logview-terminal-")
    searches = os.path.join(private.name, "searches.json")
    settings = os.path.join(private.name, "settings.json")
    for args in ([], ["--live"], ["--fixture", "unexpected"]):
        result = subprocess.run(["./bin/logview", *args], capture_output=True, text=True)
        assert result.returncode == 2, (args, result.returncode)
    master, slave = pty.openpty()
    screen = pyte.Screen(160, 48)
    stream = pyte.Stream(screen)
    decoder = codecs.getincrementaldecoder("utf-8")("replace")
    transcript = bytearray()

    def resize(cols, rows):
        fcntl.ioctl(master, termios.TIOCSWINSZ, struct.pack("HHHH", rows, cols, 0, 0))
        screen.resize(rows, cols)

    resize(160, 48)
    env = dict(os.environ, TERM="xterm-256color", COLORTERM="truecolor", CLICOLOR="1")
    env.pop("NO_COLOR", None)

    # This baseline exercises honest unavailability independent of the host's
    # display session. Deliberate OSC52 payload checks run separately below.
    env.pop("DISPLAY", None)
    env.pop("WAYLAND_DISPLAY", None)
    args = ["--config", ".lab/source.json"] if lab else ["--fixture"]
    args += ["--searches", searches, "--settings", settings]
    proc = subprocess.Popen(["./bin/logview", *args], stdin=slave, stdout=slave,
                            stderr=slave, env=env, start_new_session=True)
    os.close(slave)

    def wait(want, absent=None):
        deadline = time.monotonic() + 8
        while time.monotonic() < deadline:
            if select.select([master], [], [], 0.05)[0]:
                try:
                    data = os.read(master, 65536)
                except OSError:
                    break
                transcript.extend(data)
                stream.feed(decoder.decode(data))
            text = "\n".join(screen.display)
            if want in text and (absent is None or absent not in text):
                # A stable footer proves the whole frame has arrived.
                if time.monotonic() < deadline and select.select([master], [], [], 0.1)[0]:
                    continue
                return text
        raise AssertionError(f"Expected {want!r}, absent {absent!r}\n" + "\n".join(screen.display))

    def send(keys, want, absent=None):
        os.write(master, keys.encode())
        return wait(want, absent)

    def vim_scenarios():
        count = "lab events" if lab else "fictional events"
        assert not os.path.exists(settings), "startup/navigation wrote settings"

        # Settings are draft-only until explicitly saved, from either opener.
        send(",", "Vim-style facet input: ON (default)")
        send("\r", "Vim-style facet input: OFF")
        send("\x1b", "● LOGS")
        assert not os.path.exists(settings), "cancelled settings wrote preferences"
        send("\x10Settings", "› Settings")
        send("\r", "Vim-style facet input: ON (default)")
        send("\x1b", "● LOGS")

        # The right App pane starts in NORMAL: j/k and arrows move, not type.
        send("f", "● Edit: App")
        wait("› [ ] checkout-api")
        send("j", "› [ ] payment-worker")
        send("k", "› [ ] checkout-api")
        send("\x1b[B", "› [ ] payment-worker")
        send(" ", "Selected: payment-worker")
        send(" ", "Selected: ALL")
        send("\x1b[A", "› [ ] checkout-api")

        # Enter starts INSERT; j/k are literal keyword text, never selection.
        send("\r", "● Edit: App")
        text = send("jk", "No suggestions")
        assert "jk" in text and "Selected: ALL" in text
        text = send("\x1b", "● Edit: App")
        assert "jk" in text, "INSERT Esc discarded keyword draft"
        text = send("j", "● Edit: App")
        assert "jk" in text and "jkj" not in text, "Esc did not return to NORMAL"
        send("\x1b", "22 " + count)
        send("f", "› [ ] checkout-api")
        assert "jk" not in "\n".join(screen.display), "cancel retained keyword"

        # Time navigation stays NORMAL; i owns literal Message input.
        send("\tj\t", "› Start inclusive")
        send("j", "› End exclusive")
        send("k", "› Start inclusive")
        send("\x1b[B", "› End exclusive")
        send("\x1b[A", "› Start inclusive")
        send("\tjj\t", "● Edit: Message")
        text = send("ijk", "jk")
        assert "● Edit: Message" in text
        text = send("\r", "● Edit: Message")
        assert "jk" in text, "INSERT Enter discarded message draft"
        text = send("j", "● Edit: Message")
        assert "jk" in text and "jkj" not in text, "Enter did not return to NORMAL"
        text = send("i\t", "● FILTER EDITOR")
        assert "jk" in text, "Tab discarded INSERT draft"
        send("k", "Edit: Levels")  # Sidebar navigation, not inserted text.
        send("\x1b", "22 " + count)

        # Ctrl+S applies from either mode; an app keyword alone is no predicate.
        send("f", "● Edit: App")
        send("\x13", "22 " + count)
        wait("● LOGS")
        send("f", "● Edit: App")
        send("ijk", "No suggestions")
        send("\x13", "22 " + count)
        wait("● LOGS")
        assert not os.path.exists(settings), "filter actions wrote preferences"
        assert not os.path.exists(searches), "filter actions wrote saved searches"

        # Turn OFF explicitly before the unchanged direct-entry regression suite.
        send(",", "Vim-style facet input: ON (default)")
        send(" ", "Vim-style facet input: OFF")
        send("\x13", "Settings saved")
        wait("● LOGS")
        with open(settings, encoding="utf-8") as f:
            assert json.load(f) == {"vim_mode": False}
        assert os.stat(settings).st_mode & 0o077 == 0
        send(",", "Vim-style facet input: OFF")
        send("\x1b", "● LOGS")

    def search_scenarios():
        count = "lab events" if lab else "fictional events"
        # Both text editors own q/y/Space; cancelling neither quits nor writes.
        send("S", "SAVE NAMED SEARCH")
        send("q y ", "> q y ")
        send("\x1b", "● LOGS")
        assert not os.path.exists(searches)
        send("f", "FILTER EDITOR")
        send("q y ", "q y ")
        send("\x1b", "22 " + count)
        assert not os.path.exists(searches)
        send("S", "SAVE NAMED SEARCH")
        send("all\r", "Saved search explicitly")
        assert os.stat(searches).st_mode & 0o077 == 0
        send("\x1b[Z", "● WORKSPACE")
        send("\x1b[B", "› all")
        send("\r", "22 " + count)
        wait("● LOGS")

        # Tab and Shift+Tab switch panes; only the sidebar cycles facets.
        # Reverse/forward movement wraps, while Time owns Start/End movement.
        send("f", "FILTER EDITOR")
        send("\x1b[Zk", "Edit: Exact fields")
        send("\x1b[A", "Edit: Trace IDs")
        send("j", "Edit: Exact fields")
        send("\x1b[B", "Edit: App")
        send("j\t", "› Start inclusive")
        send("j", "› End exclusive")
        send("k", "› Start inclusive")
        send("\x1b[B", "› End exclusive")
        send("\x1b[A", "› Start inclusive")
        send("\t\x1b[Z", "› Start inclusive")
        send("\x1b", "22 " + count)

        # Keyword suggestions are NOT applied unless explicitly selected.
        send("f", "FILTER EDITOR")
        send("payment", "[ ] payment-worker")
        send("\r", "Selected: payment-worker")
        send("\x15checkout", "[ ] checkout-api")
        send("\r", "Selected: payment-worker OR checkout-api")
        send("\x13", "22 " + count)
        send("f", "FILTER EDITOR")
        send("\x18", "Selected: ALL")
        send("payment", "[ ] payment-worker")
        send("\r", "Selected: payment-worker")
        send("\tjj\t\x15WARN+", "› Levels")
        text = send("\x13", "Search loaded")
        assert "App exact: payment-worker" in text
        assert "WARN,ERROR,FATAL" in text
        assert "Legacy record without severity" not in text

        # Restore ALL, then exact app-only query for event-derived predicates.
        send("R", "RESTORE SAVED SEARCH")
        send("\r", "22 " + count)
        send("f", "FILTER EDITOR")
        send("payment", "[ ] payment-worker")
        send("\r", "Selected: payment-worker")
        send("\x13", "Search loaded")
        send("\r", "● INSPECTOR")
        send("f", "FILTER FROM EVENT")
        send("\x1b", "● INSPECTOR")
        send("f", "FILTER FROM EVENT")
        send("g", "› /trace.id")
        text = send("\r", "Search loaded")
        assert "/trace.id=" in text and "App exact: payment-worker" in text
        send("f", "FILTER FROM EVENT")
        send("g", "› /trace.id")
        send("j", "› /fields/job/attempt = 1")
        text = send("\r", "Search loaded")
        assert "/fields/job/attempt=1" in text and "App exact: payment-worker" in text
        send("S", "SAVE NAMED SEARCH")
        send("app trace attempt\r", "Saved search explicitly")
        with open(searches, encoding="utf-8") as f:
            saved = json.load(f)
        exact = saved[1]["Query"]
        assert exact["Scope"]["Services"] == ["payment-worker"]
        assert [f["Path"] for f in exact["Fields"]] == ["/trace.id", "/fields/job/attempt"]
        assert exact["Fields"][1]["Value"] == 1

        # Local literal find covers FULL safe structured data and explicit case.
        send("/", "LOCAL LITERAL FIND")
        send("q y ", "> q y ")
        send("\x15SWEDENCENTRAL\x14", "Case sensitive: true")
        send("\r", "No local literal matches")
        send("/\x14", "Case sensitive: false")
        send("\r", "Local literal match")
        send("n", "Local literal match")
        send("N", "Local literal match")
        send("\x1b", "● LOGS")
        send("R", "RESTORE SAVED SEARCH")
        send("\r", "22 " + count)
        send("R", "RESTORE SAVED SEARCH")
        send("\x1b[B\r", "/fields/job/attempt=1")
        send("\x1b", "● LOGS")
        send("f", "FILTER EDITOR")
        send("\tk\tq y ", "› Exact field predicates")
        send("\x1b[B\r", "1 exact field predicates active")
        text = send("\x13", "Search loaded")
        assert "/fields/job/attempt=1" not in text
        assert "/trace.id=" in text and "App exact: payment-worker" in text

        # A private test-only saved query uses page size 3: actual history with
        # the normal tiny dataset, not a mandatory 501+ seed or a UI mock.
        history = copy.deepcopy(saved[0])
        history["Name"] = "history page three"
        history["Query"]["Limits"]["Events"] = 3
        with open(searches, "w", encoding="utf-8") as f:
            json.dump([history], f)
        send("R", "RESTORE SAVED SEARCH")
        send("\r", "3 " + count)
        send("\r", "● INSPECTOR")
        text = send("o", "6 " + count)
        assert "Settlement batch completed" in text
        send("o", "9 " + count)
        send("\x1b", "● LOGS")

        # Empty, UNKNOWN, phrase and offset validation all exercise real reads.
        with open(searches, "w", encoding="utf-8") as f:
            json.dump([saved[0]], f)
        send("R", "RESTORE SAVED SEARCH")
        send("\r", "22 " + count)
        send("f", "FILTER EDITOR")
        send("\tj\t\x152026-10-23T14:26:00", "› Start inclusive")
        send("\x13", "Start: RFC3339 requires")
        send("\x152026-10-23T16:26:00+02:00", "16:26:00+02:00")
        send("\tj\t\x15UNKNOWN", "› Levels")
        text = send("\x13", "1 " + count)
        assert "Legacy record without severity" in text

        # Clear is draft-only until applied and retains the explicit time window.
        send("f", "FILTER EDITOR")
        send("\x0c", "Selected: ALL")
        send("\x1b", "1 " + count)
        send("f", "FILTER EDITOR")
        send("\x0c", "Selected: ALL")
        send("\tj\t", "2026-10-23T14:26:00Z")
        send("\x13", "22 " + count)

        send("f", "FILTER EDITOR")
        send("\tjj\t\x15ALL\tj\tno-such-message-xyz", "› Message")
        send("\x13", "No events in this window")
        send("f", "FILTER EDITOR")
        send("\tjjj\t\x15Payment gateway timeout", "› Message")
        send("\x13", "1 " + count)
        resize(80, 24); os.kill(proc.pid, signal.SIGWINCH)
        wait("● LOGS")
        send("f", "FILTER EDITOR")
        send("\tj\t", "› Start inclusive")
        send("\x1b", "● LOGS")
        resize(160, 48); os.kill(proc.pid, signal.SIGWINCH)
        wait("q quit")

    try:
        text = wait("Local Elasticsearch" if lab else "Offline fictional data")
        if lab:
            assert "22 lab events" in text and "ELASTICSEARCH · LOCAL LAB · TLS" in text
            assert "FIXTURE · OFFLINE" not in text and "No network access" not in text
        else:
            assert "22 fictional events" in text and "FIXTURE · OFFLINE" in text
        assert not os.path.exists(settings), "startup wrote default settings"
        assert "● LOGS" in text and "● INSPECTOR" not in text
        assert "Payment gateway timeout" in text and "[REDACTED]" in text
        assert "8c3e7a21b9f04450" in text
        send("\r", "● INSPECTOR", "● LOGS")
        send("j", "› level:")
        send("\x1b[A", "› time:")
        for field in ("level", "service", "instance", "trace.id", "message"):
            send("j", "› " + field + ":")
        send("\r", "Order 8042 remains pending;")
        for field in ("fields", "auth", "token", "duration_ms", "error", "stack_trace"):
            send("j", "/" + field + " · safe" if field in ("fields",) else field)
        send("\r", "/app/payments/worker/process.go:127")
        text = send("G", "region: swedencentral")
        assert "/app/payments/worker/process.go:127" in text
        assert "token: [REDACTED]" in text
        send("g", "POLICY-SAFE EVENT")
        send("y", "Copy unavailable")
        send("\x10", "COMMAND PALETTE")
        send("q y \r", "No matching implemented actions")
        send("\x15fltfld", "› Filter from selected event field")
        send("\r", "FILTER FROM EVENT")
        send("\x1b", "● INSPECTOR")
        send("\x10older\r", "No older history available")
        send("\x1b", "● INSPECTOR")
        send("\x10literal\r", "LOCAL LITERAL FIND")
        send("q y ", "> q y ")
        send("\x1b", "● INSPECTOR")
        send(" ", "Live follow is not available")
        send("\x1b", "● LOGS", "● INSPECTOR")
        send("j", "Retry queued with backoff", "Order 8042 remains pending;")
        send("k", "Order 8042 remains pending;")
        send("\x1b[Z", "● WORKSPACE", "● LOGS")
        # Narrowing while a disappearing panel owns focus must repair it.
        send("\x10", "COMMAND PALETTE")
        resize(80, 24); os.kill(proc.pid, signal.SIGWINCH)
        wait("COMMAND PALETTE")
        send("\x1b", "● LOGS", "WORKSPACE")
        wait("● LOGS", "WORKSPACE")
        assert "INSPECTOR" not in "\n".join(screen.display)
        send("j", "› 14:26:03.039")
        send("k", "› 14:26:03.038")
        send("\r", "● INSPECTOR", "● LOGS")
        send("G", "region: swedencentral")
        send("\x1b", "● LOGS", "INSPECTOR")
        send("G", "Heartbeat")
        send("g", "POST /orders accepted")
        resize(160, 48); os.kill(proc.pid, signal.SIGWINCH)
        wait("WORKSPACE")
        send("\r", "● INSPECTOR")
        resize(100, 30); os.kill(proc.pid, signal.SIGWINCH)
        wait("● LOGS", "INSPECTOR")
        send("j", "› 14:26:01.038")
        resize(40, 10); os.kill(proc.pid, signal.SIGWINCH)
        wait("Resize to at least 60")
        resize(160, 48); os.kill(proc.pid, signal.SIGWINCH)
        wait("q quit")
        if search:
            vim_scenarios()
            search_scenarios()
        else:
            assert not os.path.exists(searches), "ordinary navigation wrote saved searches"
            assert not os.path.exists(settings), "ordinary navigation wrote settings"
        os.write(master, b"q")
        proc.wait(timeout=5)
        assert proc.returncode == 0
        for forbidden in (b"fixture-secret-", b"\x1b]52;", b"https://invalid.example", "\u202e".encode(), b"Q0FOQVJZ"):
            assert forbidden not in transcript, repr(forbidden)
        copy_scenarios(args, env)
        print(f"PASS: {'local Elastic TLS' if lab else 'fixture'} PTY — explicit mode, 22 safe events, multiline inspector, focus, navigation, resize recovery, unavailable actions, clean exit")
        print("PASS: palette PTY — modal q/y/Space, fuzzy field action, disabled history, find handoff, focus restore and narrow resize, safe output")
        if search:
            print("PASS: Vim facets PTY — default read-only startup, settings cancel/openers/private OFF save, NORMAL app/Time navigation, Space toggle, Enter/i literal INSERT, Esc/Enter NORMAL, Tab sidebar, cancel, Ctrl+S from both modes")
            print("PASS: search PTY — pane/facet focus and wrap, Time Start/End, draft clear/cancel/apply with time retained, exact app OR, WARN+/UNKNOWN, offset validation, phrase/empty, safe structured literal find/case/n/N, inspector trace+typed field AND, explicit private save/restore, anchored older history, narrow editor")
    finally:
        if proc.poll() is None:
            proc.terminate()
            proc.wait(timeout=5)
        os.close(master)
        private.cleanup()


def copy_scenarios(args, env):
    master, slave = pty.openpty()
    fcntl.ioctl(master, termios.TIOCSWINSZ, struct.pack("HHHH", 48, 160, 0, 0))
    proc = subprocess.Popen(["./bin/logview", *args, "--trust-osc52"], stdin=slave,
                            stdout=slave, stderr=slave, env=env, start_new_session=True)
    os.close(slave)
    transcript = bytearray()

    def read_until(want):
        deadline = time.monotonic() + 8
        while time.monotonic() < deadline:
            if select.select([master], [], [], 0.1)[0]:
                transcript.extend(os.read(master, 65536))
            if want in transcript:
                return
        raise AssertionError("Copy PTY missing " + repr(want))

    def key(value, want):
        transcript.clear()
        os.write(master, value.encode())
        read_until(want.encode())

    def copied(value, expected):
        key(value, "Copy sent (OSC52)")
        payloads = re.findall(rb"\x1b\]52;c;([^\x07]*)\x07", transcript)
        assert len(payloads) == 1, payloads
        assert base64.b64decode(payloads[0]).decode() == expected
        assert b"receipt unverified" in transcript
        assert b"fixture-secret-" not in transcript

    try:
        read_until(b"Search loaded")
        key("\r", "INSPECTOR")
        for path in ("level", "service", "instance", "trace.id"):
            key("j", "/" + path + " ")
        copied("y", "8c3e7a21b9f04450")
        copied("Y", "/trace.id=8c3e7a21b9f04450")
        key("j", "/message ")
        copied("y", "Payment gateway timeout after 2000ms\nOrder 8042 remains pending; retry scheduled.")
        key("j", "/fields ")
        key("j", "/fields/auth ")
        copied("y", '{"token":"[REDACTED]"}')
        key("j", "/fields/auth/token ")
        copied("y", "[REDACTED]")
        transcript.clear()
        fcntl.ioctl(master, termios.TIOCSWINSZ, struct.pack("HHHH", 24, 80, 0, 0))
        os.kill(proc.pid, signal.SIGWINCH)
        read_until(b"LOGS")
        key("\r", "INSPECTOR")  # Hidden-panel repair returns to logs; Enter opens full-width.
        copied("Y", "/fields/auth/token=[REDACTED]")
        key("f", "FILTER FROM EVENT")
        key("\r", "redacted value")
        key("\x1b", "INSPECTOR")
        os.write(master, b"q"); proc.wait(timeout=5)
        assert proc.returncode == 0
        print("PASS: copy PTY — exact y/Y OSC52 payloads, full multiline value, stable redacted object, selected f, 160×48→80×24 recovery; transmission only, receiving clipboard NOT verified")
    finally:
        if proc.poll() is None:
            proc.terminate(); proc.wait(timeout=5)
        os.close(master)


if __name__ == "__main__":
    main()
