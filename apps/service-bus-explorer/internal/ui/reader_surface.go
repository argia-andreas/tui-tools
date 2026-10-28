package ui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"terminalui"
)

func (m Model) readerSurface(bounds terminalui.Rect) terminalui.SurfaceSpec {
	s := terminalui.SurfaceSpec{ID: "reader", Title: "INSPECTOR / " + m.readerMode(), CancelID: "inspector-back", FocusID: m.focus,
		Body:       m.readerBody(bounds),
		FocusOrder: []string{"inspector", "inspector-back"},
		Actions:    []terminalui.Action{{ID: "inspector-back", Label: "Esc Back"}}}
	if m.dlq {
		label := "^R Resend to queue…"
		if m.resource.Queue == "" {
			label = "^R Send copy to…"
		}
		a := terminalui.Action{ID: "inspector-resend", Label: label}
		if m.reader.mode != "" {
			a.Unavailable = "Clear selection first"
		} else if m.reader.copying {
			a.Busy = true
		}
		s.Actions = append(s.Actions, a)
		s.FocusOrder = append(s.FocusOrder, a.ID)
	}
	return s
}

func (m Model) fullLoadSurface(bounds terminalui.Rect) terminalui.SurfaceSpec {
	body := terminalui.SurfaceBodyBounds(bounds)
	s := terminalui.SurfaceSpec{ID: "full-content", Title: "INSPECTOR / FULL CONTENT", CancelID: "load-cancel", FocusID: "load-cancel",
		Actions: []terminalui.Action{{ID: "load-cancel", Label: "Cancel"}}, FocusOrder: []string{"load-cancel"},
		Body: terminalui.Frame{Bounds: body, Lines: []string{"", "  Load full message for copy?", "  Preview is truncated; clipboard unchanged.", "  Read-only exact lookup; no receive or lock.", "  Full byte size is shown after loading.", "", "  Tab choose · Enter select · Esc cancel"}}}
	if m.reader.loading != "" {
		s.Body.Lines = []string{"", "  Loading exact full message…", "  Read-only; no lock or send.", "  Esc cancels; clipboard unchanged."}
	} else {
		s.Actions = append(s.Actions, terminalui.Action{ID: "load-full", Label: "Load full message"})
		s.FocusOrder = append(s.FocusOrder, "load-full")
		if m.reader.choice == 1 {
			s.FocusID = "load-full"
		}
	}
	return s
}

func (m Model) fullLoadInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	bounds := m.readerBounds()
	spec := m.fullLoadSurface(bounds)
	frame := terminalui.RenderSurface(spec, bounds, m.theme.TerminalStyle(m.ascii))
	r := terminalui.HandleSurface(msg, spec, frame)
	if r.Intent != nil {
		switch r.Intent.Kind {
		case terminalui.Focus:
			m.reader.choice = 0
			if r.Intent.ID == "load-full" {
				m.reader.choice = 1
			}
		case terminalui.Dismiss:
			return m.readerAction("load-cancel")
		case terminalui.Activate:
			return m.readerAction(r.Intent.ID)
		}
	}
	return m, nil
}

func (m Model) readerAction(id string) (tea.Model, tea.Cmd) {
	if m.review.phase != "" || m.screen != "messages" {
		return m, nil
	}
	if m.reader.load != "" {
		switch id {
		case "load-cancel":
			if m.reader.cancel != nil {
				m.reader.cancel()
			}
			m.reader.cancel = nil
			m.reader.load, m.reader.loading = "", ""
			m.reader.epoch++
			m.status = "Full-content load cancelled; clipboard unchanged"
		case "load-full":
			if m.reader.loading != "" || m.readerBounds().W == 0 {
				return m, nil
			}
			m.reader.loading = m.reader.load
			ctx, cancel := context.WithTimeout(m.life, 10*time.Second)
			m.reader.cancel = cancel
			epoch, selected, store, resource, dlq := m.reader.epoch, m.inspected(), m.store, m.resource, m.dlq
			return m, func() tea.Msg {
				defer cancel()
				message, err := store.PeekMessage(ctx, resource, dlq, selected.Sequence)
				return fullMessage{epoch, message, err}
			}
		}
		return m, nil
	}
	switch id {
	case "inspector-back":
		m.reader.mode, m.reader.pending = "", ""
		m.focus, m.reader.revealed = "content", false
	case "inspector-resend":
		if m.reader.copying {
			return m, nil
		}
		return m.openSelected()
	}
	return m, nil
}

func (m Model) readerLocalAction(id string) (tea.Model, tea.Cmd) {
	if m.screen != "messages" || m.review.phase != "" || m.reader.load != "" || m.reader.copying || len(m.reader.lines) == 0 {
		return m, nil
	}
	m.focus = "inspector"
	switch id {
	case "reader-line":
		return m.copyText("line")
	case "reader-body":
		return m.copyText("body")
	case "reader-selection":
		return m.copyText("selection")
	case "reader-char":
		m.toggleReaderSelection("char")
	case "reader-visual-line":
		m.toggleReaderSelection("line")
	case "reader-clear":
		m.reader.mode, m.reader.pending = "", ""
	case "reader-help":
		m.help, m.helpOffset = true, 0
	}
	return m, nil
}

func (m *Model) toggleReaderSelection(mode string) {
	if m.reader.mode == mode {
		m.reader.mode = ""
	} else {
		if m.reader.mode == "" {
			m.reader.anchor = m.reader.cursor
		}
		m.reader.mode = mode
	}
}
