package ui

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) copyField() (tea.Model, tea.Cmd) {
	if m.focus != "inspector" {
		m.status = "Focus inspector and select a field first"
		return m, nil
	}
	data, name, index, err := m.fieldData()
	if err != nil {
		m.status = err.Error()
		return m, nil
	}
	if m.readerMode == 2 {
		data = []byte(hex.EncodeToString(data))
		name += " (hex)"
	}
	ctx, trust := m.options.Context, m.options.TrustOSC52
	return m, func() tea.Msg {
		var cmds [][]string
		if runtime.GOOS == "darwin" {
			cmds = append(cmds, []string{"pbcopy"})
		}
		if os.Getenv("WAYLAND_DISPLAY") != "" {
			cmds = append(cmds, []string{"wl-copy"})
		}
		if os.Getenv("DISPLAY") != "" {
			cmds = append(cmds, []string{"xclip", "-selection", "clipboard"})
		}
		for _, args := range cmds {
			path, err := exec.LookPath(args[0])
			if err != nil {
				continue
			}
			deadline, cancel := context.WithTimeout(ctx, 2*time.Second)
			defer cancel()
			cmd := exec.CommandContext(deadline, path, args[1:]...)
			cmd.WaitDelay = 100 * time.Millisecond
			cmd.Stdin = bytes.NewReader(data)
			if cmd.Run() != nil {
				return statusMsg("Copy failed · native clipboard rejected/timed out")
			}
			return statusMsg(fmt.Sprintf("Copy command completed · %s[%d] · %d bytes · receipt unverified", name, index, len(data)))
		}
		if !trust {
			return statusMsg("Copy unavailable · no native helper; use --trust-osc52 for terminal transmission")
		}
		seq := "\x1b]52;c;" + base64.StdEncoding.EncodeToString(data) + "\a"
		n, err := os.Stdout.WriteString(seq)
		if err != nil || n != len(seq) {
			return statusMsg("Copy failed · terminal output unavailable")
		}
		return statusMsg(fmt.Sprintf("Copy sent (OSC52) · %d bytes · receipt unverified", len(data)))
	}
}
