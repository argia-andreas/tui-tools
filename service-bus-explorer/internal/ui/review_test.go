package ui_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"servicebusexplorer/internal/config"
	"servicebusexplorer/internal/explorer"
	"servicebusexplorer/internal/fixture"
)

func receive(t *testing.T, m tea.Model) (tea.Model, tea.Cmd) {
	t.Helper()
	m = press(m, "ctrl+p")
	m = press(m, "receive next")
	m, cmd := key(m, "enter")
	if cmd == nil {
		t.Fatal("receive action missing")
	}
	return m, cmd
}

func acquiredReview(t *testing.T, m tea.Model) tea.Model {
	t.Helper()
	m, cmd := receive(t, m)
	m, _ = m.Update(cmd()) // Don't execute the production one-second lock timer.
	if !strings.Contains(m.View(), "ACTUAL RECEIVED MESSAGE") {
		t.Fatal(ansi.Strip(m.View()))
	}
	return m
}

func finishOperation(m tea.Model, cmd tea.Cmd) tea.Model {
	m, release := m.Update(cmd())
	if release != nil {
		m, _ = m.Update(release())
	}
	return m
}

func TestReviewActsOnAcquiredNotSelectedAndRetainsOriginal(t *testing.T) {
	s, _ := fixture.New()
	m := start(t, s, config.Default(), "fixture", false)
	m = press(m, "]")
	m = press(m, "j") // selected preview 0002, next acquired 0001
	m = acquiredReview(t, m)
	view := ansi.Strip(m.View())
	for _, want := range []string{"ID: orders-dlq-0001", "› Cancel / release original", "[ ] Remove original", "New MessageID:", "no renewal"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q\n%s", want, view)
		}
	}
	if strings.Contains(view, "orders-dlq-0002") {
		t.Fatal("review used peek-selected message")
	}
	assertFrame(t, m, 160, 48)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	assertFrame(t, m, 80, 24)
	if !strings.Contains(m.View(), "Delete original") {
		t.Fatal("compact review hid confirmation controls")
	}
	m = press(m, "pgdown")
	if !strings.Contains(m.View(), "HELD BODY / read-only") {
		t.Fatal("compact body inaccessible")
	}
	_, hidden := key(m, "enter")
	if hidden != nil {
		t.Fatal("hidden confirmation submitted from body view")
	}
	m = press(m, "pgup")
	m = press(m, "tab")
	m, send := key(m, "enter")
	_, repeat := key(m, "enter")
	if repeat != nil {
		t.Fatal("repeat Enter submitted another operation")
	}
	m = finishOperation(m, send)
	if !strings.Contains(m.View(), "no cleanup requested") {
		t.Fatal(ansi.Strip(m.View()))
	}
	ctx := context.Background()
	resources, _ := s.Discover(ctx)
	page, _ := s.Peek(ctx, resources[0], true, 0, 20)
	if len(page.Messages) != 12 || page.Messages[0].ID != "orders-dlq-0001" {
		t.Fatal("resend removed original")
	}
	c, _ := s.Counts(ctx, resources[0])
	if c.Active != 129 {
		t.Fatalf("expected exactly one new copy, got %d", c.Active)
	}
}

type reviewStore struct {
	explorer.Store
	mode     string
	late     chan struct{}
	deadline time.Time
}

func (s reviewStore) ReceiveNextDLQ(ctx context.Context, r explorer.Resource) (explorer.Delivery, error) {
	if s.late != nil {
		<-s.late
		ctx = context.Background()
	} // simulate a receive racing cancellation
	d, e := s.Store.ReceiveNextDLQ(ctx, r)
	if e != nil {
		return nil, e
	}
	return &reviewDelivery{Delivery: d, mode: s.mode, deadline: s.deadline}, nil
}

type reviewDelivery struct {
	explorer.Delivery
	mode     string
	deadline time.Time
}

func (d *reviewDelivery) LockedUntil() time.Time {
	if !d.deadline.IsZero() {
		return d.deadline
	}
	return d.Delivery.LockedUntil()
}
func (d *reviewDelivery) Resend(ctx context.Context, id string) (explorer.SendResult, error) {
	if d.mode == "reject" {
		return explorer.SendResult{State: "rejected"}, errors.New("unsupported encoding")
	}
	r, e := d.Delivery.Resend(ctx, id)
	if d.mode == "ambiguous" && e == nil {
		return explorer.SendResult{State: "unknown"}, errors.New("ack lost after acceptance")
	}
	return r, e
}
func (d *reviewDelivery) Complete(ctx context.Context) error {
	if d.mode == "cleanup-failed" {
		_ = d.Delivery.Abandon(ctx)
		return errors.New("settlement acknowledgement unknown")
	}
	return d.Delivery.Complete(ctx)
}

func TestReplayOutcomesAndSeparatelyAuthorizedCleanup(t *testing.T) {
	for _, tc := range []struct {
		mode, want  string
		cleanup     bool
		active, dlq int64
	}{
		{"ambiguous", "Send outcome UNKNOWN", true, 129, 12},
		{"cleanup-failed", "Original cleanup unconfirmed/unknown", true, 129, 12},
		{"", "Separately authorized original cleanup confirmed", true, 129, 11},
		{"reject", "Resend rejected; original retained", false, 128, 12},
	} {
		t.Run(tc.mode+tc.want, func(t *testing.T) {
			s, _ := fixture.New()
			m := start(t, reviewStore{Store: s, mode: tc.mode}, config.Default(), "fixture", false)
			m = press(m, "]")
			m = acquiredReview(t, m)
			if tc.cleanup {
				m = press(m, "tab")
				m = press(m, "tab")
				m = press(m, "enter")
				m = press(m, "up")
			} else {
				m = press(m, "tab")
			}
			m, cmd := key(m, "enter")
			m = finishOperation(m, cmd)
			if !strings.Contains(m.View(), tc.want) {
				t.Fatal(ansi.Strip(m.View()))
			}
			r, _ := s.Discover(context.Background())
			c, _ := s.Counts(context.Background(), r[0])
			if c.Active != tc.active || c.DeadLetter != tc.dlq {
				t.Fatalf("wrong broker effects: %+v", c)
			}
		})
	}
}

func TestDeleteRequiresExactTypedAuthorization(t *testing.T) {
	s, _ := fixture.New()
	m := start(t, s, config.Default(), "fixture", false)
	m = press(m, "]")
	m = acquiredReview(t, m)
	m = press(m, "tab")
	m = press(m, "tab")
	m = press(m, "tab")
	for _, text := range []string{"", "Delete", "delete ", "q"} {
		m = press(m, text)
		_, cmd := key(m, "enter")
		if cmd != nil {
			t.Fatal("non-exact delete authorized")
		}
		// Leaving/re-entering the field clears its text.
		m = press(m, "tab")
		m = press(m, "up")
	}
	m = press(m, "delete")
	m, cmd := key(m, "enter")
	m = finishOperation(m, cmd)
	if !strings.Contains(m.View(), "Deletion confirmed") {
		t.Fatal(ansi.Strip(m.View()))
	}
	r, _ := s.Discover(context.Background())
	c, _ := s.Counts(context.Background(), r[0])
	if c.DeadLetter != 11 || c.Active != 128 {
		t.Fatalf("delete had wrong effects: %+v", c)
	}
	_, quit := key(m, "q")
	if quit == nil {
		t.Fatal("delete result retained text-field ownership of q")
	}
}

func TestCancelledLateAcquisitionIsReleasedAndQuitWaits(t *testing.T) {
	for _, quit := range []bool{false, true} {
		t.Run(map[bool]string{false: "escape", true: "quit"}[quit], func(t *testing.T) {
			s, _ := fixture.New()
			gate := make(chan struct{})
			m := start(t, reviewStore{Store: s, late: gate}, config.Default(), "fixture", false)
			m = press(m, "]")
			m, cmd := receive(t, m)
			k := "esc"
			if quit {
				k = "q"
			}
			m = press(m, k)
			close(gate)
			m, release := m.Update(cmd())
			if release == nil {
				t.Fatal("late delivery discarded instead of released")
			}
			m, exit := m.Update(release())
			if quit && exit == nil {
				t.Fatal("quit did not finish after release")
			}
			r, _ := s.Discover(context.Background())
			d, e := s.ReceiveNextDLQ(context.Background(), r[0])
			if e != nil {
				t.Fatal("late delivery still held", e)
			}
			if d.Snapshot().DeliveryCount != 11 {
				t.Fatal("late delivery not released")
			}
			_ = d.Abandon(context.Background())
		})
	}
}

func TestExpiredReviewCannotSendAndPendingQuitPreservesOutcome(t *testing.T) {
	s, _ := fixture.New()
	m := start(t, reviewStore{Store: s, deadline: time.Now().Add(-time.Second)}, config.Default(), "fixture", false)
	m = press(m, "]")
	m = acquiredReview(t, m)
	m, cmd := key(m, "tab")
	m = finishOperation(m, cmd)
	if !strings.Contains(m.View(), "Lock expired") {
		t.Fatal(ansi.Strip(m.View()))
	}
	r, _ := s.Discover(context.Background())
	c, _ := s.Counts(context.Background(), r[0])
	if c.Active != 128 {
		t.Fatal("expired review sent")
	}

	s, _ = fixture.New()
	m = start(t, reviewStore{Store: s, mode: "ambiguous"}, config.Default(), "fixture", false)
	m = press(m, "]")
	m = acquiredReview(t, m)
	m = press(m, "tab")
	m, cmd = key(m, "enter")
	m = press(m, "q")
	m = finishOperation(m, cmd)
	if !strings.Contains(m.(interface{ ExitSummary() string }).ExitSummary(), "Send outcome UNKNOWN") {
		t.Fatal("quit lost pending send outcome")
	}
}
