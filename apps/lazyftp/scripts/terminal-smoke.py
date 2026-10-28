#!/usr/bin/env python3
"""Exercise the actual terminal app, decoding its screen rather than grepping ANSI."""
import codecs
import fcntl
import json
import os
import pty
import re
import select
import signal
import struct
import subprocess
import tempfile
import termios
import time

import pyte


def main(args=(), icons=True):
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
    proc = subprocess.Popen(["./bin/lazyftp", "--demo", *args], stdin=slave, stdout=slave,
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

    def click(x, y, want, absent=None):
        # SGR coordinates are one-based; pyte and UI hit regions are zero-based.
        return send(f"\x1b[<0;{x+1};{y+1}M\x1b[<0;{x+1};{y+1}m", want, absent)

    def locate(label):
        for y, row in enumerate(screen.display):
            x = row.find(label)
            if x >= 0:
                return x, y
        raise AssertionError(f"Missing clickable label {label!r}\n" + "\n".join(screen.display))

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
        folder_icon, markdown_icon = ("\uf07b", "\ue73e") if icons else ("▸", "≡")
        wait(f"{folder_icon} src/")
        wait(f"{markdown_icon} README.md")
        # Styled labels must not reset their trailing fill to the terminal's
        # default background (visible as dark rectangles in real terminals).
        expect_background(0, 40, "0b1423")
        expect_background(2, 50, "0b1423")
        notice_row = next(i for i, row in enumerate(screen.display) if "FTP · unencrypted" in row)
        expect_background(notice_row, 20, "101d2f")

        # Ctrl+L/H use the terminal's actual control bytes and stay directional.
        send("\x0c\x0c\r", "shipment.json")
        send("h", "orders.csv", "shipment.json")
        send("\x08\x08j\r", "browser.go")
        send("h", "README.md", "browser.go")

        # Exercise actual terminal mouse reports, not synthetic model messages.
        # Marking a directory must not open it, and a press/release pair must
        # toggle only once. Find src so fixture directory names can still vary.
        _, src_row = locate("src/")
        click(23, src_row, "1 selected")
        assert "LOCAL /project/src" not in "\n".join(screen.display)
        assert "README.md" in "\n".join(screen.display)
        click(24, src_row, "0 selected", "1 selected")
        click(30, src_row, "README.md", "browser.go")
        open_x, open_y = locate("Open/Inspect")
        click(open_x + 2, open_y, "browser.go", "README.md")
        wait("LOCAL /project/src")
        click(26, 2, "README.md", "browser.go")
        assert "LOCAL /project/src" not in "\n".join(screen.display)

        # Pane stops precede controls: Tab visits right, then the Actions button.
        send("\t\t\r", "ACTIONS · Ctrl+P")
        send("\x1b", "README.md", "ACTIONS · Ctrl+P")
        send("\x08", "README.md")  # Return to left without traversing controls.

        actions_x, actions_y = locate("Actions ctrl+p")
        click(actions_x + 2, actions_y, "ACTIONS · Ctrl+P")
        send("kybrd", "Query: kybrd", "Toggle navigation sidebar")
        shortcuts_x, shortcuts_y = locate("Keyboard shortcuts")
        click(shortcuts_x + 2, shortcuts_y, " HELP ", "ACTIONS · Ctrl+P")
        modal_w = min(82, screen.columns - 6)
        modal_h = min(20, screen.lines - 4)
        modal_x = (screen.columns - modal_w) // 2
        modal_y = (screen.lines - modal_h) // 2
        click(modal_x + modal_w - 3, modal_y, "Query: kybrd", " HELP ")
        send("\x1b", "README.md", "ACTIONS · Ctrl+P")

        # F1 and the visible Help button preserve the parent query. Closing
        # Help is not cancellation of the captured palette underneath it.
        send("\x10", "ACTIONS · Ctrl+P")
        send("inspector", "Query: inspector")
        help_x, help_y = locate("[Help F1]")
        click(help_x + 2, help_y, " HELP ", "ACTIONS · Ctrl+P")
        send("q\r", " HELP ")  # no workspace quit or action
        send("\x1b", "Query: inspector", " HELP ")
        send("\x1bOP", " HELP ", "ACTIONS · Ctrl+P")
        send("\x1b", "Query: inspector", " HELP ")
        send("\x1b", "README.md", "ACTIONS · Ctrl+P")

        # F1 does not clear or accept a filter draft, and '?' stays text.
        send("/", "Filter:")
        send("rea", "Filter: rea")
        send("\x1bOP", " HELP ")
        wait("Filter")
        send("\x1b", "Filter: rea", " HELP ")
        send("?", "Filter: rea?")
        send("\x1b", "README.md", "Filter:")

        # Captured text must not quit, move the file cursor or open help.
        send("\x10", "ACTIONS · Ctrl+P")
        send("qj?", "No results")
        send("\r", "No results")
        send("\x10", "README.md", "ACTIONS · Ctrl+P")

        # Dismiss over a neighboring workspace control without activating it.
        info_x, info_y = locate("[ Info ]")
        send("\x10", "ACTIONS · Ctrl+P")
        click(info_x + 2, info_y, "README.md", "ACTIONS · Ctrl+P")
        assert "INSPECTOR" not in "\n".join(screen.display)

        # Shared palette activation reaches the real, nonmutating inspector.
        # Resize while open so the click cannot use a cached result rectangle.
        send("\x10", "ACTIONS · Ctrl+P")
        send("file inspector", "Query: file inspector", "Toggle navigation sidebar")
        resize(120, 24)
        os.kill(proc.pid, signal.SIGWINCH)
        wait("Toggle file inspector")
        # Wait for a screen at the new geometry before looking up the row.
        deadline = time.monotonic() + 5
        while "Toggle file inspector" not in screen.display[6] and time.monotonic() < deadline:
            wait("Toggle file inspector")
        assert "Toggle file inspector" in screen.display[6], "palette did not finish resizing"
        inspector_x, inspector_y = locate("Toggle file inspector")
        click(inspector_x + 2, inspector_y, "INSPECTOR", "ACTIONS · Ctrl+P")
        send("]", "README.md", "INSPECTOR")
        resize(160, 40)
        os.kill(proc.pid, signal.SIGWINCH)
        wait("WORKSPACE")

        # Parent navigation above restores the initial cursor and selection for
        # the original keyboard-only scenario below.
        send("j", f"› {folder_icon} src/")
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
        print(f"PTY smoke PASS {args!r} (icons={icons}): mouse navigation/selection/shortcuts, F1, "
              f"keyboard navigation, selection, palette, themes, disabled actions, "
              f"sidebars, Blob fixture, resize, clean exit ({time.monotonic()-started:.2f}s)")
    finally:
        if proc.poll() is None:
            proc.kill()
            proc.wait(timeout=5)
        os.close(master)


def surfaces(mouse=True):
    """Real executable; disposable local files only, no remote connection."""
    with tempfile.TemporaryDirectory(prefix="lazyftp-surfaces-") as directory:
        source = os.path.join(directory, "source.txt")
        with open(source, "w") as file:
            file.write("KEEP source bytes\nsecond line\n")
        config = os.path.join(directory, "rclone.conf")
        with open(config, "w") as file:
            file.write("[fixture]\ntype = local\n")
        bookmarks = os.path.join(directory, "bookmarks.json")
        with open(bookmarks, "w") as file:
            json.dump([{"name": name, "remote": "fixture", "root": "/", "local": directory}
                       for name in ("Offline local", "Offline alternate")], file)
        master, slave = pty.openpty()
        screen = pyte.Screen(100, 30)
        stream = pyte.Stream(screen)
        decoder = codecs.getincrementaldecoder("utf-8")("replace")
        output = bytearray()
        fcntl.ioctl(master, termios.TIOCSWINSZ, struct.pack("HHHH", 30, 100, 0, 0))
        env = dict(os.environ, TERM="xterm-256color", COLORTERM="truecolor", CLICOLOR="1")
        env.pop("NO_COLOR", None)
        args = [] if mouse else ["--mouse=false"]
        proc = subprocess.Popen(["./bin/lazyftp", "--left", directory, "--connections", bookmarks,
                                 "--rclone-config", config, "--icons=false", *args],
                                stdin=slave, stdout=slave, stderr=slave, env=env, start_new_session=True)
        os.close(slave)

        def read():
            data = os.read(master, 65536)
            output.extend(data)
            stream.feed(decoder.decode(data))
            return data

        def wait(want, absent=None):
            deadline = time.monotonic() + 5
            while time.monotonic() < deadline:
                ready = select.select([master], [], [], 0.05)[0]
                if ready:
                    try:
                        read()
                    except OSError:
                        break
                current = "\n".join(screen.display)
                # A redraw can span reads. Wait for the complete frame before
                # locating its controls, not just an early matching body row.
                if not ready and want in current and (absent is None or absent not in current):
                    return current
            raise AssertionError(f"Expected {want!r}, absent {absent!r}\n" + "\n".join(screen.display))

        def send(keys, want, absent=None):
            os.write(master, keys.encode())
            return wait(want, absent)

        def click(label, want, absent=None):
            for y, row in enumerate(screen.display):
                x = row.find(label)
                if x >= 0:
                    return send(f"\x1b[<0;{x+2};{y+1}M\x1b[<0;{x+2};{y+1}m", want, absent)
            raise AssertionError(f"Missing control {label!r}\n" + "\n".join(screen.display))

        try:
            wait("Offline local")
            if mouse:
                click("Offline local", "Saved & configured", "Connection details")
                click("[ Open ]", "Connection details")
                send("\x1b", "Saved & configured", "Connection details")
                click("New connection", "FTP / FTPS")
                click("FTP / FTPS", "Connection name")
                send("scratch-slice3", "scratch-slice3")
                click("[ Help ]", "HELP")
                send("\x1b", "scratch-slice3", " HELP ")
                send("\x1b", "FTP / FTPS", "Connection name")
                send("\x1b", "Saved & configured", "FTP / FTPS")
                click("[ Close ]", "source.txt", "Saved & configured")
            else:
                # Navigation changes selection only. Tab reaches the explicit
                # Open control; no Connect or Save action is invoked.
                send("j", "›   Offline alternate", "Connection details")
                send("k", "›   Offline local", "Connection details")
                send("\t", "›[ Open ]", "Connection details")
                send("\r", "Connection details")
                send("\x1b", "Saved & configured", "Connection details")
                send("\x1bOP", " HELP ")
                send("\x1b", "Saved & configured", " HELP ")
                send("\x1b", "source.txt", "Saved & configured")
            send("/", "Filter:")
            send("source.txt", "Filter: source.txt")
            send("\r", "1 entries", "Filter:")
            if not mouse:
                send(" ", "1 selected")
                send("\x10", "ACTIONS · Ctrl+P")
                send("inspector", "Query: inspector")
                send("\x1bOP", " HELP ", "ACTIONS · Ctrl+P")
                send("\x1b", "Query: inspector", " HELP ")
                send("\x1b", "source.txt", "ACTIONS · Ctrl+P")
            send("D", "CONFIRM FILE DELETION")
            wait("File: source.txt")
            if mouse:
                click("[ Cancel ]", "source.txt", "CONFIRM FILE DELETION")
            else:
                send("\r", "source.txt", "CONFIRM FILE DELETION")
            send("R\x15renamed.txt", "renamed.txt")
            if mouse:
                click("[ Review ]", "RENAME FILE?")
                click("[ Cancel ]", "Rename canceled", "RENAME FILE?")
            else:
                send("\r", "RENAME FILE?")
                send("\r", "Rename canceled", "RENAME FILE?")
            send("p", "CONTENT PREVIEW")
            wait("KEEP source bytes")
            if mouse:
                click("[ Close ]", "source.txt", "CONTENT PREVIEW")
            else:
                send("\t\r", "source.txt", "CONTENT PREVIEW")
            # No confirm path above may mutate the disposable source either.
            with open(source) as file:
                assert file.read() == "KEEP source bytes\nsecond line\n"
            assert not os.path.exists(os.path.join(directory, "renamed.txt"))
            os.write(master, b"q")
            deadline = time.monotonic() + 5
            while time.monotonic() < deadline:
                if select.select([master], [], [], 0.05)[0]:
                    try:
                        if not read():
                            break
                    except OSError:
                        break
                elif proc.poll() is not None:
                    break
            proc.wait(timeout=0.2)
            assert proc.returncode == 0, proc.returncode
            # Inspect raw DECSET, including grouped modes and shutdown output.
            # Disable/reset codes are allowed with --mouse=false.
            enabled = {int(mode) for group in re.findall(rb"\x1b\[\?([0-9;]+)h", output)
                       for mode in group.split(b";")}
            mouse_modes = {9, 1000, 1001, 1002, 1003, 1005, 1006, 1015, 1016}
            if mouse:
                assert {1002, 1006} <= enabled, f"default did not enable cell-motion/SGR: {enabled}"
            else:
                assert not enabled & mouse_modes, f"--mouse=false enabled mouse modes: {enabled & mouse_modes}"
            print(f"PASS: local surfaces PTY ({'default mouse' if mouse else '--mouse=false keyboard only'}) — "
                  "select-only chooser/explicit Open, Help, cancel, preview/Close, clean quit; "
                  "raw mouse modes checked, source bytes unchanged, no remote connection")
        finally:
            if proc.poll() is None:
                proc.kill()
                proc.wait(timeout=5)
            os.close(master)


if __name__ == "__main__":
    main()
    main(("--icons=false",), icons=False)
    with tempfile.TemporaryDirectory() as directory:
        config = os.path.join(directory, "config.json")
        for preferences, overrides, icons in (
            ('{"theme":"midnight"}', (), True),
            ('{"icons":null}', (), True),
            ('{"icons":true}', (), True),
            ('{"icons":false}', (), False),
            ('{"icons":false}', ("--icons",), True),
            ('{"icons":true}', ("--icons=false",), False),
        ):
            with open(config, "w") as file:
                file.write(preferences)
            main(("--config", config, *overrides), icons=icons)
    surfaces()
    surfaces(mouse=False)
