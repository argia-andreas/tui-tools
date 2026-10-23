// Package storage defines browsing and single-file transfer contracts.
package storage

import (
	"context"
	"time"
)

// Location identifies a path. An empty Remote identifies the local filesystem.
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

// Transferer operates on full file paths, not directory destinations. Copy must
// reject directories and existing destinations unless overwrite is explicit.
// FTP cannot guarantee atomic no-clobber against concurrent external writers.
type Transferer interface {
	Stat(context.Context, Location) (Entry, error)
	Copy(ctx context.Context, source, destination Location, overwrite bool) error
}
