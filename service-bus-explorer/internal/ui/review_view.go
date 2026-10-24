package ui

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func (m Model) splitReview() bool {
	if m.review.selected {
		return m.height >= 35
	}
	// Leave room for the seven-row action bar and both review panes.
	return m.width >= 110 && m.height >= 35
}

func (m Model) reviewView(width, height int) []string {
	r := m.review
	if r.selected {
		return m.selectedReviewView(width, height)
	}
	if width < 80 || height < 18 {
		return m.panel("DLQ / REVIEW", width, height, []string{"", m.paint("  Resize to at least 80×24 to review or confirm.", m.theme.Warning), m.paint("  Esc cancels · Ctrl+C quits safely", m.theme.Muted)}, true)
	}
	if r.phase == "acquiring" {
		return m.panel("RECEIVE NEXT / DLQ", width, height, []string{
			"", m.paint("  Acquiring next available message… (up to 5s)", m.theme.Warning), "",
			m.paint("  Not the selected peek row. No scanning for an ID.", m.theme.Foreground),
			m.paint("  Receiving/releasing can increase delivery count.", m.theme.Warning), "",
			m.paint("  Esc cancel · navigation held until acquisition/release finishes", m.theme.Muted),
		}, true)
	}
	if r.phase == "result" || r.phase == "releasing" {
		rows := []string{"", m.paint("  "+strings.ToUpper(r.phase), m.theme.Focus), ""}
		for _, line := range strings.Split(ansi.Wrap(clean(r.outcome, false), max(1, width-8), ""), "\n") {
			rows = append(rows, m.paint("  "+line, m.theme.Foreground))
		}
		if r.phase == "releasing" {
			rows = append(rows, "", m.paint("  Releasing retained original… (bounded; SDK recovery allowed)", m.theme.Warning))
		} else {
			rows = append(rows, "", m.paint("  Enter / Esc return to safe peek · q quit", m.theme.Focus), m.paint("  This result cannot resend again.", m.theme.Muted))
		}
		return m.panel("DLQ / OUTCOME", width, height, rows, true)
	}
	var actions []string
	if r.phase == "review" {
		mark := "[ ]"
		if r.cleanup {
			mark = "[x]"
		}
		cleanup := "  " + mark + " Remove original ONLY after confirmed send"
		if r.selected {
			cleanup = "  " + mark + " Delete original after confirmed resend (fixed for this review)"
		}
		if r.choice == 2 {
			cleanup = m.selectedLine("› "+mark+" Remove original ONLY after confirmed send", width-2)
		} else {
			cleanup = m.paint(cleanup, m.theme.Foreground)
		}
		actions = append(actions, cleanup)
		cancelLabel := "Cancel / release"
		if r.selected && r.delivery == nil {
			cancelLabel = "Cancel"
		}
		buttons := lipgloss.JoinHorizontal(lipgloss.Top,
			m.button(cancelLabel, r.choice == 0, m.theme.Focus), m.paint(" \n \n ", m.theme.Foreground),
			m.button("Resend copy", r.choice == 1, m.theme.Focus))
		if !r.selected {
			buttons = lipgloss.JoinHorizontal(lipgloss.Top, buttons, m.paint("  \n  \n  ", m.theme.Foreground), m.button("Delete original", r.choice == 3, m.theme.DeadLetter))
		}
		actions = append(actions, strings.Split(buttons, "\n")...)
		if r.choice == 3 {
			actions = append(actions, m.paint("  Delete original · type delete: "+r.typed+"_", m.theme.DeadLetter))
		} else {
			actions = append(actions, m.paint("  Release/expiry may increase delivery count. PgUp/PgDn body.", m.theme.Muted))
		}
		if r.selected {
			back := "  b Back to edit"
			if r.delivery != nil {
				back += " (releases lock)"
			}
			actions[len(actions)-1] = m.paint(back+" · PgUp/PgDn body · Esc cancel", m.theme.Muted)
		}
		actions = m.panel("ACTIONS / Tab choose · Enter confirm · Esc cancel", width, 7, actions, true)
		height -= len(actions)
	}
	leftWidth := width
	split := m.splitReview()
	if split {
		leftWidth = width * 60 / 100
	}
	var rows []string
	add := func(text, color string) {
		for _, line := range strings.Split(ansi.Wrap(clean(text, false), max(1, leftWidth-6), ""), "\n") {
			rows = append(rows, m.paint("  "+line, color))
		}
	}
	if r.selected {
		if r.delivery == nil {
			add("SELECTED PEEK COPY · no lock acquired · original retained", m.theme.Focus)
		} else {
			add("EXACT SELECTED ORIGINAL · locked for optional cleanup", m.theme.Warning)
		}
	} else {
		add("ACTUAL RECEIVED MESSAGE · not the peek selection", m.theme.Warning)
	}
	add("ID: "+r.message.ID, m.theme.Foreground)
	add(fmt.Sprintf("Sequence: %d · deliveries: %d", r.message.Sequence, r.message.DeliveryCount), m.theme.Muted)
	add("Source: "+m.resource.Profile+" / "+m.resource.Namespace+" / "+m.resource.Name()+" / DLQ", m.theme.Muted)
	destination := m.resource.Queue
	if destination == "" {
		destination = m.resource.Topic
	}
	if r.selected {
		destination = r.destination.Queue
	}
	add("Resend destination: "+destination, m.theme.Focus)
	add("New MessageID: "+r.newID, m.theme.Foreground)
	if r.delivery != nil {
		add("Lock until: "+r.deadline.UTC().Format("15:04:05 UTC")+fmt.Sprintf(" · %ds left · no renewal", max(0, int(time.Until(r.deadline).Seconds()))), m.theme.Warning)
	}
	if split {
		rows = append(rows, "")
	}
	add("Resend creates a new copy; duplicate processing is possible.", m.theme.Warning)
	if m.resource.Subscription != "" && !r.selected {
		add("TOPIC FAN-OUT: other matching subscriptions may receive this copy.", m.theme.DeadLetter)
	}
	if split {
		add("Body/properties/correlation preserved; fresh original TTL; immediate enqueue; broker DLQ fields stripped.", m.theme.Muted)
		add("Unsupported sessions/AMQP formats are rejected, never silently converted.", m.theme.Muted)
		add("No app-level retries; SDK recovery may repeat wire attempts. Not exactly-once.", m.theme.Muted)
		rows = append(rows, "")
	}
	if r.phase != "review" {
		add(strings.ToUpper(r.phase)+"… no repeat submission", m.theme.Warning)
		add("Cancellation cannot roll back broker acceptance. Waiting for confirmed/unknown outcome.", m.theme.Muted)
	}
	if !split && len(rows) > height-2 {
		rows = append(rows[:height-3], m.paint("  PgDn: full identity, body and replay policy", m.theme.Warning))
	}
	title, bodyTitle := "DLQ / REVIEW HELD MESSAGE", "HELD BODY"
	if r.selected && r.delivery == nil {
		title, bodyTitle = "DLQ / REVIEW SELECTED COPY", "COPY BODY"
	}
	left := m.panel(title, leftWidth, height, rows, true)
	bodyWidth := width - leftWidth - 6
	if !split {
		bodyWidth = width - 6
	}
	body := []string{m.paint("  "+clean(r.message.ContentType, false), m.theme.Focus), ""}
	if !split {
		for _, text := range []string{"ID: " + r.message.ID, "Source: " + m.resource.Profile + " / " + m.resource.Namespace + " / " + m.resource.Name() + " / DLQ"} {
			for _, line := range strings.Split(ansi.Wrap(clean(text, false), bodyWidth, ""), "\n") {
				body = append(body, m.paint("  "+line, m.theme.Foreground))
			}
		}
		if r.selected {
			for _, line := range strings.Split(ansi.Wrap(clean("Destination queue: "+r.destination.Queue, false), bodyWidth, ""), "\n") {
				body = append(body, m.paint("  "+line, m.theme.Focus))
			}
		}
		body = append(body, "")
	}
	for _, line := range strings.Split(clean(string(r.message.Body), true), "\n") {
		for _, wrapped := range strings.Split(ansi.Wrap(line, bodyWidth, ""), "\n") {
			body = append(body, m.paint("  "+wrapped, m.theme.Foreground))
		}
	}
	if r.message.PreviewTruncated {
		body = append(body, m.paint("  [preview truncated; outgoing bytes not truncated]", m.theme.Warning))
	}
	props, _ := json.MarshalIndent(r.message.Properties, "", "  ")
	body = append(body, "", m.paint("  APPLICATION PROPERTIES", m.theme.Focus))
	for _, line := range strings.Split(clean(string(props), true), "\n") {
		for _, wrapped := range strings.Split(ansi.Wrap(line, bodyWidth, ""), "\n") {
			body = append(body, m.paint("  "+wrapped, m.theme.Foreground))
		}
	}
	if !split {
		body = append(body, "", m.paint("  Resend preserves body/properties/correlation and TTL duration.", m.theme.Muted), m.paint("  Immediate enqueue; broker DLQ fields stripped; unsupported", m.theme.Muted), m.paint("  sessions/AMQP formats rejected. TTL is a new lifetime.", m.theme.Muted))
	}
	if split {
		offset := min(r.scroll, max(0, len(body)-(height-2)))
		right := m.panel(bodyTitle+" / properties · PgUp/PgDn", width-leftWidth, height, body[offset:], false)
		for i := range left {
			left[i] += right[i]
		}
		return append(left, actions...)
	}
	// Narrow terminals retain confirmations and reveal body/properties on demand.
	if r.scroll > 0 {
		offset := min(max(0, r.scroll-1), max(0, len(body)-(height-3)))
		body = append([]string{m.paint("  PgUp to return to review / Esc cancels", m.theme.Warning)}, body[offset:]...)
		// The body page is read-only: don't show confirm buttons behind it.
		return m.panel(bodyTitle+" / read-only", width, height+len(actions), body, true)
	}
	return append(left, actions...)
}
