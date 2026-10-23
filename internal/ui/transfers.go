package ui

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strings"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"lazyftp/internal/storage"
)

type copyState struct {
	source, destination storage.Location
	size                int64
	checking, conflict  bool
	err                 error
}

type destinationChecked struct {
	operation int
	exists    bool
	err       error
}
type copyFinished struct {
	source, destination storage.Location
	err                 error
}
type transferJob struct {
	source, destination storage.Location
	state               string
	err                 error
}

func (m Model) copyEntry() (storage.Entry, string) {
	p := m.panes[m.active()]
	if p.loading || p.err != nil {
		return storage.Entry{}, "Wait for a successful directory listing"
	}
	var chosen []storage.Entry
	for _, e := range p.entries {
		if p.selected[e.Name] {
			chosen = append(chosen, e)
		}
	}
	if len(chosen) > 1 {
		return storage.Entry{}, "Select exactly one file; batch copies arrive later"
	}
	if len(chosen) == 0 {
		entries := p.visible()
		if len(entries) == 0 {
			return storage.Entry{}, "No file to copy"
		}
		chosen = append(chosen, entries[p.cursor])
	}
	if chosen[0].IsDir {
		return storage.Entry{}, "Choose a file; folder transfers arrive later"
	}
	return chosen[0], ""
}

func (m Model) copyDisabled() string {
	if m.transferer == nil {
		return "Transfers arrive in live mode; demo is read-only"
	}
	if m.job != nil && m.job.state == "Running" {
		return "A transfer is already running"
	}
	if m.panes[0].location.Remote != "" && m.panes[1].location.Remote != "" {
		return "Remote-to-remote copies are not supported yet"
	}
	_, reason := m.copyEntry()
	return reason
}

func (m Model) checkDestination() (tea.Model, tea.Cmd) {
	p := *m.pending
	p.checking, p.conflict, p.err = true, false, nil
	m.pending, m.modal, m.choice = &p, "copy", 0
	m.operation++
	op := m.operation
	return m, func() tea.Msg {
		e, err := m.transferer.Stat(m.ctx, p.destination)
		if errors.Is(err, fs.ErrNotExist) {
			return destinationChecked{operation: op}
		}
		if err == nil && e.IsDir {
			err = fmt.Errorf("destination is a directory; choose another filename")
		}
		return destinationChecked{operation: op, exists: err == nil, err: err}
	}
}

func (m Model) copyButtons() []string {
	if m.pending.checking {
		return []string{"Cancel"}
	}
	if m.pending.err != nil {
		return []string{"Cancel", "Rename"}
	}
	if m.pending.conflict {
		return []string{"Cancel", "Skip", "Overwrite", "Rename"}
	}
	return []string{"Cancel", "Copy", "Rename"}
}

func (m Model) copyKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if key == "esc" {
		m.modal, m.pending = "", nil
		return m, nil
	}
	if m.modal == "rename" {
		switch key {
		case "enter":
			if m.query == "" || m.query == "." || m.query == ".." || strings.ContainsAny(m.query, "/\\") || strings.IndexFunc(m.query, unicode.IsControl) >= 0 {
				m.status = "Rename needs a single filename, not a path"
				return m, nil
			}
			p := *m.pending
			p.destination.Path = path.Join(path.Dir(p.destination.Path), m.query)
			m.pending = &p
			return m.checkDestination()
		case "backspace", "ctrl+h":
			m.query = dropRune(m.query)
		case "ctrl+u":
			m.query = ""
		case " ":
			m.query += " "
		default:
			if msg.Type == tea.KeyRunes {
				m.query += string(msg.Runes)
			}
		}
		return m, nil
	}
	buttons := m.copyButtons()
	switch key {
	case "tab", "right", "down":
		m.choice = (m.choice + 1) % len(buttons)
	case "shift+tab", "left", "up":
		m.choice = (m.choice + len(buttons) - 1) % len(buttons)
	case "enter":
		switch buttons[min(m.choice, len(buttons)-1)] {
		case "Cancel":
			m.modal, m.pending = "", nil
		case "Skip":
			m.job = &transferJob{source: m.pending.source, destination: m.pending.destination, state: "Skipped"}
			m.status, m.modal, m.pending = "Skipped — destination preserved", "", nil
		case "Rename":
			m.modal, m.query = "rename", path.Base(m.pending.destination.Path)
		case "Copy", "Overwrite":
			p := *m.pending
			overwrite := p.conflict
			m.job = &transferJob{source: p.source, destination: p.destination, state: "Running"}
			m.jobs, m.modal, m.pending = true, "", nil
			m.status = "Copying " + p.source.String()
			return m, func() tea.Msg {
				return copyFinished{source: p.source, destination: p.destination, err: m.transferer.Copy(m.ctx, p.source, p.destination, overwrite)}
			}
		}
	}
	return m, nil
}

func (m Model) copyLines(w, h int) (string, []string) {
	p := m.pending
	title := "CONFIRM SINGLE-FILE COPY"
	lines := []string{"  From: " + text(p.source.String()), "  To:   " + text(p.destination.String()), fmt.Sprintf("  Size: %d B", p.size)}
	if m.modal == "rename" {
		return "RENAME DESTINATION", []string{"  New filename (Ctrl+U clears):", "  " + text(m.query) + "▏", "", "  Enter checks name · Esc cancels"}
	}
	detail := "  Destination is new. Copy requires confirmation."
	if p.checking {
		detail = "  Checking destination…"
	}
	if p.conflict {
		title = "DESTINATION EXISTS"
		detail = "  Overwrite replaces this file. Cancel is the default."
	}
	if p.err != nil {
		detail = "  " + text(p.err.Error())
	}
	lines = append(lines, m.paint(detail, m.theme.Warning))
	if h >= 10 {
		lines = append(lines, "", m.paint("  This choice applies to this file only.", m.theme.Muted), "  Tab / arrows choose · Enter confirms · Esc cancels")
	}
	for len(lines) < h-3 {
		lines = append(lines, "")
	}
	row := m.paint(" ", m.theme.Foreground)
	for i, label := range m.copyButtons() {
		button := " [" + label + "] "
		color := m.theme.Focus
		if label == "Overwrite" {
			color = m.theme.Error
		}
		style := lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Background(lipgloss.Color(m.theme.Background))
		if i == m.choice {
			background := m.theme.Selection
			if label == "Overwrite" {
				background = m.theme.Error
			}
			style = style.Bold(true).Foreground(lipgloss.Color(m.theme.SelectionText)).Background(lipgloss.Color(background))
		}
		row += style.Render(button)
	}
	return title, append(lines, row)
}
