package ui

import (
	"bytes"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"logviewer/internal/config"
	"logviewer/internal/logs"
)

// editorAction executes semantic editor intents, independently of local key
// grammar. In particular Apply is not Enter, and Cancel is not INSERT's Esc.
func (m Model) editorAction(id string) (tea.Model, tea.Cmd) {
	e := m.editor
	if e == nil {
		return m, nil
	}
	switch id {
	case "cancel":
		if m.suggestionCancel != nil {
			m.suggestionCancel()
		}
		m.focus, m.editor = e.returnFocus, nil
		m.suggestionGeneration++
	case "apply":
		if e.kind != "filter" {
			return m, nil
		}
		q, err := e.query()
		if err != nil {
			e.error = err.Error()
			return m, nil
		}
		if m.suggestionCancel != nil {
			m.suggestionCancel()
		}
		m.options.Query = q
		m.focus, m.editor = e.returnFocus, nil
		m.suggestionGeneration++
		return m.refresh()
	case "save-settings":
		if e.kind != "settings" {
			return m, nil
		}
		settings := config.Settings{VimMode: e.vimMode}
		if m.options.SettingsPath != "" {
			if err := config.SaveSettings(m.options.SettingsPath, settings); err != nil {
				e.error = "Settings save failed · check private destination"
				return m, nil
			}
		}
		m.settings = settings
		m.focus, m.editor = e.returnFocus, nil
		m.status = "Settings applied for this session"
		if m.options.SettingsPath != "" {
			m.status = "Settings saved"
		}
	case "toggle-vim":
		if e.kind == "settings" {
			e.vimMode = !e.vimMode
		}
	case "toggle-case":
		if e.kind == "find" {
			e.caseSensitive = !e.caseSensitive
		}
	case "find":
		if e.kind != "find" {
			return m, nil
		}
		m.find, m.findCase = e.text, e.caseSensitive
		m.focus, m.editor = e.returnFocus, nil
		m.findNext(false)
	case "save-search":
		if e.kind != "save" {
			return m, nil
		}
		if m.options.SearchesPath == "" {
			e.error = "Saved searches unavailable: no explicit --searches path"
			return m, nil
		}
		if strings.TrimSpace(e.text) == "" {
			e.error = "Enter a name"
			return m, nil
		}
		if err := config.SaveSearch(m.options.SearchesPath, config.SavedSearch{Name: e.text, Query: m.options.Query}); err != nil {
			e.error = "Save failed · check name and private destination"
			return m, nil
		}
		m.focus, m.editor = e.returnFocus, nil
		m.reloadSaved()
		m.status = "Saved search explicitly · exact apps and absolute UTC window retained"
	case "restore-search":
		if e.kind != "restore" || e.suggestion < 0 || e.suggestion >= len(e.saved) {
			return m, nil
		}
		q := e.saved[e.suggestion].Query
		if q.Scope.Source != m.options.Query.Scope.Source {
			e.error = "Cannot restore a different source"
			return m, nil
		}
		if err := logs.ValidateQuery(q); err != nil {
			e.error = "Saved query invalid"
			return m, nil
		}
		q.Scope.Services = append([]string{}, q.Scope.Services...)
		q.Levels = append([]string{}, q.Levels...)
		q.Fields = append([]logs.FieldFilter{}, q.Fields...)
		q.TraceIDs = append([]string{}, q.TraceIDs...)
		m.options.Query = q
		m.focus, m.editor = e.returnFocus, nil
		return m.refresh()
	case "field-apply":
		if e.kind != "field" || e.suggestion < 0 || e.suggestion >= len(e.choices) {
			return m, nil
		}
		choice := e.choices[e.suggestion]
		if choice.unavailable != "" {
			e.error = "Unavailable: " + choice.unavailable
			return m, nil
		}
		q := m.options.Query
		q.Fields = append([]logs.FieldFilter{}, q.Fields...)
		duplicate := false
		for _, f := range q.Fields {
			if f.Path == choice.path && bytes.Equal(f.Value, choice.value) {
				duplicate = true
			}
		}
		if !duplicate {
			q.Fields = append(q.Fields, logs.FieldFilter{Path: choice.path, Value: append([]byte{}, choice.value...)})
		}
		if err := logs.ValidateQuery(q); err != nil {
			e.error = "Exact filter unavailable for this value or query budget"
			return m, nil
		}
		m.options.Query = q
		m.focus, m.editor = e.returnFocus, nil
		return m.refresh()
	case "toggle-app":
		if e.kind != "filter" || e.suggestion < 0 || e.suggestion >= len(e.suggestions.Items) {
			return m, nil
		}
		app := e.suggestions.Items[e.suggestion]
		apps := append([]string{}, e.draft.Scope.Services...)
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
		// Retain suggestions for repeated exact toggles; the keyword is not a filter.
		e.draft.Scope.Services = apps
	case "remove-predicate":
		if e.kind == "filter" && e.predicate >= 0 && e.predicate < len(e.draft.Fields) {
			fields := append([]logs.FieldFilter{}, e.draft.Fields...)
			e.draft.Fields = append(fields[:e.predicate], fields[e.predicate+1:]...)
			e.predicate = min(e.predicate, max(0, len(e.draft.Fields)-1))
		}
	case "clear-filters":
		if e.kind != "filter" {
			return m, nil
		}
		e.draft.Scope.Services, e.draft.Fields = nil, nil
		e.inputs[0], e.inputs[3], e.inputs[4], e.inputs[5] = "", "ALL", "", ""
		e.predicate, e.suggestion, e.error = 0, 0, ""
		return m.suggest()
	case "clear-apps":
		if e.kind != "filter" {
			return m, nil
		}
		e.draft.Scope.Services = nil
		return m.suggest()
	case "clear-fields":
		if e.kind == "filter" {
			e.draft.Fields = nil
		}
	}
	return m, nil
}
