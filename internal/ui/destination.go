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
type ConnectRequested struct{ Local string }

// ConnectionOpened supplies endpoints chosen explicitly by the user.
type ConnectionOpened struct {
	Left, Right storage.Location
	Locations   []storage.Location
	Browser     storage.Browser
	Transferer  storage.Transferer
}

const connectButton = "[ Connect a remote ]"
const folderButton = "[ Open local folder ]"

func destinationRows(h int) (int, int) {
	connect := max(3, (h-2)/2)
	return connect, connect + 2
}

func (m Model) destinationPane(w, h int) []string {
	lines := make([]string, max(0, h-2))
	connect, folder := destinationRows(h)
	center := func(s string) string {
		return strings.Repeat(" ", max(0, (w-2-lipgloss.Width(s))/2)) + s
	}
	if connect-2 < len(lines) {
		lines[connect-2] = m.paint(center("Choose a destination"), m.theme.Foreground)
	}
	if connect < len(lines) {
		button := lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.SelectionText)).Background(lipgloss.Color(m.theme.Selection)).Render(connectButton)
		lines[connect] = center(button)
	}
	if folder < len(lines) {
		lines[folder] = m.paint(center(folderButton), m.theme.Muted)
	}
	return m.block("REMOTE Not connected", w, h, lines, m.focus == 2, m.theme.Remote)
}

func (m Model) connectionDisabled() string {
	if m.queuePlanning || m.queueIndex < len(m.queue) || (m.job != nil && m.job.state == "Running") {
		return "Finish or cancel the current transfer queue first"
	}
	return ""
}

func (m Model) folderKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "ctrl+p":
		m.modal = ""
	case "enter":
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
		return m, m.navigate(1, storage.Location{Path: folder})
	case "ctrl+u":
		m.folderPath = ""
	case "backspace", "ctrl+h":
		m.folderPath = dropRune(m.folderPath)
	case " ":
		m.folderPath += " "
	default:
		if msg.Type == tea.KeyRunes {
			for _, r := range msg.Runes {
				if !unicode.IsControl(r) {
					m.folderPath += string(r)
				}
			}
		}
	}
	return m, nil
}
