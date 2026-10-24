package ui

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"logviewer/internal/config"
	"logviewer/internal/logs"
)

type editor struct {
	kind, text         string
	field, returnFocus int
	draft              logs.Query
	inputs             [7]string
	sidebar            bool
	suggestions        logs.Values
	suggestion         int
	suggestionError    bool
	suggestionLoading  bool
	caseSensitive      bool
	error              string
	saved              []config.SavedSearch
	choices            []fieldChoice
	predicate          int
}
type suggestionsMsg struct {
	generation uint64
	values     logs.Values
	err        error
}

func (m Model) openFilters() (tea.Model, tea.Cmd) {
	q := m.options.Query
	q.Scope.Services = append([]string{}, q.Scope.Services...)
	q.Levels = append([]string{}, q.Levels...)
	q.Fields = append([]logs.FieldFilter{}, q.Fields...)
	q.TraceIDs = append([]string{}, q.TraceIDs...)
	levels := strings.Join(q.Levels, ",")
	if levels == "" {
		levels = "ALL"
	}
	m.editor = &editor{kind: "filter", draft: q, returnFocus: m.focus,
		inputs: [7]string{"", q.Window.Start.UTC().Format(time.RFC3339Nano), q.Window.End.UTC().Format(time.RFC3339Nano), levels, q.Message.Text, strings.Join(q.TraceIDs, ","), ""}}
	return m.suggest()
}

func (m Model) suggest() (tea.Model, tea.Cmd) {
	if m.suggestionCancel != nil {
		m.suggestionCancel()
	}
	ctx, cancel := context.WithTimeout(m.options.Context, 10*time.Second)
	m.suggestionCancel = cancel
	m.suggestionGeneration++
	generation := m.suggestionGeneration
	q, err := m.editor.query()
	if err != nil {
		q = m.editor.draft
	}
	// App selection is an OR facet, not a constraint on its own suggestions.
	// Other predicates, including an inspected /service predicate, still apply.
	q.Scope.Services = nil
	q.Levels = append([]string{}, q.Levels...)
	q.Fields = append([]logs.FieldFilter{}, q.Fields...)
	keyword := m.editor.inputs[0]
	m.editor.suggestions = logs.Values{}
	m.editor.suggestionLoading = true
	return m, func() tea.Msg {
		defer cancel()
		values, err := m.options.Service.Values(ctx, q, "service", keyword)
		return suggestionsMsg{generation, values, err}
	}
}

func (e *editor) query() (logs.Query, error) {
	q := e.draft
	var err error
	q.Window.Start, err = config.ParseTime(e.inputs[1])
	if err != nil {
		return q, fmt.Errorf("Start: RFC3339 requires Z or explicit offset")
	}
	q.Window.End, err = config.ParseTime(e.inputs[2])
	if err != nil {
		return q, fmt.Errorf("End: RFC3339 requires Z or explicit offset")
	}
	q.Window.Start, q.Window.End = q.Window.Start.UTC(), q.Window.End.UTC()
	level := strings.ToUpper(strings.TrimSpace(e.inputs[3]))
	switch level {
	case "ALL", "":
		q.Levels = nil
	case "WARN+":
		q.Levels = []string{"WARN", "ERROR", "FATAL"}
	default:
		q.Levels = strings.Split(level, ",")
		for i := range q.Levels {
			q.Levels[i] = strings.TrimSpace(q.Levels[i])
		}
	}
	q.Message = logs.MessageFilter{}
	if e.inputs[4] != "" {
		q.Message = logs.MessageFilter{Mode: "phrase", Text: e.inputs[4]}
	}
	q.TraceIDs = nil
	if strings.TrimSpace(e.inputs[5]) != "" {
		for _, id := range strings.Split(e.inputs[5], ",") {
			q.TraceIDs = append(q.TraceIDs, strings.TrimSpace(id))
		}
	}
	if err := logs.ValidateQuery(q); err != nil {
		return q, fmt.Errorf("Invalid filters: check time order, exact levels and message")
	}
	return q, nil
}

func (m Model) openRestore() (tea.Model, tea.Cmd) {
	if m.options.SearchesPath == "" {
		m.status = "Saved searches unavailable: no explicit --searches path"
		return m, nil
	}
	saved, err := config.LoadSearches(m.options.SearchesPath)
	if err != nil {
		m.status = "Saved searches read failed · check private file"
		return m, nil
	}
	m.reloadSaved()
	m.editor = &editor{kind: "restore", saved: saved, returnFocus: m.focus}
	return m, nil
}

func appendInput(text string, msg tea.KeyMsg) string {
	switch msg.Type {
	case tea.KeyBackspace, tea.KeyDelete:
		r := []rune(text)
		if len(r) > 0 {
			return string(r[:len(r)-1])
		}
	case tea.KeyCtrlU:
		return ""
	case tea.KeySpace:
		if len(text) < 4096 {
			return text + " "
		}
	case tea.KeyRunes:
		for _, r := range msg.Runes {
			if !unicode.IsControl(r) && !unicode.In(r, unicode.Cf) && len(text) < 4096 {
				text += string(r)
			}
		}
	}
	return text
}

func (m Model) edit(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	e := m.editor
	key := msg.String()
	if key == "esc" {
		if m.suggestionCancel != nil {
			m.suggestionCancel()
		}
		m.focus = e.returnFocus
		m.editor = nil
		m.suggestionGeneration++
		return m, nil
	}
	// Editors own every printable key, including q, y and Space. No global
	// action is dispatched while input is owned here.
	switch e.kind {
	case "palette":
		return m.editPalette(msg)
	case "field":
		return m.editField(msg)
	case "find":
		switch key {
		case "ctrl+t":
			e.caseSensitive = !e.caseSensitive
		case "enter":
			m.find, m.findCase = e.text, e.caseSensitive
			m.focus = e.returnFocus
			m.editor = nil
			m.findNext(false)
		default:
			e.text = appendInput(e.text, msg)
		}
	case "save":
		if key == "enter" {
			if strings.TrimSpace(e.text) == "" {
				e.error = "Enter a name"
				break
			}
			if err := config.SaveSearch(m.options.SearchesPath, config.SavedSearch{Name: e.text, Query: m.options.Query}); err != nil {
				e.error = "Save failed · check name and private destination"
				break
			}
			m.focus = e.returnFocus
			m.editor = nil
			m.reloadSaved()
			m.status = "Saved search explicitly · exact apps and absolute UTC window retained"
		} else {
			e.text = appendInput(e.text, msg)
		}
	case "restore":
		switch key {
		case "down", "j":
			e.suggestion = min(len(e.saved)-1, e.suggestion+1)
		case "up", "k":
			e.suggestion = max(0, e.suggestion-1)
		case "enter":
			if len(e.saved) == 0 {
				break
			}
			q := e.saved[e.suggestion].Query
			if q.Scope.Source != m.options.Query.Scope.Source {
				e.error = "Cannot restore a different source"
				break
			}
			if err := logs.ValidateQuery(q); err != nil {
				e.error = "Saved query invalid"
				break
			}
			m.options.Query = q
			m.focus = e.returnFocus
			m.editor = nil
			return m.refresh()
		}
	case "filter":
		if msg.Type == tea.KeyRunes && len(msg.Runes) > 1 && (e.sidebar || e.field == 1 || e.field == 2 || e.field == 6) {
			var commands []tea.Cmd
			for _, r := range msg.Runes {
				next, cmd := m.edit(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
				m = next.(Model)
				commands = append(commands, cmd)
			}
			return m, tea.Batch(commands...)
		}
		if key == "tab" || key == "shift+tab" {
			e.sidebar = !e.sidebar
			return m, nil
		}
		if key == "ctrl+l" {
			e.draft.Scope.Services = nil
			e.draft.Fields = nil
			e.inputs[0], e.inputs[3], e.inputs[4], e.inputs[5] = "", "ALL", "", ""
			e.predicate, e.suggestion, e.error = 0, 0, ""
			return m.suggest()
		}
		if e.sidebar && key != "ctrl+s" {
			switch key {
			case "down", "j", "up", "k":
				field := e.field
				if field == 2 {
					field = 1
				}
				if key == "down" || key == "j" {
					field = (field + 1) % 7
					if field == 2 {
						field = 3
					}
				} else {
					field = (field + 6) % 7
					if field == 2 {
						field = 1
					}
				}
				e.field = field
				if field == 0 {
					return m.suggest()
				}
			}
			return m, nil
		}
		// j/k navigate non-text fields only; app keywords and other text
		// inputs must retain these characters literally.
		if e.field == 1 || e.field == 2 || e.field == 6 {
			if key == "j" {
				key = "down"
			} else if key == "k" {
				key = "up"
			}
		}
		switch key {
		case "up":
			if e.field == 1 || e.field == 2 {
				e.field = 1
				break
			}
			if e.field == 6 {
				e.predicate = max(0, e.predicate-1)
				break
			}
			e.suggestion = max(0, e.suggestion-1)
		case "down":
			if e.field == 1 || e.field == 2 {
				e.field = 2
				break
			}
			if e.field == 6 {
				e.predicate = min(max(0, len(e.draft.Fields)-1), e.predicate+1)
				break
			}
			e.suggestion = min(max(0, len(e.suggestions.Items)-1), e.suggestion+1)
		case "ctrl+x":
			if e.field == 0 {
				e.draft.Scope.Services = nil
				return m.suggest()
			}
		case "ctrl+r":
			e.draft.Fields = nil
		case "enter", "ctrl+s":
			if key == "enter" && e.field == 6 {
				if len(e.draft.Fields) > 0 {
					e.draft.Fields = append(e.draft.Fields[:e.predicate], e.draft.Fields[e.predicate+1:]...)
					e.predicate = min(e.predicate, max(0, len(e.draft.Fields)-1))
				}
				break
			}
			if key == "enter" && e.field == 0 {
				if len(e.suggestions.Items) == 0 {
					break
				}
				app := e.suggestions.Items[e.suggestion]
				apps := e.draft.Scope.Services
				found := -1
				for i, s := range apps {
					if s == app {
						found = i
						break
					}
				}
				if found >= 0 {
					apps = append(apps[:found], apps[found+1:]...)
				} else {
					apps = append(apps, app)
				}
				e.draft.Scope.Services = apps
				// Keep the current suggestion list so repeated Enter toggles the
				// same exact value; the keyword itself is NEVER a filter.
				break
			}
			q, err := e.query()
			if err != nil {
				e.error = err.Error()
				break
			}
			if m.suggestionCancel != nil {
				m.suggestionCancel()
			}
			m.options.Query = q
			m.focus = e.returnFocus
			m.editor = nil
			m.suggestionGeneration++
			return m.refresh()
		default:
			if e.field == 6 {
				break
			} // Predicate list owns keys, but is not text input.
			before := e.inputs[e.field]
			e.inputs[e.field] = appendInput(before, msg)
			if e.field == 0 && before != e.inputs[0] {
				return m.suggest()
			}
		}
	}
	return m, nil
}

func (m Model) editorLines() (string, []string) {
	e := m.editor
	switch e.kind {
	case "palette":
		return m.paletteLines()
	case "field":
		return m.fieldPickerLines()
	case "find":
		return "LOCAL LITERAL FIND", []string{"LOADED policy-safe event data only (all structured fields)", "Ctrl+T toggles case · Enter find · Esc cancel", fmt.Sprintf("Case sensitive: %t", e.caseSensitive), "> " + e.text}
	case "save":
		return "SAVE NAMED SEARCH", []string{"Explicit private save · no event payloads or automatic history", "Exact selected apps and absolute time window are saved", "Enter save/replace name · Esc cancel", "> " + e.text, e.error}
	default:
		lines := []string{"Up/Down choose · Enter restore · Esc cancel", "Only same-source queries can be restored"}
		start := max(0, e.suggestion-3)
		for i := start; i < min(len(e.saved), start+8); i++ {
			prefix := "  "
			if i == e.suggestion {
				prefix = "› "
			}
			lines = append(lines, prefix+e.saved[i].Name)
		}
		if len(e.saved) == 0 {
			lines = append(lines, "No saved searches")
		}
		return "RESTORE SAVED SEARCH", append(lines, e.error)
	}
}
