package storage_test

import (
	"context"
	"errors"
	"io/fs"
	"path"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"

	"lazyftp/internal/storage"
)

func TestDemoTrees(t *testing.T) {
	browser := storage.NewDemo()
	for _, remote := range []string{"", "demo-ftp", "demo-blob"} {
		t.Run(remote, func(t *testing.T) {
			var visit func(string)
			hasEmpty, hasSpace, hasUnicode, hasNested := false, false, false, false
			visit = func(p string) {
				t.Helper()
				loc := storage.Location{Remote: remote, Path: p}
				entries, err := browser.List(context.Background(), loc)
				if err != nil {
					t.Fatal(err)
				}
				if len(entries) == 0 {
					hasEmpty = true
				}
				for i, entry := range entries {
					if entry.Modified.IsZero() || entry.Size < 0 {
						t.Fatalf("bad metadata: %+v", entry)
					}
					if i > 0 {
						prev := entries[i-1]
						if (!prev.IsDir && entry.IsDir) || (prev.IsDir == entry.IsDir && prev.Name > entry.Name) {
							t.Fatalf("unsorted entries: %+v", entries)
						}
					}
					hasSpace = hasSpace || strings.Contains(entry.Name, " ")
					hasUnicode = hasUnicode || utf8.RuneCountInString(entry.Name) < len(entry.Name)
					if entry.IsDir {
						visit(path.Join(p, entry.Name))
					} else if p != "/" {
						hasNested = true
					}
				}
				again, err := storage.NewDemo().List(context.Background(), loc)
				if err != nil || !reflect.DeepEqual(entries, again) {
					t.Fatalf("not deterministic at %+v", loc)
				}
				if len(entries) > 0 {
					entries[0].Name = "changed"
					fresh, _ := browser.List(context.Background(), loc)
					if fresh[0].Name == "changed" {
						t.Fatal("listing aliases internal storage")
					}
				}
			}
			visit("/")
			if !hasEmpty || !hasSpace || !hasUnicode || !hasNested {
				t.Fatalf("fixture missing cases: empty=%v space=%v unicode=%v nested=%v", hasEmpty, hasSpace, hasUnicode, hasNested)
			}
		})
	}
}

func TestDemoPathsAndErrors(t *testing.T) {
	browser := storage.NewDemo()
	for _, loc := range []storage.Location{{Path: "/project"}, {Path: "/downloads"}, {Remote: "demo-ftp", Path: "/incoming"}, {Remote: "demo-ftp", Path: "/archive"}, {Remote: "demo-blob", Path: "/assets"}} {
		if entries, err := browser.List(context.Background(), loc); err != nil || len(entries) == 0 {
			t.Fatalf("required location %+v: %v", loc, err)
		}
	}
	want, _ := browser.List(context.Background(), storage.Location{Path: "/project"})
	for _, p := range []string{"project", "/downloads/../project/", "/project/."} {
		got, err := browser.List(context.Background(), storage.Location{Path: p})
		if err != nil || !reflect.DeepEqual(want, got) {
			t.Fatalf("normalization of %q failed", p)
		}
	}
	root, _ := browser.List(context.Background(), storage.Location{Path: "/"})
	emptyPath, _ := browser.List(context.Background(), storage.Location{})
	if !reflect.DeepEqual(root, emptyPath) {
		t.Fatal("empty path is not root")
	}
	for _, loc := range []storage.Location{{Path: "/missing"}, {Remote: "unknown"}, {Path: "/project/README.md"}, {Remote: "demo-blob", Path: "/incoming"}} {
		if _, err := browser.List(context.Background(), loc); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("%+v: expected not-exist, got %v", loc, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := browser.List(ctx, storage.Location{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
}
