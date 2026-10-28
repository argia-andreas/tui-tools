// Package clipboard writes message content without reading the clipboard.
package clipboard

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	osc52 "github.com/aymanbagabas/go-osc52/v2"
	"github.com/charmbracelet/x/term"
)

// Write copies text verbatim. confirmed is true only when a local native writer
// exits successfully. A terminal OSC52 request returns false, nil: terminals may
// ignore it, and we never query the user's clipboard to verify it.
func Write(text string) (confirmed bool, err error) {
	remote := os.Getenv("SSH_CONNECTION") != "" || os.Getenv("SSH_CLIENT") != "" ||
		os.Getenv("SSH_TTY") != "" || os.Getenv("MOSH_IP") != ""
	if !remote {
		var candidates [][]string
		switch runtime.GOOS {
		case "darwin":
			candidates = append(candidates, []string{"pbcopy"})
		case "linux":
			if os.Getenv("WAYLAND_DISPLAY") != "" {
				candidates = append(candidates, []string{"wl-copy"})
			}
			if os.Getenv("DISPLAY") != "" {
				candidates = append(candidates, []string{"xclip", "-selection", "clipboard"}, []string{"xsel", "--clipboard", "--input"})
			}
		}
		for _, candidate := range candidates {
			path, err := exec.LookPath(candidate[0])
			if err != nil {
				continue
			}
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			cmd := exec.CommandContext(ctx, path, candidate[1:]...)
			cmd.Stdin = strings.NewReader(text)
			// Bound waiting on inherited pipes, too. Child output is discarded:
			// neither its diagnostics nor the payload may reach the terminal.
			cmd.WaitDelay = 100 * time.Millisecond
			err = cmd.Run()
			cancel()
			if err != nil {
				return false, errors.New("clipboard writer failed or timed out; check your local clipboard service and retry")
			}
			return true, nil
		}
	}

	terminal := os.Getenv("TERM")
	if terminal == "" || terminal == "dumb" || !term.IsTerminal(os.Stdout.Fd()) {
		return false, errors.New("clipboard unavailable; use an OSC52-capable interactive terminal or install a local clipboard writer")
	}
	if len(text) > 1<<20 {
		return false, errors.New("terminal clipboard copy exceeds the 1 MiB limit; copy a smaller message or use a local clipboard writer")
	}
	sequence := osc52.New(text)
	// tmux handles ordinary OSC52 with `set-clipboard on`. Do not use
	// Tmux(): its DCS wrapper additionally requires allow-passthrough on.
	if os.Getenv("TMUX") == "" && strings.HasPrefix(terminal, "screen") {
		sequence = sequence.Screen()
	}
	output := sequence.String()
	n, err := os.Stdout.WriteString(output)
	if err != nil || n != len(output) {
		return false, errors.New("terminal clipboard request could not be written; check terminal output and retry")
	}
	return false, nil
}
