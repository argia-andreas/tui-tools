package ui

import (
	"os"
	"path/filepath"
	"strings"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"lazyftp/internal/storage"
)

// ConnectRequested asks the host to open its connection chooser without
// discarding the workspace or its in-memory transfers.
type ConnectRequested struct {
	Local       string
	Destination bool
	Pane        int
	New         bool
}

// OpenLocalFolderRequested opens the invoking pane's local-folder prompt.
type OpenLocalFolderRequested struct{ Pane int }

// ConnectionOpened attaches one pane. Local restores a companion folder only
// during startup; subsequent source choices leave it empty.
type ConnectionOpened struct {
	Pane       int
	Location   storage.Location
	Local      string
	Locations  []storage.Location
	Browser    storage.Browser
	Transferer storage.Transferer
}

const detachButton = "[ Detach ]"

var sourceButtons = []struct{ label, action string }{
	{"[ Local files ]", "folder"},
	{"[ Saved connection ]", "connect"},
	{"[ New connection ]", "new"},
}

func paneName(i int) string {
	if i == 0 {
		return "LEFT"
	}
	return "RIGHT"
}

func destinationRows(h int) int { return max(3, (h-8)/2) }

func (m Model) destinationPane(i, w, h int) []string {
	lines := make([]string, max(0, h-2))
	first := destinationRows(h)
	center := func(s string) string {
		return strings.Repeat(" ", max(0, (w-2-lipgloss.Width(s))/2)) + s
	}
	if first-2 < len(lines) {
		lines[first-2] = m.paint(center("Choose a source"), m.theme.Foreground)
	}
	for choice, button := range sourceButtons {
		row, label := first+2*choice, button.label
		if row < len(lines) {
			if m.focus == i+1 && choice == m.panes[i].sourceChoice {
				label = lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.SelectionText)).Background(lipgloss.Color(m.theme.Selection)).Render(label)
			} else {
				label = m.paint(label, m.theme.Muted)
			}
			lines[row] = center(label)
		}
	}
	return m.block(paneName(i)+" · No source", w, h, lines, m.focus == i+1, m.theme.Muted)
}

func (m Model) connectionDisabled() string {
	if m.transferer == nil {
		return "Demo is read-only; choose sources in live mode"
	}
	if m.queuePlanning || m.queueIndex < len(m.queue) || (m.job != nil && m.job.state == "Running") {
		return "Finish or cancel the current transfer queue first"
	}
	return ""
}

func (m Model) folderKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		return m.openFolder()
	case "ctrl+u":
		m.folderPath = ""
	case "backspace", "ctrl+h":
		m.folderPath = dropRune(m.folderPath)
	case " ":
		if m.choice == 0 {
			m.folderPath += " "
		}
	default:
		if m.choice == 0 && msg.Type == tea.KeyRunes {
			for _, r := range msg.Runes {
				if !unicode.IsControl(r) {
					m.folderPath += string(r)
				}
			}
		}
	}
	return m, nil
}

func (m Model) openFolder() (tea.Model, tea.Cmd) {
	if m.modal != "folder" {
		return m, nil
	}
	if reason := m.connectionDisabled(); reason != "" {
		m.status = reason
		return m, nil
	}
	folder, err := filepath.Abs(m.folderPath)
	if strings.TrimSpace(m.folderPath) == "" {
		m.status = "Choose an existing local folder"
		return m, nil
	}
	if err == nil {
		var info os.FileInfo
		info, err = os.Stat(folder)
		if err == nil && !info.IsDir() {
			m.status = "Choose a directory, not a file"
			return m, nil
		}
	}
	if err != nil {
		m.status = "Local folder could not be opened"
		return m, nil
	}
	m.modal, m.status = "", ""
	m.focus, m.endpoint = m.folderPane+1, m.folderPane
	return m, m.navigate(m.folderPane, storage.Location{Path: folder})
}
