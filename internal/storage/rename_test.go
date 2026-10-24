package storage_test

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"lazyftp/internal/storage"
)

func TestRenameLocal(t *testing.T) {
	var browser storage.Browser = storage.NewRclone("")
	renamer, ok := browser.(storage.Renamer)
	if !ok {
		t.Fatal("Rclone Browser must implement Renamer")
	}
	for _, contents := range []string{"", "asymmetric 雪\x00\xff"} {
		dir := t.TempDir()
		src := storage.Location{Path: filepath.Join(dir, "source ; $x")}
		dst := storage.Location{Path: filepath.Join(dir, "-destination 雪")}
		if err := os.WriteFile(src.Path, []byte(contents), 0600); err != nil {
			t.Fatal(err)
		}
		if err := renamer.Rename(context.Background(), src, dst); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Lstat(src.Path); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("source still exists: %v", err)
		}
		if got, err := os.ReadFile(dst.Path); err != nil || string(got) != contents {
			t.Fatalf("destination bytes: %q, %v", got, err)
		}
	}
}

func TestRenameLocalCollisions(t *testing.T) {
	var renamer storage.Renamer = storage.NewRclone("")
	for _, kind := range []string{"identical", "different", "directory", "symlink", "dangling-symlink", "same-path"} {
		t.Run(kind, func(t *testing.T) {
			dir := t.TempDir()
			src := storage.Location{Path: filepath.Join(dir, "source")}
			dst := storage.Location{Path: filepath.Join(dir, "destination")}
			if err := os.WriteFile(src.Path, []byte("source bytes"), 0600); err != nil {
				t.Fatal(err)
			}
			var err error
			switch kind {
			case "identical":
				err = os.WriteFile(dst.Path, []byte("source bytes"), 0600)
			case "different":
				err = os.WriteFile(dst.Path, []byte("destination bytes"), 0600)
			case "directory":
				err = os.Mkdir(dst.Path, 0700)
			case "symlink", "dangling-symlink":
				if runtime.GOOS == "windows" {
					t.Skip("symlink privileges required")
				}
				target := src.Path
				if kind == "dangling-symlink" {
					target += "-missing"
				}
				err = os.Symlink(target, dst.Path)
			case "same-path":
				dst = src
			}
			if err != nil {
				t.Fatal(err)
			}
			before, err := os.Lstat(dst.Path)
			if err != nil {
				t.Fatal(err)
			}
			if err := renamer.Rename(context.Background(), src, dst); !errors.Is(err, fs.ErrExist) {
				t.Fatalf("want collision, got %v", err)
			}
			after, err := os.Lstat(dst.Path)
			if err != nil || !os.SameFile(before, after) {
				t.Fatalf("destination replaced: %v", err)
			}
			if got, err := os.ReadFile(src.Path); err != nil || string(got) != "source bytes" {
				t.Fatalf("source changed: %q %v", got, err)
			}
			if kind == "different" {
				if got, err := os.ReadFile(dst.Path); err != nil || string(got) != "destination bytes" {
					t.Fatalf("destination changed: %q %v", got, err)
				}
			}
		})
	}
}

func TestRenameValidationAndCancellation(t *testing.T) {
	var renamer storage.Renamer = storage.NewRclone("")
	dir := t.TempDir()
	src := storage.Location{Path: filepath.Join(dir, "source")}
	if err := os.WriteFile(src.Path, []byte("data"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, pair := range [][2]storage.Location{
		{src, {Path: filepath.Join(dir, "other", "destination")}},
		{src, {Remote: "other", Path: src.Path}},
		{{Remote: "one", Path: "/file"}, {Remote: "two", Path: "/renamed"}},
		{{Remote: "one", Path: "/a/file"}, {Remote: "one", Path: "/b/renamed"}},
	} {
		if err := renamer.Rename(context.Background(), pair[0], pair[1]); err == nil {
			t.Fatalf("invalid pair accepted: %+v", pair)
		}
	}
	for _, name := range []string{"", ".", "..", "file/", "bad\\name", "bad\x00name"} {
		for _, remote := range []string{"", "fixture"} {
			bad := storage.Location{Remote: remote, Path: name}
			good := storage.Location{Remote: remote, Path: "good"}
			for _, pair := range [][2]storage.Location{{bad, good}, {good, bad}} {
				if err := renamer.Rename(context.Background(), pair[0], pair[1]); err == nil || !strings.Contains(err.Error(), "basenames") {
					t.Fatalf("invalid basename %+v: %v", pair, err)
				}
			}
		}
	}
	for _, remote := range []string{"", "fixture"} {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if err := renamer.Rename(ctx, storage.Location{Remote: remote, Path: src.Path}, storage.Location{Remote: remote, Path: src.Path + "-new"}); !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled rename: %v", err)
		}
	}
	if err := renamer.Rename(context.Background(), storage.Location{Path: src.Path + "-missing"}, storage.Location{Path: src.Path + "-new"}); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("missing source: %v", err)
	}
	for _, kind := range []string{"directory", "symlink"} {
		file := filepath.Join(dir, kind)
		var err error
		if kind == "directory" {
			err = os.Mkdir(file, 0700)
		} else {
			if runtime.GOOS == "windows" {
				continue
			}
			err = os.Symlink(src.Path, file)
		}
		if err != nil {
			t.Fatal(err)
		}
		if err := renamer.Rename(context.Background(), storage.Location{Path: file}, storage.Location{Path: file + "-new"}); err == nil {
			t.Fatalf("renamed %s", kind)
		}
	}
}

// This fixture simulates a destination created after preflight, and a remote
// failure/cancellation after the mutating command has started.
func TestRenameRemoteCommandSafety(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell fixture")
	}
	dir := t.TempDir()
	script := `#!/bin/sh
command=$1
shift
ignore=no
while test "$#" -gt 0; do
  case "$1" in
    --ignore-existing) ignore=yes ;;
    --) shift; break ;;
  esac
  shift
done
case "$command" in
  lsjson)
    test "$1" = 'fixture:/' || exit 91
    if test -f "$RENAME_STATE"; then
      printf '[{"Name":"source","Size":4},{"Name":"destination","Size":4}]'
    else
      printf '[{"Name":"source","Size":4}]'
    fi ;;
  moveto)
    test "$ignore" = yes && test "$1" = 'fixture:/source' && test "$2" = 'fixture:/destination' || exit 92
    touch "$RENAME_STATE"
    case "$RENAME_MODE" in
      fail) printf 'https://user:secret@example.invalid/?sig=credential' >&2; exit 1 ;;
      wait) while :; do :; done ;;
    esac ;;
  *) exit 93 ;;
esac
`
	if err := os.WriteFile(filepath.Join(dir, "rclone"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	state := filepath.Join(dir, "state")
	t.Setenv("RENAME_STATE", state)
	var renamer storage.Renamer = storage.NewRclone("")
	for _, mode := range []string{"skip", "fail", "wait"} {
		t.Run(mode, func(t *testing.T) {
			os.Remove(state)
			t.Setenv("RENAME_MODE", mode)
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			err := renamer.Rename(ctx, storage.Location{Remote: "fixture", Path: "/source"}, storage.Location{Remote: "fixture", Path: "/destination"})
			if err == nil || !strings.Contains(err.Error(), "partly succeeded") || strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), "credential") {
				t.Fatalf("want safe uncertainty error: %v", err)
			}
			if mode == "skip" && !errors.Is(err, fs.ErrExist) {
				t.Fatalf("skipped move: %v", err)
			}
			if mode == "wait" && !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("canceled command: %v", err)
			}
		})
	}
}
