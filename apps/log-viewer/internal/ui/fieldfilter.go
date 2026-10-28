package ui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"terminalui"
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
	fields := m.inspectorFields()
	if len(fields) > 0 {
		path := fields[min(m.fieldIndex, len(fields)-1)].path
		for i, c := range choices {
			if c.path == path {
				m.editor.suggestion = i
				break
			}
		}
	}
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
		return m.editorAction("field-apply")
	}
	return m, nil
}

func (m Model) fieldPickerLines(row func(string, int, terminalui.RegionKind)) (string, []string) {
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
		row("field-row:"+c.path, len(lines), terminalui.RowRegion)
		lines = append(lines, label)
	}
	return "FILTER FROM EVENT · EXACT", lines
}
