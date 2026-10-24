package ui

import (
	"context"
	"crypto/rand"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"servicebusexplorer/internal/explorer"
)

// Review owns one delivery until an explicit settlement or bounded release.
// Unlike stale peek replies, even a cancelled acquisition must be accounted for.
type reviewState struct {
	phase, outcome, typed, newID string
	delivery                     explorer.Delivery
	message                      explorer.Message
	deadline                     time.Time
	cancel                       context.CancelFunc
	cancelled, quitting, cleanup bool
	choice, scroll               int
}

type acquired struct {
	delivery explorer.Delivery
	err      error
}
type operated struct {
	outcome string
	settled bool
}
type released struct{ err error }
type lockTick uint64

func (m Model) acquire() (tea.Model, tea.Cmd) {
	if m.review.phase != "" || m.screen != "messages" || !m.dlq {
		return m, nil
	}
	if m.cancel != nil {
		m.cancel()
	}
	m.generation++
	m.loading = false
	m.reviewID++
	ctx, cancel := context.WithTimeout(m.life, 5*time.Second)
	m.review = reviewState{phase: "acquiring", cancel: cancel}
	m.status = "Receiving NEXT available DLQ message · not the selected peek row · release can increase delivery count"
	return m, func() tea.Msg {
		defer cancel()
		d, err := m.store.ReceiveNextDLQ(ctx, m.resource)
		return acquired{d, err}
	}
}

func (m Model) lockTimer() tea.Cmd {
	id := m.reviewID
	return tea.Tick(time.Second, func(time.Time) tea.Msg { return lockTick(id) })
}

func (m Model) release() (tea.Model, tea.Cmd) {
	if m.review.delivery == nil {
		if m.review.quitting {
			m.stop()
			return m, tea.Quit
		}
		m.review.phase = "result"
		m.status = m.review.outcome
		return m, nil
	}
	m.review.phase = "releasing"
	d := m.review.delivery
	return m, func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		return released{d.Abandon(ctx)}
	}
}

func (m Model) quit() (tea.Model, tea.Cmd) {
	if m.review.phase == "" {
		m.stop()
		return m, tea.Quit
	}
	m.review.quitting = true
	switch m.review.phase {
	case "acquiring":
		m.review.cancelled = true
		m.review.cancel()
		m.status = "Quitting · awaiting acquisition and releasing any late delivery…"
		return m, nil
	case "sending", "deleting", "releasing":
		m.status = "Quitting after final outcome/release · cancellation is not rollback"
		return m, nil
	default:
		if m.review.phase == "review" {
			m.review.outcome = "Quit requested; no resend or delete requested."
		}
		return m.release()
	}
}

// ExitSummary survives leaving the alternate terminal screen, including quit
// while a mutation was pending. It contains no message bodies or credentials.
func (m Model) ExitSummary() string { return clean(m.review.outcome, false) }

func (m Model) reviewUpdate(msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	finish := func(next tea.Model, cmd tea.Cmd) (tea.Model, tea.Cmd, bool) { return next, cmd, true }
	switch msg := msg.(type) {
	case acquired:
		m.review.delivery = msg.delivery
		if msg.delivery != nil {
			m.review.message, m.review.deadline = msg.delivery.Snapshot(), msg.delivery.LockedUntil()
			if m.review.cancelled || msg.err != nil {
				m.review.outcome = "Acquisition cancelled; no resend or delete requested."
				return finish(m.release())
			}
			m.review.phase = "review"
			m.review.newID = "sbx-" + rand.Text()
			m.status = "LOCK HELD · review actual received message · Cancel selected by default"
			return m, m.lockTimer(), true
		}
		m.review.phase = "result"
		m.review.outcome = "No DLQ message acquired."
		if msg.err != nil {
			m.review.outcome += " " + msg.err.Error()
		}
		if m.review.quitting {
			m.stop()
			return m, tea.Quit, true
		}
		return m, nil, true
	case operated:
		m.review.outcome = msg.outcome
		if msg.settled {
			m.review.delivery = nil
		}
		// A terminal send result cannot send again. Release any retained original
		// immediately, including ambiguous sends, instead of holding a useless lock.
		return finish(m.release())
	case released:
		m.review.delivery = nil
		m.review.phase = "result"
		if msg.err == nil {
			m.review.outcome += " Lock release confirmed (delivery count may increase)."
		} else {
			m.review.outcome += " Lock release unknown; original may remain locked until expiry."
		}
		m.status = m.review.outcome
		if m.review.quitting {
			m.stop()
			return m, tea.Quit, true
		}
		return m, nil, true
	case lockTick:
		if uint64(msg) != m.reviewID || m.review.phase != "review" {
			return m, nil, true
		}
		if !time.Now().Before(m.review.deadline) {
			m.review.outcome = "Lock expired; resend/delete disabled. Receive and review again explicitly."
			return finish(m.release())
		}
		return m, m.lockTimer(), true
	case tea.KeyMsg:
		if m.review.phase == "" {
			return m, nil, false
		}
		if msg.String() == "ctrl+c" || (msg.String() == "q" && (m.review.phase != "review" || m.review.choice != 3)) {
			return finish(m.quit())
		}
		switch m.review.phase {
		case "acquiring":
			if msg.String() == "esc" {
				m.review.cancelled = true
				m.review.cancel()
				m.status = "Cancelling acquisition · any late delivery will be released"
			}
		case "review":
			return finish(m.reviewKey(msg))
		case "result":
			if msg.String() == "enter" || msg.String() == "esc" {
				m.status = m.review.outcome
				m.review = reviewState{}
				return finish(m.peek(m.from))
			}
		}
		return m, nil, true
	}
	return m, nil, false
}

func (m Model) reviewKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if !time.Now().Before(m.review.deadline) {
		m.review.outcome = "Lock expired; no operation requested. Receive and review again."
		return m.release()
	}
	if (m.width < 80 || m.height < 24) && msg.String() != "esc" {
		return m, nil
	}
	if m.width < 110 && m.review.scroll > 0 && msg.String() != "pgup" && msg.String() != "pgdown" && msg.String() != "esc" {
		return m, nil // Never submit a confirmation hidden behind the narrow body view.
	}
	switch msg.String() {
	case "esc":
		m.review.outcome = "Cancelled; no resend or delete requested."
		return m.release()
	case "tab", "down":
		m.review.choice = (m.review.choice + 1) % 4
		m.review.typed = ""
	case "shift+tab", "up":
		m.review.choice = (m.review.choice + 3) % 4
		m.review.typed = ""
	case "pgdown":
		if m.width < 110 && m.review.scroll == 0 {
			m.review.scroll = 1
		} else {
			m.review.scroll += max(1, m.height-10)
		}
	case "pgup":
		m.review.scroll = max(0, m.review.scroll-max(1, m.height-10))
	case "backspace", "ctrl+h":
		runes := []rune(m.review.typed)
		m.review.typed = string(runes[:max(0, len(runes)-1)])
	case "enter":
		switch m.review.choice {
		case 0:
			m.review.outcome = "Cancelled; no resend or delete requested."
			return m.release()
		case 1:
			return m.operate(false)
		case 2:
			m.review.cleanup = !m.review.cleanup
		case 3:
			if m.review.typed == "delete" {
				return m.operate(true)
			}
		}
	default:
		if m.review.choice == 3 && (msg.Type == tea.KeyRunes || msg.Type == tea.KeySpace) {
			m.review.typed += clean(string(msg.Runes), false)
			if msg.Type == tea.KeySpace && len(msg.Runes) == 0 {
				m.review.typed += " "
			}
			m.review.typed = string([]rune(m.review.typed)[:min(16, len([]rune(m.review.typed)))])
		} else if m.review.choice == 2 && msg.String() == " " {
			m.review.cleanup = !m.review.cleanup
		}
	}
	return m, nil
}

func (m Model) operate(deleting bool) (tea.Model, tea.Cmd) {
	d, id, cleanup, deadline := m.review.delivery, m.review.newID, m.review.cleanup, m.review.deadline
	m.review.phase = "sending"
	if deleting {
		m.review.phase = "deleting"
	}
	m.status = "Operation in flight · no app-level retry (SDK recovery may retry) · cancellation is not rollback"
	return m, func() tea.Msg {
		ctx, cancel := context.WithDeadline(context.Background(), minTime(deadline, time.Now().Add(10*time.Second)))
		defer cancel()
		if deleting {
			if err := d.Complete(ctx); err != nil {
				return operated{"Deletion unconfirmed/unknown; check DLQ before trying again.", true}
			}
			return operated{"Deletion confirmed for the reviewed DLQ message.", true}
		}
		result, err := d.Resend(ctx, id)
		switch result.State {
		case "confirmed":
			outcome := "Send confirmed · new ID " + id + "."
			if cleanup {
				if err := d.Complete(ctx); err != nil {
					return operated{outcome + " Original cleanup unconfirmed/unknown; possible duplicate on later replay.", true}
				}
				return operated{outcome + " Separately authorized original cleanup confirmed.", true}
			}
			return operated{outcome + " Original retained; no cleanup requested.", false}
		case "rejected":
			return operated{fmt.Sprintf("Resend rejected; original retained. %v", err), false}
		default:
			return operated{"Send outcome UNKNOWN — check destination before retrying. Original retained; no cleanup attempted.", false}
		}
	}
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}
