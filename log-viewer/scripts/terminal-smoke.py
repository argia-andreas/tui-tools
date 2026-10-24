#!/usr/bin/env python3
"""Exercise the real offline logview executable through a terminal, no mocks."""
import codecs
import fcntl
import os
import pty
import select
import signal
import struct
import subprocess
import sys
import termios
import time

import pyte


def main():
    lab = "--lab" in sys.argv
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
    args = ["--config", ".lab/source.json"] if lab else ["--fixture"]
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

    try:
        text = wait("Local Elasticsearch" if lab else "Offline fictional data")
        if lab:
            assert "22 lab events" in text and "ELASTICSEARCH · LOCAL LAB · TLS" in text
            assert "FIXTURE · OFFLINE" not in text and "No network access" not in text
        else:
            assert "22 fictional events" in text and "FIXTURE · OFFLINE" in text
        assert "● LOGS" in text and "● INSPECTOR" not in text
        assert "Payment gateway timeout" in text and "[REDACTED]" in text
        assert "8c3e7a21b9f04450" in text
        send("\r", "● INSPECTOR", "● LOGS")
        text = send("G", "region: swedencentral")
        assert "/app/payments/worker/process.go:127" in text
        assert "token: [REDACTED]" in text
        send("g", "POLICY-SAFE EVENT")
        send("y", "Field yank is not available")
        send("\x10", "Command palette is not available")
        send(" ", "Live follow is not available")
        send("\x1b", "● LOGS", "● INSPECTOR")
        send("j", "Retry queued with backoff", "Order 8042 remains pending;")
        send("k", "Order 8042 remains pending;")
        send("\x1b[Z", "● WORKSPACE", "● LOGS")
        # Narrowing while a disappearing panel owns focus must repair it.
        resize(80, 24); os.kill(proc.pid, signal.SIGWINCH)
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
        os.write(master, b"q")
        proc.wait(timeout=5)
        assert proc.returncode == 0
        for forbidden in (b"fixture-secret-", b"\x1b]52;", b"https://invalid.example", "\u202e".encode(), b"Q0FOQVJZ"):
            assert forbidden not in transcript, repr(forbidden)
        print(f"PASS: {'local Elastic TLS' if lab else 'fixture'} PTY — explicit mode, 22 safe events, multiline inspector, focus, navigation, resize recovery, unavailable actions, clean exit")
    finally:
        if proc.poll() is None:
            proc.terminate()
            proc.wait(timeout=5)
        os.close(master)


if __name__ == "__main__":
    main()
