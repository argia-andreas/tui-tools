#!/usr/bin/env python3
"""Exercise the real fixture or seeded-emulator binary through a PTY."""
import argparse
import base64
import codecs
import fcntl
import os
import pty
import re
import select
import signal
import struct
import subprocess
import termios
import tempfile
import time

import pyte


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--emulator", action="store_true", help="requires the disposable lab already seeded")
    parser.add_argument("--mouse-off", action="store_true", help="exercise keyboard fallback without application mouse reporting")
    args = parser.parse_args()
    rejected = subprocess.run(["./bin/sbx"], capture_output=True, text=True)
    assert rejected.returncode == 2 and "--fixture" in rejected.stderr
    rejected = subprocess.run(["./bin/sbx", "--fixture", "--emulator"], capture_output=True, text=True)
    assert rejected.returncode == 2
    master, slave = pty.openpty()
    slave_name = os.ttyname(slave)
    preferences = tempfile.TemporaryDirectory(prefix="sbx-terminal-")
    config_path = os.path.join(preferences.name, "preferences.json")
    screen = pyte.Screen(160, 48)
    stream = pyte.Stream(screen)
    decoder = codecs.getincrementaldecoder("utf-8")("replace")

    def resize(cols, rows):
        fcntl.ioctl(master, termios.TIOCSWINSZ, struct.pack("HHHH", rows, cols, 0, 0))
        screen.resize(rows, cols)

    resize(160, 48)
    env = dict(os.environ, TERM="xterm-256color", COLORTERM="truecolor", CLICOLOR="1")
    env["SSH_CONNECTION"] = "synthetic-terminal-test"  # Exercise OSC52, not the orb desktop clipboard.
    env.pop("NO_COLOR", None)
    command = ["./bin/sbx", "--emulator" if args.emulator else "--fixture", "--config", config_path]
    if args.mouse_off:
        command.append("--mouse=false")
    proc = subprocess.Popen(command, stdin=slave, stdout=slave,
                            stderr=slave, env=env, start_new_session=True)
    os.close(slave)
    output = bytearray()

    def wait(want, absent=None):
        deadline = time.monotonic() + 8
        while time.monotonic() < deadline:
            if select.select([master], [], [], 0.05)[0]:
                try:
                    data = os.read(master, 65536)
                except OSError:
                    break
                output.extend(data)
                stream.feed(decoder.decode(data))
            text = "\n".join(screen.display)
            matched = want(text) if callable(want) else (want.search(text) if isinstance(want, re.Pattern) else want in text)
            if matched and (absent is None or absent not in text):
                # Bubble Tea may stream one redraw across several PTY reads.
                if not select.select([master], [], [], 0.05)[0]:
                    return text
        raise AssertionError(f"Expected {want!r}, absent {absent!r}\n" + "\n".join(screen.display))

    def send(keys, want, absent=None):
        os.write(master, keys.encode())
        return wait(want, absent)

    def focused(title):
        def matches(text):
            borders = {}
            for y, row in enumerate(screen.display):
                for pane in ("WORKSPACE", "orders / DEAD-LETTER", "INSPECTOR"):
                    x = row.find(("┌ " if pane == "INSPECTOR" else "┌─ ") + pane)
                    if x >= 0:
                        borders[pane] = screen.buffer[y][x].fg == "65c7f7"
            footer_ready = ("h/l collapse/expand" in text) == (title == "WORKSPACE")
            return footer_ready and len(borders) == 3 and all(active == (pane == title) for pane, active in borders.items())
        return matches

    def action(query, want):
        send("\x10", "ACTIONS · Ctrl+P")
        send(query, "Query: " + query)
        return send("\r", want, "ACTIONS · Ctrl+P")

    def click(label, want, absent=None, min_x=0):
        # The foreground Help close can coexist with a docked review close.
        rows = list(enumerate(screen.display))
        if label == "[x]":
            rows.reverse()
        y, row = next((y, row) for y, row in rows if row.find(label, min_x) >= 0)
        x = row.index(label, min_x)
        os.write(master, f"\x1b[<0;{x+1};{y+1}M\x1b[<0;{x+1};{y+1}m".encode())
        return wait(want, absent)

    def dlq_review():
        # A non-first highlighted row, not receive-next, is the resend source.
        for cleanup in (False, True):
            text = wait(re.compile(r"› \d+\s+orders-dlq-\d{4}"))
            previous_id = re.search(r"› \d+\s+(orders-dlq-\d{4})", text).group(1)
            text = send("j", re.compile(r"› \d+\s+(?!" + re.escape(previous_id) + r")orders-dlq-\d{4}"))
            selected_id = re.search(r"› \d+\s+(orders-dlq-\d{4})", text).group(1)
            send("\r", "INSPECTOR / NORMAL")
            wait("ID: " + selected_id)
            send("y", "y… press y")
            send("y", "Copy requested")
            copies = re.findall(rb"\x1b\]52;c;([^\x07\x1b]*)", output)
            assert copies and base64.b64decode(copies[-1]) == b"{\n", "yy copied decorations or wrong logical line"
            send("V", "VISUAL LINE")
            send("j", "2 lines selected")
            send("y", "Copy requested · 2 lines")
            copies = re.findall(rb"\x1b\]52;c;([^\x07\x1b]*)", output)
            assert base64.b64decode(copies[-1]).startswith(b'{\n  "amount":'), "visual-line copy lost source text"
            send("Y", "Copy requested · full body")
            copies = re.findall(rb"\x1b\]52;c;([^\x07\x1b]*)", output)
            assert base64.b64decode(copies[-1]).endswith(b"}"), "Y did not copy the whole body"
            if not args.mouse_off and not args.emulator:
                # Explicit Back returns directly, then reopen the same reader.
                click("Esc Back", "orders / DEAD-LETTER")
                send("\r", "INSPECTOR / NORMAL")
                text = click("^R Resend to queue…", "Destination queue: orders")
            else:
                text = send("\x12", "Destination queue: orders")
            assert "CHOOSE QUEUE" not in text and "Change queue" not in text
            wait("[ ] Delete original after confirmed resend")
            # Help is view-only even over configuration; blocked Actions cannot
            # become a palette or write printable keys into the parent.
            if not args.emulator:
                wait("Actions - Review active")
                send("\x10", "Destination queue: orders", "Query:")
                send("\x1bOP", " HELP ")
                send("q\r\x10", " HELP ")
                send("\x1b", "Destination queue: orders", " HELP ")
            if not args.mouse_off and not args.emulator:
                if cleanup:
                    click("[ ] Delete original", "[x] Delete original after confirmed resend")
                click("Review resend", "Resend destination: orders")
            elif cleanup:
                send("\t\t\r", "[x] Delete original after confirmed resend")
                send("\x1b[A", "›[ Review resend ]")
                send("\r", "Resend destination: orders")
            else:
                send("\t", "›[ Review resend ]")
                send("\r", "Resend destination: orders")
            wait("ID: " + selected_id)
            if cleanup:
                wait("EXACT SELECTED ORIGINAL")
            else:
                text = wait("SELECTED PEEK COPY")
                assert "Lock until:" not in text
            if not args.mouse_off and not args.emulator:
                click("Resend copy", "cleanup confirmed" if cleanup else "no cleanup requested")
            else:
                send("\t", "›[ Resend copy ]")
                send("\r", "cleanup confirmed" if cleanup else "no cleanup requested")
            send("\r", "Snapshot retained")
            send("\x1b", "orders / DEAD-LETTER")
            send("r", re.compile(r"› \d+\s+orders-dlq-0001"))
            wait("PEEK ONLY", selected_id if cleanup else None)
            if not cleanup:
                wait(selected_id)
        action("receive next", "ACTUAL RECEIVED MESSAGE")
        text = wait("›[ Cancel / release ]")
        assert re.search(r"ID: orders-dlq-\d{4}", text)
        assert "[ ] Remove original" in text
        if not args.emulator:
            if args.mouse_off:
                send("\x1bOP", " HELP ")
            else:
                click("Help f1", " HELP ")
            send("q\r\x10", " HELP ")
            if args.mouse_off:
                send("\x1b", "›[ Cancel / release ]", " HELP ")
            else:
                click("[x]", "›[ Cancel / release ]", " HELP ")
        if not args.mouse_off and not args.emulator:
            click("Cancel / release", "Lock release confirmed")
        else:
            send("\r", "Lock release confirmed")
        send("\r", "orders / DEAD-LETTER")
        action("receive next", "ACTUAL RECEIVED MESSAGE")
        retained_id = re.search(r"ID: (orders-dlq-\d{4})", wait("New MessageID:")).group(1)
        send("\t", "›[ Resend copy ]")
        send("\r", "no cleanup requested")
        wait("Lock release confirmed")
        send("\r", "orders / DEAD-LETTER")
        wait(retained_id)  # resend must not implicitly delete the acquired original
        action("receive next", "ACTUAL RECEIVED MESSAGE")
        deleted_id = re.search(r"ID: (orders-dlq-\d{4})", wait("New MessageID:")).group(1)
        send("\t\t\t", "›[ ! Delete original ]")
        send("\r", "type delete: _")
        send("delx\x08\x0c", "type delete: del_")
        send("\x08\x08\x08", "type delete: _")
        send("delete", "type delete: delete_")
        send("\r", "Deletion confirmed")
        send("\r", "orders / DEAD-LETTER")
        wait("PEEK ONLY", deleted_id)

    try:
        if args.emulator:
            text = wait("Emulator connected")
            assert "EMULATOR · LOCAL" in text and "FIXTURE" not in text
            assert "runtime counts unavailable" in text and "— queued" in text
            send("t", "events/billing")
            for _ in range(14):
                if "› orders " in "\n".join(screen.display):
                    break
                selected_row = next(row for row in screen.display if "› " in row and "Dashboard" not in row)
                send("j", "TABLE", selected_row)
            else:
                raise AssertionError("orders not selectable in discovered entities")
            send("\r", "orders / ACTIVE")
            wait("orders-active-0001")
            send("n", "orders-active-0021", "orders-active-0001")
            send("r", "orders-active-0021", "orders-active-0041")
            send("p", "orders-active-0001", "orders-active-0021")
            send("]", "orders / DEAD-LETTER")
            wait("orders-dlq-0001")
            assert "PEEK ONLY" in "\n".join(screen.display)
            dlq_review()
            send("[", "orders-active-0001", "orders-dlq-0001")
            resize(80, 24)
            os.kill(proc.pid, signal.SIGWINCH)
            wait("q quit", "WORKSPACE")
            send("q", "Unsaved changes / Quit?")
            wait("›[ Back / Cancel ]")
            send("\r", "q quit", "Unsaved changes / Quit?")
            assert proc.poll() is None, "default quit confirmation did not cancel"
            send("q", "Unsaved changes / Quit?")
            send("\x1b[Z", "›[ Discard & quit ]")
            os.write(master, b"\r")
            proc.wait(timeout=5)
            assert proc.returncode == 0
            print("PASS: real emulator PTY — discovery, unavailable counts, paging/refresh, selected copy to queue, exact-selected cleanup, receive-next/review, cancel/release, retained original, typed delete, resize, clean exit")
            return
        # The footer arrives after the complete frame; matching the first card
        # could return while the PTY is still streaming the remaining rows.
        wait("Offline sample data")
        text = wait("128 queued")
        assert "FIXTURE · OFFLINE" in text and "WORKSPACE" in text
        modes = [sequence in output for sequence in (b"\x1b[?1002h", b"\x1b[?1006h")]
        assert modes == [not args.mouse_off] * 2, "mouse reporting did not follow --mouse flag"
        if not args.mouse_off:
            # A card/table/message row selects only. Explicit controls open the
            # current native identity, not the first or previously painted row.
            click("payments", "Selected 2/14", "payments / ACTIVE", min_x=24)
            click("Open Active", "payments / ACTIVE")
            wait("payments-active-0001")
            send("i", "payments-active-0001", "INSPECTOR")
            click("Next n", "payments-active-0021", "payments-active-0001")
            click("Previous p", "payments-active-0001", "payments-active-0021")
            send("i", "INSPECTOR")
            send("\x1b", "Selected 2/14")
            click("Table t", "TABLE")
            click("shipping", "Selected 5/14", "shipping / DEAD-LETTER", min_x=24)
            click("Peek DLQ", "shipping / DEAD-LETTER")
            wait("shipping-dlq-0002")
            click("shipping-dlq-0002", "› 107", min_x=24)
            # If the click had inspected, j would move the reader, not the list.
            send("j", "› 114")
            click("Inspect / Open", "ID: shipping-dlq-0003")
            click("Esc Back", "shipping / DEAD-LETTER")
            click("Active [", "shipping-active-0001")
            click("DLQ ]", "shipping-dlq-0001")
            click("Refresh r", "PEEK ONLY")
            send("\x1b", "TABLE")
            # Disclosure is independent from its label/data selection.
            click("fixture / local-lab", "▾ Queues")
            click("▾ fixture", "TABLE", "▾ Queues")
            click("▸ fixture", "▾ Queues")
            click("Settings", "Color theme: midnight")
            click("Navigation sidebar: Expanded", "Navigation sidebar: Collapsed", "WORKSPACE")
            click("Navigation sidebar: Collapsed", "WORKSPACE")
            click("Message inspector: Visible", "Message inspector: Hidden")
            click("Message inspector: Hidden", "Message inspector: Visible")
            # Environment inclusion replaces the old inline Settings watch rows.
            click("Dashboard inclusion", "Mode: All entities")
            send("\x1b", "Color theme: midnight")
            click("Dashboard", "TABLE")
            send("kk", "Selected 1/14")
            click("Grid g", "128 queued")
        if args.mouse_off:
            send("\x10", "ACTIONS · Ctrl+P")
        else:
            click("Actions ctrl+p", "ACTIONS · Ctrl+P")
        send("settings", "Query: settings")
        if args.mouse_off:
            send("\x1bOP", " HELP ")
        else:
            click("Help F1", " HELP ")
        send("qj?\x10\r", " HELP ")
        assert proc.poll() is None, "captured Help input quit"
        resize(100, 30)
        os.kill(proc.pid, signal.SIGWINCH)
        wait(" HELP ")
        if args.mouse_off:
            send("\x1b", "Query: settings", " HELP ")
        else:
            click("[x]", "Query: settings", " HELP ")
        if args.mouse_off:
            send("\r", "Color theme:", "ACTIONS · Ctrl+P")
        else:
            click("Open Settings", "Color theme:", "ACTIONS · Ctrl+P")
        send("\x10", "ACTIONS · Ctrl+P")
        send("\x10", "Color theme:", "ACTIONS · Ctrl+P")
        send("\x1b", "128 queued")
        resize(160, 48)
        os.kill(proc.pid, signal.SIGWINCH)
        text = wait("Counts: available")
        for label in ("orders", "payments", "notifications", "inventory", "shipping",
                      "invoices", "audit-log", "email-outbox", "webhook-dispatch",
                      "reports", "customer-sync", "empty"):
            assert label in text, label
        assert "12 dead-letter" in text
        assert "…│" not in text, "grid gutters overflow the panel"
        count_row = next(y for y, row in enumerate(screen.display) if "128 queued" in row)
        count_text = screen.display[count_row]
        left = count_text.index("128 queued")
        right = count_text.index("dead-letter", left) + len("dead-letter")
        background = screen.buffer[count_row][left].bg
        assert all(screen.buffer[count_row][x].bg == background for x in range(left, right)), "unstyled spaces break the count-row background"
        send("t", "events/billing")
        send("g", "128 queued")
        send("b", "128 queued", "WORKSPACE")
        send("b", "WORKSPACE")
        send("]", "MaxDeliveryCountExceeded")
        wait('"currency": "EUR"')
        assert "PEEK ONLY" in "\n".join(screen.display)
        # Literal terminal control bytes: both edges clamp, never wrap.
        send("\x08\x08", focused("WORKSPACE"))
        send("\x0c", focused("orders / DEAD-LETTER"))
        send("\x0c", focused("INSPECTOR"))
        send("\x0cj\x08j", "› 107")
        send("k", "› 100")
        send("b", "INSPECTOR", "WORKSPACE")
        send("\x08\x08j", "› 107")
        send("k", "› 100")
        send("b", "WORKSPACE")
        send("j", "› 107")
        send("[", "orders-active-0001", "MaxDeliveryCountExceeded")
        send("n", "orders-active-0021", "orders-active-0001")
        # Refresh must not advance to the third page.
        send("r", "orders-active-0021", "orders-active-0041")
        send("p", "orders-active-0001", "orders-active-0021")
        send("i", "orders-active-0001", "INSPECTOR")
        send("i", "INSPECTOR")
        send("\x1b", "128 queued", "INSPECTOR")
        send("\x08", "WORKSPACE")
        send("j", "› Queues")
        send("j", "› Topics")
        send("\r", "Topic subscriptions")
        wait("events/shipping")
        send("\r", "events/billing / ACTIVE")
        send("\x1b", "Topic subscriptions")
        send("\x1b", "128 queued")
        send("\x10", "ACTIONS · Ctrl+P")
        send("qjk", "No results")
        assert proc.poll() is None, "palette query q quit the process"
        send("\x08\x0c", "Query: qj")
        send("\x1b", "128 queued", "ACTIONS · Ctrl+P")
        # Distinct palette colors are observable terminal output, not just names.
        action("gruvbox", "128 queued")
        row = next(y for y, text in enumerate(screen.display) if "128 queued" in text)
        col = screen.display[row].index("128 queued")
        assert screen.buffer[row][col].bg == "282828"
        action("light", "128 queued")
        row = next(y for y, text in enumerate(screen.display) if "128 queued" in text)
        col = screen.display[row].index("128 queued")
        assert screen.buffer[row][col].bg == "f6f8fb"
        action("midnight", "128 queued")
        # Dirty normal-workspace quit is a shared cancel-first confirmation,
        # not an immediate exit or a text field that consumes the q shortcut.
        send("q", "Unsaved changes / Quit?")
        wait("›[ Back / Cancel ]")
        send("\r", "128 queued", "Unsaved changes / Quit?")
        assert proc.poll() is None, "default quit confirmation did not cancel"
        # A focused sidebar must not trap keys after it auto-hides on resize.
        send("\x08", "WORKSPACE")
        resize(80, 24)
        os.kill(proc.pid, signal.SIGWINCH)
        wait("128 queued", "WORKSPACE")
        # With both side regions unavailable, either direction stays on content.
        send("\x08\x0c\x08\x0c", "q quit")
        send("\r", "orders / ACTIVE")
        resize(50, 10)
        os.kill(proc.pid, signal.SIGWINCH)
        wait("Resize terminal")
        resize(160, 48)
        os.kill(proc.pid, signal.SIGWINCH)
        wait("INSPECTOR")
        send("]", "orders / DEAD-LETTER")
        dlq_review()
        action("settings", "Color theme: midnight")
        if args.mouse_off:
            send("\r", "Color theme: gruvbox")
        else:
            click("Color theme: midnight", "Color theme: gruvbox")
        # Inclusion is separate from discovery scope and the appearance rows.
        for label in ("Navigation sidebar:", "Message inspector:", "Dashboard layout:",
                      "Save preferences", "Environments / discovery scope", "Dashboard inclusion"):
            send("j", "› " + label)
        send("\r", "Mode: All entities")
        send("m", "Mode: Selected entities")
        wait("[ ] local-lab / orders")
        send("a", "[x] local-lab / orders")
        if not args.mouse_off:
            # Select a row first; only the explicit Toggle control changes it.
            click("local-lab / orders", "[x] local-lab / orders")
            click("Toggle", "[ ] local-lab / orders")
            click("Toggle", "[x] local-lab / orders")
        send(" ", "[ ] local-lab / orders")
        send("s", "Preferences saved")
        send("\x1b", "Color theme: gruvbox")
        send("\x1b", "13 watched entities")
        os.write(master, b"q")
        deadline = time.monotonic() + 5
        while proc.poll() is None and time.monotonic() < deadline:
            if select.select([master], [], [], 0.05)[0]:
                try:
                    os.read(master, 65536)
                except OSError:
                    break
        proc.wait(timeout=2)
        assert proc.returncode == 0, proc.returncode
        # Reopen the actual binary against the saved file, not an in-memory model.
        screen.reset()
        decoder.reset()
        slave = os.open(slave_name, os.O_RDWR | os.O_NOCTTY)
        proc = subprocess.Popen(command, stdin=slave, stdout=slave, stderr=slave,
                                env=env, start_new_session=True)
        os.close(slave)
        wait("13 watched entities", "128 queued")
        action("Dashboard inclusion", "Mode: Selected entities")
        wait("[ ] local-lab / orders")
        os.write(master, b"q")
        proc.wait(timeout=5)
        assert proc.returncode == 0
        print("PASS: real PTY — shared Actions/palette/Help capture and return, mouse reporting/fallback, layouts/themes, selected copy to queue, exact-selected cleanup, receive-next/review, cancel/release, retained-original resend, typed delete, save/restart watchlist, resize, clean exit")
    finally:
        if proc.poll() is None:
            proc.terminate()
            try:
                proc.wait(timeout=3)
            except subprocess.TimeoutExpired:
                proc.kill()
                proc.wait(timeout=2)
        os.close(master)
        preferences.cleanup()


if __name__ == "__main__":
    main()
