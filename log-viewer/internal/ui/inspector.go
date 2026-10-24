package ui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

func (m Model) inspectorLines(width int) []string {
	if len(m.page.Events) == 0 {
		return []string{"No selected event"}
	}
	e := m.page.Events[m.selected]
	lines := []string{"POLICY-SAFE EVENT", ""}
	for _, field := range []string{"time", "level", "service", "instance", "trace.id"} {
		if value := e.Text(field); value != "" {
			lines = append(lines, field, "  "+value, "")
		}
	}
	lines = append(lines, "MESSAGE", e.Text("message"), "", "STRUCTURED FIELDS")
	var fields any
	d := json.NewDecoder(bytes.NewReader(e.Values["fields"]))
	d.UseNumber()
	if d.Decode(&fields) == nil {
		lines = append(lines, fieldLines(fields, "")...)
	}
	var wrapped []string
	for _, line := range lines {
		wrapped = append(wrapped, strings.Split(ansi.Hardwrap(line, max(1, width), true), "\n")...)
	}
	return wrapped
}

// Keep JSON types and object boundaries. Multiline strings are displayed as
// multiline content, not escaped JSON; this is not a separate unsafe "raw" view.
func fieldLines(value any, indent string) []string {
	switch value := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(value))
		for key := range value {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		if len(keys) == 0 {
			return []string{indent + "{}"}
		}
		var lines []string
		for _, key := range keys {
			child := value[key]
			switch child.(type) {
			case map[string]any, []any:
				lines = append(lines, indent+key+":")
				lines = append(lines, fieldLines(child, indent+"  ")...)
			default:
				parts := fieldLines(child, "")
				lines = append(lines, indent+key+": "+parts[0])
				for _, part := range parts[1:] {
					lines = append(lines, indent+"  "+part)
				}
			}
		}
		return lines
	case []any:
		if len(value) == 0 {
			return []string{indent + "[]"}
		}
		var lines []string
		for i, child := range value {
			lines = append(lines, fmt.Sprintf("%s[%d]", indent, i))
			lines = append(lines, fieldLines(child, indent+"  ")...)
		}
		return lines
	case string:
		lines := strings.Split(value, "\n")
		for i := range lines {
			lines[i] = indent + lines[i]
		}
		return lines
	default:
		data, _ := json.Marshal(value)
		return []string{indent + string(data)}
	}
}
