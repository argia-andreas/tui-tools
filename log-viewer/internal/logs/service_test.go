package logs_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"unicode"

	"logviewer/internal/fixture"
	"logviewer/internal/logs"
)

type providerFunc func(context.Context, logs.Query, string) (logs.NativePage, error)

func (f providerFunc) Search(c context.Context, q logs.Query, cursor string) (logs.NativePage, error) {
	return f(c, q, cursor)
}

func fixtureService(t *testing.T, policy logs.OutputPolicy) *logs.Service {
	t.Helper()
	p, err := fixture.New()
	if err != nil {
		t.Fatal(err)
	}
	s, err := logs.NewService(p, policy)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestFixtureSafeOutput(t *testing.T) {
	s := fixtureService(t, fixture.Policy())
	page, err := s.Search(context.Background(), fixture.Query())
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Events) != 22 || page.More {
		t.Fatalf("incomplete fixture page: %d, %v", len(page.Events), page.More)
	}
	data, _ := json.Marshal(page)
	for _, forbidden := range []string{"fixture-secret-", "Q0FOQVJZ", "https://invalid.example", `\u001b`, `\u202e`, "never rendered"} {
		if strings.Contains(string(data), forbidden) {
			t.Fatalf("unsafe output includes %q", forbidden)
		}
	}
	selected := page.Events[5]
	if !strings.Contains(selected.Text("message"), "\nOrder 8042 remains pending") {
		t.Fatal("multiline message lost")
	}
	if !strings.Contains(string(selected.Values["fields"]), `"token":"[REDACTED]"`) {
		t.Fatal("nested path not redacted")
	}
	if !strings.Contains(string(selected.Values["fields"]), `"retryable":true`) {
		t.Fatal("boolean became text")
	}
	if page.Events[17].Text("level") != "UNKNOWN" {
		t.Fatal("missing severity was invented")
	}
	if !strings.Contains(string(page.Events[17].Values["fields"]), `9007199254740993`) {
		t.Fatal("large integer lost precision")
	}
	if !strings.Contains(string(page.Events[7].Values["fields"]), `"receipt":null`) {
		t.Fatal("null lost")
	}
	if !strings.Contains(string(page.Events[8].Values["fields"]), "Åsa") {
		t.Fatal("safe UTF-8 lost")
	}
	if selected.Handle == "" || selected.Handle == page.Events[4].Handle {
		t.Fatal("event handles missing or reused")
	}
	policy := fixture.Policy()
	policy.Fields = []string{"message"}
	limited, err := fixtureService(t, policy).Search(context.Background(), fixture.Query())
	if err != nil {
		t.Fatal(err)
	}
	if len(limited.Events[5].Values) != 1 || limited.Events[5].Text("service") != "" {
		t.Fatal("output field restriction bypassed")
	}
}

func TestReadBoundariesAndFailures(t *testing.T) {
	s := fixtureService(t, fixture.Policy())
	q := fixture.Query()
	q.Window.Start = q.Window.Start.Add(11*60*1e9 + 3*1e9 + 39*1e6)
	q.Window.End = q.Window.Start.Add(967 * 1e6)
	page, err := s.Search(context.Background(), q)
	if err != nil || len(page.Events) != 2 {
		t.Fatalf("half-open boundary should include two equal-time records, not end: %+v, %v", page, err)
	}
	q = fixture.Query()
	q.Limits.Events = 3
	page, err = s.Search(context.Background(), q)
	if err != nil || len(page.Events) != 3 || !page.More {
		t.Fatal("event budget not disclosed", err)
	}
	q = fixture.Query()
	q.Limits.Bytes = 1
	page, err = s.Search(context.Background(), q)
	if err != nil || len(page.Events) != 0 || !page.More {
		t.Fatal("byte budget not disclosed", err)
	}
	for _, change := range []func(*logs.Query){
		func(q *logs.Query) { q.Scope.Source = "other" },
		func(q *logs.Query) { q.Window.End = q.Window.Start },
		func(q *logs.Query) { q.Limits.Events = 501 },
		func(q *logs.Query) { q.Limits.Bytes = 0 },
	} {
		q = fixture.Query()
		change(&q)
		page, err = s.Search(context.Background(), q)
		if err == nil || len(page.Events) != 0 {
			t.Fatal("invalid request exposed data")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.Search(ctx, fixture.Query()); !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation ignored", err)
	}
	p := providerFunc(func(context.Context, logs.Query, string) (logs.NativePage, error) {
		return logs.NativePage{}, errors.New("private endpoint fixture-secret-error-canary")
	})
	s, _ = logs.NewService(p, fixture.Policy())
	if _, err := s.Search(context.Background(), fixture.Query()); err == nil || strings.Contains(err.Error(), "canary") {
		t.Fatal("unsafe backend error", err)
	}
	p = providerFunc(func(context.Context, logs.Query, string) (logs.NativePage, error) {
		return logs.NativePage{Events: []logs.Event{{Message: strings.Repeat("x", 1<<20)}}}, nil
	})
	s, _ = logs.NewService(p, fixture.Policy())
	if _, err := s.Search(context.Background(), fixture.Query()); err == nil {
		t.Fatal("oversized event accepted")
	}
	policy := fixture.Policy()
	policy.RedactPaths = []string{"/fields/bad~2escape"}
	if _, err := logs.NewService(p, policy); err == nil {
		t.Fatal("invalid redaction policy accepted")
	}
}

func FuzzSafeOutput(f *testing.F) {
	for _, seed := range []string{"hello Åsa", "fixture-secret-canary", "fixture-\x1b[31msecret-canary", "fixture-\u202esecret-canary", "\x1b]52;c;abc\a", "\n\t\x00\u202e", "\x9b31m"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input string) {
		if len(input) > 4096 {
			return
		}
		p := providerFunc(func(context.Context, logs.Query, string) (logs.NativePage, error) {
			return logs.NativePage{Events: []logs.Event{{Message: input, Fields: map[string]json.RawMessage{"nested": json.RawMessage(`[{"token":"hidden"}]`)}}}}, nil
		})
		policy := fixture.Policy()
		policy.RedactPaths = append(policy.RedactPaths, "/fields/nested/*/token")
		s, err := logs.NewService(p, policy)
		if err != nil {
			t.Fatal(err)
		}
		page, err := s.Search(context.Background(), fixture.Query())
		if err != nil {
			t.Fatal(err)
		}
		message := page.Events[0].Text("message")
		if strings.Contains(message, "fixture-secret-canary") {
			t.Fatal("terminal cleanup reconstructed an unredacted secret")
		}
		for _, r := range message {
			if (unicode.IsControl(r) && r != '\n') || unicode.Is(unicode.Cf, r) {
				t.Fatalf("unsafe rune %U", r)
			}
		}
		if strings.Contains(string(page.Events[0].Values["fields"]), "hidden") {
			t.Fatal("nested-array path redaction failed")
		}
	})
}
