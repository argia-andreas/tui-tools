#!/usr/bin/env python3
"""Exercise the real fixture or seeded-emulator binary through a PTY."""
import argparse
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
    env.pop("NO_COLOR", None)
    command = ["./bin/sbx", "--emulator" if args.emulator else "--fixture", "--config", config_path]
    proc = subprocess.Popen(command, stdin=slave, stdout=slave,
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
                    x = row.find("┌─ " + pane)
                    if x >= 0:
                        borders[pane] = screen.buffer[y][x].fg == "65c7f7"
            footer_ready = ("h/l collapse/expand" in text) == (title == "WORKSPACE")
            return footer_ready and len(borders) == 3 and all(active == (pane == title) for pane, active in borders.items())
        return matches

    def action(query, want):
        send("\x10", "Ctrl+P / ACTIONS")
        send(query, "> " + query + "_")
        return send("\r", want, "Ctrl+P / ACTIONS")

    def dlq_review():
        # A non-first highlighted row, not receive-next, is the resend source.
        for cleanup in (False, True):
            text = wait(re.compile(r"› \d+\s+orders-dlq-\d{4}"))
            previous_id = re.search(r"› \d+\s+(orders-dlq-\d{4})", text).group(1)
            text = send("j", re.compile(r"› \d+\s+(?!" + re.escape(previous_id) + r")orders-dlq-\d{4}"))
            selected_id = re.search(r"› \d+\s+(orders-dlq-\d{4})", text).group(1)
            send("\r", "SELECTED MESSAGE / OPEN")
            wait("ID: " + selected_id)
            text = send("\t\r", "Destination queue: orders")
            assert "CHOOSE QUEUE" not in text and "Change queue" not in text
            wait("[ ] Delete original after confirmed resend")
            if cleanup:
                send("\t\t\r", "[x] Delete original after confirmed resend")
                send("\x1b[A", "› Review resend")
            else:
                send("\t", "› Review resend")
            send("\r", "Resend destination: orders")
            wait("ID: " + selected_id)
            if cleanup:
                wait("EXACT SELECTED ORIGINAL")
            else:
                text = wait("SELECTED PEEK COPY")
                assert "Lock until:" not in text
            send("\t", "› Resend copy")
            send("\r", "cleanup confirmed" if cleanup else "no cleanup requested")
            send("\r", "orders / DEAD-LETTER")
            wait("PEEK ONLY", selected_id if cleanup else None)
            if not cleanup:
                wait(selected_id)
        action("receive next", "ACTUAL RECEIVED MESSAGE")
        text = wait("› Cancel / release")
        assert re.search(r"ID: orders-dlq-\d{4}", text)
        assert "[ ] Remove original" in text
        send("\r", "Lock release confirmed")
        send("\r", "orders / DEAD-LETTER")
        action("receive next", "ACTUAL RECEIVED MESSAGE")
        retained_id = re.search(r"ID: (orders-dlq-\d{4})", wait("New MessageID:")).group(1)
        send("\t", "› Resend copy")
        send("\r", "no cleanup requested")
        wait("Lock release confirmed")
        send("\r", "orders / DEAD-LETTER")
        wait(retained_id)  # resend must not implicitly delete the acquired original
        action("receive next", "ACTUAL RECEIVED MESSAGE")
        deleted_id = re.search(r"ID: (orders-dlq-\d{4})", wait("New MessageID:")).group(1)
        send("\t\t\t", "› Delete original")
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
            os.write(master, b"q")
            proc.wait(timeout=5)
            assert proc.returncode == 0
            print("PASS: real emulator PTY — discovery, unavailable counts, paging/refresh, selected copy to queue, exact-selected cleanup, receive-next/review, cancel/release, retained original, typed delete, resize, clean exit")
            return
        # The footer arrives after the complete frame; matching the first card
        # could return while the PTY is still streaming the remaining rows.
        wait("Offline sample data")
        text = wait("128 queued")
        assert "FIXTURE · OFFLINE" in text and "WORKSPACE" in text
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
        send("\t", "WORKSPACE")
        send("j", "› Queues")
        send("j", "› Topics")
        send("\r", "Topic subscriptions")
        wait("events/shipping")
        send("\r", "events/billing / ACTIVE")
        send("\x1b", "Topic subscriptions")
        send("\x1b", "128 queued")
        send("\x10", "Ctrl+P / ACTIONS")
        send("qjk", "No matching actions")
        assert proc.poll() is None, "palette query q quit the process"
        send("\x08\x0c", "> qj_")
        send("\x1b", "128 queued", "Ctrl+P / ACTIONS")
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
        # A focused sidebar must not trap keys after it auto-hides on resize.
        send("\t", "WORKSPACE")
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
        send("\r", "Color theme: gruvbox")
        # First watch row follows the five layout/save controls.
        for label in ("Navigation sidebar:", "Message inspector:", "Dashboard layout:",
                      "Save preferences", "[x] fixture / local-lab / orders"):
            send("j", "› " + label)
        send("\r", "[ ] fixture / local-lab / orders")
        send("s", "Preferences saved")
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
        action("settings", "Color theme: gruvbox")
        wait("[ ] fixture / local-lab / orders")
        os.write(master, b"q")
        proc.wait(timeout=5)
        assert proc.returncode == 0
        print("PASS: real PTY — layouts/palette/themes, selected copy to queue, exact-selected cleanup, receive-next/review, cancel/release, retained-original resend, typed delete, save/restart watchlist, resize, clean exit")
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
