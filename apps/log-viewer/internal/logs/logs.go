// Package logs owns log investigation contracts independent of terminal and transport.
package logs

import (
	"context"
	"encoding/json"
	"time"
)

type Window struct{ Start, End time.Time }
type Limits struct{ Events, Bytes int }
type Scope struct {
	Source   string
	Services []string
}
type MessageFilter struct {
	Mode, Text    string
	CaseSensitive bool
}
type FieldFilter struct {
	Path  string
	Value json.RawMessage
}
type Query struct {
	Scope    Scope
	Window   Window
	Limits   Limits
	Levels   []string
	Message  MessageFilter
	TraceIDs []string
	Fields   []FieldFilter
}

// Event is untrusted provider data and never goes directly to presentation.
type Event struct {
	Ref                                        NativeRef
	Time                                       time.Time
	Level, Service, Instance, Message, TraceID string
	Fields                                     map[string]json.RawMessage
	Incomplete                                 bool
}

type NativeRef struct{ Index, ID string }

type NativePage struct {
	Events  []Event
	More    bool
	Cursor  string
	Partial bool
}

// Small scripted providers may support Search alone; real history/suggestions
// providers implement HistoryProvider. Context/follow remain later slices.
type Provider interface {
	Search(context.Context, Query, string) (NativePage, error)
}

// History and suggestions are optional for small scripted test providers.
type HistoryProvider interface {
	Provider
	Close(context.Context, string) error
	Values(context.Context, Query, string, string) (Values, error)
}
type Values struct {
	Items         []string
	More, Partial bool
}

type SafeEvent struct {
	Handle     string
	Values     map[string]json.RawMessage
	Incomplete bool
}

func (e SafeEvent) Text(field string) string {
	var s string
	_ = json.Unmarshal(e.Values[field], &s)
	return s
}

type Page struct {
	Events  []SafeEvent
	More    bool
	Next    string
	Partial bool
}
