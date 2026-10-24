package ui

import (
	"path"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"lazyftp/internal/storage"
)

type deleteFinished struct {
	location storage.Location
	err      error
}

func (m Model) deleteDisabled() string {
	if _, ok := m.browser.(storage.Deleter); !ok || m.transferer == nil {
		return "Demo is read-only; deletion is unavailable"
	}
	if m.deleting {
		return "Wait for the current deletion to finish"
	}
	if m.focus != 1 && m.focus != 2 {
		return "Focus a file pane to delete"
	}
	if m.queuePlanning || m.queueIndex < len(m.queue) || (m.job != nil && m.job.state == "Running") {
		return "Finish or cancel the current transfer queue first"
	}
	p := m.panes[m.active()]
	if m.active() == 1 && m.disconnected {
		return "Choose a connected file pane"
	}
	if p.loading || p.err != nil {
		return "Wait for a successful directory listing"
	}
	entries := p.visible()
	if p.cursor < 0 || p.cursor >= len(entries) || entries[p.cursor].IsDir {
		return "Choose a file, not a directory, to delete"
	}
	return ""
}

func (m Model) confirmDelete() (tea.Model, tea.Cmd) {
	if reason := m.deleteDisabled(); reason != "" {
		m.status = reason
		return m, nil
	}
	p := m.panes[m.active()]
	loc := p.location
	loc.Path = path.Join(loc.Path, p.visible()[p.cursor].Name)
	m.deleteTarget, m.modal, m.choice = &loc, "delete", 0
	return m, nil
}

func (m Model) deleteKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "ctrl+p":
		m.modal, m.deleteTarget = "", nil
	case "tab", "shift+tab", "left", "right", "up", "down":
		m.choice = 1 - m.choice
	case "enter":
		loc := *m.deleteTarget
		m.modal, m.deleteTarget = "", nil
		if m.choice == 0 {
			return m, nil
		}
		m.deleting = true
		m.status = "Deleting " + loc.String()
		deleter := m.browser.(storage.Deleter)
		return m, func() tea.Msg {
			return deleteFinished{location: loc, err: deleter.Delete(m.ctx, loc)}
		}
	}
	return m, nil
}

func (m Model) finishDelete(msg deleteFinished) (tea.Model, tea.Cmd) {
	m.deleting = false
	m.status = "Deleted: " + msg.location.String()
	if msg.err != nil {
		m.status = "Delete failed: " + msg.err.Error()
	}
	parent := msg.location
	parent.Path = path.Dir(parent.Path)
	var cmds []tea.Cmd
	for i := range m.panes {
		if m.panes[i].location == parent {
			if msg.err == nil {
				delete(m.panes[i].selected, path.Base(msg.location.Path))
			}
			m.panes[i].generation++
			m.panes[i].loading = true
			cmds = append(cmds, m.list(i))
		}
	}
	return m, tea.Batch(cmds...)
}

func (m Model) deleteLines(h int) (string, []string) {
	parent := *m.deleteTarget
	parent.Path = path.Dir(parent.Path)
	lines := []string{
		"  File: " + text(path.Base(m.deleteTarget.Path)),
		"  In:   " + text(parent.String()),
		m.paint("  Permanently delete this file? This cannot be undone.", m.theme.Warning),
		"  Tab / arrows choose · Enter confirms · Esc cancels",
	}
	lines = lines[:min(len(lines), h-4)]
	for len(lines) < h-3 {
		lines = append(lines, "")
	}
	row := " "
	for i, label := range []string{"Cancel", "Delete"} {
		button := " [" + label + "] "
		if m.choice == i {
			background := m.theme.Selection
			if i == 1 {
				background = m.theme.Error
			}
			row += lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(m.theme.SelectionText)).Background(lipgloss.Color(background)).Render(button)
		} else {
			row += m.paint(button, m.theme.Muted)
		}
	}
	return "CONFIRM FILE DELETION", append(lines, row)
}
