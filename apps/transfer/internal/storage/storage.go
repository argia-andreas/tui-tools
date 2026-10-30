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

// Deleter removes one file at its full path, never directories or prefixes.
type Deleter interface {
	Delete(context.Context, Location) error
}

// Renamer renames a regular file within the same remote and directory, refusing
// existing destinations even when their contents match. Remote backends cannot
// guarantee atomic no-clobber against concurrent external writers. A failed
// remote rename may have partly succeeded; callers must refresh before retrying.
type Renamer interface {
	Rename(ctx context.Context, source, destination Location) error
}

// Transferer operates on full file paths, not directory destinations. Copy must
// reject directories and existing destinations unless overwrite is explicit.
// FTP cannot guarantee atomic no-clobber against concurrent external writers.
type Transferer interface {
	Stat(context.Context, Location) (Entry, error)
	Copy(ctx context.Context, source, destination Location, overwrite bool) error
}
