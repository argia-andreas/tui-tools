package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"servicebusexplorer/internal/explorer"
)

type prepared struct {
	replay   explorer.Replay
	delivery explorer.Delivery
	message  explorer.Message
	err      error
}

func (m Model) openSelected() (tea.Model, tea.Cmd) {
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
		if r.Queue != "" && r.Profile == m.resource.Profile && r.Namespace == m.resource.Namespace &&
			strings.Contains(strings.ToLower(r.Queue), strings.ToLower(m.review.query)) {
			queues = append(queues, r)
		}
	}
	return queues
}

func (m Model) prepareSelected() (tea.Model, tea.Cmd) {
	m.review.phase, m.review.choice, m.review.scroll = "preparing", 0, 0
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

func (m Model) selectedKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if key == "esc" {
		if m.review.phase == "configure" && m.resource.Queue == "" {
			m.review.phase, m.review.query, m.review.queueIndex, m.review.choice = "destination", "", 0, 0
		} else {
			m.review = reviewState{}
			m.focus = "inspector"
			m.status = "PEEK ONLY · no locks acquired · no messages removed"
		}
		return m, nil
	}
	if m.width < 80 || m.height < 24 {
		return m, nil
	}
	if m.review.phase == "destination" {
		switch key {
		case "up", "ctrl+p":
			m.review.queueIndex = max(0, m.review.queueIndex-1)
		case "down", "ctrl+n":
			m.review.queueIndex = min(max(0, len(m.destinationQueues())-1), m.review.queueIndex+1)
		case "backspace", "ctrl+h":
			runes := []rune(m.review.query)
			m.review.query = string(runes[:max(0, len(runes)-1)])
			m.review.queueIndex = 0
		case "ctrl+u":
			m.review.query, m.review.queueIndex = "", 0
		case "enter":
			queues := m.destinationQueues()
			if len(queues) > 0 {
				m.review.destination = queues[m.review.queueIndex]
				m.review.phase, m.review.choice = "configure", 0
			}
		default:
			if msg.Type == tea.KeyRunes || msg.Type == tea.KeySpace {
				text := string(msg.Runes)
				if msg.Type == tea.KeySpace && len(msg.Runes) == 0 {
					text = " "
				}
				m.review.query += clean(text, false)
				m.review.query = string([]rune(m.review.query)[:min(128, len([]rune(m.review.query)))])
				m.review.queueIndex = 0
			}
		}
		return m, nil
	}
	choices := 2
	if m.review.phase == "configure" {
		choices = 4
		if m.resource.Queue != "" {
			choices = 3
		}
	}
	switch key {
	case "tab", "down", "right":
		m.review.choice = (m.review.choice + 1) % choices
	case "shift+tab", "up", "left":
		m.review.choice = (m.review.choice + choices - 1) % choices
	case "pgdown":
		m.review.scroll += max(1, m.height-15)
	case "pgup":
		m.review.scroll = max(0, m.review.scroll-max(1, m.height-15))
	case " ":
		if m.review.phase == "configure" && m.review.choice == 2 {
			m.review.cleanup = !m.review.cleanup
		}
	case "enter":
		switch m.review.choice {
		case 0:
			m.review = reviewState{}
			m.focus = "inspector"
			m.status = "Cancelled; no send, deletion or acquisition requested."
		case 1:
			return m.prepareSelected()
		case 2:
			m.review.cleanup = !m.review.cleanup
		case 3:
			m.review.phase, m.review.query, m.review.queueIndex = "destination", "", 0
		}
	}
	return m, nil
}

func (m Model) selectedView(width, height int) []string {
	r := m.review
	if r.phase == "preparing" {
		rows := []string{"", "  Reading the exact selected DLQ message; nothing has been sent."}
		if r.cleanup {
			rows = append(rows, "", "  Finding and locking selected original: at most 100 messages / 5s.",
				"  Other messages may be temporarily locked; release can increase",
				"  delivery counts. Uncertain release stops this resend.", "",
				"  Bounded release/close may take additional time.")
		} else {
			rows = append(rows, "", "  Read-only peek; no lock acquisition or scanning.")
		}
		rows = append(rows, "", "  Esc cancels · any late acquired delivery will be released")
		for i := range rows {
			rows[i] = m.paint(rows[i], m.theme.Warning)
		}
		return m.panel("PREPARE SELECTED DLQ MESSAGE", width, height, rows, true)
	}
	if r.phase == "destination" {
		rows := []string{m.paint("  Filter queues: "+r.query+"_", m.theme.Focus),
			m.paint("  "+m.resource.Profile+" / "+m.resource.Namespace+" · includes unwatched queues", m.theme.Muted), ""}
		queues := m.destinationQueues()
		capacity := max(1, height-8)
		start := r.queueIndex / capacity * capacity
		for i := start; i < min(len(queues), start+capacity); i++ {
			line := "  " + clean(queues[i].Queue, false)
			if i == r.queueIndex {
				line = m.selectedLine("› "+clean(queues[i].Queue, false), width-2)
			} else {
				line = m.paint(line, m.theme.Foreground)
			}
			rows = append(rows, line)
		}
		if len(queues) == 0 {
			rows = append(rows, m.paint("  No matching destination queues", m.theme.Warning))
		}
		rows = append(rows, "", m.paint("  Type filter · arrows choose · Enter selects · Esc back · Ctrl+C quits", m.theme.Muted))
		return m.panel("SEND COPY TO / CHOOSE QUEUE", width, height, rows, true)
	}
	rows := []string{}
	add := func(text, color string) {
		for _, line := range strings.Split(ansi.Wrap(clean(text, false), max(1, width-6), ""), "\n") {
			rows = append(rows, m.paint("  "+line, color))
		}
	}
	add("SELECTED DLQ MESSAGE · read-only peek · no locks acquired", m.theme.Focus)
	add("ID: "+r.message.ID, m.theme.Foreground)
	add(fmt.Sprintf("Sequence: %d · source: %s / %s / %s / DLQ", r.message.Sequence, m.resource.Profile, m.resource.Namespace, m.resource.Name()), m.theme.Muted)
	barHeight := 8
	title, label := "CONFIGURE SELECTED RESEND", "Review resend"
	add("Destination queue: "+r.destination.Queue, m.theme.Focus)
	add("Resend creates a new copy; duplicate processing is possible.", m.theme.Warning)
	if r.cleanup {
		add("Cleanup searches up to 100 messages / 5s before sending.", m.theme.Warning)
		add("Other messages may be locked temporarily.", m.theme.Warning)
		add("Releasing can increase delivery counts; release may take extra time.", m.theme.Warning)
	} else {
		add("Copy-only uses peek. Original stays in the DLQ; no lock or scan.", m.theme.Muted)
	}
	buttons := lipgloss.JoinHorizontal(lipgloss.Top, m.button("Cancel", r.choice == 0, m.theme.Focus),
		m.paint(" \n \n ", m.theme.Foreground), m.button(label, r.choice == 1, m.theme.Focus))
	if r.phase == "configure" && m.resource.Queue == "" {
		buttons = lipgloss.JoinHorizontal(lipgloss.Top, buttons, m.paint(" \n \n ", m.theme.Foreground), m.button("Change queue", r.choice == 3, m.theme.Focus))
	}
	actions := strings.Split(buttons, "\n")
	if r.phase == "configure" {
		mark := "[ ]"
		if r.cleanup {
			mark = "[x]"
		}
		line := "  " + mark + " Delete original after confirmed resend"
		if r.choice == 2 {
			line = m.selectedLine("› "+mark+" Delete original after confirmed resend", width-2)
		} else {
			line = m.paint(line, m.theme.Foreground)
		}
		actions = append([]string{line, ""}, actions...)
	}
	actions = append(actions, m.paint("  Tab/arrows choose · Enter select · Esc back · PgUp/PgDn scroll", m.theme.Muted))
	return append(m.panel(title, width, height-barHeight, rows, true), m.panel("ACTIONS / no send yet", width, barHeight, actions, true)...)
}

// Selected-message operations replace the inspector's contents, not its owner.
// Keep the existing lifecycle, but one body and one explicit confirmation surface.
func (m Model) selectedReviewView(width, height int) []string {
	r := m.review
	if width < 80 || height < 18 {
		return m.panel("INSPECTOR / RESEND", width, height, []string{"", "  Resize to at least 80×24 to review or confirm.", "  Esc cancels · Ctrl+C quits safely"}, true)
	}
	if r.phase == "configure" || r.phase == "destination" || r.phase == "preparing" {
		return m.selectedView(width, height)
	}
	var rows []string
	add := func(text, color string) {
		for _, line := range strings.Split(ansi.Wrap(clean(text, false), width-6, ""), "\n") {
			rows = append(rows, m.paint("  "+line, color))
		}
	}
	add("ID: "+r.message.ID+fmt.Sprintf(" · Sequence: %d", r.message.Sequence), m.theme.Foreground)
	add("Source: "+m.resource.Profile+" / "+m.resource.Namespace+" / "+m.resource.Name()+" / DLQ", m.theme.Muted)
	add("Resend destination: "+r.destination.Queue, m.theme.Focus)
	if r.phase == "result" || r.phase == "releasing" {
		add(strings.ToUpper(r.phase), m.theme.Focus)
		add(r.outcome, m.theme.Warning)
		if r.phase == "releasing" {
			add("Releasing retained original… cancellation is not rollback.", m.theme.Warning)
		} else {
			add("Enter / Esc Back to inspector · this result cannot resend again.", m.theme.Focus)
			add("Reading keeps this snapshot; Esc then r refreshes the list.", m.theme.Muted)
		}
		return m.panel("INSPECTOR / OUTCOME", width, height, rows, true)
	}
	if r.delivery == nil {
		add("SELECTED PEEK COPY · original retained · no lock acquired", m.theme.Focus)
	} else {
		add("EXACT SELECTED ORIGINAL · locked for optional cleanup", m.theme.Warning)
		add("Lock until: "+r.deadline.UTC().Format("15:04:05 UTC")+fmt.Sprintf(" · %ds left · no renewal", max(0, int(time.Until(r.deadline).Seconds()))), m.theme.Warning)
	}
	add("New MessageID: "+r.newID, m.theme.Foreground)
	add("New copy; duplicate processing is possible. Not an atomic move.", m.theme.Warning)
	var body []string
	props, err := json.MarshalIndent(r.message.Properties, "", "  ")
	if err != nil {
		props = []byte("[properties cannot be represented as JSON]")
	}
	for _, line := range strings.Split(clean(string(r.message.Body), true)+"\n\nAPPLICATION PROPERTIES\n"+string(props), "\n") {
		for _, part := range strings.Split(ansi.Wrap(line, width-6, ""), "\n") {
			body = append(body, m.paint("  "+part, m.theme.Foreground))
		}
	}
	if r.message.PreviewTruncated {
		body = append(body, m.paint("  [preview truncated; outgoing bytes are full]", m.theme.Warning))
	}
	if !m.splitReview() && r.scroll > 0 {
		top := min(max(0, r.scroll-1), max(0, len(body)-(height-4)))
		return m.panel("COPY BODY / read-only · PgUp return · Esc cancel", width, height, append([]string{m.paint("  Confirmations hidden; Enter cannot submit.", m.theme.Warning)}, body[top:]...), true)
	}
	if r.phase != "review" {
		add(strings.ToUpper(r.phase)+"… no repeat submission", m.theme.Warning)
		add("Cancellation cannot roll back broker acceptance. Await outcome.", m.theme.Warning)
		return m.panel("INSPECTOR / "+strings.ToUpper(r.phase), width, height, rows, true)
	}
	barHeight := 7
	capacity := height - barHeight - 2 - len(rows) - 1
	if capacity > 0 {
		rows = append(rows, m.paint("  FULL PREPARED BODY / PgUp/PgDn · properties below", m.theme.Muted))
		top := min(r.scroll, max(0, len(body)-capacity))
		rows = append(rows, body[top:min(len(body), top+capacity)]...)
	}
	mark := "[ ]"
	if r.cleanup {
		mark = "[x]"
	}
	actions := []string{m.paint("  "+mark+" Delete original after confirmed resend (fixed)", m.theme.Muted)}
	buttons := lipgloss.JoinHorizontal(lipgloss.Top, m.button("Cancel", r.choice == 0, m.theme.Focus), m.paint(" \n \n ", m.theme.Foreground), m.button("Resend copy", r.choice == 1, m.theme.Focus))
	actions = append(actions, strings.Split(buttons, "\n")...)
	back := "  b Back to edit"
	if r.delivery != nil {
		back += " (releases lock)"
	}
	actions = append(actions, m.paint(back+" · PgUp/PgDn body · Esc cancel", m.theme.Muted))
	return append(m.panel("INSPECTOR / FINAL REVIEW", width, height-barHeight, rows, true), m.panel("ACTIONS / Cancel default · Tab choose · Enter confirm", width, barHeight, actions, true)...)
}
