package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sahilm/fuzzy"
)

// Only implemented actions belong here. Availability is recomputed while the
// palette is open, including after asynchronous reads and terminal resizes.
type action struct {
	label, key, unavailable string
}

func (m Model) paletteActions() []action {
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
	filter := "Edit source filters"
	filterUnavailable := ""
	if m.focus == 2 {
		filter, filterUnavailable = "Filter from selected event field", selected
	}
	return []action{
		{filter, "f", filterUnavailable},
		{"Find literal in loaded safe data", "/", ""},
		{"Inspect selected event", "enter", selected},
		{"Load older history", "o", older},
		{"Reload current search", "r", ""},
		{"Next local find match", "n", find},
		{"Previous local find match", "N", find},
		{"Save named search", "S", saved},
		{"Restore saved search", "R", saved},
		{"Return to logs", "esc", ""},
		{"Quit log viewer", "q", ""},
	}
}

func (m Model) paletteMatches() []action {
	actions := m.paletteActions()
	if m.editor.text == "" {
		return actions
	}
	labels := make([]string, len(actions))
	for i, a := range actions {
		labels[i] = a.label
	}
	var matches []action
	for _, match := range fuzzy.Find(m.editor.text, labels) {
		matches = append(matches, actions[match.Index])
	}
	return matches
}

func (m Model) editPalette(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	e := m.editor
	matches := m.paletteMatches()
	e.suggestion = max(0, min(e.suggestion, len(matches)-1))
	switch msg.String() {
	case "up", "shift+tab":
		e.suggestion = max(0, e.suggestion-1)
	case "down", "tab":
		e.suggestion = min(max(0, len(matches)-1), e.suggestion+1)
	case "enter":
		if len(matches) == 0 {
			break
		}
		a := matches[e.suggestion]
		if a.unavailable != "" {
			// The selected action's current reason stays visible; do not close
			// the modal or dispatch a key for an unavailable action.
			break
		}
		m.focus = e.returnFocus
		m.editor = nil
		key := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(a.key)}
		if a.key == "enter" {
			// Unlike Enter in the sidebar, this action explicitly inspects.
			m.focus = 1
			key = tea.KeyMsg{Type: tea.KeyEnter}
		} else if a.key == "esc" {
			key = tea.KeyMsg{Type: tea.KeyEsc}
		}
		return m.Update(key)
	default:
		before := e.text
		e.text = appendInput(e.text, msg)
		if e.text != before {
			e.suggestion = 0
		}
	}
	return m, nil
}

func (m Model) paletteLines() (string, []string) {
	e := m.editor
	matches := m.paletteMatches()
	selected := max(0, min(e.suggestion, len(matches)-1))
	lines := []string{"Type fuzzy action · ↑/↓ or Tab choose · Enter run · Esc back", "> " + e.text, ""}
	// Reserve the bottom of even a 60×16 terminal for the availability reason.
	visible := max(1, m.height-12)
	start := max(0, selected-visible+1)
	for i := start; i < min(len(matches), start+visible); i++ {
		a := matches[i]
		prefix, availability := "  ", ""
		if i == selected {
			prefix = "› "
		}
		if a.unavailable != "" {
			availability = " · unavailable"
		}
		lines = append(lines, prefix+a.label+" ["+a.key+"]"+availability)
	}
	if len(matches) == 0 {
		lines = append(lines, "No matching implemented actions · Ctrl+U clear")
	} else {
		lines = append(lines, fmt.Sprintf("%d / %d actions", selected+1, len(matches)))
		reason := "Available · Enter runs selected action"
		if matches[selected].unavailable != "" {
			reason = matches[selected].unavailable
		}
		lines = append(lines, reason)
	}
	return "COMMAND PALETTE", lines
}
