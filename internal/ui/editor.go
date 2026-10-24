package ui

import (
	"path"

	tea "github.com/charmbracelet/bubbletea"
	"lazyftp/internal/editor"
)

type editFinished struct {
	pane int
	err  error
}

func (m Model) editDisabled() string {
	if m.transferer == nil {
		return "Demo is read-only; edit real files in live mode"
	}
	if m.focus != 1 && m.focus != 2 {
		return "Focus a file pane to edit"
	}
	if m.queuePlanning || m.queueIndex < len(m.queue) || (m.job != nil && m.job.state == "Running") {
		return "Finish or cancel the current transfer queue first"
	}
	p := m.panes[m.active()]
	if p.loading || p.err != nil {
		return "Wait for a successful directory listing"
	}
	entries := p.visible()
	if p.cursor < 0 || p.cursor >= len(entries) || entries[p.cursor].IsDir {
		return "Choose a file, not a directory, to edit"
	}
	return ""
}

func (m Model) startEdit() (tea.Model, tea.Cmd) {
	if reason := m.editDisabled(); reason != "" {
		m.status = reason
		return m, nil
	}
	i := m.active()
	p := m.panes[i]
	loc := p.location
	loc.Path = path.Join(loc.Path, p.visible()[p.cursor].Name)
	return m, tea.Exec(editor.New(m.ctx, m.transferer, loc), func(err error) tea.Msg {
		return editFinished{pane: i, err: err}
	})
}
