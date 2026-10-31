package ui

import (
	"context"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"streams/internal/config"
	"streams/internal/stream"
	"terminalui"
)

func (m Model) actions() []terminalui.Action {
	event, topic, cursor, connection := "", "", "", ""
	if m.view != "events" || len(m.records) == 0 {
		event = "Select an event first"
	}
	if _, ok := m.selectedTopic(); !ok {
		topic = "Select a topic first"
	}
	if m.request.Topic.Name != "" && m.view == "events" {
		topic = ""
	}
	if len(m.cursor.Positions) == 0 || m.view != "events" {
		cursor = "No read position yet"
	}
	if m.options.OpenConnection == nil {
		connection = "Fixture mode has no network connections"
	}
	busy := ""
	if m.loading {
		busy = "Read in progress; Pause to cancel"
	}
	resumeLabel := "Resume"
	if !m.paused {
		resumeLabel = "Continue"
	}
	return []terminalui.Action{
		{ID: "topics", Label: "Topics", Keys: []string{"t"}},
		{ID: "groups", Label: "Groups", Keys: []string{"g"}},
		{ID: "open", Label: "Open / Inspect", Keys: []string{"enter"}},
		{ID: "refresh", Label: "Refresh", Keys: []string{"r"}},
		{ID: "seek", Label: "Seek / choose partitions", Keys: []string{"s"}, Unavailable: topic},
		{ID: "follow", Label: "Follow from now / jump latest", Keys: []string{"F"}, Unavailable: topic},
		{ID: "pause", Label: "Pause", Keys: []string{" "}, Unavailable: func() string {
			if !m.following && !m.loading {
				return "No active read"
			}
			return ""
		}()},
		{ID: "resume", Label: resumeLabel, Keys: []string{"c"}, Unavailable: func() string {
			if busy != "" {
				return busy
			}
			return cursor
		}()},
		{ID: "find", Label: "Find loaded / filter topics", Keys: []string{"/"}},
		{ID: "next", Label: "Next loaded match", Keys: []string{"n"}, Unavailable: func() string {
			if m.find == "" {
				return "Set a loaded-window find first"
			}
			return event
		}()},
		{ID: "copy", Label: "Copy selected field", Keys: []string{"y"}, Unavailable: event},
		{ID: "mode", Label: "Reader JSON / raw / hex", Keys: []string{"v"}, Unavailable: event},
		{ID: "back", Label: "Back", Keys: []string{"esc"}},
		{ID: "connection", Label: "Switch connection", Unavailable: connection},
		{ID: "internal", Label: "Toggle internal topics"},
		{ID: "sidebar", Label: "Toggle sidebar"},
		{ID: "inspector", Label: "Toggle inspector"},
		{ID: "density", Label: "Toggle Compact / Comfortable event rows"},
		{ID: "ascii", Label: "Toggle ASCII borders"},
		{ID: "save", Label: "Save appearance preferences"},
		{ID: "help", Label: "Help", Keys: []string{"f1", "?"}},
		{ID: "palette", Label: "Actions", Keys: []string{"ctrl+p"}},
		{ID: "quit", Label: "Quit streams", Keys: []string{"q"}},
	}
}
func (m Model) execute(id string) (tea.Model, tea.Cmd) {
	if m.switching {
		switch id {
		case "open", "refresh", "topics", "groups", "seek", "follow", "resume", "connection":
			m.status = "Connection switch in progress"
			return m, nil
		}
	}
	for _, a := range m.actions() {
		if a.ID == id && a.Unavailable != "" {
			m.status = a.Unavailable
			return m, nil
		}
	}
	switch id {
	case "quit":
		m.Close()
		return m, tea.Quit
	case "palette":
		m.paletteOpen = true
		m.palette = terminalui.Palette{}
	case "help":
		m.help = true
		m.helpOffset = 0
	case "topics", "groups":
		m.catalogGen++
		m.readGen = m.session.invalidate()
		m.following = false
		m.paused = true
		m.loading = true
		m.detail = false
		m.view = id
		m.focus = "list"
		m.row, m.top = 0, 0
		return m, m.catalog(id)
	case "refresh":
		if m.view == "topics" || m.view == "groups" {
			m.catalogGen++
			m.loading = true
			return m, m.catalog(m.view)
		}
		if m.view == "group" {
			m.groupGen++
			m.loading = true
			return m, m.groupCommand(m.group.Name)
		}
		q := m.request
		q.Mode = "recent"
		q.Cursor = nil
		q.Follow = false
		return m.begin(q, true)
	case "open":
		if m.focus == "sidebar" || m.view == "topics" {
			t, ok := m.selectedTopic()
			if !ok {
				return m, nil
			}
			q := stream.ReadRequest{Topic: t.Ref, Mode: "recent"}
			for _, p := range t.Partitions {
				q.Partitions = append(q.Partitions, p.ID)
			}
			if len(q.Partitions) > 32 {
				m.request = q
				m.editor = &editor{kind: "seek", text: "recent 0", focus: "input", hint: seekHint}
				return m, nil
			}
			return m.begin(q, true)
		}
		if m.view == "events" && len(m.records) > 0 {
			m.detail = true
			m.focus = "inspector"
			m.modalFocus = "field:1"
			m.field, m.readerWindow, m.hscroll = 1, 0, 0
			m.scroll = 0
		}
		if m.view == "groups" && len(m.groups) > 0 {
			name := m.groups[min(m.row, len(m.groups)-1)]
			m.groupGen++
			m.group = stream.GroupObservation{Name: name}
			m.view = "group"
			m.loading = true
			m.row, m.top = 0, 0
			return m, m.groupCommand(name)
		}
	case "seek":
		m.editor = &editor{kind: "seek", text: "recent all", hint: seekHint, focus: "input"}
	case "follow":
		q := m.request
		if m.view != "events" {
			t, ok := m.selectedTopic()
			if !ok {
				return m, nil
			}
			q.Topic = t.Ref
			q.Partitions = nil
			for _, p := range t.Partitions {
				q.Partitions = append(q.Partitions, p.ID)
			}
		}
		q.Mode = "follow"
		q.Cursor = nil
		q.Follow = true
		return m.begin(q, true)
	case "pause":
		m.readGen = m.session.invalidate()
		m.loading = false
		m.following = false
		m.paused = true
		m.status = "PAUSED · fetching stopped · accepted delivery positions retained"
	case "resume":
		q := m.request
		c := stream.CloneCursor(m.cursor)
		q.Mode = "resume"
		q.Cursor = &c
		return m.begin(q, false)
	case "find":
		if m.view == "topics" {
			m.editor = &editor{kind: "topics", text: m.filter, focus: "input", hint: "Case-insensitive topic-name filter; does not read events"}
		} else {
			m.editor = &editor{kind: "find", text: m.find, focus: "input", hint: "Case-sensitive literal in loaded keys/values only; no broker search"}
		}
	case "next":
		m.findNext()
	case "copy":
		return m.copyField()
	case "mode":
		m.readerMode = (m.readerMode + 1) % 3
		m.scroll = 0
	case "back":
		if m.detail {
			m.detail = false
			m.focus = "list"
		} else if m.view == "group" {
			m.view = "groups"
			m.row, m.top = 0, 0
		} else if m.view == "events" {
			return m.execute("topics")
		}
	case "connection":
		m.editor = &editor{kind: "connection", text: m.options.Connection, focus: "input", hint: "Exact saved connection name: " + fmt.Sprint(m.options.Connections)}
	case "internal":
		m.showInternal = !m.showInternal
		m.topicIndex = 0
	case "sidebar":
		m.settings.Sidebar = !m.settings.Sidebar
		m.focus = "list"
	case "inspector":
		m.settings.Inspector = !m.settings.Inspector
		m.focus = "list"
	case "ascii":
		m.settings.ASCII = !m.settings.ASCII
	case "density":
		m.settings.Comfortable = !m.settings.Comfortable
		if m.settings.Comfortable && m.focus == "sidebar" {
			m.focus = "list"
		}
		m.status = m.densityLabel() + " event rows · Save appearance to remember"
	case "save":
		path, s := m.options.SettingsPath, m.settings
		if path == "" {
			m.status = "No settings path"
			return m, nil
		}
		return m, func() tea.Msg {
			if err := config.SaveSettings(path, s); err != nil {
				return statusMsg(err.Error())
			}
			return statusMsg("Appearance saved · no credentials, events or cursors persisted")
		}
	}
	return m, nil
}
func (m Model) groupCommand(name string) tea.Cmd {
	ctx, b, gen := m.options.Context, m.options.Backend, m.connectionGen
	ticket := m.groupGen
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		g, err := b.Group(ctx, name)
		return groupMsg{gen: gen, ticket: ticket, group: g, err: err}
	}
}

func (m *Model) findNext() {
	if m.find == "" || len(m.records) == 0 {
		return
	}
	for step := 1; step <= len(m.records); step++ {
		i := (m.row + step) % len(m.records)
		r := m.records[i]
		if containsLiteral(r.Key, m.find) || containsLiteral(r.Value, m.find) {
			m.row = i
			m.field = 1
			m.scroll = 0
			m.status = "Match in loaded window only"
			return
		}
	}
	m.status = "No literal match in loaded keys/values"
}
