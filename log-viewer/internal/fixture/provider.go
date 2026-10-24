// Package fixture is an offline dataset, not an Elasticsearch emulator.
package fixture

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"time"

	"logviewer/internal/logs"
)

//go:embed events.json
var data []byte

type Provider struct{ events []logs.Event }

func New() (*Provider, error) {
	var events []logs.Event
	if err := json.Unmarshal(data, &events); err != nil {
		return nil, fmt.Errorf("invalid embedded fixtures")
	}
	return &Provider{events: events}, nil
}

func (p *Provider) Search(ctx context.Context, q logs.Query, cursor string) (logs.NativePage, error) {
	if q.Scope.Source != "fixture" || cursor != "" {
		return logs.NativePage{}, fmt.Errorf("unsupported fixture request")
	}
	page := logs.NativePage{}
	for _, event := range p.events {
		if err := ctx.Err(); err != nil {
			return logs.NativePage{}, err
		}
		if event.Time.Before(q.Window.Start) || !event.Time.Before(q.Window.End) {
			continue
		}
		if len(page.Events) == q.Limits.Events {
			page.More = true
			break
		}
		page.Events = append(page.Events, event)
	}
	return page, nil
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
