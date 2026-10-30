package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"terminalui"
)

// Availability is recomputed from current application state, never retained by
// a rendered button or a palette row. IDs describe operations, not keystrokes.
func (m Model) paletteActions() []terminalui.Action {
	selected := ""
	if len(m.page.Events) == 0 {
		selected = "No selected event"
	}
	older := ""
	switch {
	case m.loading:
		older = "Read already in progress"
	case m.budgetStopped:
		older = "Session budget reached; reload to start again"
	case m.page.Next == "":
		older = "No older history available"
	}
	saved := ""
	if m.options.SearchesPath == "" {
		saved = "Requires explicit --searches path"
	}
	find := ""
	if m.find == "" {
		find = "Enter a local literal find first"
	}
	filter := terminalui.Action{ID: "filters", Label: "Edit source filters", Keys: []string{"f"}}
	if m.focus == 2 {
		filter = terminalui.Action{ID: "field-filter", Label: "Filter from selected event field", Keys: []string{"f"}, Unavailable: selected}
	}
	actions := []terminalui.Action{
		filter,
		{ID: "find", Label: "Find literal in loaded safe data", Keys: []string{"/"}},
		{ID: "inspect", Label: "Inspect selected event", Keys: []string{"enter"}, Unavailable: selected},
		{ID: "older", Label: "Load older history", Keys: []string{"o"}, Unavailable: older},
		{ID: "reload", Label: "Reload current search", Keys: []string{"r"}},
		{ID: "find-next", Label: "Next local find match", Keys: []string{"n"}, Unavailable: find},
		{ID: "find-previous", Label: "Previous local find match", Keys: []string{"N"}, Unavailable: find},
		{ID: "save", Label: "Save named search", Keys: []string{"S"}, Unavailable: saved},
		{ID: "restore", Label: "Restore saved search", Keys: []string{"R"}, Unavailable: saved},
		{ID: "settings", Label: "Settings", Keys: []string{","}},
		{ID: "return", Label: "Return to logs", Keys: []string{"esc"}},
		{ID: "quit", Label: "Quit logs", Keys: []string{"q", "ctrl+c"}},
	}
	if m.focus == 2 {
		actions = append(actions,
			terminalui.Action{ID: "yank-value", Label: "Yank selected field value", Keys: []string{"y"}, Unavailable: selected},
			terminalui.Action{ID: "yank-pair", Label: "Yank selected field key=value", Keys: []string{"Y"}, Unavailable: selected})
	}
	return actions
}

// Both keyboard and component intents enter this checked app-local boundary.
// Shared components cannot open an editor, execute a query or copy event data.
func (m Model) execute(id string) (tea.Model, tea.Cmd) {
	if id == "help" {
		m.help, m.helpOffset = true, 0
		return m, nil
	}
	if id == "palette" {
		if m.editor != nil {
			return m, nil
		}
		m.palette = terminalui.Palette{}
		m.editor = &editor{kind: "palette", returnFocus: m.focus}
		return m, nil
	}
	found := false
	for _, a := range m.paletteActions() {
		if a.ID == id {
			found = true
			if a.Unavailable != "" || a.Busy {
				m.status = a.Unavailable
				return m, nil
			}
			break
		}
	}
	if !found {
		return m, nil
	}
	switch id {
	case "quit":
		m.cancel()
		return m, tea.Sequence(m.closeCursor(m.page.Next), tea.Quit)
	case "filters":
		return m.openFilters()
	case "field-filter":
		return m.openFieldPicker()
	case "find":
		m.editor = &editor{kind: "find", text: m.find, caseSensitive: m.findCase, returnFocus: m.focus}
	case "find-next", "find-previous":
		m.findNext(id == "find-previous")
	case "older":
		m.loading = true
		return m, m.searchCommand(true)
	case "reload":
		return m.refresh()
	case "save":
		m.editor = &editor{kind: "save", returnFocus: m.focus}
	case "restore":
		return m.openRestore()
	case "settings":
		m.editor = &editor{kind: "settings", returnFocus: m.focus, vimMode: m.settings.VimMode}
	case "return":
		m.detail, m.focus = false, 1
	case "inspect":
		m.focus, m.detail = 2, m.width < 120
		m.revealField()
	case "yank-value", "yank-pair":
		return m.yank(id == "yank-pair")
	}
	return m, nil
}

func (m Model) paletteFrame() terminalui.Frame {
	w, h := min(82, m.width-6), min(18, m.height-4)
	return m.palette.View(m.paletteActions(), terminalui.Rect{X: (m.width - w) / 2, Y: (m.height - h) / 2, W: w, H: h}, m.options.Theme.Interactions())
}

func (m Model) editPalette(msg tea.Msg) (tea.Model, tea.Cmd) {
	result := m.palette.Handle(msg, m.paletteActions(), m.paletteFrame())
	if result.Intent == nil {
		return m, nil
	}
	switch result.Intent.Kind {
	case terminalui.Dismiss:
		m.focus, m.editor = m.editor.returnFocus, nil
	case terminalui.Activate:
		m.focus, m.editor = m.editor.returnFocus, nil
		return m.execute(result.Intent.ID)
	case terminalui.Help:
		return m.execute("help")
	}
	return m, nil
}

func (m Model) headerActions() []terminalui.Action {
	actions := []terminalui.Action{{ID: "palette", Label: "Actions", Keys: []string{"ctrl+p"}}}
	if m.editor != nil {
		actions[0].Unavailable = "Draft owns input"
		actions[0].Keys = nil
		return append(actions, terminalui.Action{ID: "help", Label: "Help", Keys: []string{"f1"}})
	}
	actions = append(actions, terminalui.Action{ID: "help", Label: "Help", Keys: []string{"f1"}})
	for _, action := range m.paletteActions() {
		if action.ID == "inspect" {
			action.Label = "Inspect"
			actions = append(actions, action)
		}
	}
	return actions
}

func (m Model) actionBar() terminalui.Frame {
	width := min(76, m.width)
	focus := ""
	if m.editor == nil {
		focus = m.navigationFocus()
	}
	return terminalui.RenderActionBar(m.headerActions(), terminalui.Rect{X: m.width - width, W: width, H: 1}, focus, m.options.Theme.Interactions())
}

func (m Model) paletteView() string {
	background := m
	background.editor, background.focus = nil, -1
	base := terminalui.Frame{Bounds: terminalui.Rect{W: m.width, H: m.height}, Lines: strings.Split(background.View(), "\n")}
	return strings.Join(terminalui.Overlay(base, m.paletteFrame(), m.options.Theme.Interactions()).Lines, "\n")
}
