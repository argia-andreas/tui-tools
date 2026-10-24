// Package fixture is an offline dataset, not an Elasticsearch emulator.
package fixture

import (
	"context"
	"crypto/rand"
	_ "embed"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode"

	"logviewer/internal/logs"
)

//go:embed events.json
var data []byte

type Provider struct {
	events   []logs.Event
	mu       sync.Mutex
	sessions map[string]snapshot
}
type snapshot struct {
	query     logs.Query
	remaining []logs.Event
	expires   time.Time
}

func New() (*Provider, error) {
	var events []logs.Event
	if err := json.Unmarshal(data, &events); err != nil {
		return nil, fmt.Errorf("invalid embedded fixtures")
	}
	for i := range events {
		events[i].Ref = logs.NativeRef{Index: "fixture", ID: fmt.Sprintf("%08d", i)}
	}
	return &Provider{events: events, sessions: make(map[string]snapshot)}, nil
}

func (p *Provider) Search(ctx context.Context, q logs.Query, cursor string) (logs.NativePage, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if q.Scope.Source != "fixture" || logs.ValidateQuery(q) != nil {
		return logs.NativePage{}, fmt.Errorf("unsupported fixture request")
	}
	for key, entry := range p.sessions {
		if !time.Now().Before(entry.expires) {
			delete(p.sessions, key)
		}
	}
	var matches []logs.Event
	if cursor != "" {
		entry, ok := p.sessions[cursor]
		if !ok {
			return logs.NativePage{}, fmt.Errorf("fixture history expired; reload required")
		}
		delete(p.sessions, cursor)
		q = entry.query
		matches = entry.remaining
	} else {
		for _, event := range p.events {
			if matchesQuery(event, q) {
				matches = append(matches, event)
			}
		}
		slices.SortFunc(matches, func(a, b logs.Event) int {
			if c := b.Time.Compare(a.Time); c != 0 {
				return c
			}
			return strings.Compare(b.Ref.ID, a.Ref.ID)
		})
	}
	if err := ctx.Err(); err != nil {
		return logs.NativePage{}, err
	}
	n := min(q.Limits.Events, len(matches))
	page := logs.NativePage{Events: slices.Clone(matches[:n]), More: n < len(matches)}
	slices.Reverse(page.Events)
	if page.More {
		if len(p.sessions) >= 16 {
			for key := range p.sessions {
				delete(p.sessions, key)
				break
			}
		}
		page.Cursor = rand.Text()
		p.sessions[page.Cursor] = snapshot{query: q, remaining: matches[n:], expires: time.Now().Add(2 * time.Minute)}
	}
	return page, nil
}

func matchesQuery(e logs.Event, q logs.Query) bool {
	level := e.Level
	if level == "" {
		level = "UNKNOWN"
	}
	if e.Time.Before(q.Window.Start) || !e.Time.Before(q.Window.End) || (len(q.Scope.Services) > 0 && !slices.Contains(q.Scope.Services, e.Service)) || (len(q.Levels) > 0 && !slices.Contains(q.Levels, level)) || (len(q.TraceIDs) > 0 && !slices.Contains(q.TraceIDs, e.TraceID)) {
		return false
	}
	// Deterministic word-phrase exercise only, not an Elastic analyzer emulator.
	tokens := func(s string) string {
		return strings.Join(strings.FieldsFunc(strings.ToLower(s), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }), " ")
	}
	if q.Message.Text != "" && !strings.Contains(" "+tokens(e.Message)+" ", " "+tokens(q.Message.Text)+" ") {
		return false
	}
	for _, filter := range q.Fields {
		var raw json.RawMessage
		switch filter.Path {
		case "/service":
			raw, _ = json.Marshal(e.Service)
		case "/instance":
			raw, _ = json.Marshal(e.Instance)
		case "/trace.id":
			raw, _ = json.Marshal(e.TraceID)
		case "/level":
			raw, _ = json.Marshal(level)
		default:
			var value any
			b, _ := json.Marshal(e.Fields)
			d := json.NewDecoder(strings.NewReader(string(b)))
			d.UseNumber()
			_ = d.Decode(&value)
			for _, part := range strings.Split(strings.TrimPrefix(filter.Path, "/fields/"), "/") {
				object, ok := value.(map[string]any)
				if !ok {
					return false
				}
				value = object[part]
			}
			raw, _ = json.Marshal(value)
		}
		var a, b any
		left := json.NewDecoder(strings.NewReader(string(raw)))
		left.UseNumber()
		right := json.NewDecoder(strings.NewReader(string(filter.Value)))
		right.UseNumber()
		if left.Decode(&a) != nil || right.Decode(&b) != nil || a != b {
			return false
		}
	}
	return true
}

func (p *Provider) Close(_ context.Context, cursor string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.sessions, cursor)
	return nil
}

func (p *Provider) Values(ctx context.Context, q logs.Query, field, keyword string) (logs.Values, error) {
	if field != "service" || q.Scope.Source != "fixture" || logs.ValidateQuery(q) != nil {
		return logs.Values{}, fmt.Errorf("unsupported fixture suggestions")
	}
	values := logs.Values{}
	for _, event := range p.events {
		if err := ctx.Err(); err != nil {
			return logs.Values{}, err
		}
		if matchesQuery(event, q) && strings.Contains(strings.ToLower(event.Service), strings.ToLower(keyword)) && !slices.Contains(values.Items, event.Service) {
			values.Items = append(values.Items, event.Service)
		}
	}
	slices.Sort(values.Items)
	return values, nil
}

// Query freezes the fictional window. Never label fixture timestamps as "now".
func Query() logs.Query {
	start := time.Date(2026, 10, 23, 14, 15, 0, 0, time.UTC)
	return logs.Query{
		Scope: logs.Scope{Source: "fixture"}, Window: logs.Window{Start: start, End: start.Add(15 * time.Minute)},
		Limits: logs.Limits{Events: 500, Bytes: 32 << 20},
	}
}

func Policy() logs.OutputPolicy {
	return logs.OutputPolicy{
		Source:      "fixture",
		Fields:      []string{"time", "level", "service", "instance", "message", "trace.id", "fields"},
		RedactPaths: []string{"/fields/auth/token"},
		Patterns:    []string{`fixture-secret-[A-Za-z0-9-]+`},
	}
}
