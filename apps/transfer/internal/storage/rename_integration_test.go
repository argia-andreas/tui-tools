//go:build integration

package storage_test

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"lazyftp/internal/storage"
)

// Exercises the real rclone command without requiring a lab service. The local
// backend is addressed as a remote to cover the moveto path and verification.
func TestRenameRcloneLocalIntegration(t *testing.T) {
	if _, err := exec.LookPath("rclone"); err != nil {
		t.Fatal("integration prerequisite unavailable: install rclone")
	}
	dir := t.TempDir()
	config := filepath.Join(dir, "rclone.conf")
	if err := os.WriteFile(config, []byte("[rename-test]\ntype = local\n"), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	var renamer storage.Renamer = storage.NewRclone(config)
	src := storage.Location{Remote: "rename-test", Path: filepath.Join(dir, "source 雪")}
	dst := storage.Location{Remote: "rename-test", Path: filepath.Join(dir, "destination ; $x")}
	if err := os.WriteFile(src.Path, []byte("bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := renamer.Rename(ctx, src, dst); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(src.Path); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("source still exists: %v", err)
	}
	if got, err := os.ReadFile(dst.Path); err != nil || string(got) != "bytes" {
		t.Fatalf("destination: %q %v", got, err)
	}
	if err := os.WriteFile(src.Path, []byte("bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := renamer.Rename(ctx, src, dst); !errors.Is(err, fs.ErrExist) {
		t.Fatalf("identical destination must be rejected: %v", err)
	}
	// Pin the crucial rclone behavior separately from our preflight: this flag
	// must retain the source even if a destination appears before moveto runs.
	if err := exec.CommandContext(ctx, "rclone", "moveto", "--config", config,
		"--ignore-existing", "--retries", "1", "--", src.String(), dst.String()).Run(); err != nil {
		t.Fatalf("moveto --ignore-existing: %v", err)
	}
	for _, loc := range []storage.Location{src, dst} {
		if got, err := os.ReadFile(loc.Path); err != nil || string(got) != "bytes" {
			t.Fatalf("skipped moveto changed %q: %q %v", loc.Path, got, err)
		}
	}
}
