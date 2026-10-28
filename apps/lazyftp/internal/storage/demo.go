package storage

import (
	"context"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"time"
)

type demoBrowser struct {
	directories map[Location][]Entry
}

// NewDemo returns an isolated, deterministic in-memory browser. It never reads
// or writes the host filesystem, and remote names do not initiate connections.
func NewDemo() Browser {
	stamp := time.Date(2026, time.October, 1, 12, 0, 0, 0, time.UTC)
	dir := func(name string) Entry { return Entry{Name: name, IsDir: true, Modified: stamp} }
	file := func(name string, size int64) Entry {
		return Entry{Name: name, Size: size, Modified: stamp}
	}
	return &demoBrowser{directories: map[Location][]Entry{
		{Path: "/"}:                                      {dir("project"), dir("downloads"), file("Welcome.md", 824)},
		{Path: "/project"}:                               {dir("src"), dir("empty"), file("README.md", 2048), file("project notes.txt", 512), file("配置.json", 128)},
		{Path: "/project/src"}:                           {file("main.go", 1370), file("browser.go", 2840)},
		{Path: "/project/empty"}:                         {},
		{Path: "/downloads"}:                             {dir("photos"), file("release.tar.gz", 4194304), file("zero bytes.txt", 0)},
		{Path: "/downloads/photos"}:                      {file("夏の写真.png", 245760)},
		{Remote: "demo-ftp", Path: "/"}:                  {dir("incoming"), dir("archive"), file("server notice.txt", 256)},
		{Remote: "demo-ftp", Path: "/incoming"}:          {dir("batch 01"), dir("empty"), file("orders.csv", 8192), file("résumé.pdf", 32768)},
		{Remote: "demo-ftp", Path: "/incoming/batch 01"}: {file("shipment.json", 1536)},
		{Remote: "demo-ftp", Path: "/incoming/empty"}:    {},
		{Remote: "demo-ftp", Path: "/archive"}:           {dir("2025"), file("index.csv", 4096)},
		{Remote: "demo-ftp", Path: "/archive/2025"}:      {file("annual report.pdf", 98304)},
		{Remote: "demo-blob", Path: "/"}:                 {dir("assets"), file("manifest.json", 736)},
		{Remote: "demo-blob", Path: "/assets"}:           {dir("icons"), dir("empty"), file("hero image.png", 524288), file("样式.css", 3072)},
		{Remote: "demo-blob", Path: "/assets/icons"}:     {file("logo.svg", 1024)},
		{Remote: "demo-blob", Path: "/assets/empty"}:     {},
	}}
}

// List treats empty and relative paths as rooted paths and cleans dot segments.
// Results are independent copies, with directories first and raw names ordered
// lexicographically within each group. Missing directories wrap fs.ErrNotExist.
func (b *demoBrowser) List(ctx context.Context, loc Location) ([]Entry, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	loc.Path = path.Clean("/" + loc.Path)
	entries, ok := b.directories[loc]
	if !ok {
		return nil, fmt.Errorf("demo directory %q on %q: %w", loc.Path, loc.Remote, fs.ErrNotExist)
	}
	result := make([]Entry, len(entries))
	copy(result, entries)
	sort.Slice(result, func(i, j int) bool {
		if result[i].IsDir != result[j].IsDir {
			return result[i].IsDir
		}
		return result[i].Name < result[j].Name
	})
	return result, nil
}
