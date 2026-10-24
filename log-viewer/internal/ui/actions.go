package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/sahilm/fuzzy"
	"logviewer/internal/logs"
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
	actions := []action{
		{filter, "f", filterUnavailable},
		{"Find literal in loaded safe data", "/", ""},
		{"Inspect selected event", "enter", selected},
		{"Load older history", "o", older},
		{"Reload current search", "r", ""},
		{"Next local find match", "n", find},
		{"Previous local find match", "N", find},
		{"Save named search", "S", saved},
		{"Restore saved search", "R", saved},
		{"Settings", ",", ""},
		{"Return to logs", "esc", ""},
		{"Quit log viewer", "q", ""},
	}
	if m.focus == 2 {
		actions = append(actions, action{"Yank selected field value", "y", selected}, action{"Yank selected field key=value", "Y", selected})
	}
	return actions
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
	inner := m.width - 2
	t := m.options.Theme
	query := strings.ReplaceAll(logs.Clean(e.text), "\n", " ↵ ")
	rule := m.style(t.Border).Render(strings.Repeat("─", inner))
	lines := []string{m.style(t.Foreground).Render(fit(" > "+query+"▏", inner)), rule}
	visible := max(1, m.height-7)
	start := max(0, selected-visible+1)
	for i := start; i < min(len(matches), start+visible); i++ {
		a := matches[i]
		label := a.label
		color := t.Foreground
		if a.unavailable != "" {
			label += " · unavailable"
			color = t.Muted
		}
		prefix := "  "
		style := m.style(color)
		if i == selected {
			prefix = "› "
			style = m.style(t.SelectionText).Background(lipgloss.Color(t.Selection))
		}
		shortcut := " [" + a.key + "] "
		row := prefix + fit(label, inner-2-ansi.StringWidth(shortcut)) + shortcut
		lines = append(lines, style.Render(row))
	}
	if len(matches) == 0 {
		lines = append(lines, m.style(t.Muted).Render(" No matching implemented actions"))
	}
	reason := "Type to fuzzy-find actions · Ctrl+U clear"
	if len(matches) > 0 && matches[selected].unavailable != "" {
		reason = matches[selected].unavailable
	}
	lines = append(lines, rule, m.style(t.Muted).Render(fit(" "+reason, inner)),
		m.style(t.Muted).Render(fit(fmt.Sprintf(" %d/%d · ↑↓/Tab choose · Enter run · Esc back", min(selected+1, len(matches)), len(matches)), inner)))
	return "COMMAND PALETTE", lines
}

func (m Model) paletteView() string {
	// Render the unchanged workspace beneath the modal, with no competing
	// bright focus. ANSI-aware cuts preserve terminal cells and Unicode widths.
	background := m
	background.editor, background.focus = nil, -1
	rows := strings.Split(background.View(), "\n")
	for i, row := range rows {
		rows[i] = m.style(m.options.Theme.Border).Render(fit(ansi.Strip(row), m.width))
	}
	width := min(82, m.width-6)
	height := min(m.height-4, min(9, max(1, len(m.paletteMatches())))+7)
	modal := m
	modal.width, modal.height = width, height
	modal.options.Theme.Background = m.options.Theme.Surface
	title, lines := modal.paletteLines()
	box := strings.Split(modal.panel(title, lines, width, height, modal.focus), "\n")
	x, y := (m.width-width)/2, (m.height-height)/2
	for i, row := range box {
		under := rows[y+i]
		rows[y+i] = ansi.Cut(under, 0, x) + row + ansi.Cut(under, x+width, m.width)
	}
	return strings.Join(rows, "\n")
}
