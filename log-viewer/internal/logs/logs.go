// Package logs owns log investigation contracts independent of terminal and transport.
package logs

import (
	"context"
	"encoding/json"
	"time"
)

type Window struct{ Start, End time.Time }
type Limits struct{ Events, Bytes int }
type Scope struct{ Source string }
type Query struct {
	Scope  Scope
	Window Window
	Limits Limits
}

// Event is untrusted provider data and never goes directly to presentation.
type Event struct {
	Time                                       time.Time
	Level, Service, Instance, Message, TraceID string
	Fields                                     map[string]json.RawMessage
}

type NativePage struct {
	Events []Event
	More   bool
}

// The tracer bullet introduces only the provider operation it actually uses.
// Paging, context and suggestions arrive with their vertical slices.
type Provider interface {
	Search(context.Context, Query, string) (NativePage, error)
}

type SafeEvent struct {
	Handle string
	Values map[string]json.RawMessage
}

func (e SafeEvent) Text(field string) string {
	var s string
	_ = json.Unmarshal(e.Values[field], &s)
	return s
}

type Page struct {
	Events []SafeEvent
	More   bool
}
