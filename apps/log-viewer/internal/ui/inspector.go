package ui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

type inspectorField struct {
	path, label string
	value       any
	depth       int
	container   bool
}

func (m Model) inspectorFields() []inspectorField {
	if len(m.page.Events) == 0 {
		return nil
	}
	var fields []inspectorField
	var walk func(string, string, any, int)
	walk = func(path, label string, value any, depth int) {
		f := inspectorField{path: path, label: label, value: value, depth: depth}
		switch value.(type) {
		case map[string]any, []any:
			f.container = true
		}
		fields = append(fields, f)
		if m.collapsed[path] {
			return
		}
		switch v := value.(type) {
		case map[string]any:
			keys := make([]string, 0, len(v))
			for k := range v {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				walk(path+"/"+pointerKey(k), k, v[k], depth+1)
			}
		case []any:
			for i, child := range v {
				walk(fmt.Sprintf("%s/%d", path, i), fmt.Sprintf("[%d]", i), child, depth+1)
			}
		}
	}
	for _, key := range []string{"time", "level", "service", "instance", "trace.id", "message", "fields"} {
		data, ok := m.page.Events[m.selected].Values[key]
		if !ok {
			continue
		}
		var value any
		d := json.NewDecoder(bytes.NewReader(data))
		d.UseNumber()
		if d.Decode(&value) == nil {
			walk("/"+pointerKey(key), key, value, 0)
		}
	}
	return fields
}

func pointerKey(key string) string {
	return strings.ReplaceAll(strings.ReplaceAll(key, "~", "~0"), "/", "~1")
}

// Each rendered line retains its owning field. Display wrapping never changes
// the full policy-safe value used by yank or exact filtering.
func (m Model) inspectorRows(width int) ([]string, []int) {
	fields := m.inspectorFields()
	lines := []string{m.style(m.options.Theme.Muted).Render("POLICY-SAFE EVENT")}
	owners := []int{-1}
	for i, f := range fields {
		marker := "  "
		if m.focus == 2 && i == m.fieldIndex {
			marker = "› "
		}
		label := strings.Repeat("  ", f.depth) + f.label
		var parts []string
		if f.container {
			state := "▾"
			if m.collapsed[f.path] {
				state = "▸"
			}
			parts = []string{m.style(m.options.Theme.Focus).Render(label + " " + state)}
		} else {
			parts = m.scalarLines(f.value)
			if text, ok := f.value.(string); ok && f.label == "stack_trace" {
				parts = strings.Split(text, "\n")
				for n := range parts {
					parts[n] = m.stackLine(parts[n])
				}
			}
			if f.path == "/level" {
				color := m.options.Theme.Foreground
				if f.value == "ERROR" {
					color = m.options.Theme.Error
				}
				if f.value == "WARN" {
					color = m.options.Theme.Warning
				}
				parts = []string{m.style(color).Render(fmt.Sprint(f.value))}
			}
			parts[0] = m.style(m.options.Theme.Focus).Render(label+": ") + parts[0]
		}
		var wrapped []string
		for _, part := range parts {
			wrapped = append(wrapped, strings.Split(ansi.Hardwrap(part, max(1, width-2), true), "\n")...)
		}
		if !f.container && !m.expanded[f.path] && len(wrapped) > 1 {
			wrapped = []string{ansi.Truncate(wrapped[0], max(1, width-3), "") + "…"}
		}
		for n, line := range wrapped {
			prefix := "  "
			if n == 0 {
				prefix = marker
			}
			line = prefix + line
			if m.focus == 2 && i == m.fieldIndex && (n == 0 || len(lines) == m.inspectorOffset) {
				t := m.options.Theme
				line = m.style(t.SelectionText).Background(lipgloss.Color(t.Selection)).Render(fit(ansi.Strip(line), width))
			}
			lines = append(lines, line)
			owners = append(owners, i)
		}
	}
	return lines, owners
}

func (m Model) inspectorLines(width int) []string {
	lines, _ := m.inspectorRows(width)
	return lines
}

func (m *Model) revealField() {
	fields := m.inspectorFields()
	m.fieldIndex = max(0, min(m.fieldIndex, len(fields)-1))
	_, owners := m.inspectorRows(m.inspectorWidth())
	for row, owner := range owners {
		if owner == m.fieldIndex {
			if row < m.inspectorOffset || row >= m.inspectorOffset+max(1, m.height-12) {
				m.inspectorOffset = row
			}
			break
		}
	}
	m.clampInspector()
}

func (m *Model) toggleField() {
	fields := m.inspectorFields()
	if len(fields) == 0 {
		return
	}
	f := fields[min(m.fieldIndex, len(fields)-1)]
	if f.container {
		m.collapsed = cloneFlags(m.collapsed)
		m.collapsed[f.path] = !m.collapsed[f.path]
	} else {
		m.expanded = cloneFlags(m.expanded)
		m.expanded[f.path] = !m.expanded[f.path]
	}
	m.revealField()
}

func cloneFlags(flags map[string]bool) map[string]bool {
	copy := make(map[string]bool, len(flags)+1)
	for k, v := range flags {
		copy[k] = v
	}
	return copy
}

// Values have already passed the service's output policy. Only trusted theme
// styling is added here, before ANSI-aware wrapping.
func (m Model) scalarLines(value any) []string {
	t := m.options.Theme
	switch value := value.(type) {
	case string:
		lines := strings.Split(value, "\n")
		for i := range lines {
			lines[i] = m.style(t.Foreground).Render(lines[i])
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
		return []string{m.style(color).Render(string(data))}
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
