package ui

import (
	"fmt"
	"slices"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"logviewer/internal/logs"
	"terminalui"
)

// Style only after cleaning user-controlled labels and draft values.
func oneLine(text string) string {
	return strings.ReplaceAll(logs.Clean(text), "\n", " ↵ ")
}

func (m Model) filterBody(bounds terminalui.Rect) terminalui.Frame {
	e, t := m.editor, m.options.Theme
	left := min(28, bounds.W/3)
	right, height := bounds.W-left, bounds.H-2
	f := terminalui.Frame{Bounds: bounds}
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
		field := []int{0, 1, 3, 4, 5, 6}[i]
		if len(nav) < height-2 {
			f.Regions = append(f.Regions, terminalui.Region{ID: fmt.Sprintf("facet-%d", field), Kind: terminalui.DestinationRegion, Bounds: terminalui.Rect{X: bounds.X + 1, Y: bounds.Y + 1 + len(nav), W: left - 2, H: 1}})
		}
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
	facet := m.facetFrame(terminalui.Rect{X: bounds.X + left + 2, Y: bounds.Y + 1, W: right - 4, H: height - 2})
	lines := facet.Lines
	visibleLines := len(lines)
	// Keep validation feedback visible even on a 60×16 terminal.
	if e.error != "" {
		errors := strings.Split(ansi.Hardwrap(oneLine(e.error), right-4, true), "\n")
		lines = lines[:min(len(lines), max(0, height-3-len(errors)))]
		visibleLines = len(lines)
		lines = append(lines, "")
		for _, line := range errors {
			lines = append(lines, m.style(t.Error).Render(line))
		}
	}
	for i := range lines {
		lines[i] = " " + lines[i]
	}
	// Regions clipped out by validation or panel boundaries must not remain
	// clickable beneath the error text or shared footer.
	for _, r := range facet.Regions {
		if r.Bounds.Y+r.Bounds.H <= bounds.Y+1+min(visibleLines, height-2) {
			f.Regions = append(f.Regions, r)
		}
	}
	f.Regions = append(f.Regions,
		terminalui.Region{ID: "facet-nav", Kind: terminalui.PaneRegion, Bounds: terminalui.Rect{X: bounds.X, Y: bounds.Y, W: left, H: height}},
		terminalui.Region{ID: "facet-input", Kind: terminalui.PaneRegion, Bounds: terminalui.Rect{X: bounds.X + left, Y: bounds.Y, W: right, H: height}})
	navOwner, editOwner := -1, m.focus
	if e.sidebar {
		navOwner, editOwner = m.focus, -1
	}
	if e.surfaceFocus != "" {
		navOwner, editOwner = -1, -1
	}
	title := "Edit: " + names[active]
	modeHelp := "Tab panes · ↑/↓ j/k navigate · Ctrl+S apply & back"
	cancelHelp := "Ctrl+L clear filters (keeps time) · Esc cancel"
	if !e.sidebar && e.field != 1 && e.field != 2 && e.field != 6 {
		modeHelp = "Tab panes · Type to edit · Ctrl+S apply & back"
	}
	if m.settings.VimMode {
		mode := "NORMAL"
		modeHelp = "Tab panes · j/k move · Enter/i type · Ctrl+S apply"
		if e.sidebar {
			modeHelp = "Shift+Tab edit · Tab actions · ↑/↓ j/k move"
		} else if e.field == 6 {
			modeHelp = "Tab panes · j/k move · Enter remove · Ctrl+S apply"
		}
		if e.insert {
			mode = "INSERT"
			modeHelp = "Tab panes · Enter/Esc normal · Ctrl+S apply & back"
			cancelHelp = "Ctrl+U clear text · Ctrl+L clear filters (keeps time)"
		}
		title += " · " + mode
	}
	if e.surfaceFocus != "" {
		modeHelp = "Tab/Shift+Tab focus · arrows choose · Enter activate"
	}
	body := lipgloss.JoinHorizontal(lipgloss.Top,
		m.panel("FILTER EDITOR", nav, left, height, navOwner),
		m.panel(title, lines, right, height, editOwner))
	semantics := m.style(t.Muted).Render(fit(" "+cancelHelp, m.width))
	keys := m.style(t.Focus).Render(fit(" "+modeHelp, m.width))
	f.Lines = append(strings.Split(body, "\n"), semantics, keys)
	return f
}

func (m Model) facetFrame(bounds terminalui.Rect) terminalui.Frame {
	e, t := m.editor, m.options.Theme
	width := bounds.W
	f := terminalui.Frame{Bounds: bounds}
	finish := func(lines []string) terminalui.Frame { f.Lines = lines; return f }
	muted := func(s string) string { return m.style(t.Muted).Render(oneLine(s)) }
	input := func(label string, index, row int) []string {
		f.Regions = append(f.Regions, terminalui.Region{ID: fmt.Sprintf("input-%d", index), Kind: terminalui.FieldRegion, Bounds: terminalui.Rect{X: bounds.X, Y: bounds.Y + row, W: width, H: 2}})
		prefix, color := "  ", t.Muted
		if e.field == index && !e.sidebar {
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
		lines = append(lines, input("Suggest apps · keyword only", 0, len(lines))...)
		visible := min(8, max(1, m.height-14))
		start := max(0, e.suggestion-visible+1)
		for i := start; i < min(len(e.suggestions.Items), start+visible); i++ {
			app := e.suggestions.Items[i]
			f.Regions = append(f.Regions, terminalui.Region{ID: "app-row:" + app, Kind: terminalui.RowRegion, Bounds: terminalui.Rect{X: bounds.X, Y: bounds.Y + len(lines), W: width, H: 1}})
			f.Regions = append(f.Regions, terminalui.Region{ID: "app-toggle:" + app, Kind: terminalui.ControlRegion, Bounds: terminalui.Rect{X: bounds.X + 2, Y: bounds.Y + len(lines), W: 3, H: 1}})
			mark, prefix := "[ ] ", "  "
			if slices.Contains(e.draft.Scope.Services, app) {
				mark = "[x] "
			}
			color := t.Foreground
			if i == e.suggestion && !e.sidebar && !e.insert {
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
		help := "↑/↓ choose · Enter toggle · Ctrl+X clear apps"
		if m.settings.VimMode {
			help = "↑/↓ j/k choose · Space toggle · Enter/i keyword"
			if e.insert {
				help = "Type keyword · Enter/Esc normal · Ctrl+X clear apps"
			}
		}
		return finish(append(lines, "", muted(help)))
	case 1, 2:
		lines := input("Start inclusive RFC3339", 1, 0)
		lines = append(lines, input("End exclusive RFC3339", 2, len(lines))...)
		help := "↑/↓ or j/k choose From/To · Ctrl+U clears text"
		if m.settings.VimMode {
			help = "↑/↓ j/k choose From/To · Enter/i type"
			if e.insert {
				help = "Type timestamp · Ctrl+U clears · Esc normal"
			}
		}
		return finish(append(lines, muted(help),
			muted("Z or explicit offset · UTC [from, to)")))
	case 3:
		return finish(append(input("Levels", 3, 0), muted("ALL / UNKNOWN / WARN+ / exact levels"),
			muted("WARN+ = WARN, ERROR, FATAL"), muted("Exact levels: comma-separated")))
	case 4:
		label := "Elastic analyzed phrase"
		if !m.options.LocalLab {
			label = "Fixture phrase approximation"
		}
		return finish(append(input("Message", 4, 0), muted(label), muted("Empty means all messages")))
	case 5:
		return finish(append(input("Trace IDs", 5, 0), muted("Exact values · comma-separated OR")))
	default:
		prefix, color := "› ", t.Focus
		if e.sidebar {
			prefix, color = "  ", t.Muted
		}
		lines := []string{m.style(color).Render(prefix + "Exact field predicates"),
			muted(fmt.Sprintf("%d exact field predicates active", len(e.draft.Fields))), ""}
		if len(e.draft.Fields) == 0 {
			lines = append(lines, muted("None · add from inspector with f"))
		}
		visible := max(1, m.height-13)
		start := max(0, e.predicate-visible+1)
		for i := start; i < min(len(e.draft.Fields), start+visible); i++ {
			field := e.draft.Fields[i]
			f.Regions = append(f.Regions, terminalui.Region{ID: "predicate-row:" + field.Path + ":" + string(field.Value), Kind: terminalui.RowRegion, Bounds: terminalui.Rect{X: bounds.X, Y: bounds.Y + len(lines), W: width, H: 1}})
			f, mark, color := e.draft.Fields[i], "  ", t.Foreground
			if i == e.predicate && !e.sidebar {
				mark, color = "› ", t.Focus
			}
			lines = append(lines, m.style(color).Render(mark+oneLine(f.Path+"="+string(f.Value))))
		}
		return finish(append(lines, "", muted("↑/↓ or j/k choose · Enter remove · Ctrl+R clear fields")))
	}
}
