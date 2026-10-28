package ui

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type copiedMsg struct{ status string }

func (m Model) yank(withKey bool) (tea.Model, tea.Cmd) {
	fields := m.inspectorFields()
	if m.focus != 2 || len(fields) == 0 {
		m.status = "Copy unavailable · select an inspector field first"
		return m, nil
	}
	if m.page.Events[m.selected].Incomplete {
		m.status = "Copy refused · incomplete event; full field unavailable"
		return m, nil
	}
	f := fields[min(m.fieldIndex, len(fields)-1)]
	data, err := json.Marshal(f.value)
	if err != nil {
		m.status = "Copy refused · field cannot be encoded"
		return m, nil
	}
	payload := string(data)
	if value, ok := f.value.(string); ok {
		payload = value
	}
	if withKey {
		payload = f.path + "=" + payload
	}
	ctx := m.options.Context
	trust := m.options.TrustOSC52
	m.status = "Copying policy-safe field · " + f.path
	return m, func() tea.Msg {
		var candidates [][]string
		if runtime.GOOS == "darwin" {
			candidates = append(candidates, []string{"pbcopy"})
		}
		if os.Getenv("WAYLAND_DISPLAY") != "" {
			candidates = append(candidates, []string{"wl-copy"})
		}
		if os.Getenv("DISPLAY") != "" {
			candidates = append(candidates, []string{"xclip", "-selection", "clipboard"})
		}
		for _, candidate := range candidates {
			path, err := exec.LookPath(candidate[0])
			if err != nil {
				continue
			}
			deadline, cancel := context.WithTimeout(ctx, 2*time.Second)
			defer cancel()
			cmd := exec.CommandContext(deadline, path, candidate[1:]...)
			cmd.WaitDelay = 100 * time.Millisecond
			cmd.Stdin = strings.NewReader(payload)
			if cmd.Run() != nil {
				return copiedMsg{"Copy failed · native clipboard rejected/timed out · " + f.path}
			}
			return copiedMsg{fmt.Sprintf("Copy command completed · %s · %d bytes · receipt unverified", f.path, len(payload))}
		}
		if !trust {
			return copiedMsg{"Copy unavailable · no native clipboard; --trust-osc52 required for terminal copy"}
		}
		// One deliberate write, never log-controlled escape text or shell input.
		sequence := "\x1b]52;c;" + base64.StdEncoding.EncodeToString([]byte(payload)) + "\a"
		if n, err := os.Stdout.WriteString(sequence); err != nil || n != len(sequence) {
			return copiedMsg{"Copy failed · OSC52 output unavailable"}
		}
		return copiedMsg{fmt.Sprintf("Copy sent (OSC52) · %s · %d bytes · receipt unverified", f.path, len(payload))}
	}
}
