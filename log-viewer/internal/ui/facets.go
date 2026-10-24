package ui

import (
	"fmt"
	"slices"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"logviewer/internal/logs"
)

// Style only after cleaning user-controlled labels and draft values.
func oneLine(text string) string {
	return strings.ReplaceAll(logs.Clean(text), "\n", " ↵ ")
}

func (m Model) filterView(header string) string {
	e, t := m.editor, m.options.Theme
	left := min(28, m.width/3)
	right, height := m.width-left, m.height-3
	names := []string{"App", "Time", "Levels", "Message", "Trace IDs", "Exact fields"}
	active := max(0, e.field-1)
	if e.field < 3 {
		active = min(e.field, 1)
	}
	apps := strings.Join(e.draft.Scope.Services, " OR ")
	if apps == "" {
		apps = "ALL"
	}
	summaries := []string{apps, "UTC [from, to)", e.inputs[3], e.inputs[4], e.inputs[5], fmt.Sprintf("%d predicates", len(e.draft.Fields))}
	var nav []string
	for i, name := range names {
		prefix, color := "  ", t.Foreground
		if i == active {
			prefix, color = "› ", t.Focus
		}
		nav = append(nav, m.style(color).Render(prefix+name))
		if height >= 22 {
			summary := summaries[i]
			if summary == "" {
				summary = "—"
			}
			nav = append(nav, m.style(t.Muted).Render("  "+oneLine(summary)), "")
		}
	}
	lines := m.facetLines(right - 4)
	// Keep validation feedback visible even on a 60×16 terminal.
	if e.error != "" {
		errors := strings.Split(ansi.Hardwrap(oneLine(e.error), right-4, true), "\n")
		lines = lines[:min(len(lines), max(0, height-3-len(errors)))]
		lines = append(lines, "")
		for _, line := range errors {
			lines = append(lines, m.style(t.Error).Render(line))
		}
	}
	for i := range lines {
		lines[i] = " " + lines[i]
	}
	body := lipgloss.JoinHorizontal(lipgloss.Top,
		m.panel("FILTER EDITOR", nav, left, height, -1),
		m.panel("Edit: "+names[active], lines, right, height, m.focus))
	semantics := m.style(t.Muted).Render(fit(" Apps OR · facets AND · UTC [from, to)", m.width))
	keys := m.style(t.Focus).Render(fit(" Tab/Shift+Tab facet · Ctrl+S apply · Esc cancel", m.width))
	return header + "\n" + body + "\n" + semantics + "\n" + keys
}

func (m Model) facetLines(width int) []string {
	e, t := m.editor, m.options.Theme
	muted := func(s string) string { return m.style(t.Muted).Render(oneLine(s)) }
	input := func(label string, index int) []string {
		prefix, color := "  ", t.Muted
		if e.field == index {
			prefix, color = "› ", t.Focus
		}
		value := oneLine(e.inputs[index])
		if value == "" {
			value = "—"
		}
		if ansi.StringWidth(value) > width-4 {
			value = ansi.TruncateLeft(value, ansi.StringWidth(value)-max(1, width-5), "…")
		}
		return []string{m.style(color).Render(prefix + label),
			m.style(t.Foreground).Background(lipgloss.Color(t.Surface)).Render(fit("  "+value, width)), ""}
	}
	switch e.field {
	case 0:
		apps := strings.Join(e.draft.Scope.Services, " OR ")
		if apps == "" {
			apps = "ALL"
		}
		lines := []string{muted("Selected: " + apps), ""}
		lines = append(lines, input("Suggest apps · keyword only", 0)...)
		visible := min(8, max(1, m.height-14))
		start := max(0, e.suggestion-visible+1)
		for i := start; i < min(len(e.suggestions.Items), start+visible); i++ {
			app := e.suggestions.Items[i]
			mark, prefix := "[ ] ", "  "
			if slices.Contains(e.draft.Scope.Services, app) {
				mark = "[x] "
			}
			color := t.Foreground
			if i == e.suggestion {
				prefix, color = "› ", t.Focus
			}
			lines = append(lines, m.style(color).Render(prefix+mark+oneLine(app)))
		}
		switch {
		case e.suggestionLoading:
			lines = append(lines, muted("Loading suggestions…"))
		case e.suggestionError:
			lines = append(lines, muted("Suggestions unavailable · selections retained"))
		case len(e.suggestions.Items) == 0:
			lines = append(lines, muted("No suggestions · change keyword or filters"))
		}
		if e.suggestions.More || e.suggestions.Partial {
			lines = append(lines, muted("Partial suggestions · narrow keyword"))
		}
		return append(lines, "", muted("↑/↓ choose · Enter toggle · Ctrl+X clear apps"))
	case 1, 2:
		lines := input("Start inclusive RFC3339", 1)
		lines = append(lines, input("End exclusive RFC3339", 2)...)
		return append(lines, muted("Z or explicit offset · Ctrl+U clears text"))
	case 3:
		return append(input("Levels", 3), muted("ALL / UNKNOWN / WARN+ / exact levels"),
			muted("WARN+ = WARN, ERROR, FATAL"), muted("Exact levels: comma-separated"))
	case 4:
		label := "Elastic analyzed phrase"
		if !m.options.LocalLab {
			label = "Fixture phrase approximation"
		}
		return append(input("Message", 4), muted(label), muted("Empty means all messages"))
	case 5:
		return append(input("Trace IDs", 5), muted("Exact values · comma-separated OR"))
	default:
		lines := []string{m.style(t.Focus).Render("› Exact field predicates"),
			muted(fmt.Sprintf("%d exact field predicates active", len(e.draft.Fields))), ""}
		if len(e.draft.Fields) == 0 {
			lines = append(lines, muted("None · add from inspector with f"))
		}
		visible := max(1, m.height-13)
		start := max(0, e.predicate-visible+1)
		for i := start; i < min(len(e.draft.Fields), start+visible); i++ {
			f, mark, color := e.draft.Fields[i], "  ", t.Foreground
			if i == e.predicate {
				mark, color = "› ", t.Focus
			}
			lines = append(lines, m.style(color).Render(mark+oneLine(f.Path+"="+string(f.Value))))
		}
		return append(lines, "", muted("↑/↓ choose · Enter remove · Ctrl+R clear all"))
	}
}
