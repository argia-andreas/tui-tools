//go:build linux

package clipboard_test

import (
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"servicebusexplorer/internal/clipboard"
)

const payload = "private message\n雪\n\x1b]52;c;injected\a\x00\n"

func environment(t *testing.T) string {
	t.Helper()
	for _, key := range []string{"SSH_CONNECTION", "SSH_CLIENT", "SSH_TTY", "MOSH_IP", "WAYLAND_DISPLAY", "DISPLAY", "TMUX"} {
		t.Setenv(key, "")
	}
	t.Setenv("TERM", "xterm-256color")
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	return dir
}

func writer(t *testing.T, dir, name, script string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"+script), 0700); err != nil {
		t.Fatal(err)
	}
}

func capture(t *testing.T, f func()) string {
	t.Helper()
	file, err := os.CreateTemp(t.TempDir(), "output")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	stdout, stderr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = file, file
	defer func() { os.Stdout, os.Stderr = stdout, stderr }()
	f()
	if _, err := file.Seek(0, 0); err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(file)
	if err != nil {
		t.Fatal(err)
	}
	return string(got)
}

func TestNativeWritersCopyExactBytes(t *testing.T) {
	for _, tc := range []struct{ name, display, args string }{
		{"wl-copy", "WAYLAND_DISPLAY", ""},
		{"xclip", "DISPLAY", "-selection clipboard"},
		{"xsel", "DISPLAY", "--clipboard --input"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := environment(t)
			t.Setenv(tc.display, "test-display")
			t.Setenv("COPY_RESULT", filepath.Join(dir, "copied"))
			writer(t, dir, tc.name, "[ \"$*\" = \""+tc.args+"\" ] || exit 9\n/bin/cat > \"$COPY_RESULT\"\n")
			for _, text := range []string{payload, "", "\n\n"} {
				output := capture(t, func() {
					confirmed, err := clipboard.Write(text)
					if !confirmed || err != nil {
						t.Fatalf("Write = %v, %v", confirmed, err)
					}
				})
				got, err := os.ReadFile(os.Getenv("COPY_RESULT"))
				if err != nil || string(got) != text || output != "" {
					t.Fatalf("copy mismatch or unexpected output: read error %v", err)
				}
			}
		})
	}
}

func TestNativeFailureDoesNotDiscloseOrFallBack(t *testing.T) {
	for _, script := range []string{"/bin/cat\n/bin/cat >&2 <<EOF\nprivate message\nEOF\nexit 7\n", "exec /bin/sleep 30\n"} {
		t.Run(script[:4], func(t *testing.T) {
			dir := environment(t)
			t.Setenv("WAYLAND_DISPLAY", "test-display")
			writer(t, dir, "wl-copy", script)
			start := time.Now()
			output := capture(t, func() {
				confirmed, err := clipboard.Write(payload)
				if confirmed || err == nil || strings.Contains(err.Error(), "private message") {
					t.Fatalf("unsafe failure result: %v, %v", confirmed, err)
				}
			})
			if output != "" || time.Since(start) > 5*time.Second {
				t.Fatal("writer leaked output or exceeded bounded lifetime")
			}
		})
	}
}

// A real PTY exercises the public interface without substituting a terminal
// detector or output writer inside the package under test.
func terminalOutput(t *testing.T, f func()) string {
	t.Helper()
	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer master.Close()
	if err := unix.IoctlSetPointerInt(int(master.Fd()), unix.TIOCSPTLCK, 0); err != nil {
		t.Fatal(err)
	}
	n, err := unix.IoctlGetInt(int(master.Fd()), unix.TIOCGPTN)
	if err != nil {
		t.Fatal(err)
	}
	slave, err := os.OpenFile(fmt.Sprintf("/dev/pts/%d", n), os.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer slave.Close()
	result := make(chan string, 1)
	go func() {
		data, _ := io.ReadAll(master) // PTY returns EIO after the last slave closes.
		result <- string(data)
	}()
	stdout := os.Stdout
	os.Stdout = slave
	defer func() { os.Stdout = stdout }()
	f()
	slave.Close()
	select {
	case got := <-result:
		return got
	case <-time.After(5 * time.Second):
		t.Fatal("PTY output did not finish")
		return ""
	}
}

func TestOSC52ExactBytesAndRemotePreference(t *testing.T) {
	for _, remote := range []string{"", "SSH_CONNECTION", "SSH_CLIENT", "SSH_TTY", "MOSH_IP"} {
		t.Run("remote="+remote, func(t *testing.T) {
			dir := environment(t)
			if remote != "" {
				t.Setenv(remote, "remote")
				t.Setenv("WAYLAND_DISPLAY", "remote-display")
				writer(t, dir, "wl-copy", "exit 9\n")
			}
			for _, text := range []string{payload, "", strings.Repeat("雪", (1<<20)/3) + "x"} {
				got := terminalOutput(t, func() {
					confirmed, err := clipboard.Write(text)
					if confirmed || err != nil {
						t.Fatalf("OSC52 = %v, %v", confirmed, err)
					}
				})
				want := "\x1b]52;c;" + base64.StdEncoding.EncodeToString([]byte(text)) + "\a"
				if got != want {
					t.Fatal("OSC52 output was not exactly the base64-encoded payload")
				}
			}
		})
	}
}

func TestTmuxUsesStandardOSC52(t *testing.T) {
	environment(t)
	t.Setenv("TMUX", "/tmp/tmux/session,1,0")
	t.Setenv("TERM", "screen-256color")
	got := terminalOutput(t, func() {
		if confirmed, err := clipboard.Write(payload); confirmed || err != nil {
			t.Fatalf("Write = %v, %v", confirmed, err)
		}
	})
	if want := "\x1b]52;c;" + base64.StdEncoding.EncodeToString([]byte(payload)) + "\a"; got != want {
		t.Fatal("tmux should use ordinary OSC52 (set-clipboard on), not require DCS passthrough")
	}
}

func TestTerminalWriteFailureIsNotSuccess(t *testing.T) {
	environment(t)
	got := terminalOutput(t, func() {
		name := fmt.Sprintf("/proc/self/fd/%d", os.Stdout.Fd())
		readOnly, err := os.Open(name)
		if err != nil {
			t.Fatal(err)
		}
		defer readOnly.Close()
		stdout := os.Stdout
		os.Stdout = readOnly
		defer func() { os.Stdout = stdout }()
		confirmed, err := clipboard.Write(payload)
		if confirmed || err == nil || !strings.Contains(err.Error(), "could not be written") || strings.Contains(err.Error(), "private message") {
			t.Fatalf("Write = %v, %v", confirmed, err)
		}
	})
	if got != "" {
		t.Fatal("failed write emitted output")
	}
}

func TestUnavailableAndOversizeRejectWithoutOutput(t *testing.T) {
	for _, mode := range []string{"non-tty", "dumb", "unset-term", "oversize", "no-display"} {
		t.Run(mode, func(t *testing.T) {
			dir := environment(t)
			text := payload
			switch mode {
			case "dumb":
				t.Setenv("TERM", "dumb")
			case "unset-term":
				t.Setenv("TERM", "")
			case "oversize":
				text = strings.Repeat("x", 1<<20+1)
			case "no-display":
				writer(t, dir, "wl-copy", "exit 0\n")
				writer(t, dir, "xclip", "exit 0\n")
			}
			check := func() {
				confirmed, err := clipboard.Write(text)
				if confirmed || err == nil || strings.Contains(err.Error(), "private message") {
					t.Fatalf("Write = %v, %v", confirmed, err)
				}
				if mode == "oversize" && !strings.Contains(err.Error(), "1 MiB") {
					t.Fatal("size limit was not explained")
				}
			}
			var output string
			if mode == "oversize" || mode == "dumb" || mode == "unset-term" {
				output = terminalOutput(t, check)
			} else {
				output = capture(t, check)
			}
			if output != "" {
				t.Fatal("rejected copy emitted terminal output")
			}
		})
	}
}
