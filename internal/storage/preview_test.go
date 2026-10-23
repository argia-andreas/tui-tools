package storage_test

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"lazyftp/internal/storage"
)

func fakePreviewRclone(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("controlled rclone fixture requires a POSIX shell")
	}
	dir := t.TempDir()
	// Validate argument boundaries too: paths/config may contain spaces and
	// shell metacharacters, but must never be interpreted by a shell.
	script := `#!/bin/sh
test "$1" = cat && test "$2" = --head || exit 91
shift 3
while test "$#" -gt 0; do
  case "$1" in
    --config) test "$2" = 'config with spaces ; $x' || exit 92; shift 2 ;;
    --) shift; test "$#" = 1 && test "$1" = 'fixture:/space ; $x' || exit 93; break ;;
    *) shift ;;
  esac
done
case "$PREVIEW_MODE" in
  fail) printf 'partial'; printf 'https://user:secret@example.invalid/?sig=credential' >&2; exit 1 ;;
  flood) while :; do printf '0123456789abcdef'; printf 'secret' >&2; done ;;
  wait) while :; do :; done ;;
  *) exec /bin/cat "$PREVIEW_FILE" ;;
esac
`
	if err := os.WriteFile(filepath.Join(dir, "rclone"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("PREVIEW_MODE", "")
}

func TestReadPrefixes(t *testing.T) {
	fakePreviewRclone(t)
	var reader storage.Reader = storage.NewRclone("config with spaces ; $x")
	for _, payload := range []struct {
		name string
		data []byte
	}{
		{"empty", nil},
		{"text", []byte("Asymmetric café\nsecond line\n")},
		{"binary-control", []byte{0, 0xff, 'A', '\r', '\n', 0x1b, '[', '2', 'J', 7, 'Z'}},
		{"large", bytes.Repeat([]byte("0123456789abcdef"), 10000)},
	} {
		t.Run(payload.name, func(t *testing.T) {
			file := filepath.Join(t.TempDir(), "file ; $x")
			if err := os.WriteFile(file, payload.data, 0600); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PREVIEW_FILE", file)
			for _, loc := range []storage.Location{{Path: file}, {Remote: "fixture", Path: "/space ; $x"}} {
				for _, limit := range []int64{0, 1, 7, 11, 12, 31, 32769, 200000, 1<<63 - 1} {
					got, err := reader.Read(context.Background(), loc, limit)
					want := payload.data[:min(int64(len(payload.data)), limit)]
					if err != nil || !bytes.Equal(got, want) {
						t.Fatalf("Read(%+v, %d): got %d bytes, err=%v; want %d exact bytes", loc, limit, len(got), err, len(want))
					}
				}
			}
		})
	}
}

func TestReadLocalRejections(t *testing.T) {
	reader := storage.NewRclone("")
	dir := t.TempDir()
	file := filepath.Join(dir, "file")
	if err := os.WriteFile(file, []byte("data"), 0600); err != nil {
		t.Fatal(err)
	}
	paths := []string{dir}
	if runtime.GOOS != "windows" {
		link := filepath.Join(dir, "link")
		if err := os.Symlink(file, link); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, link)
		fifo := filepath.Join(dir, "fifo")
		if err := exec.Command("mkfifo", fifo).Run(); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, fifo, "/dev/null")
	}
	for _, path := range paths {
		if data, err := reader.Read(context.Background(), storage.Location{Path: path}, 8); err == nil || len(data) != 0 {
			t.Fatalf("Read nonregular %q: data=%q err=%v", path, data, err)
		}
	}
	if _, err := reader.Read(context.Background(), storage.Location{Path: filepath.Join(dir, "missing")}, 8); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("missing file: %v", err)
	}
}

func TestReadLimitsAndCancellation(t *testing.T) {
	fakePreviewRclone(t)
	reader := storage.NewRclone("config with spaces ; $x")
	for _, loc := range []storage.Location{{Path: "missing"}, {Remote: "fixture", Path: "/space ; $x"}} {
		if _, err := reader.Read(context.Background(), loc, -1); err == nil {
			t.Fatal("negative limit succeeded")
		}
		if data, err := reader.Read(context.Background(), loc, 0); err != nil || len(data) != 0 {
			t.Fatalf("zero limit performed I/O: %q %v", data, err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, err := reader.Read(ctx, loc, 8); !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled context: %v", err)
		}
	}
	remote := storage.Location{Remote: "fixture", Path: "/space ; $x"}
	t.Setenv("PREVIEW_MODE", "wait")
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := reader.Read(ctx, remote, 32); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("in-flight deadline: %v", err)
	}

	t.Setenv("PREVIEW_MODE", "fail")
	if data, err := reader.Read(context.Background(), remote, 32); err == nil || len(data) != 0 || strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), "credential") {
		t.Fatalf("backend failure must be generic: data=%q err=%v", data, err)
	}

	t.Setenv("PREVIEW_MODE", "flood")
	ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	got, err := reader.Read(ctx, remote, 19)
	if err != nil || string(got) != "0123456789abcdef012" {
		t.Fatalf("backend ignored --head: got %q err=%v", got, err)
	}
}
