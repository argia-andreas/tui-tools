package ui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"logviewer/internal/logs"
)

type fieldChoice struct {
	path        string
	value       json.RawMessage
	unavailable string
}

func (m Model) openFieldPicker() (tea.Model, tea.Cmd) {
	if len(m.page.Events) == 0 {
		m.status = "No selected event to filter"
		return m, nil
	}
	supported := map[string]bool{}
	for _, path := range m.options.Service.FilterFields() {
		supported[path] = true
	}
	var choices []fieldChoice
	var walk func(string, any)
	walk = func(path string, value any) {
		data, _ := json.Marshal(value)
		choice := fieldChoice{path: path, value: data}
		switch v := value.(type) {
		case map[string]any:
			choice.unavailable = "object · choose a scalar child"
			choices = append(choices, choice)
			for k, child := range v {
				walk(path+"/"+strings.ReplaceAll(strings.ReplaceAll(k, "~", "~0"), "/", "~1"), child)
			}
			return
		case []any:
			choice.unavailable = "array · exact filtering unavailable"
			choices = append(choices, choice)
			for i, child := range v {
				walk(path+"/"+strconv.Itoa(i), child)
			}
			return
		case nil:
			choice.unavailable = "null · exact filtering unavailable"
		case string:
			if strings.Contains(v, "[REDACTED]") {
				choice.unavailable = "redacted value · cannot filter"
			}
		}
		if choice.unavailable == "" && !supported[path] {
			choice.unavailable = "source does not support this field"
		}
		choices = append(choices, choice)
	}
	for k, data := range m.page.Events[m.selected].Values {
		var v any
		d := json.NewDecoder(bytes.NewReader(data))
		d.UseNumber()
		if d.Decode(&v) == nil {
			walk("/"+strings.ReplaceAll(strings.ReplaceAll(k, "~", "~0"), "/", "~1"), v)
		}
	}
	sort.Slice(choices, func(i, j int) bool {
		// Trace is the first common investigation action. Supported scalar
		// fields follow, then explicitly unavailable values; no syntax needed.
		rank := func(c fieldChoice) int {
			if c.unavailable != "" {
				return 2
			}
			if c.path == "/trace.id" {
				return 0
			}
			return 1
		}
		if rank(choices[i]) != rank(choices[j]) {
			return rank(choices[i]) < rank(choices[j])
		}
		return choices[i].path < choices[j].path
	})
	m.editor = &editor{kind: "field", returnFocus: m.focus, choices: choices}
	return m, nil
}

func (m Model) editField(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	e := m.editor
	switch msg.String() {
	case "j", "down":
		e.suggestion = min(max(0, len(e.choices)-1), e.suggestion+1)
	case "k", "up":
		e.suggestion = max(0, e.suggestion-1)
	case "g", "home":
		e.suggestion = 0
	case "G", "end":
		e.suggestion = max(0, len(e.choices)-1)
	case "enter":
		if len(e.choices) == 0 {
			break
		}
		choice := e.choices[e.suggestion]
		if choice.unavailable != "" {
			e.error = "Unavailable: " + choice.unavailable
			break
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
			q.Fields = append(q.Fields, logs.FieldFilter{Path: choice.path, Value: choice.value})
		}
		if err := logs.ValidateQuery(q); err != nil {
			e.error = "Exact filter unavailable for this value or query budget"
			break
		}
		m.options.Query = q
		m.focus = e.returnFocus
		m.editor = nil
		return m.refresh()
	}
	return m, nil
}

func (m Model) fieldPickerLines() (string, []string) {
	e := m.editor
	lines := []string{"j/k choose field · Enter exact filter · Esc returns inspector", "AND with current filters · full safe typed value, not display truncation", e.error}
	visible := max(1, m.height-8)
	start := max(0, e.suggestion-visible/2)
	start = min(start, max(0, len(e.choices)-visible))
	for i := start; i < min(len(e.choices), start+visible); i++ {
		c := e.choices[i]
		prefix := "  "
		if i == e.suggestion {
			prefix = "› "
		}
		label := fmt.Sprintf("%s%s = %s", prefix, c.path, string(c.value))
		if c.unavailable != "" {
			label = prefix + c.path + " · UNAVAILABLE: " + c.unavailable
		}
		lines = append(lines, label)
	}
	return "FILTER FROM EVENT · EXACT", lines
}
