package elastic_test

import (
	"context"
	"encoding/json"
	"logviewer/internal/config"
	"logviewer/internal/elastic"
	"logviewer/internal/logs"
	"os"
	"os/exec"
	"slices"
	"testing"
	"time"
)

func labClient(t *testing.T) (*elastic.Client, config.Config) {
	t.Helper()
	path := os.Getenv("LOGVIEW_LAB_CONFIG")
	if path == "" {
		t.Skip("explicit local lab integration only")
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	c, err := elastic.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := c.Shutdown(context.Background()); err != nil {
			t.Error(err)
		}
	})
	return c, cfg
}
func scenario(t *testing.T, action string) {
	t.Helper()
	cmd := exec.Command("python3", "../../scripts/lab.py", "scenario-"+action)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("owned lab scenario %s: %v (%s)", action, err, out)
	}
}

func TestLocalLabHistorySnapshot(t *testing.T) {
	c, cfg := labClient(t)
	ctx := context.Background()
	scenario(t, "remove")
	t.Cleanup(func() { scenario(t, "remove") })
	scenario(t, "add")
	q := cfg.Query()
	q.Window = logs.Window{Start: time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC), End: time.Date(2026, 10, 24, 12, 0, 1, 0, time.UTC)}
	q.Scope.Services = []string{"slice3-alpha", "slice3-beta"}
	q.Levels = []string{"WARN", "ERROR", "FATAL"}
	q.Message = logs.MessageFilter{Mode: "phrase", Text: "EXACT PHRASE"}
	q.Limits.Events = 500
	first, err := c.Search(ctx, q, "")
	if err != nil || len(first.Events) != 500 || !first.More || first.Cursor == "" {
		t.Fatalf("first page count=%d more=%v err=%v", len(first.Events), first.More, err)
	}
	scenario(t, "append")
	older, err := c.Search(ctx, q, first.Cursor)
	if err != nil || len(older.Events) != 100 || older.More || older.Cursor != "" {
		t.Fatalf("older count=%d more=%v err=%v", len(older.Events), older.More, err)
	}
	seen := map[logs.NativeRef]bool{}
	for _, page := range []logs.NativePage{first, older} {
		for _, event := range page.Events {
			if seen[event.Ref] || event.Ref.ID == "slice3-history-0605" || event.Level != "WARN" || !slices.Contains(q.Scope.Services, event.Service) {
				t.Fatalf("snapshot duplicate or filter escape: %+v", event.Ref)
			}
			seen[event.Ref] = true
		}
	}
	if len(seen) != 600 {
		t.Fatalf("lost tied rows: %d", len(seen))
	}
	for _, token := range []string{first.Cursor, "unrecognized"} {
		if _, err := c.Search(ctx, q, token); err == nil {
			t.Fatal("replay/foreign cursor accepted")
		}
	}
	fresh, err := c.Search(ctx, q, "")
	if err != nil {
		t.Fatal(err)
	}
	tail, err := c.Search(ctx, q, fresh.Cursor)
	if err != nil || len(fresh.Events)+len(tail.Events) != 601 {
		t.Fatal("fresh search did not see inserted event", err)
	}
	// Frozen facets cannot be replaced on a valid native token.
	frozen, err := c.Search(ctx, q, "")
	if err != nil {
		t.Fatal(err)
	}
	changed := q
	changed.Levels = nil
	if _, err := c.Search(ctx, changed, frozen.Cursor); err == nil {
		t.Fatal("changed filters accepted")
	}
	closed, err := c.Search(ctx, q, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Close(ctx, closed.Cursor); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Search(ctx, q, closed.Cursor); err == nil {
		t.Fatal("closed cursor became fresh search")
	}
	unknown := q
	unknown.Levels = []string{"UNKNOWN"}
	p, err := c.Search(ctx, unknown, "")
	if err != nil || len(p.Events) != 2 {
		t.Fatalf("UNKNOWN missing/custom count=%d err=%v", len(p.Events), err)
	}
	for _, e := range p.Events {
		if e.Level != "UNKNOWN" {
			t.Fatal("unknown normalized to ordinary severity")
		}
	}
	large := unknown
	large.Fields = []logs.FieldFilter{{Path: "/fields/order/id", Value: json.RawMessage(`9007199254740993`)}}
	p, err = c.Search(ctx, large, "")
	if err != nil || len(p.Events) != 1 || p.Events[0].Ref.ID != "slice3-history-0602" {
		t.Fatal("exact int64 predicate rounded into neighboring ID", err)
	}
	values, err := c.Values(ctx, q, "service", "SlIcE3-")
	if err != nil || !slices.Equal(values.Items, []string{"slice3-alpha", "slice3-beta"}) || values.More {
		t.Fatalf("scoped suggestions: %+v err=%v", values, err)
	}
	limited := q
	limited.Limits.Events = 1
	values, err = c.Values(ctx, limited, "service", "slice3-")
	if err != nil || len(values.Items) != 1 || !values.More {
		t.Fatalf("unbounded suggestions: %+v err=%v", values, err)
	}
	for _, keyword := range []string{".*", "*", "?", `\`, "@", `"`} {
		values, err = c.Values(ctx, q, "service", keyword)
		if err != nil || len(values.Items) != 0 {
			t.Fatalf("suggestion keyword %q was not literal: %+v err=%v", keyword, values, err)
		}
	}
	if _, err := c.Values(ctx, q, "fields.auth.token", ""); err == nil {
		t.Fatal("arbitrary suggestion field accepted")
	}
	t.Log("600 unique equal-time refs exactly once, 500+100 PIT snapshot excludes append; fresh search 601; frozen/closed/replayed cursors fail; app OR + severity AND + analyzed phrase; bounded scoped literal suggestions")
}

func TestLocalLabTypedFieldsAndTrace(t *testing.T) {
	c, cfg := labClient(t)
	ctx := context.Background()
	for _, tc := range []struct {
		path, value string
		count       int
	}{
		{"/service", `"checkout-api"`, 10},
		{"/instance", `"payments-5fb-a"`, 7},
		{"/trace.id", `"2fb1b5a1f0104cad"`, 2},
		{"/level", `"UNKNOWN"`, 1},
		{"/fields/job/id", `"pay-8042"`, 3},
		{"/fields/job/attempt", `1`, 2},
		{"/fields/job/retryable", `true`, 1},
		{"/fields/http/status_code", `200`, 2},
		{"/fields/order/id", `8042`, 5},
		{"/fields/region", `"swedencentral"`, 1},
	} {
		t.Run(tc.path, func(t *testing.T) {
			q := cfg.Query()
			q.Fields = []logs.FieldFilter{{Path: tc.path, Value: json.RawMessage(tc.value)}}
			page, err := c.Search(ctx, q, "")
			if err != nil || len(page.Events) != tc.count {
				t.Fatalf("typed filter count=%d want=%d err=%v; old labs require python3 scripts/lab.py reseed-indexed-fields", len(page.Events), tc.count, err)
			}
		})
	}
	q := cfg.Query()
	q.TraceIDs = []string{"8c3e7a21b9f04450", "2fb1b5a1f0104cad"}
	q.Fields = []logs.FieldFilter{{Path: "/fields/job/attempt", Value: json.RawMessage(`1`)}, {Path: "/fields/job/retryable", Value: json.RawMessage(`true`)}}
	page, err := c.Search(ctx, q, "")
	if err != nil || len(page.Events) != 1 || page.Events[0].Ref.ID != "fixture-0005" {
		t.Fatalf("trace OR / field AND: %+v err=%v", page, err)
	}
	q.TraceIDs = []string{"2fb1b5a1f0104cad"}
	page, err = c.Search(ctx, q, "")
	if err != nil || len(page.Events) != 0 {
		t.Fatal("trace facet not ANDed", err)
	}
}

func TestLocalLabNewestWindowChronologicalPages(t *testing.T) {
	c, cfg := labClient(t)
	q := cfg.Query()
	q.Limits.Events = 3
	first, err := c.Search(context.Background(), q, "")
	if err != nil || len(first.Events) != 3 || !first.More {
		t.Fatal("bounded newest window failed", err)
	}
	for i, id := range []string{"fixture-0019", "fixture-0020", "fixture-0021"} {
		if first.Events[i].Ref.ID != id {
			t.Fatal("did not fetch newest window and display chronologically")
		}
	}
	older, err := c.Search(context.Background(), q, first.Cursor)
	if err != nil || len(older.Events) != 3 || !older.More {
		t.Fatal("older window failed", err)
	}
	for i, id := range []string{"fixture-0016", "fixture-0017", "fixture-0018"} {
		if older.Events[i].Ref.ID != id {
			t.Fatal("older paging order is not chronological")
		}
	}
	if err := c.Close(context.Background(), older.Cursor); err != nil {
		t.Fatal(err)
	}
}
