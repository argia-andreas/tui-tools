package ui

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/x/ansi"
	"terminalui"
)

func (m Model) splitReview() bool {
	if m.review.selected {
		return m.height >= 35
	}
	return m.width >= 110 && m.height >= 35
}

func (m Model) reviewView(width, height int) []string {
	spec := m.reviewSurfaceSpec(terminalui.Rect{W: width, H: height})
	return terminalui.RenderSurface(spec, terminalui.Rect{W: width, H: height}, m.theme.TerminalStyle(m.ascii)).Lines
}

// One current spec supplies local rendering and absolute input geometry. Review
// stays docked; only its body owns scrolling and domain state.
func (m Model) reviewSurfaceSpec(bounds terminalui.Rect) terminalui.SurfaceSpec {
	r := m.review
	body := terminalui.SurfaceBodyBounds(bounds)
	spec := terminalui.SurfaceSpec{ID: "review", Title: "DLQ / REVIEW HELD MESSAGE", CancelID: "cancel", FocusID: m.reviewFocus()}
	spec.Body.Bounds = body
	add := func(text string) {
		for _, line := range strings.Split(ansi.Wrap(clean(text, false), max(1, body.W-2), ""), "\n") {
			spec.Body.Lines = append(spec.Body.Lines, m.paint("  "+line, m.theme.Foreground))
		}
	}
	control := func(id, label string) {
		y := body.Y + len(spec.Body.Lines)
		line := m.paint("  "+label, m.theme.Foreground)
		if m.reviewFocus() == id {
			marker := "› "
			if m.ascii {
				marker = "> "
			}
			line = m.selectedLine(marker+label, body.W)
		}
		spec.Body.Lines = append(spec.Body.Lines, line)
		spec.Body.Regions = append(spec.Body.Regions, terminalui.Region{ID: id, Kind: terminalui.ControlRegion, Bounds: terminalui.Rect{X: body.X + 2, Y: y, W: ansi.StringWidth(label), H: 1}})
	}
	action := func(id, label string, keys ...string) {
		spec.Actions = append(spec.Actions, terminalui.Action{ID: id, Label: label, Keys: keys, Dangerous: id == "delete"})
		spec.FocusOrder = append(spec.FocusOrder, id)
	}
	action("cancel", "Cancel / release")
	switch r.phase {
	case "sending", "deleting", "releasing":
		spec.CancelBlocked = "Operation in flight; await outcome"
	}
	if (m.width < 80 || m.height < 24) && r.phase != "result" && r.phase != "releasing" {
		add("Resize to at least 80×24 to review or confirm.")
		add("Esc cancels · Ctrl+C quits safely")
		return spec
	}
	if r.phase == "destination" {
		spec.Title = "SEND COPY TO / CHOOSE QUEUE"
		add("Filter queues: " + r.query + "_")
		spec.Body.Regions = append(spec.Body.Regions, terminalui.Region{ID: "filter", Kind: terminalui.FieldRegion, Bounds: terminalui.Rect{X: body.X, Y: body.Y, W: body.W, H: 1}})
		add(m.resource.Profile + " / " + m.resource.Namespace + " · includes unwatched queues")
		add("")
		queues := m.destinationQueues()
		capacity := max(1, body.H-4)
		index := m.destinationIndex()
		start := index / capacity * capacity
		for i := start; i < min(len(queues), start+capacity); i++ {
			label := "  " + clean(queues[i].Queue, false)
			if i == index {
				label = "› " + clean(queues[i].Queue, false)
			}
			spec.Body.Regions = append(spec.Body.Regions, terminalui.Region{ID: destinationID(queues[i]), Kind: terminalui.RowRegion, Bounds: terminalui.Rect{X: body.X, Y: body.Y + len(spec.Body.Lines), W: body.W, H: 1}})
			spec.Body.Lines = append(spec.Body.Lines, m.paint(label, m.theme.Foreground))
		}
		if len(queues) == 0 {
			add("No matching destination queues")
		}
		action("choose", "Choose", "enter")
		spec.FocusOrder = append([]string{"filter"}, spec.FocusOrder...)
		return spec
	}
	if r.phase == "acquiring" || r.phase == "preparing" {
		spec.Title = "PREPARE SELECTED DLQ MESSAGE"
		if !r.selected {
			spec.Title = "RECEIVE NEXT / DLQ"
			add("Acquiring next available message… (up to 5s)")
			add("Not the selected peek row. No scanning for an ID.")
		} else {
			add("Reading the exact selected DLQ message; nothing has been sent.")
		}
		if r.cleanup {
			add("Finding and locking selected original: at most 100 messages / 5s.")
		}
		add("Receiving/releasing can increase delivery counts.")
		add("Esc cancels · any late acquired delivery will be released")
		return spec
	}
	add("ID: " + r.message.ID + fmt.Sprintf(" · Sequence: %d", r.message.Sequence))
	add("Source: " + m.resource.Profile + " / " + m.resource.Namespace + " / " + m.resource.Name() + " / DLQ")
	if r.phase == "result" || r.phase == "releasing" {
		spec.Title = "DLQ / OUTCOME"
		if r.selected {
			spec.Title = "INSPECTOR / OUTCOME"
		}
		add(strings.ToUpper(r.phase))
		add(r.outcome)
		if r.phase == "releasing" {
			add("Releasing retained original… cancellation is not rollback.")
		} else {
			add("Enter / Esc Back to inspector · this result cannot resend again.")
			spec.Actions[0].Label = "Back"
			spec.Actions[0].Keys = []string{"enter"}
		}
		return spec
	}
	mark := "[ ]"
	if r.cleanup {
		mark = "[x]"
	}
	if r.phase == "configure" {
		spec.Title = "CONFIGURE SELECTED RESEND"
		add("SELECTED DLQ MESSAGE · read-only peek · no locks acquired")
		add("Destination queue: " + r.destination.Queue)
		add("Resend creates a new copy; duplicate processing is possible.")
		if r.cleanup {
			add("Cleanup searches up to 100 messages / 5s before sending.")
			add("Other messages may be locked temporarily.")
			add("Releasing can increase delivery counts; release may take extra time.")
		} else {
			add("Copy-only uses peek. Original stays in the DLQ; no lock or scan.")
		}
		control("cleanup", mark+" Delete original after confirmed resend")
		if len(spec.Body.Lines) > body.H {
			return spec
		}
		action("prepare", "Review resend")
		spec.FocusOrder = append(spec.FocusOrder, "cleanup")
		if m.resource.Queue == "" {
			action("destination", "Change queue")
		}
		return spec
	}
	if r.selected {
		spec.Title = "INSPECTOR / FINAL REVIEW"
		if r.delivery == nil {
			add("SELECTED PEEK COPY · original retained · no lock acquired")
		} else {
			add("EXACT SELECTED ORIGINAL · locked for optional cleanup")
		}
	} else {
		add("ACTUAL RECEIVED MESSAGE · not the peek selection")
	}
	add("Resend destination: " + r.destination.Name())
	add("New MessageID: " + r.newID)
	if r.delivery != nil {
		add("Lock until: " + r.deadline.UTC().Format("15:04:05 UTC") + fmt.Sprintf(" · %ds left · no renewal", max(0, int(time.Until(r.deadline).Seconds()))))
	}
	add("Resend creates a new copy; duplicate processing is possible.")
	if m.resource.Subscription != "" && !r.selected {
		add("TOPIC FAN-OUT: other matching subscriptions may receive this copy.")
	}
	if r.phase != "review" {
		add(strings.ToUpper(r.phase) + "… no repeat submission")
		add("Cancellation cannot roll back broker acceptance. Await outcome.")
		return spec
	}
	if !m.splitReview() && r.scroll > 0 {
		spec.Title = "HELD BODY / read-only"
		if r.selected {
			spec.Title = "COPY BODY / read-only"
		}
		spec.Body.Lines = nil
		add("Confirmations hidden; PgUp return · Esc cancels")
		lines := m.reviewBodyLines(body.W - 2)
		top := min(max(0, r.scroll-1), max(0, len(lines)-body.H+1))
		for _, line := range lines[top:] {
			spec.Body.Lines = append(spec.Body.Lines, m.paint("  "+line, m.theme.Foreground))
		}
		spec.Actions = nil
		spec.FocusOrder = nil
		return spec
	}
	if r.selected {
		add(mark + " Delete original after confirmed resend (fixed)")
	} else {
		control("cleanup", mark+" Remove original ONLY after confirmed send")
	}
	if r.choice == 3 && !r.selected {
		start := len(spec.Body.Lines)
		add("Delete original · type delete: " + r.typed + "_")
		if len(spec.Body.Lines) <= body.H {
			spec.Body.Regions = append(spec.Body.Regions, terminalui.Region{ID: "delete-authorization", Kind: terminalui.FieldRegion,
				Bounds: terminalui.Rect{X: body.X + 2, Y: body.Y + start, W: max(0, body.W-2), H: len(spec.Body.Lines) - start}})
		}
	}
	if r.selected {
		text := "b Back to edit"
		if r.delivery != nil {
			text += " (releases lock)"
		}
		add(text + " · PgUp/PgDn body · Esc cancel")
	}
	// Never expose submission when wrapped identity/safety/authorization has
	// fallen below the visible body, even if the footer itself still fits.
	if len(spec.Body.Lines) > body.H {
		return spec
	}
	action("send", "Resend copy")
	if !r.selected {
		spec.FocusOrder = append(spec.FocusOrder, "cleanup")
		action("delete", "Delete original")
	} else {
		action("back", "Back to edit", "b")
	}
	if m.splitReview() {
		add("FULL PREPARED BODY / PgUp/PgDn · properties below")
		lines := m.reviewBodyLines(body.W - 2)
		capacity := max(1, body.H-len(spec.Body.Lines))
		top := min(r.scroll, max(0, len(lines)-capacity))
		for _, line := range lines[top:] {
			spec.Body.Lines = append(spec.Body.Lines, m.paint("  "+line, m.theme.Foreground))
		}
	}
	return spec
}

// Scroll offsets count displayed wrapped rows, including long single-line JSON.
func (m Model) reviewBodyLines(width int) []string {
	r := m.review
	props, err := json.MarshalIndent(r.message.Properties, "", "  ")
	if err != nil {
		props = []byte("[properties cannot be represented as JSON]")
	}
	text := r.message.ContentType + "\n" + clean(string(r.message.Body), true)
	if r.message.PreviewTruncated {
		text += "\n[preview truncated; outgoing bytes not truncated]"
	}
	text += "\n\nAPPLICATION PROPERTIES\n" + string(props)
	text += "\n\nResend preserves body/properties/correlation and TTL duration. TTL is a new lifetime."
	text += "\nImmediate enqueue; broker DLQ fields stripped; unsupported sessions/AMQP formats rejected."
	text += "\nNo app-level retries; SDK recovery may repeat wire attempts. Not exactly-once."
	var lines []string
	for _, line := range strings.Split(clean(text, true), "\n") {
		lines = append(lines, strings.Split(ansi.Wrap(line, max(1, width), ""), "\n")...)
	}
	return lines
}
