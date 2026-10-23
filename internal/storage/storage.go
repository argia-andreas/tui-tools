// Package storage defines the read-only browsing contract used by the demo UI.
package storage

import (
	"context"
	"time"
)

// Location identifies a directory. An empty Remote identifies the demo local tree.
type Location struct {
	Remote string
	Path   string
}

// Entry contains raw filename data, not terminal-safe display text.
type Entry struct {
	Name     string
	IsDir    bool
	Size     int64
	Modified time.Time
}

// Browser lists directories without changing their contents.
type Browser interface {
	List(ctx context.Context, loc Location) ([]Entry, error)
}
