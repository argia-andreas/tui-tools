package ui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

func (m Model) inspectorLines(width int) []string {
	if len(m.page.Events) == 0 {
		return []string{"No selected event"}
	}
	e := m.page.Events[m.selected]
	t := m.options.Theme
	lines := []string{m.style(t.Muted).Render("POLICY-SAFE EVENT"), ""}
	for _, field := range []string{"time", "level", "service", "instance", "trace.id"} {
		if value := e.Text(field); value != "" {
			color := t.Foreground
			if field == "level" {
				switch value {
				case "ERROR":
					color = t.Error
				case "WARN":
					color = t.Warning
				}
			}
			lines = append(lines, m.style(t.Focus).Render(field), "  "+m.style(color).Render(value), "")
		}
	}
	lines = append(lines, m.style(t.Focus).Bold(true).Render("MESSAGE"), m.style(t.Foreground).Render(e.Text("message")), "", m.style(t.Focus).Bold(true).Render("STRUCTURED FIELDS"))
	var fields any
	d := json.NewDecoder(bytes.NewReader(e.Values["fields"]))
	d.UseNumber()
	if d.Decode(&fields) == nil {
		lines = append(lines, m.fieldLines(fields, "")...)
	}
	var wrapped []string
	for _, line := range lines {
		wrapped = append(wrapped, strings.Split(ansi.Hardwrap(line, max(1, width), true), "\n")...)
	}
	return wrapped
}

// Keep JSON types and object boundaries. Multiline strings are displayed as
// multiline content, not escaped JSON; this is not a separate unsafe "raw" view.
// Values and keys have already passed the service's output policy. Only trusted
// theme styling is added here, before ANSI-aware wrapping.
func (m Model) fieldLines(value any, indent string) []string {
	t := m.options.Theme
	switch value := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(value))
		for key := range value {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		if len(keys) == 0 {
			return []string{indent + m.style(t.Muted).Render("{}")}
		}
		var lines []string
		for _, key := range keys {
			child := value[key]
			label := m.style(t.Focus).Render(key) + m.style(t.Muted).Render(":")
			switch child.(type) {
			case map[string]any, []any:
				lines = append(lines, indent+label)
				lines = append(lines, m.fieldLines(child, indent+"  ")...)
			default:
				parts := m.fieldLines(child, "")
				if stack, ok := child.(string); ok && key == "stack_trace" {
					parts = strings.Split(stack, "\n")
					for i := range parts {
						parts[i] = m.stackLine(parts[i])
					}
				}
				lines = append(lines, indent+label+" "+parts[0])
				for _, part := range parts[1:] {
					lines = append(lines, indent+"  "+part)
				}
			}
		}
		return lines
	case []any:
		if len(value) == 0 {
			return []string{indent + m.style(t.Muted).Render("[]")}
		}
		var lines []string
		for i, child := range value {
			lines = append(lines, indent+m.style(t.Muted).Render(fmt.Sprintf("[%d]", i)))
			lines = append(lines, m.fieldLines(child, indent+"  ")...)
		}
		return lines
	case string:
		lines := strings.Split(value, "\n")
		for i := range lines {
			lines[i] = indent + m.style(t.Foreground).Render(lines[i])
		}
		return lines
	default:
		color := t.Foreground
		switch value.(type) {
		case json.Number:
			color = t.Warning
		case bool:
			color = t.Selection
		case nil:
			color = t.Muted
		}
		data, _ := json.Marshal(value)
		return []string{indent + m.style(color).Render(string(data))}
	}
}

// Recognize only familiar Go frame shapes. Captured spans are colored in place:
// whitespace, arguments, offsets, and unrecognized lines remain unchanged.
var (
	stackPath     = regexp.MustCompile(`^(\s*)((?:/|[A-Za-z]:[\\/])[^\n]+):([0-9]+)(\s+\+0x[0-9a-fA-F]+)?$`)
	stackFunction = regexp.MustCompile(`^(\s*)((?:[[:alnum:]_~.-]+/)*[[:alnum:]_~.-]+\.(?:\(\*?[^()\s]+\)\.)?[[:alnum:]_·]+)(\(.*\))?$`)
)

func (m Model) stackLine(line string) string {
	t := m.options.Theme
	if spans := stackPath.FindStringSubmatchIndex(line); spans != nil {
		pathStart, pathEnd := spans[4], spans[5]
		numberStart, numberEnd := spans[6], spans[7]
		return line[:pathStart] + m.style(t.Muted).Render(line[pathStart:pathEnd]) +
			line[pathEnd:numberStart] + m.style(t.Warning).Render(line[numberStart:numberEnd]) +
			m.style(t.Foreground).Render(line[numberEnd:])
	}
	if spans := stackFunction.FindStringSubmatchIndex(line); spans != nil {
		start, end := spans[4], spans[5]
		return line[:start] + m.style(t.Focus).Render(line[start:end]) + m.style(t.Foreground).Render(line[end:])
	}
	return m.style(t.Foreground).Render(line)
}
