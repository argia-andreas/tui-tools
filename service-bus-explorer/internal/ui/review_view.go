package ui

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/x/ansi"
)

func (m Model) reviewView(width, height int) []string {
	r := m.review
	if width < 80 || height < 18 {
		return m.panel("DLQ / REVIEW", width, height, []string{"", m.paint("  Resize to at least 80×24 to review or confirm.", m.theme.Warning), m.paint("  Esc releases · Ctrl+C quits safely · lock still expires", m.theme.Muted)}, true)
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
	leftWidth := width
	split := width >= 110
	if split {
		leftWidth = width * 60 / 100
	}
	var rows []string
	add := func(text, color string) {
		for _, line := range strings.Split(ansi.Wrap(clean(text, false), max(1, leftWidth-6), ""), "\n") {
			rows = append(rows, m.paint("  "+line, color))
		}
	}
	add("ACTUAL RECEIVED MESSAGE · not the peek selection", m.theme.Warning)
	add("ID: "+r.message.ID, m.theme.Foreground)
	add(fmt.Sprintf("Sequence: %d · deliveries: %d", r.message.Sequence, r.message.DeliveryCount), m.theme.Muted)
	add("Source: "+m.resource.Profile+" / "+m.resource.Namespace+" / "+m.resource.Name()+" / DLQ", m.theme.Muted)
	destination := m.resource.Queue
	if destination == "" {
		destination = m.resource.Topic
	}
	add("Resend destination: "+destination, m.theme.Focus)
	add("New MessageID: "+r.newID, m.theme.Foreground)
	add("Lock until: "+r.deadline.UTC().Format("15:04:05 UTC")+fmt.Sprintf(" · %ds left · no renewal", max(0, int(time.Until(r.deadline).Seconds()))), m.theme.Warning)
	if split {
		rows = append(rows, "")
	}
	add("Resend creates a new copy; duplicate processing is possible.", m.theme.Warning)
	if m.resource.Subscription != "" {
		add("TOPIC FAN-OUT: other matching subscriptions may receive this copy.", m.theme.DeadLetter)
	}
	if split {
		add("Body/properties/correlation preserved; fresh original TTL; immediate enqueue; broker DLQ fields stripped.", m.theme.Muted)
		add("Unsupported sessions/AMQP formats are rejected, never silently converted.", m.theme.Muted)
		add("No app-level retries; SDK recovery may repeat wire attempts. Not exactly-once.", m.theme.Muted)
		rows = append(rows, "")
	}
	if r.phase == "review" {
		if !split && len(rows) > height-9 {
			rows = append(rows[:height-10], m.paint("  PgDn: full identity, body and replay policy", m.theme.Warning))
		}
		mark := "[ ]"
		if r.cleanup {
			mark = "[x]"
		}
		for i, label := range []string{"Cancel / release original", "Resend new copy to origin", mark + " Remove original ONLY after confirmed send", "Delete original · type delete: " + r.typed + "_"} {
			if i == r.choice {
				rows = append(rows, m.selectedLine("› "+label, leftWidth-2))
			} else {
				add(label, m.theme.Foreground)
			}
		}
		if split {
			rows = append(rows, "")
			add("Tab / arrows choose · Enter confirm · Esc release", m.theme.Focus)
		}
		add("Release/expiry can increase delivery count. PgUp/PgDn body.", m.theme.Muted)
	} else {
		add(strings.ToUpper(r.phase)+"… no repeat submission", m.theme.Warning)
		add("Cancellation cannot roll back broker acceptance. Waiting for confirmed/unknown outcome.", m.theme.Muted)
	}
	left := m.panel("DLQ / REVIEW HELD MESSAGE", leftWidth, height, rows, true)
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
		right := m.panel("HELD BODY / properties · PgUp/PgDn", width-leftWidth, height, body[offset:], false)
		for i := range left {
			left[i] += right[i]
		}
		return left
	}
	// Narrow terminals retain confirmations and reveal body/properties on demand.
	if r.scroll > 0 {
		offset := min(max(0, r.scroll-1), max(0, len(body)-(height-3)))
		body = append([]string{m.paint("  PgUp to return to review / Esc releases", m.theme.Warning)}, body[offset:]...)
		return m.panel("HELD BODY / read-only", width, height, body, true)
	}
	return left
}
