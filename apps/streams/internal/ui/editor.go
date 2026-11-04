package ui

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"streams/internal/stream"
	"terminalui"
)

const seekHint = "recent all | earliest 0,2 | offset 1 6408 | time all 2026-10-30T12:00:00Z"

type editor struct{ kind, text, hint, err, focus string }

func (m Model) editorSpec() terminalui.SurfaceSpec {
	e := m.editor
	b := m.overlayBounds(96, 12)
	bodyBounds := terminalui.SurfaceBodyBounds(b)
	lines := []string{"", e.hint, "", " > " + stream.SafeText([]byte(e.text)), "", e.err, "", " Enter applies · Esc cancels · Ctrl+U clears · F1 Help"}
	body := terminalui.Frame{Bounds: bodyBounds, Lines: lines, Regions: []terminalui.Region{{ID: "input", Kind: terminalui.FieldRegion, Bounds: terminalui.Rect{X: bodyBounds.X, Y: bodyBounds.Y + 3, W: bodyBounds.W, H: 1}}}}
	return terminalui.SurfaceSpec{ID: "editor", Title: strings.ToUpper(e.kind), CancelID: "cancel", Actions: []terminalui.Action{{ID: "cancel", Label: "Cancel"}, {ID: "apply", Label: "Apply"}}, FocusID: e.focus, FocusOrder: []string{"input", "cancel", "apply"}, Body: body}
}
func (m Model) editInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	spec := m.editorSpec()
	f := terminalui.RenderSurface(spec, m.overlayBounds(96, 12), m.style())
	r := terminalui.HandleSurface(msg, spec, f)
	if r.Intent != nil {
		switch r.Intent.Kind {
		case terminalui.Dismiss:
			m.editor = nil
			return m, nil
		case terminalui.Focus:
			m.editor.focus = r.Intent.ID
			return m, nil
		case terminalui.Activate:
			if r.Intent.ID == "cancel" {
				m.editor = nil
				return m, nil
			}
			if r.Intent.ID == "apply" {
				return m.applyEditor()
			}
		}
	}
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch key.String() {
	case "esc":
		m.editor = nil
	case "enter":
		if m.editor.focus == "input" {
			return m.applyEditor()
		}
	case "ctrl+u":
		m.editor.text = ""
	case " ":
		if m.editor.focus == "input" && len(m.editor.text) < 4096 {
			m.editor.text += " "
		}
	case "backspace", "ctrl+h":
		if len(m.editor.text) > 0 {
			_, n := utf8.DecodeLastRuneInString(m.editor.text)
			m.editor.text = m.editor.text[:len(m.editor.text)-n]
		}
	default:
		if m.editor.focus == "input" && key.Type == tea.KeyRunes && len(m.editor.text)+len(string(key.Runes)) <= 4096 {
			m.editor.text += string(key.Runes)
		}
	}
	return m, nil
}
func (m Model) applyEditor() (tea.Model, tea.Cmd) {
	e := m.editor
	switch e.kind {
	case "find":
		m.find = e.text
		m.editor = nil
		m.findNext()
		return m, nil
	case "topics":
		m.filter = e.text
		m.topicIndex = 0
		m.editor = nil
		return m, nil
	case "connection":
		name := e.text
		valid := false
		for _, n := range m.options.Connections {
			valid = valid || n == name
		}
		if !valid {
			m.editor.err = "Choose an exact saved connection name"
			return m, nil
		}
		m.editor = nil
		m.readGen = m.session.invalidate()
		m.following = false
		m.paused = true
		m.connectionGen++
		m.switching = true
		m.status = "Connection switch in progress"
		m.records = nil
		m.cursor = stream.Cursor{}
		m.loading = true
		ctx, cancel := context.WithCancel(m.options.Context)
		gen, open, s := m.connectionGen, m.options.OpenConnection, m.session
		s.beginConnection(gen, cancel)
		return m, func() tea.Msg {
			b, err := open(ctx, name)
			if ctx.Err() != nil {
				err = ctx.Err()
			}
			if err != nil {
				if b != nil {
					b.Close()
				}
				return connectionMsg{gen: gen, name: name, err: err}
			}
			if b == nil || !s.stageConnection(gen, b) {
				err = stream.ErrClosed
			}
			return connectionMsg{gen: gen, name: name, err: err}
		}
	case "seek":
		t := m.request.Topic
		parts := m.request.Partitions
		if m.view != "events" || t.Name == "" {
			topic, ok := m.selectedTopic()
			if !ok {
				m.editor.err = "Select a topic"
				return m, nil
			}
			t = topic.Ref
			parts = nil
			for _, p := range topic.Partitions {
				parts = append(parts, p.ID)
			}
		}
		// all always resolves from topic metadata, not the previous subset.
		for _, topic := range m.topics {
			if topic.Ref.Name == t.Name {
				parts = nil
				for _, p := range topic.Partitions {
					parts = append(parts, p.ID)
				}
				break
			}
		}
		q, err := parseSeek(e.text, t, parts)
		if err != nil {
			m.editor.err = err.Error()
			return m, nil
		}
		m.editor = nil
		return m.begin(q, true)
	}
	return m, nil
}
func parseSeek(text string, topic stream.TopicRef, available []int32) (stream.ReadRequest, error) {
	w := strings.Fields(text)
	q := stream.ReadRequest{Topic: topic}
	bad := fmt.Errorf("Use: %s", seekHint)
	if len(w) < 2 {
		return q, bad
	}
	q.Mode = w[0]
	if w[1] == "all" {
		q.Partitions = append([]int32(nil), available...)
	} else {
		seen := map[int32]bool{}
		for _, p := range strings.Split(w[1], ",") {
			n, err := strconv.ParseInt(p, 10, 32)
			if err != nil || n < 0 {
				return q, bad
			}
			id := int32(n)
			found := false
			for _, a := range available {
				found = found || a == id
			}
			if !found || seen[id] {
				return q, fmt.Errorf("Choose existing, distinct partition IDs")
			}
			seen[id] = true
			q.Partitions = append(q.Partitions, id)
		}
	}
	if len(q.Partitions) == 0 || len(q.Partitions) > 32 {
		return q, stream.ErrLimit
	}
	switch q.Mode {
	case "recent", "earliest":
		if len(w) != 2 {
			return q, bad
		}
	case "offset":
		if len(w) != 3 || len(q.Partitions) != 1 {
			return q, bad
		}
		n, err := strconv.ParseInt(w[2], 10, 64)
		if err != nil || n < 0 {
			return q, fmt.Errorf("Offset must be a nonnegative integer")
		}
		q.Offset = n
	case "time":
		if len(w) != 3 {
			return q, bad
		}
		t, err := time.Parse(time.RFC3339Nano, w[2])
		if err != nil {
			return q, fmt.Errorf("Time must include UTC Z or an explicit offset")
		}
		q.Time = t
	default:
		return q, bad
	}
	return q, nil
}
