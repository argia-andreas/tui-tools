package ui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"servicebusexplorer/internal/explorer"
	"terminalui"
)

func destinationID(r explorer.Resource) string {
	return r.Profile + "\x1f" + r.Namespace + "\x1f" + r.Queue
}

func (m Model) destinationIndex() int {
	queues := m.destinationQueues()
	if m.review.queueID != "" {
		for i, r := range queues {
			if destinationID(r) == m.review.queueID {
				return i
			}
		}
		return 0
	}
	return min(max(0, m.review.queueIndex), max(0, len(queues)-1))
}

func (m Model) reviewFocus() string {
	if m.review.phase != "review" && m.review.phase != "configure" && m.review.phase != "destination" {
		return "cancel"
	}
	if m.review.surfaceFocus != "" {
		return m.review.surfaceFocus
	}
	if m.review.phase == "destination" {
		return "filter"
	}
	if m.review.choice == 1 {
		if m.review.phase == "configure" {
			return "prepare"
		}
		return "send"
	}
	if m.review.choice == 2 {
		return "cleanup"
	}
	if m.review.choice == 3 {
		if m.review.phase == "configure" {
			return "destination"
		}
		return "delete"
	}
	return "cancel"
}

func (m *Model) reviewSetFocus(id string) {
	m.review.surfaceFocus = id
	m.review.typed = ""
	switch id {
	case "cancel":
		m.review.choice = 0
	case "prepare", "send":
		m.review.choice = 1
	case "cleanup":
		m.review.choice = 2
	case "delete", "destination":
		m.review.choice = 3
	}
}

func (m Model) reviewControlVisible(id string) bool {
	bounds := m.reviewBounds()
	frame := terminalui.RenderSurface(m.reviewSurfaceSpec(bounds), bounds, m.theme.TerminalStyle(m.ascii))
	for _, r := range frame.Regions {
		if r.ID == id {
			return true
		}
	}
	return false
}

func (m Model) reviewBack() (tea.Model, tea.Cmd) {
	if m.review.phase != "review" || !m.review.selected || !m.reviewControlVisible("back") {
		return m, nil
	}
	m.review.back, m.review.scroll = true, 0
	m.review.surfaceFocus = ""
	m.review.outcome = "Returning to resend configuration; no operation requested."
	return m.release()
}

func (m Model) reviewCleanup() (tea.Model, tea.Cmd) {
	if m.width < 80 || m.height < 24 || !m.reviewControlVisible("cleanup") {
		return m, nil
	}
	if m.review.phase == "configure" || (m.review.phase == "review" && !m.review.selected) {
		m.review.cleanup = !m.review.cleanup
	}
	return m, nil
}

func (m Model) chooseDestination() (tea.Model, tea.Cmd) {
	if m.review.phase != "destination" || m.width < 80 || m.height < 24 || !m.reviewControlVisible("choose") {
		return m, nil
	}
	queues := m.destinationQueues()
	if len(queues) == 0 {
		return m, nil
	}
	index := m.destinationIndex()
	// A removed selection must not silently authorize its replacement.
	if m.review.queueID != "" && destinationID(queues[index]) != m.review.queueID {
		return m, nil
	}
	m.review.destination = queues[index]
	m.review.phase, m.review.choice, m.review.surfaceFocus = "configure", 0, ""
	return m, nil
}

func (m Model) reviewSurfaceInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Reconciliation precedes every key/press, including compact/hidden frames.
	if m.review.phase == "review" && m.review.delivery != nil && !time.Now().Before(m.review.deadline) {
		m.review.outcome = "Lock expired; no operation requested. Receive and review again."
		return m.release()
	}
	if key, ok := msg.(tea.KeyMsg); ok {
		k := key.String()
		if k == "esc" {
			return m.reviewCancel()
		}
		if m.review.phase == "destination" && m.reviewFocus() == "filter" && k != "tab" && k != "shift+tab" {
			if k == "enter" {
				return m.chooseDestination()
			}
			return m.selectedKey(key)
		}
		if m.review.phase == "review" && (k == "pgup" || k == "pgdown" || (m.review.choice == 3 && !m.review.selected && k != "tab" && k != "shift+tab" && k != "up" && k != "down" && k != "enter")) {
			return m.reviewKey(key)
		}
	}
	bounds := m.reviewBounds()
	spec := m.reviewSurfaceSpec(bounds)
	frame := terminalui.RenderSurface(spec, bounds, m.theme.TerminalStyle(m.ascii))
	result := terminalui.HandleSurface(msg, spec, frame)
	if result.Intent == nil {
		return m, nil
	}
	intent := *result.Intent
	switch intent.Kind {
	case terminalui.Dismiss:
		return m.reviewCancel()
	case terminalui.Focus:
		// The displayed field shares the existing Delete entry owner. A press
		// focuses/clears authorization; only explicit confirmation submits it.
		if intent.ID == "delete-authorization" {
			intent.ID = "delete"
		}
		m.reviewSetFocus(intent.ID)
	case terminalui.Select:
		if m.review.phase == "destination" {
			for i, r := range m.destinationQueues() {
				if destinationID(r) == intent.ID {
					m.review.queueIndex, m.review.queueID = i, intent.ID
					break
				}
			}
		}
	case terminalui.Scroll:
		if m.review.phase == "review" {
			m.review.scroll = max(0, m.review.scroll+intent.Delta)
		}
	case terminalui.Activate:
		switch intent.ID {
		case "cancel":
			return m.reviewCancel()
		case "send":
			return m.reviewSubmit(false)
		case "delete":
			if m.review.choice != 3 {
				m.reviewSetFocus("delete")
				return m, nil
			}
			return m.reviewSubmit(true)
		case "cleanup":
			return m.reviewCleanup()
		case "back":
			return m.reviewBack()
		case "prepare":
			return m.prepareSelected()
		case "choose":
			return m.chooseDestination()
		case "destination":
			return m.changeDestination()
		}
	}
	return m, nil
}
