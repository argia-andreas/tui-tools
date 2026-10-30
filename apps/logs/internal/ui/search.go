package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"logviewer/internal/logs"
)

func (m Model) closeCursor(handle string) tea.Cmd {
	if handle == "" {
		return nil
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = m.options.Service.Close(ctx, handle)
		return nil
	}
}

func (m Model) searchCommand(older bool) tea.Cmd {
	return func() tea.Msg {
		var p logs.Page
		var err error
		if older {
			p, err = m.options.Service.Next(m.requestContext, m.page.Next)
		} else {
			p, err = m.options.Service.Search(m.requestContext, m.options.Query)
		}
		// Even providers that complete after cancellation must release their cursor.
		if m.requestContext.Err() != nil && p.Next != "" {
			m.closeCursor(p.Next)()
			p = logs.Page{}
		}
		return loadedMsg{page: p, err: err, generation: m.generation, older: older}
	}
}

func (m Model) refresh() (tea.Model, tea.Cmd) {
	m.cancel()
	closeOld := m.closeCursor(m.page.Next)
	m.generation++
	m.requestContext, m.cancel = context.WithCancel(m.options.Context)
	m.loading, m.loaded, m.loadError, m.budgetStopped = true, false, false, false
	m.page = logs.Page{}
	m.selected, m.top, m.inspectorOffset = 0, 0, 0
	m.fieldIndex = 0
	m.status = "Loading search · obsolete requests cancelled"
	return m, tea.Batch(closeOld, m.searchCommand(false))
}

func (m Model) receivePage(msg loadedMsg) (tea.Model, tea.Cmd) {
	if msg.generation != m.generation {
		return m, m.closeCursor(msg.page.Next)
	}
	m.loading, m.loaded = false, true
	if msg.err != nil {
		if msg.older {
			m.page.Next = ""
			m.status = "Older history read failed · loaded events retained · r reload"
		} else {
			m.loadError = true
			m.page = logs.Page{}
			m.status = "Source read failed · no event data retained · r retry"
		}
		return m, m.closeCursor(msg.page.Next)
	}
	anchor := ""
	if msg.older && len(m.page.Events) > 0 {
		anchor = m.page.Events[m.selected].Handle
	}
	events := msg.page.Events
	partial := msg.page.Partial
	var previous []logs.SafeEvent
	seen := map[string]bool{}
	bytes := 0
	if msg.older {
		previous = m.page.Events
		partial = partial || m.page.Partial
		// Reserve the loaded page before accepting older rows: a new page
		// cannot evict the selected event when a session budget is reached.
		for _, e := range previous {
			data, _ := json.Marshal(e)
			bytes += len(data)
			seen[e.Handle] = true
		}
	}
	// Stop collection rather than silently evicting the selected event.
	retained := make([]logs.SafeEvent, 0, min(len(events)+len(previous), 10000))
	for _, e := range events {
		if e.Handle != "" && seen[e.Handle] {
			continue
		}
		data, _ := json.Marshal(e)
		if len(retained)+len(previous) == 10000 || bytes+len(data) > 32<<20 {
			m.budgetStopped = true
			break
		}
		bytes += len(data)
		seen[e.Handle] = true
		retained = append(retained, e)
	}
	// Incoming pages precede the existing rows, including equal-time ties.
	// The stable time sort must not reverse the provider's tie-break order.
	retained = append(retained, previous...)
	sort.SliceStable(retained, func(i, j int) bool {
		a, _ := time.Parse(time.RFC3339Nano, retained[i].Text("time"))
		b, _ := time.Parse(time.RFC3339Nano, retained[j].Text("time"))
		return a.Before(b)
	})
	if msg.page.More && (len(retained) >= 10000 || bytes >= 32<<20) {
		m.budgetStopped = true
	}
	m.page = logs.Page{Events: retained, More: msg.page.More, Next: msg.page.Next, Partial: partial}
	m.selected = 0
	for i, e := range retained {
		if anchor != "" && e.Handle == anchor {
			m.selected = i
			break
		}
		if anchor == "" && e.Text("level") == "ERROR" {
			m.selected = i
			break
		}
	}
	m.keepVisible()
	m.status = "Offline fictional data · Search loaded · / local find · o older history"
	if m.options.LocalLab {
		m.status = "Local Elasticsearch · Search loaded · / local find · o older history"
	}
	if m.budgetStopped {
		close := m.closeCursor(m.page.Next)
		m.page.Next = ""
		m.page.More, m.page.Partial = true, true
		m.status = "History stopped: session budget 10000 events / 32 MiB · r reload"
		return m, close
	}
	return m, nil
}

// Search decoded strings, keys and typed values, not escaped display previews.
// Values contains ONLY the service's policy-safe output, never native records.
func searchable(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case map[string]any:
		var parts []string
		for k, child := range v {
			parts = append(parts, k, searchable(child))
		}
		return strings.Join(parts, "\n")
	case []any:
		var parts []string
		for _, child := range v {
			parts = append(parts, searchable(child))
		}
		return strings.Join(parts, "\n")
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}

func (m *Model) findNext(backwards bool) {
	if m.find == "" {
		m.status = "Local find empty · / enter a literal"
		return
	}
	needle := m.find
	if !m.findCase {
		needle = strings.ToLower(needle)
	}
	step := 1
	if backwards {
		step = -1
	}
	for offset := 1; offset <= len(m.page.Events); offset++ {
		i := (m.selected + step*offset + len(m.page.Events)) % len(m.page.Events)
		data, _ := json.Marshal(m.page.Events[i].Values)
		var decoded any
		d := json.NewDecoder(strings.NewReader(string(data)))
		d.UseNumber()
		_ = d.Decode(&decoded)
		haystack := searchable(decoded)
		if !m.findCase {
			haystack = strings.ToLower(haystack)
		}
		if strings.Contains(haystack, needle) {
			m.selected, m.inspectorOffset, m.fieldIndex = i, 0, 0
			m.keepVisible()
			m.status = fmt.Sprintf("Local literal match in LOADED safe data · case=%t · n/N next/previous", m.findCase)
			return
		}
	}
	m.status = "No local literal matches in LOADED safe data"
}
