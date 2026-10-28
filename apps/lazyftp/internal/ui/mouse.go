package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"terminalui"
)

func (m Model) mouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if !m.workspaceFits() {
		return m, nil
	}
	if m.rename != nil {
		if m.rename.running {
			return m, nil
		}
		if !m.rename.confirming {
			return m.inlineRenameInput(msg)
		}
	}
	if m.modal != "" {
		return m.surfaceInput(msg)
	}
	return m.navigationInput(msg)
}

func (m Model) navigationInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	frame := m.interactionFrame()
	r := terminalui.HandleNavigation(msg, frame, m.navigationFocus(), m.navigationSelected())
	if r.Intent != nil {
		return m.navigationIntent(*r.Intent, frame)
	}
	return m, nil
}
