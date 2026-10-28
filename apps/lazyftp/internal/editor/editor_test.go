package editor_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"lazyftp/internal/editor"
	"lazyftp/internal/storage"
)

var remoteFile = storage.Location{Remote: "fixture", Path: "/folder/file with spaces ; $x.txt"}

// This fake does real disk I/O: retaining a mutable working-file path instead
// of an immutable upload snapshot must fail even if Copy is called serially.
type memoryTransferer struct {
	mu        sync.Mutex
	data      string
	modified  time.Time
	uploads   []string
	active    int
	maxActive int
	copies    int
	onUpload  func(context.Context, string, string, int) error
}

func newTransferer() *memoryTransferer {
	return &memoryTransferer{data: "seed", modified: time.Unix(1700000000, 0)}
}

func (f *memoryTransferer) Stat(_ context.Context, loc storage.Location) (storage.Entry, error) {
	if loc.Remote == "" {
		info, err := os.Stat(loc.Path)
		if err != nil {
			return storage.Entry{}, err
		}
		return storage.Entry{Name: info.Name(), Size: info.Size(), Modified: info.ModTime()}, nil
	}
	if loc != remoteFile {
		return storage.Entry{}, fmt.Errorf("unexpected Stat location: %+v", loc)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return storage.Entry{Name: filepath.Base(loc.Path), Size: int64(len(f.data)), Modified: f.modified}, nil
}

func (f *memoryTransferer) Copy(ctx context.Context, src, dst storage.Location, overwrite bool) error {
	f.mu.Lock()
	f.copies++
	if src == remoteFile && dst.Remote == "" {
		data := f.data
		f.mu.Unlock()
		return os.WriteFile(dst.Path, []byte(data), 0600)
	}
	if src.Remote != "" || dst != remoteFile || !overwrite {
		f.mu.Unlock()
		return fmt.Errorf("unexpected Copy(%+v, %+v, overwrite=%v)", src, dst, overwrite)
	}
	f.active++
	f.maxActive = max(f.maxActive, f.active)
	n := len(f.uploads) + 1
	f.uploads = append(f.uploads, "")
	f.mu.Unlock()
	defer func() { f.mu.Lock(); f.active--; f.mu.Unlock() }()
	data, err := os.ReadFile(src.Path)
	if err != nil {
		return err
	}
	if f.onUpload != nil {
		if err := f.onUpload(ctx, src.Path, string(data), n); err != nil {
			return err
		}
	}
	f.mu.Lock()
	f.uploads[n-1] = string(data)
	f.data = string(data)
	f.modified = f.modified.Add(time.Second)
	f.mu.Unlock()
	return nil
}

func (f *memoryTransferer) state() (string, []string, int, int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.data, append([]string(nil), f.uploads...), f.maxActive, f.copies
}

func writeMarker(dir, name string) error {
	return os.WriteFile(filepath.Join(dir, name), nil, 0600)
}

func waitMarker(ctx context.Context, dir, name string) error {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			return nil
		} else if !os.IsNotExist(err) {
			return err
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("waiting for %s: %w", name, ctx.Err())
		case <-ticker.C:
		}
	}
}

// The script validates argv boundaries and records the actual editing path.
// All waits have a shell-side deadline too, so even a broken implementation
// which does not honor context cannot leave a fake editor running forever.
func installNvim(t *testing.T, body string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake nvim requires a POSIX shell")
	}
	dir := t.TempDir()
	script := `#!/bin/sh
set -eu
test "$#" = 2 && test "$1" = -- || exit 91
file=$2
case "$file" in /*) ;; *) exit 92 ;; esac
printf '%s' "$file" > "$EDITOR_TEST_DIR/path"
wait_for() {
  count=0
  until test -f "$EDITOR_TEST_DIR/$1"; do
    count=$((count + 1))
    test "$count" -lt 500 || exit 93
    sleep 0.02
  done
}
` + body + "\n"
	if err := os.WriteFile(filepath.Join(dir, "nvim"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("EDITOR_TEST_DIR", dir)
	return dir
}

type runningEditor struct {
	ctx  context.Context
	done chan error
}

func startEditor(t *testing.T, f storage.Transferer, loc storage.Location) runningEditor {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	t.Cleanup(cancel)
	cmd := editor.New(ctx, f, loc)
	cmd.SetStdin(strings.NewReader(""))
	cmd.SetStdout(io.Discard)
	cmd.SetStderr(io.Discard)
	r := runningEditor{ctx: ctx, done: make(chan error, 1)}
	go func() { r.done <- cmd.Run() }()
	return r
}

func (r runningEditor) finish(t *testing.T) error {
	t.Helper()
	select {
	case err := <-r.done:
		return err
	case <-r.ctx.Done():
		t.Fatal("editor did not finish before deadline")
		return r.ctx.Err()
	}
}

func mustWait(t *testing.T, r runningEditor, dir, name string) {
	t.Helper()
	if err := waitMarker(r.ctx, dir, name); err != nil {
		t.Fatal(err)
	}
}

func mustMarker(t *testing.T, dir, name string) {
	t.Helper()
	if err := writeMarker(dir, name); err != nil {
		t.Fatal(err)
	}
}

func TestRemoteLiveAtomicAndSameSizeSavesUseSerialSnapshots(t *testing.T) {
	dir := installNvim(t, `
test "$(cat "$file")" = seed
printf AAAA > "$file.new"
mv "$file.new" "$file"
wait_for uploading
printf BBBB > "$file"
printf CCCC > "$file.new"
mv "$file.new" "$file"
touch "$EDITOR_TEST_DIR/next-save"
wait_for latest-uploaded
: > "$file"
wait_for empty-uploaded
touch "$EDITOR_TEST_DIR/observed-live"
`)
	f := newTransferer()
	f.onUpload = func(ctx context.Context, path, data string, n int) error {
		if n == 1 {
			if data != "AAAA" {
				return fmt.Errorf("first stable save = %q, want AAAA", data)
			}
			if err := writeMarker(dir, "uploading"); err != nil {
				return err
			}
			if err := waitMarker(ctx, dir, "next-save"); err != nil {
				return err
			}
			after, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if !bytes.Equal(after, []byte(data)) {
				return fmt.Errorf("in-flight snapshot changed from %q to %q", data, after)
			}
		}
		if data == "CCCC" {
			return writeMarker(dir, "latest-uploaded")
		}
		if data == "" {
			return writeMarker(dir, "empty-uploaded")
		}
		return nil
	}
	r := startEditor(t, f, remoteFile)
	if err := r.finish(t); err != nil {
		t.Fatal(err)
	}
	mustWait(t, r, dir, "observed-live")
	data, uploads, concurrent, _ := f.state()
	if data != "" || len(uploads) < 3 || uploads[0] != "AAAA" || concurrent != 1 {
		t.Fatalf("remote=%q uploads=%q max concurrent=%d", data, uploads, concurrent)
	}
	path, err := os.ReadFile(filepath.Join(dir, "path"))
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(string(path)) != filepath.Base(remoteFile.Path) {
		t.Fatalf("working copy lost basename: %s", path)
	}
}

func TestRemoteExitFlushesImmediateSaveAndSkipsUnchanged(t *testing.T) {
	for _, changed := range []bool{false, true} {
		t.Run(fmt.Sprintf("changed=%v", changed), func(t *testing.T) {
			body := `test "$(cat "$file")" = seed`
			if changed {
				body += "\nprintf final > \"$file\""
			}
			installNvim(t, body)
			f := newTransferer()
			if err := startEditor(t, f, remoteFile).finish(t); err != nil {
				t.Fatal(err)
			}
			data, uploads, _, _ := f.state()
			if changed {
				if data != "final" || len(uploads) != 1 {
					t.Fatalf("remote=%q uploads=%q; want one final upload", data, uploads)
				}
			} else if data != "seed" || len(uploads) != 0 {
				t.Fatalf("unchanged file uploaded: remote=%q uploads=%q", data, uploads)
			}
		})
	}
}

func TestRemoteFailurePreservesLatestWorkingCopy(t *testing.T) {
	dir := installNvim(t, `
printf edit > "$file"
wait_for failed
printf rescue > "$file"
touch "$EDITOR_TEST_DIR/saved-after-failure"
wait_for exit
`)
	f := newTransferer()
	f.onUpload = func(_ context.Context, _, _ string, _ int) error {
		if err := writeMarker(dir, "failed"); err != nil {
			return err
		}
		return errors.New("fixture upload denied")
	}
	r := startEditor(t, f, remoteFile)
	mustWait(t, r, dir, "saved-after-failure")
	// Leave the editor alive across several watch intervals: failure must
	// disable automatic retries, including when the user makes another save.
	select {
	case err := <-r.done:
		t.Fatalf("Run returned before editor exit: %v", err)
	case <-time.After(650 * time.Millisecond):
	}
	mustMarker(t, dir, "exit")
	err := r.finish(t)
	assertRecovery(t, dir, err, "rescue")
	data, uploads, _, _ := f.state()
	if data != "seed" || len(uploads) != 1 {
		t.Fatalf("upload failure retried or changed remote: remote=%q attempts=%q", data, uploads)
	}
}

func TestRemoteConflictRefusesOverwrite(t *testing.T) {
	dir := installNvim(t, `
touch "$EDITOR_TEST_DIR/ready"
wait_for external-change
printf edit > "$file"
`)
	f := newTransferer()
	r := startEditor(t, f, remoteFile)
	mustWait(t, r, dir, "ready")
	f.mu.Lock()
	f.data = "peer" // same size: modification time alone must detect this.
	f.modified = f.modified.Add(time.Minute)
	f.mu.Unlock()
	mustMarker(t, dir, "external-change")
	err := r.finish(t)
	assertRecovery(t, dir, err, "edit")
	message := strings.ToLower(err.Error())
	if !strings.Contains(message, "conflict") && !strings.Contains(message, "remote changed") {
		t.Fatalf("conflict error not useful: %v", err)
	}
	data, uploads, _, _ := f.state()
	if data != "peer" || len(uploads) != 0 {
		t.Fatalf("external edit overwritten: remote=%q uploads=%q", data, uploads)
	}
}

func assertRecovery(t *testing.T, dir string, runErr error, want string) {
	t.Helper()
	path, err := os.ReadFile(filepath.Join(dir, "path"))
	if err != nil {
		t.Fatal(err)
	}
	// Keep successful recovery fixtures only for the duration of the test.
	t.Cleanup(func() {
		_ = os.RemoveAll(filepath.Dir(string(path)))
	})
	if runErr == nil || !strings.Contains(runErr.Error(), string(path)) {
		t.Fatalf("error must identify recovery path %q: %v", path, runErr)
	}
	data, err := os.ReadFile(string(path))
	if err != nil || string(data) != want {
		t.Fatalf("recovery data=%q err=%v; want %q", data, err, want)
	}
	for _, p := range []string{string(path), filepath.Dir(string(path))} {
		info, err := os.Stat(p)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm()&0077 != 0 {
			t.Fatalf("working copy is not private: %s mode=%v", p, info.Mode())
		}
	}
}

func TestLocalEditsOriginalWithoutTransfers(t *testing.T) {
	dir := installNvim(t, `
test "$(cat "$file")" = local
printf changed > "$file"
`)
	path := filepath.Join(t.TempDir(), "local ; $x.txt")
	if err := os.WriteFile(path, []byte("local"), 0600); err != nil {
		t.Fatal(err)
	}
	f := newTransferer()
	if err := startEditor(t, f, storage.Location{Path: path}).finish(t); err != nil {
		t.Fatal(err)
	}
	actualPath, err := os.ReadFile(filepath.Join(dir, "path"))
	if err != nil || string(actualPath) != path {
		t.Fatalf("editor path=%q err=%v; want original %q", actualPath, err, path)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "changed" {
		t.Fatalf("local contents=%q err=%v", data, err)
	}
	_, _, _, copies := f.state()
	if copies != 0 {
		t.Fatalf("local edit made %d transfers", copies)
	}
}

func TestMissingNvimIsUsefulError(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	path := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(path, []byte("local"), 0600); err != nil {
		t.Fatal(err)
	}
	err := startEditor(t, newTransferer(), storage.Location{Path: path}).finish(t)
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "nvim") {
		t.Fatalf("missing nvim should be identified: %v", err)
	}
}
