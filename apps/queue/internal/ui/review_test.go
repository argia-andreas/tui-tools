package ui_test

import (
	"context"
	"errors"
	"fmt"
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
	for _, want := range []string{"ID: orders-dlq-0001", "›[ Cancel / release", "[ ] Remove original", "New MessageID:", "no renewal"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q\n%s", want, view)
		}
	}
	if press(press(m, "ctrl+h"), "ctrl+l").View() != m.View() {
		t.Fatal("directional keys escaped review dialog")
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

func TestReviewControlsRemainVisibleAcrossThemesAndSizes(t *testing.T) {
	for _, theme := range []string{"midnight", "gruvbox", "light"} {
		for _, ascii := range []bool{false, true} {
			s, _ := fixture.New()
			prefs := config.Default()
			prefs.Theme = theme
			m := start(t, s, prefs, "fixture", ascii)
			m = press(m, "]")
			m = acquiredReview(t, m)
			for _, size := range [][2]int{{80, 24}, {110, 24}, {160, 24}, {160, 34}, {160, 35}, {160, 48}} {
				m, _ = m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
				assertFrame(t, m, size[0], size[1])
				for _, want := range []string{"Cancel / release", "Resend copy", "Delete original", "[ ] Remove original", "Lock until:", "duplicate processing"} {
					if !strings.Contains(ansi.Strip(m.View()), want) {
						t.Fatalf("%s ascii=%v at %v hid %q\n%s", theme, ascii, size, want, ansi.Strip(m.View()))
					}
				}
				if size[1] < 35 {
					m = press(m, "pgdown")
					if !strings.Contains(m.View(), "HELD BODY / read-only") || strings.Contains(m.View(), "Resend copy") || strings.Contains(m.View(), "Enter confirm") {
						t.Fatal("body page exposed hidden confirmations")
					}
					if _, cmd := key(m, "enter"); cmd != nil {
						t.Fatal("body page submitted a hidden confirmation")
					}
					m = press(m, "pgup")
				}
			}
			m = press(m, "esc")
		}
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

func (s reviewStore) ReceiveSelectedDLQ(ctx context.Context, r explorer.Resource, sequence int64) (explorer.Delivery, error) {
	if s.late != nil {
		<-s.late
		ctx = context.Background()
	}
	d, err := s.Store.ReceiveSelectedDLQ(ctx, r, sequence)
	if err != nil {
		return d, err
	}
	return &reviewDelivery{Delivery: d, mode: s.mode, deadline: s.deadline}, nil
}

func (s reviewStore) PrepareDLQCopy(ctx context.Context, r explorer.Resource, sequence int64) (explorer.Replay, error) {
	p, err := s.Store.PrepareDLQCopy(ctx, r, sequence)
	if err != nil {
		return nil, err
	}
	return reviewReplay{Replay: p, mode: s.mode}, nil
}

type reviewReplay struct {
	explorer.Replay
	mode string
}

func (p reviewReplay) Resend(ctx context.Context, destination explorer.Resource, id string) (explorer.SendResult, error) {
	r, err := p.Replay.Resend(ctx, destination, id)
	if p.mode == "ambiguous" && err == nil {
		return explorer.SendResult{State: "unknown"}, errors.New("ack lost after acceptance")
	}
	return r, err
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
func (d *reviewDelivery) Resend(ctx context.Context, destination explorer.Resource, id string) (explorer.SendResult, error) {
	if d.mode == "reject" {
		return explorer.SendResult{State: "rejected"}, errors.New("unsupported encoding")
	}
	r, e := d.Delivery.Resend(ctx, destination, id)
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
	m = press(m, "deletx")
	m = press(press(m, "ctrl+h"), "ctrl+l")
	if !strings.Contains(m.View(), "type delete: delet_") {
		t.Fatal("directional keys escaped delete confirmation input")
	}
	m = press(m, "e")
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

func TestHelpPreservesReviewDraftAndPreparedTargetWhileExpiryRuns(t *testing.T) {
	s, _ := fixture.New()
	m := start(t, reviewStore{Store: s, deadline: time.Now().Add(-time.Second)}, config.Default(), "fixture", false)
	m = press(m, "]")
	m, acquire := receive(t, m)
	m, timer := m.Update(acquire())
	if timer == nil {
		t.Fatal("lock timer missing")
	}
	before := m.View()
	m = clickText(t, m, "Actions - Review active", false)
	m = press(m, "ctrl+p")
	if strings.Contains(m.View(), "Query:") {
		t.Fatal("palette opened over review")
	}
	m = press(m, "f1")
	for _, k := range []string{"enter", "q", "ctrl+p", "DELETE"} {
		m = press(m, k)
	}
	m = press(m, "esc")
	if m.View() != before {
		t.Fatal("Help changed review authorization/selection")
	}
	m = press(m, "f1")
	m, release := m.Update(timer())
	if release == nil {
		t.Fatal("Help blocked lock expiry")
	}
	m, _ = m.Update(release())
	if !strings.Contains(m.View(), " HELP ") {
		t.Fatal("expiry dismissed Help")
	}
	m = clickText(t, m, "[x]", false)
	if !strings.Contains(m.View(), "Lock expired") {
		t.Fatal("expired parent not updated")
	}
	r, _ := s.Discover(context.Background())
	c, _ := s.Counts(context.Background(), r[0])
	if c.Active != 128 {
		t.Fatal("Help sent a message")
	}

	m = start(t, s, config.Default(), "fixture", false)
	m = inspectorKeys(m, "]", "j", "enter", "ctrl+r")
	draft := m.View()
	m = inspectorKeys(m, "f1", "q", "enter", "ctrl+p", "esc")
	if m.View() != draft {
		t.Fatal("Help discarded resend configuration")
	}
	// Prepare without executing any send, then return from Help unchanged.
	m = inspectorKeys(m, "tab", "enter")
	prepared := m.View()
	if !strings.Contains(prepared, "SELECTED PEEK COPY") || !strings.Contains(prepared, "ID: orders-dlq-0002") {
		t.Fatal("test did not prepare selected target")
	}
	m = inspectorKeys(m, "f1", "esc")
	if m.View() != prepared {
		t.Fatal("Help discarded prepared identity")
	}
	m = inspectorKeys(m, "esc", "esc") // cancel, then leave result without sending
	m = acquiredReview(t, m)
	m = inspectorKeys(m, "tab", "tab", "tab", "enter", "del")
	typed := m.View()
	if !strings.Contains(typed, "type delete: del_") {
		t.Fatal("test did not enter authorization")
	}
	m = inspectorKeys(m, "f1", "q", "enter", "ctrl+p", "esc")
	if m.View() != typed {
		t.Fatal("Help changed typed delete authorization")
	}
}

func configureSelected(t *testing.T, m tea.Model, cleanup bool) tea.Model {
	t.Helper()
	m = press(m, "enter")
	if !strings.Contains(m.View(), "ID: orders-dlq-0002") {
		t.Fatal("open did not use selected row: " + ansi.Strip(m.View()))
	}
	m = press(m, "ctrl+r")
	if !strings.Contains(m.View(), "Destination queue: orders") || !strings.Contains(m.View(), "[ ] Delete original after confirmed resend") || strings.Contains(m.View(), "Change queue") || strings.Contains(m.View(), "CHOOSE QUEUE") {
		t.Fatal(ansi.Strip(m.View()))
	}
	if cleanup {
		m = press(m, "tab")
		m = press(m, "tab")
		m = press(m, "enter")
		m = press(m, "up")
	} else {
		m = press(m, "tab")
	}
	return m
}

func TestSelectedDLQCopyAndOptionalExactOriginalCleanup(t *testing.T) {
	for _, cleanup := range []bool{false, true} {
		s, _ := fixture.New()
		m := start(t, s, config.Default(), "fixture", false)
		m = press(press(m, "]"), "j")
		m = configureSelected(t, m, cleanup)
		m = press(m, "enter") // prepare, not send
		view := ansi.Strip(m.View())
		if !strings.Contains(view, "ID: orders-dlq-0002") || !strings.Contains(view, "Resend destination: orders") || strings.Contains(view, "type delete:") {
			t.Fatal(view)
		}
		if strings.Contains(view, "Lock until:") != cleanup {
			t.Fatal("copy-only falsely claimed a lock, or cleanup hid it")
		}
		if strings.Contains(view, "releases lock") != cleanup {
			t.Fatal("back-to-edit help misrepresented lock ownership")
		}
		resources, _ := s.Discover(context.Background())
		var destination explorer.Resource
		for _, r := range resources {
			if r.Queue == "orders" {
				destination = r
			}
		}
		before, _ := s.Counts(context.Background(), destination)
		if before.Active != 128 {
			t.Fatal("preparation sent before confirmation")
		}
		m = press(m, "tab")
		m, cmd := key(m, "enter")
		if _, repeat := key(m, "enter"); repeat != nil {
			t.Fatal("repeated Enter resubmitted")
		}
		m = finishOperation(m, cmd)
		want := "Original retained; no cleanup requested"
		if cleanup {
			want = "Separately authorized original cleanup confirmed"
		}
		if !strings.Contains(m.View(), want) {
			t.Fatal(ansi.Strip(m.View()))
		}
		page, _ := s.Peek(context.Background(), resources[0], true, 0, 20)
		found := false
		for _, message := range page.Messages {
			found = found || message.ID == "orders-dlq-0002"
		}
		if found == cleanup || page.Messages[0].ID != "orders-dlq-0001" {
			t.Fatal("cleanup removed a neighbor or retained selected original")
		}
		after, _ := s.Counts(context.Background(), destination)
		if after.Active != 129 {
			t.Fatal("resend did not add exactly one copy to the source queue")
		}
		copies, _ := s.Peek(context.Background(), destination, false, 0, 20)
		for copies.NextSequence != nil {
			copies, _ = s.Peek(context.Background(), destination, false, *copies.NextSequence, 20)
		}
		copy := copies.Messages[len(copies.Messages)-1]
		if !strings.Contains(string(copy.Body), "orders-dlq-0002") || copy.ID == "orders-dlq-0002" {
			t.Fatal("wrong selected content, destination or new ID")
		}
	}
}

func TestSelectedReviewBackAndCompactBodyCannotSend(t *testing.T) {
	for _, theme := range []string{"midnight", "gruvbox", "light"} {
		for _, ascii := range []bool{false, true} {
			s, _ := fixture.New()
			prefs := config.Default()
			prefs.Theme = theme
			m := start(t, s, prefs, "fixture", ascii)
			m = press(press(m, "]"), "j")
			m = configureSelected(t, m, true)
			for _, size := range [][2]int{{80, 24}, {160, 24}, {160, 48}} {
				m, _ = m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
				assertFrame(t, m, size[0], size[1])
				for _, want := range []string{"[x] Delete original after confirmed resend", "Review resend", "Destination queue: orders", "100 messages / 5s", "delivery counts"} {
					if !strings.Contains(m.View(), want) {
						t.Fatalf("configuration hid %q: %s", want, ansi.Strip(m.View()))
					}
				}
			}
			m = press(m, "enter")
			m = press(m, "b") // release before editing
			if !strings.Contains(m.View(), "CONFIGURE SELECTED RESEND") {
				t.Fatal(ansi.Strip(m.View()))
			}
			m = press(press(m, "tab"), "enter")
			m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
			assertFrame(t, m, 80, 24)
			m = press(m, "pgdown")
			if _, hidden := key(m, "enter"); hidden != nil {
				t.Fatal("body page submitted hidden confirmation")
			}
			m = press(m, "esc")
			if !strings.Contains(m.View(), "Lock release confirmed") {
				t.Fatal(ansi.Strip(m.View()))
			}
		}
	}
}

func TestSelectedReplayUnknownAndPartialOutcomes(t *testing.T) {
	for _, tc := range []struct {
		mode, want string
		cleanup    bool
	}{
		{"ambiguous", "Send outcome UNKNOWN", false},
		{"ambiguous", "Send outcome UNKNOWN", true},
		{"cleanup-failed", "Original cleanup unconfirmed/unknown", true},
	} {
		s, _ := fixture.New()
		m := start(t, reviewStore{Store: s, mode: tc.mode}, config.Default(), "fixture", false)
		m = press(press(m, "]"), "j")
		m = configureSelected(t, m, tc.cleanup)
		m = press(press(m, "enter"), "tab")
		m, cmd := key(m, "enter")
		m = finishOperation(m, cmd)
		if !strings.Contains(m.View(), tc.want) {
			t.Fatal(ansi.Strip(m.View()))
		}
		resources, _ := s.Discover(context.Background())
		page, _ := s.Peek(context.Background(), resources[0], true, 0, 20)
		if len(page.Messages) != 12 || page.Messages[1].ID != "orders-dlq-0002" {
			t.Fatal("uncertain send/cleanup removed the selected original")
		}
	}
}

func TestSelectedLateAcquisitionIsReleasedAfterCancel(t *testing.T) {
	s, _ := fixture.New()
	late := make(chan struct{})
	m := start(t, reviewStore{Store: s, late: late}, config.Default(), "fixture", false)
	m = press(press(m, "]"), "j")
	m = configureSelected(t, m, true)
	m, acquire := key(m, "enter")
	m = press(m, "esc")
	close(late)
	m = finishOperation(m, acquire)
	if !strings.Contains(m.View(), "nothing sent or deleted") || !strings.Contains(m.View(), "Lock release confirmed") {
		t.Fatal(ansi.Strip(m.View()))
	}
	resources, _ := s.Discover(context.Background())
	d, err := s.ReceiveSelectedDLQ(context.Background(), resources[0], 107)
	if err != nil {
		t.Fatalf("late selected delivery was not released: %v", err)
	}
	_ = d.Abandon(context.Background())
}

func TestSelectedDisappearedMessageNeverFallsBackToNeighbor(t *testing.T) {
	for _, cleanup := range []bool{false, true} {
		s, _ := fixture.New()
		m := start(t, s, config.Default(), "fixture", false)
		m = press(press(m, "]"), "j")
		m = configureSelected(t, m, cleanup)
		resources, _ := s.Discover(context.Background())
		d, err := s.ReceiveSelectedDLQ(context.Background(), resources[0], 107)
		if err != nil {
			t.Fatal(err)
		}
		if err := d.Complete(context.Background()); err != nil {
			t.Fatal(err)
		}
		m = press(m, "enter")
		if !strings.Contains(m.View(), "nothing sent or deleted") || strings.Contains(m.View(), "Resend copy") {
			t.Fatal(ansi.Strip(m.View()))
		}
		for _, r := range resources {
			if r.Queue == "empty" {
				c, _ := s.Counts(context.Background(), r)
				if c.Active != 0 {
					t.Fatal("missing selected message caused fallback send")
				}
			}
		}
	}
}

func TestSelectedQueueShortcutAndSubscriptionDestinationFilter(t *testing.T) {
	m := press(workspace(t), "]")
	m = press(press(press(m, "ctrl+p"), "resend selected"), "enter")
	if !strings.Contains(m.View(), "Destination queue: orders") || strings.Contains(m.View(), "CHOOSE QUEUE") || strings.Contains(m.View(), "Change queue") {
		t.Fatal("queue shortcut did not configure a direct resend: " + ansi.Strip(m.View()))
	}
	m = press(m, "esc")
	if !strings.Contains(m.View(), "INSPECTOR / NORMAL") || !strings.Contains(m.View(), "Resend to queue") || strings.Contains(m.View(), "CHOOSE QUEUE") {
		t.Fatal("back from queue configuration did not restore the inspector")
	}
	m = press(workspace(t), "ctrl+h")
	for range 4 {
		m = press(m, "j")
	}
	m = press(m, "j")                         // queues group
	m = press(m, "h")                         // collapse queues
	m = press(press(press(m, "j"), "j"), "j") // topics, events, billing
	m = press(m, "]")
	if !strings.Contains(m.View(), "events/billing / DEAD-LETTER") {
		t.Fatal(ansi.Strip(m.View()))
	}
	m = press(m, "ctrl+p")
	m = press(m, "send selected")
	m = press(m, "enter")
	for _, query := range []string{"q", "j", "k"} {
		m = press(m, query)
		if !strings.Contains(m.View(), "Filter queues: "+query+"_") {
			t.Fatal("filter lost printable key ownership: " + ansi.Strip(m.View()))
		}
		m = press(press(m, "ctrl+h"), "ctrl+l")
		if !strings.Contains(m.View(), "Filter queues: _") {
			t.Fatal("directional keys escaped destination filter")
		}
		m = press(m, "ctrl+u")
	}
	m = press(m, "no-such-queue")
	m = press(m, "enter")
	if !strings.Contains(m.View(), "No matching destination queues") {
		t.Fatal("empty destination filter allowed selection")
	}
}

// Capture a visible point so tests can deliberately reuse stale pixels after
// resize/body scrolling. This does not interpret private review state.
func reviewPoint(t *testing.T, m tea.Model, label string) tea.MouseMsg {
	t.Helper()
	for y, row := range strings.Split(ansi.Strip(m.View()), "\n") {
		if x := strings.Index(row, label); x >= 0 {
			return tea.MouseMsg{X: ansi.StringWidth(row[:x]), Y: y, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress}
		}
	}
	t.Fatalf("review target %q missing\n%s", label, ansi.Strip(m.View()))
	return tea.MouseMsg{}
}

func TestReviewPointerCurrentVisibilityAndExpiry(t *testing.T) {
	for _, size := range [][2]int{{79, 24}, {80, 23}, {80, 24}} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			s, _ := fixture.New()
			m := acquiredReview(t, press(start(t, s, config.Default(), "fixture", false), "]"))
			m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
			point := reviewPoint(t, m, "Resend copy")
			m, _ = m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
			m, cmd := m.Update(point)
			if cmd != nil {
				m = finishOperation(m, cmd)
			}
			resources, _ := s.Discover(context.Background())
			counts, _ := s.Counts(context.Background(), resources[0])
			want := int64(128)
			if size == [2]int{80, 24} {
				want++
			}
			if counts.Active != want {
				t.Fatalf("stale/visible press effects: %+v", counts)
			}
		})
	}
	for _, expired := range []bool{false, true} {
		s, _ := fixture.New()
		deadline := time.Time{}
		if expired {
			deadline = time.Now().Add(-time.Second)
		}
		m := acquiredReview(t, press(start(t, reviewStore{Store: s, deadline: deadline}, config.Default(), "fixture", false), "]"))
		m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
		point := reviewPoint(t, m, "Resend copy")
		if !expired {
			m = press(m, "pgdown")
		} else {
			m, _ = m.Update(tea.WindowSizeMsg{Width: 79, Height: 24})
		}
		m, cmd := m.Update(point)
		if cmd != nil {
			m = finishOperation(m, cmd)
		}
		if expired && !strings.Contains(m.View(), "Lock expired") {
			t.Fatal("size gate ran before expiry reconciliation")
		}
		resources, _ := s.Discover(context.Background())
		counts, _ := s.Counts(context.Background(), resources[0])
		if counts.Active != 128 {
			t.Fatal("hidden/expired pointer sent")
		}
	}
}

func TestReviewPointerDeleteAuthorizationAndBusyRefusal(t *testing.T) {
	s, _ := fixture.New()
	m := acquiredReview(t, press(start(t, s, config.Default(), "fixture", false), "]"))
	m = clickText(t, m, "Delete original", false) // focus authorization, never delete
	m = press(m, "Delete")
	point := reviewPoint(t, m, "Delete original ]")
	if _, cmd := m.Update(point); cmd != nil {
		t.Fatal("nonexact pointer authorization deleted")
	}
	m = press(press(m, "tab"), "up")
	m = press(m, "delete")
	point = reviewPoint(t, m, "Delete original ]")
	m, cmd := m.Update(point)
	if cmd == nil {
		t.Fatal("exact pointer authorization did not delete")
	}
	if _, repeat := m.Update(point); repeat != nil {
		t.Fatal("pending delete repeated")
	}
	if _, cancel := key(m, "esc"); cancel != nil {
		t.Fatal("pending delete cancelled")
	}
	m = finishOperation(m, cmd)
	if !strings.Contains(m.View(), "Deletion confirmed") {
		t.Fatal(ansi.Strip(m.View()))
	}
	resources, _ := s.Discover(context.Background())
	counts, _ := s.Counts(context.Background(), resources[0])
	if counts.DeadLetter != 11 || counts.Active != 128 {
		t.Fatalf("wrong pointer delete effects: %+v", counts)
	}
}

func TestReviewTypedDeletePromptPointerFocusAndCompactCapture(t *testing.T) {
	s, _ := fixture.New()
	m := acquiredReview(t, press(start(t, s, config.Default(), "fixture", false), "]"))
	m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = clickText(t, m, "Delete original", false)
	m = press(m, "Delete")
	if _, cmd := key(m, "enter"); cmd != nil {
		t.Fatal("nonexact authorization submitted")
	}
	prompt := reviewPoint(t, m, "type delete:")
	var cmd tea.Cmd
	m, cmd = m.Update(prompt)
	if cmd != nil || !strings.Contains(ansi.Strip(m.View()), "type delete: _") {
		t.Fatal("visible authorization prompt did not focus the existing delete-entry owner", ansi.Strip(m.View()))
	}
	m = press(m, "delete")
	confirm := reviewPoint(t, m, "Delete original ]")
	for _, point := range []tea.MouseMsg{prompt, confirm} {
		point.Action = tea.MouseActionRelease
		m, cmd = m.Update(point)
		if cmd != nil || !strings.Contains(ansi.Strip(m.View()), "type delete: delete_") {
			t.Fatal("release changed authorization or confirmed deletion")
		}
	}
	m = press(m, "pgdown")
	if strings.Contains(ansi.Strip(m.View()), "type delete:") {
		t.Fatal("compact body retained authorization prompt")
	}
	for _, point := range []tea.MouseMsg{prompt, confirm} {
		m, cmd = m.Update(point)
		if cmd != nil {
			t.Fatal("hidden prompt/confirmation pixels requested an operation")
		}
	}
	if _, cmd := key(m, "enter"); cmd != nil {
		t.Fatal("hidden confirmation accepted Enter")
	}
	m = press(m, "pgup")
	if !strings.Contains(ansi.Strip(m.View()), "type delete: delete_") {
		t.Fatal("hidden prompt pixels changed retained authorization", ansi.Strip(m.View()))
	}
	resources, _ := s.Discover(context.Background())
	counts, _ := s.Counts(context.Background(), resources[0])
	if counts.DeadLetter != 12 || counts.Active != 128 {
		t.Fatalf("focus/typing/hidden input mutated messages: %+v", counts)
	}
	m, cmd = m.Update(reviewPoint(t, m, "Delete original ]"))
	if cmd == nil {
		t.Fatal("explicit confirmation did not accept exact authorization")
	}
	m = finishOperation(m, cmd)
	counts, _ = s.Counts(context.Background(), resources[0])
	if counts.DeadLetter != 11 || counts.Active != 128 || !strings.Contains(m.View(), "Deletion confirmed") {
		t.Fatalf("wrong explicit delete outcome: %+v\n%s", counts, ansi.Strip(m.View()))
	}
}

func TestReviewPointerLateCancelAndOutcomes(t *testing.T) {
	s, _ := fixture.New()
	late := make(chan struct{})
	m := press(start(t, reviewStore{Store: s, late: late}, config.Default(), "fixture", false), "]")
	m, acquire := receive(t, m)
	m = clickText(t, m, "[x]", false)
	close(late)
	m = finishOperation(m, acquire)
	if !strings.Contains(m.View(), "Lock release confirmed") {
		t.Fatal("late pointer cancel lost delivery")
	}
	for _, tc := range []struct{ mode, want string }{{"ambiguous", "Send outcome UNKNOWN"}, {"cleanup-failed", "Original cleanup unconfirmed/unknown"}} {
		s, _ := fixture.New()
		m := acquiredReview(t, press(start(t, reviewStore{Store: s, mode: tc.mode}, config.Default(), "fixture", false), "]"))
		m = clickText(t, m, "[ ] Remove original", false)
		point := reviewPoint(t, m, "Resend copy")
		m, cmd := m.Update(point)
		if cmd == nil {
			t.Fatal("pointer send missing")
		}
		point.Action = tea.MouseActionRelease
		if _, repeat := m.Update(point); repeat != nil {
			t.Fatal("release repeated send")
		}
		m = finishOperation(m, cmd)
		if !strings.Contains(m.View(), tc.want) {
			t.Fatal(ansi.Strip(m.View()))
		}
	}
}

func TestSelectedPointerPreparedIdentitySurvivesListChange(t *testing.T) {
	s, _ := fixture.New()
	m := press(press(start(t, s, config.Default(), "fixture", false), "]"), "j")
	m = press(press(m, "enter"), "ctrl+r")
	m = clickText(t, m, "Review resend", false)
	// Remove the preceding list row after preparation; the prepared snapshot,
	// not a new list index, must remain the outgoing message.
	resources, _ := s.Discover(context.Background())
	d, err := s.ReceiveNextDLQ(context.Background(), resources[0])
	if err != nil {
		t.Fatal(err)
	}
	if err = d.Complete(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(m.View(), "ID: orders-dlq-0002") {
		t.Fatal("prepared target changed")
	}
	point := reviewPoint(t, m, "Resend copy")
	m, cmd := m.Update(point)
	if cmd == nil {
		t.Fatal("selected pointer send missing")
	}
	m = finishOperation(m, cmd)
	if !strings.Contains(m.View(), "no cleanup requested") {
		t.Fatal(ansi.Strip(m.View()))
	}
	page, err := s.Peek(context.Background(), resources[0], false, 0, 20)
	if err != nil {
		t.Fatal(err)
	}
	for page.NextSequence != nil {
		page, err = s.Peek(context.Background(), resources[0], false, *page.NextSequence, 20)
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(page.Messages) == 0 {
		t.Fatal("send produced no visible copy")
	}
	last := page.Messages[len(page.Messages)-1]
	if !strings.Contains(string(last.Body), "orders-dlq-0002") {
		t.Fatal("pointer sent replacement list row")
	}
}

func TestDestinationPointerSelectOnlyAndExplicitChoose(t *testing.T) {
	m := press(workspace(t), "ctrl+h")
	for range 5 {
		m = press(m, "j")
	}
	m = press(m, "h")
	m = inspectorKeys(m, "j", "j", "j", "]", "ctrl+p", "send selected", "enter")
	if !strings.Contains(m.View(), "CHOOSE QUEUE") {
		t.Fatal(ansi.Strip(m.View()))
	}
	m = press(m, "orders")
	m = clickText(t, m, "› orders", false)
	if !strings.Contains(m.View(), "CHOOSE QUEUE") || strings.Contains(m.View(), "CONFIGURE SELECTED RESEND") {
		t.Fatal("destination row implicitly activated")
	}
	m = clickText(t, m, "Choose", true)
	if !strings.Contains(m.View(), "CHOOSE QUEUE") {
		t.Fatal("release chose queue")
	}
	m = clickText(t, m, "Choose", false)
	if !strings.Contains(m.View(), "Destination queue: orders") || !strings.Contains(m.View(), "[ ] Delete original") {
		t.Fatal("explicit Choose failed or enabled cleanup")
	}
	m = clickText(t, m, "Change queue", false)
	m = press(m, "no-such-queue")
	m = clickText(t, m, "Choose", false)
	if !strings.Contains(m.View(), "No matching destination queues") {
		t.Fatal("empty filtered queue activated")
	}
}

func TestSelectedPointerLateCancelAndReleaseBeforeEdit(t *testing.T) {
	s, _ := fixture.New()
	late := make(chan struct{})
	m := press(press(start(t, reviewStore{Store: s, late: late}, config.Default(), "fixture", false), "]"), "j")
	m = press(press(m, "enter"), "ctrl+r")
	m = clickText(t, m, "[ ] Delete original", false)
	point := reviewPoint(t, m, "Review resend")
	m, acquire := m.Update(point)
	if acquire == nil {
		t.Fatal("preparation missing")
	}
	m = clickText(t, m, "[x]", false)
	close(late)
	m = finishOperation(m, acquire)
	if !strings.Contains(m.View(), "nothing sent or deleted") || !strings.Contains(m.View(), "Lock release confirmed") {
		t.Fatal(ansi.Strip(m.View()))
	}
	// The next preparation uses the same exact message, now available again.
	m = press(press(m, "esc"), "ctrl+r")
	m = clickText(t, m, "[ ] Delete original", false)
	m = clickText(t, m, "Review resend", false)
	point = reviewPoint(t, m, "Back to edit b ]")
	m, release := m.Update(point)
	if release == nil || strings.Contains(m.View(), "CONFIGURE SELECTED RESEND") {
		t.Fatal("Back edited before lock release")
	}
	m, _ = m.Update(release())
	if !strings.Contains(m.View(), "CONFIGURE SELECTED RESEND") {
		t.Fatal("confirmed release did not restore configuration")
	}
}
