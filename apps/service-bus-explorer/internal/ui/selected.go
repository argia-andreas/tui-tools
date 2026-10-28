package ui

import (
	"context"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"servicebusexplorer/internal/explorer"
)

type prepared struct {
	replay   explorer.Replay
	delivery explorer.Delivery
	message  explorer.Message
	err      error
}

func (m Model) openSelected() (tea.Model, tea.Cmd) {
	if m.mode == "azure" {
		m.status = "Azure connection is read-only · use the inspector to view the selected message"
		return m, nil
	}
	if m.review.phase != "" || m.screen != "messages" || !m.dlq || m.loading || len(m.page.Messages) == 0 {
		return m, nil
	}
	if m.reader.mode != "" {
		m.status = "Esc clears selection before resend"
		return m, nil
	}
	m.reviewID++
	m.reader.epoch++
	m.reader.copying = false
	m.reader.load, m.reader.loading = "", ""
	if m.reader.cancel != nil {
		m.reader.cancel()
	}
	m.focus, m.reader.revealed = "inspector", true
	m.review = reviewState{phase: "destination", selected: true, message: m.page.Messages[m.message]}
	if m.resource.Queue != "" {
		m.review.destination = m.resource
		m.review.phase = "configure"
	}
	m.status = "SELECTED DLQ MESSAGE · read-only peek · no lock acquired"
	return m, nil
}

func (m Model) destinationQueues() []explorer.Resource {
	var queues []explorer.Resource
	for _, i := range m.items {
		r := i.resource
		if r.Queue != "" && r.Profile == m.resource.Profile && r.Namespace == m.resource.Namespace && strings.Contains(strings.ToLower(r.Queue), strings.ToLower(m.review.query)) {
			queues = append(queues, r)
		}
	}
	return queues
}

func (m Model) prepareSelected() (tea.Model, tea.Cmd) {
	if m.review.phase != "configure" || !m.review.selected || m.width < 80 || m.height < 24 || !m.reviewControlVisible("prepare") {
		return m, nil
	}
	target := m.review.destination
	if target.Queue == "" || target.Profile != m.resource.Profile || target.Namespace != m.resource.Namespace {
		return m, nil
	}
	m.review.phase, m.review.choice, m.review.scroll = "preparing", 0, 0
	m.review.surfaceFocus = ""
	m.review.cancelled, m.review.back = false, false
	m.reviewID++
	ctx, cancel := context.WithTimeout(m.life, 10*time.Second)
	m.review.cancel = cancel
	store, source, sequence, cleanup := m.store, m.resource, m.review.message.Sequence, m.review.cleanup
	m.status = "Preparing exact selected message · no send until final confirmation"
	return m, func() tea.Msg {
		defer cancel()
		var replay explorer.Replay
		var delivery explorer.Delivery
		var err error
		if cleanup {
			delivery, err = store.ReceiveSelectedDLQ(ctx, source, sequence)
			replay = delivery
		} else {
			replay, err = store.PrepareDLQCopy(ctx, source, sequence)
		}
		if err != nil {
			return prepared{replay: replay, delivery: delivery, err: err}
		}
		message, err := store.PeekMessage(ctx, source, true, sequence)
		return prepared{replay: replay, delivery: delivery, message: message, err: err}
	}
}

func (m Model) cancelSelected() (tea.Model, tea.Cmd) {
	if m.review.phase != "configure" && m.review.phase != "destination" {
		return m, nil
	}
	if m.review.phase == "configure" && m.resource.Queue == "" {
		return m.changeDestination()
	}
	m.review = reviewState{}
	m.focus = "inspector"
	m.status = "PEEK ONLY · no locks acquired · no messages removed"
	return m, nil
}

func (m Model) changeDestination() (tea.Model, tea.Cmd) {
	if m.review.phase != "configure" || !m.review.selected || m.resource.Queue != "" {
		return m, nil
	}
	m.review.phase, m.review.query, m.review.queueIndex, m.review.choice = "destination", "", 0, 0
	m.review.surfaceFocus, m.review.queueID = "", ""
	return m, nil
}

// The destination's local filter owns printable keys, not workspace shortcuts.
// Selecting a row is distinct from the explicit Choose operation.
func (m Model) selectedKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.review.phase != "destination" || m.width < 80 || m.height < 24 {
		return m, nil
	}
	queues := m.destinationQueues()
	index := m.destinationIndex()
	switch msg.String() {
	case "up", "ctrl+p":
		index = max(0, index-1)
	case "down", "ctrl+n":
		index = min(max(0, len(queues)-1), index+1)
	case "enter":
		return m.chooseDestination()
	case "backspace", "ctrl+h":
		runes := []rune(m.review.query)
		m.review.query = string(runes[:max(0, len(runes)-1)])
		m.review.queueIndex, m.review.queueID = 0, ""
		return m, nil
	case "ctrl+u":
		m.review.query, m.review.queueID = "", ""
		m.review.queueIndex = 0
		return m, nil
	default:
		if msg.Type == tea.KeyRunes || msg.Type == tea.KeySpace {
			text := string(msg.Runes)
			if msg.Type == tea.KeySpace && len(msg.Runes) == 0 {
				text = " "
			}
			m.review.query += clean(text, false)
			m.review.query = string([]rune(m.review.query)[:min(128, len([]rune(m.review.query)))])
			m.review.queueIndex, m.review.queueID = 0, ""
		}
		return m, nil
	}
	m.review.queueIndex = index
	if len(queues) > 0 {
		m.review.queueID = destinationID(queues[index])
	}
	return m, nil
}
