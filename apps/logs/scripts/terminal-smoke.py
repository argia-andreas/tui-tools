#!/usr/bin/env python3
"""Exercise the real offline logs executable through a terminal, no mocks."""
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
    private = tempfile.TemporaryDirectory(prefix="logs-terminal-")
    searches = os.path.join(private.name, "searches.json")
    settings = os.path.join(private.name, "settings.json")
    for args in ([], ["--live"], ["--fixture", "unexpected"]):
        result = subprocess.run(["./bin/logs", *args], capture_output=True, text=True)
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
    proc = subprocess.Popen(["./bin/logs", *args], stdin=slave, stdout=slave,
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

    def click_text(label, want, absent=None):
        rows = list(enumerate(screen.display))
        if label == "[x]":
            rows.reverse()  # Foreground Help may cover a parent Surface close.
        for y, line in rows:
            x = line.find(label)
            if x >= 0:
                # SGR press and release exercise the actual terminal decoder.
                return send(f"\x1b[<0;{x+1};{y+1}M\x1b[<0;{x+1};{y+1}m", want, absent)
        raise AssertionError("Visible control missing: " + label)

    def usability_scenarios():
        def painted_body(first, last):
            for y in range(first, last):
                for x in range(screen.columns):
                    assert screen.buffer[y][x].bg != "default", (
                        "terminal-default pane cell", x, y, screen.display[y])

        # Check the actual terminal cell colors, not ANSI substring presence.
        # A contrasting terminal default (e.g. gray) must never show through.
        painted_body(4, screen.lines - 4)
        assert any(c.bg == "8af0ce" for row in screen.buffer.values()
                   for c in row.values()), "selected log surface lost"
        resize(61, 19); os.kill(proc.pid, signal.SIGWINCH)
        text = wait("payment-worker Payment")
        assert "APP / SERVICE  MESSAGE" in text
        assert all(len(line) == 61 for line in screen.display)
        painted_body(4, screen.lines - 4)
        send("f", "NORMAL: i type · Space check app")
        text = send("ipayment", "[ ] payment-worker")
        for hint in ("Selected: ALL (none checked)",
                     "Apply: ALL apps; keyword ignored",
                     "Keyword: suggestions only",
                     "INSERT: keyword only · Esc/Enter normal"):
            assert hint in text, (hint, text)
        painted_body(2, screen.lines - 2)
        keyword_row = next(y for y, line in enumerate(screen.display)
                           if "  payment" in line)
        assert any(c.bg == "101d2f" for c in screen.buffer[keyword_row].values()), (
            "intentional keyword input surface lost")
        send("\x1b", "NORMAL: i type · Space check app")
        send(" ", "Apply: checked apps only")
        send(" ", "Apply: ALL apps; keyword ignored")
        send("\x13", "App exact: ALL")
        wait("22 " + ("lab events" if lab else "fictional events"))
        resize(160, 48); os.kill(proc.pid, signal.SIGWINCH)
        wait("WORKSPACE")
        print("PASS: usability PTY — 61-column service delimiter/geometry, keyword not checked/applied, NORMAL/INSERT ownership, explicit pane-cell backgrounds, selected/input surfaces retained")

    def mouse_scenarios():
        assert b"\x1b[?1006h" in transcript, "default mouse reporting missing"
        click_text("Actions ctrl+p", "ACTIONS · Ctrl+P")
        send("qj?", "No results")
        send("\x15older", "No older history available")
        click_text("Load older history", "ACTIONS · Ctrl+P")
        # Outside dismissal over Inspect must not inspect as well.
        click_text("Inspect enter", "● LOGS", "ACTIONS · Ctrl+P")
        assert "● INSPECTOR" not in "\n".join(screen.display)
        click_text("Actions ctrl+p", "ACTIONS · Ctrl+P")
        send("inspect", "Inspect selected event")
        click_text("Help F1", "F1/Esc close")
        send("q\x10", "F1/Esc close")  # Captured, not quit or palette toggle.
        click_text("[x]", "Inspect selected event", "F1/Esc close")
        assert "inspect" in "\n".join(screen.display), "Help lost palette query"
        send("\x1bOP", "F1/Esc close")
        send("\x1b", "Inspect selected event", "F1/Esc close")
        click_text("Inspect selected event", "● INSPECTOR", "ACTIONS · Ctrl+P")
        send("\x1b", "● LOGS")
        click_text("Help f1", "F1/Esc close")
        click_text("[x]", "● LOGS", "F1/Esc close")
        print("PASS: mouse palette/Help PTY — reporting, Actions, text capture, unavailable row, consumed outside press, Inspect, single press/release, Help click/F1/Esc/close with palette query preserved and no quit leakage")

    def editor_pointer_scenarios():
        count = "lab events" if lab else "fictional events"
        send("f", "FILTER EDITOR")
        send("ijk", "No suggestions")
        # Cancel is immediate even in INSERT; it is not a synthetic Esc.
        click_text("[ Cancel", "22 " + count, "FILTER EDITOR")
        send("f", "FILTER EDITOR")
        assert "jk" not in "\n".join(screen.display)
        send("icheckout", "checkout-api")
        # Apply is independent of Enter's INSERT/toggle grammar.
        click_text("[ Apply", "22 " + count, "FILTER EDITOR")
        assert "App exact: ALL" in "\n".join(screen.display)
        send(",", "Vim-style facet input: ON (default)")
        click_text("Vim-style facet input", "Vim-style facet input: OFF")
        click_text("[ Cancel", "● LOGS", "PREFERENCES")
        assert not os.path.exists(settings), "pointer Cancel wrote settings"
        send("/", "LOCAL LITERAL FIND")
        send("SWEDENCENTRAL", "SWEDENCENTRAL")
        click_text("Case sensitive", "Case sensitive: true")
        click_text("[ Find", "No local literal matches", "LOCAL LITERAL FIND")
        send("/\x15\x14\r", "Local find empty")
        print("PASS: editor pointer PTY — direct INSERT Cancel/Apply, draft discarded, keyword not toggled, settings toggle/cancel without write, case-sensitive Find, single SGR press/release")

    def navigation_pointer_scenarios():
        click_text("POST /orders accepted", "● LOGS", "● INSPECTOR")
        click_text("Inspect enter", "● INSPECTOR", "● LOGS")
        click_text("[ Back esc ]", "● LOGS", "● INSPECTOR")
        click_text("Payment gateway timeout after", "● LOGS", "● INSPECTOR")
        click_text("Inspect enter", "● INSPECTOR")
        click_text("fields ▾", "token: [REDACTED]")
        click_text("▾", "fields ▸", "token: [REDACTED]")
        click_text("▸", "token: [REDACTED]")
        # Wheel is routed to the pointed reader without changing pane focus.
        send("\x1b[<65;140;15M", "● INSPECTOR")
        send("g", "POLICY-SAFE EVENT")
        click_text("[ Back esc ]", "● LOGS", "● INSPECTOR")
        resize(80, 24); os.kill(proc.pid, signal.SIGWINCH)
        wait("● LOGS", "WORKSPACE")
        click_text("Payment gateway timeout after", "● LOGS", "● INSPECTOR")
        click_text("Inspect enter", "● INSPECTOR", "● LOGS")
        click_text("[ Back esc ]", "● LOGS", "● INSPECTOR")
        resize(160, 48); os.kill(proc.pid, signal.SIGWINCH)
        wait("WORKSPACE")
        send("f\t\t", "›[ Cancel esc ]")
        send("\r", "● LOGS", "FILTER EDITOR")
        print("PASS: navigation pointer PTY — select-first rows, explicit Inspect/Back, separate disclosure once per press/release, reader-local wheel, current 80-column geometry and keyboard footer traversal")

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
        click_text("Actions", "Draft owns input")
        assert "ACTIONS · Ctrl+P" not in "\n".join(screen.display)
        send("\x10", "Draft owns input")
        click_text("Help f1", "F1/Esc close")
        assert "first Esc" in "\n".join(screen.display)
        click_text("[x]", "No suggestions", "F1/Esc close")
        send("\x1bOP", "F1/Esc close")
        send("\x1b", "No suggestions", "F1/Esc close")
        text = send("z", "jkz")
        assert "INSERT" in text, "Help did not return to INSERT"
        send("\x7f", "jk")
        print("PASS: INSERT Help PTY — disabled Actions click/Ctrl+P, visible Help click and F1, pointer close/Esc, retained INSERT draft and literal continuation")
        text = send("\x1b", "● Edit: App")
        assert "jk" in text, "INSERT Esc discarded keyword draft"
        text = send("j", "● Edit: App")
        assert "jk" in text and "jkj" not in text, "Esc did not return to NORMAL"
        send("\x1b", "22 " + count)
        send("f", "› [ ] checkout-api")
        assert "jk" not in "\n".join(screen.display), "cancel retained keyword"

        # Time navigation stays NORMAL; i owns literal Message input.
        send("\tj\x1b[Z", "› Start inclusive")
        send("j", "› End exclusive")
        send("k", "› Start inclusive")
        send("\x1b[B", "› End exclusive")
        send("\x1b[A", "› Start inclusive")
        send("\tjj\x1b[Z", "● Edit: Message")
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

        # Tab reaches sidebar then actions; Shift+Tab returns to editing.
        # Reverse/forward movement wraps, while Time owns Start/End movement.
        send("f", "FILTER EDITOR")
        send("\tk", "Edit: Exact fields")
        send("\x1b[A", "Edit: Trace IDs")
        send("j", "Edit: Exact fields")
        send("\x1b[B", "Edit: App")
        send("j\x1b[Z", "› Start inclusive")
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
        send("\tjj\x1b[Z\x15WARN+", "› Levels")
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
        click_text("/fields/job/attempt = 1", "FILTER FROM EVENT")
        text = click_text("[ Apply exact filter", "Search loaded", "FILTER FROM EVENT")
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
        send("\tk\x1b[Zq y ", "› Exact field predicates")
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
        send("\tj\x1b[Z\x152026-10-23T14:26:00", "› Start inclusive")
        send("\x13", "Start: RFC3339 requires")
        send("\x152026-10-23T16:26:00+02:00", "16:26:00+02:00")
        send("\tj\x1b[Z\x15UNKNOWN", "› Levels")
        text = send("\x13", "1 " + count)
        assert "Legacy record without severity" in text

        # Clear is draft-only until applied and retains the explicit time window.
        send("f", "FILTER EDITOR")
        send("\x0c", "Selected: ALL")
        send("\x1b", "1 " + count)
        send("f", "FILTER EDITOR")
        send("\x0c", "Selected: ALL")
        send("\tj\x1b[Z", "2026-10-23T14:26:00Z")
        send("\x13", "22 " + count)

        send("f", "FILTER EDITOR")
        send("\tjj\x1b[Z\x15ALL\tj\x1b[Zno-such-message-xyz", "› Message")
        send("\x13", "No events in this window")
        send("f", "FILTER EDITOR")
        send("\tjjj\x1b[Z\x15Payment gateway timeout", "› Message")
        send("\x13", "1 " + count)
        resize(80, 24); os.kill(proc.pid, signal.SIGWINCH)
        wait("● LOGS")
        send("f", "FILTER EDITOR")
        send("\tj\x1b[Z", "› Start inclusive")
        send("\x1b", "● LOGS")
        resize(160, 48); os.kill(proc.pid, signal.SIGWINCH)
        wait("q quit")

    try:
        text = wait("Local Elasticsearch" if lab else "Offline fictional data")
        assert "LOGS  /  " in text and "LOGVIEW" not in text, "app header rename missing"
        if lab:
            assert "22 lab events" in text and "ELASTICSEARCH · LOCAL LAB · TLS" in text
            assert "FIXTURE · OFFLINE" not in text and "No network access" not in text
        else:
            assert "22 fictional events" in text and "FIXTURE · OFFLINE" in text
        assert not os.path.exists(settings), "startup wrote default settings"
        assert "● LOGS" in text and "● INSPECTOR" not in text
        assert "Payment gateway timeout" in text and "[REDACTED]" in text
        assert "8c3e7a21b9f04450" in text
        usability_scenarios()
        mouse_scenarios()
        navigation_pointer_scenarios()
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
        send("\x10", "ACTIONS · Ctrl+P")
        send("q y \r", "No results")
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
        send("\x10", "ACTIONS · Ctrl+P")
        resize(80, 24); os.kill(proc.pid, signal.SIGWINCH)
        wait("ACTIONS · Ctrl+P")
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
            editor_pointer_scenarios()
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
        copy_scenarios(args + ["--mouse=false"], env)
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
    proc = subprocess.Popen(["./bin/logs", *args, "--trust-osc52"], stdin=slave,
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
        assert b"\x1b[?1006h" not in transcript and b"\x1b[?1002h" not in transcript, "--mouse=false enabled reporting"
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
