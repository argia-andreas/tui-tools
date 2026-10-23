#!/usr/bin/env python3
"""Exercise the real fixture or seeded-emulator binary through a PTY."""
import argparse
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
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--emulator", action="store_true", help="requires the disposable lab already seeded")
    args = parser.parse_args()
    rejected = subprocess.run(["./bin/sbx"], capture_output=True, text=True)
    assert rejected.returncode == 2 and "--fixture" in rejected.stderr
    rejected = subprocess.run(["./bin/sbx", "--fixture", "--emulator"], capture_output=True, text=True)
    assert rejected.returncode == 2
    master, slave = pty.openpty()
    screen = pyte.Screen(160, 48)
    stream = pyte.Stream(screen)
    decoder = codecs.getincrementaldecoder("utf-8")("replace")

    def resize(cols, rows):
        fcntl.ioctl(master, termios.TIOCSWINSZ, struct.pack("HHHH", rows, cols, 0, 0))
        screen.resize(rows, cols)

    resize(160, 48)
    env = dict(os.environ, TERM="xterm-256color", COLORTERM="truecolor", CLICOLOR="1")
    env.pop("NO_COLOR", None)
    proc = subprocess.Popen(["./bin/sbx", "--emulator" if args.emulator else "--fixture"], stdin=slave, stdout=slave,
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
            if want in text and (absent is None or absent not in text):
                return text
        raise AssertionError(f"Expected {want!r}, absent {absent!r}\n" + "\n".join(screen.display))

    def send(keys, want, absent=None):
        os.write(master, keys.encode())
        return wait(want, absent)

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
            send("[", "orders-active-0001", "orders-dlq-0001")
            resize(80, 24)
            os.kill(proc.pid, signal.SIGWINCH)
            wait("q quit", "WORKSPACE")
            os.write(master, b"q")
            proc.wait(timeout=5)
            assert proc.returncode == 0
            print("PASS: real emulator PTY — discovery, unavailable counts, Active/DLQ, paging/refresh, resize, clean exit")
            return
        # The footer arrives after the complete frame; matching the first card
        # could return while the PTY is still streaming the remaining rows.
        text = wait("Offline sample data")
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
        # A focused sidebar must not trap keys after it auto-hides on resize.
        send("\t", "WORKSPACE")
        resize(80, 24)
        os.kill(proc.pid, signal.SIGWINCH)
        wait("128 queued", "WORKSPACE")
        send("\r", "orders / ACTIVE")
        resize(50, 10)
        os.kill(proc.pid, signal.SIGWINCH)
        wait("Resize terminal")
        resize(160, 48)
        os.kill(proc.pid, signal.SIGWINCH)
        wait("INSPECTOR")
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
        print("PASS: real PTY — 12 cards, table, sidebar, Active/DLQ, body, paging, topics, resize, clean exit")
    finally:
        if proc.poll() is None:
            proc.terminate()
            try:
                proc.wait(timeout=3)
            except subprocess.TimeoutExpired:
                proc.kill()
                proc.wait(timeout=2)
        os.close(master)


if __name__ == "__main__":
    main()
