//go:build integration

package storage_test

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"lazyftp/internal/storage"
)

func TestReadLabIntegration(t *testing.T) {
	if _, err := exec.LookPath("rclone"); err != nil {
		t.Fatalf("integration requires rclone and make lab-up lab-seed: %v", err)
	}
	config, err := filepath.Abs("../../.lab/rclone.conf")
	if err != nil {
		t.Fatal(err)
	}
	var reader storage.Reader = storage.NewRclone(config)
	for _, fixture := range []struct {
		remote, path, data string
	}{
		{"lab-ftp", "/incoming/ftp only.txt", "Only on FTP.\n"},
		{"lab-ftp", "/incoming/Unicode/日本語.txt", "FTP 日本語 notes.\n"},
		{"lab-ftp", "/incoming/zero bytes.bin", ""},
		{"lab-blob", "/files/assets/blob only.txt", "Only on Blob.\n"},
		{"lab-blob", "/files/assets/Unicode/naïve résumé.txt", "Blob naïve résumé notes.\n"},
		{"lab-blob", "/files/assets/zero bytes.bin", ""},
	} {
		t.Run(fixture.remote+fixture.path, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			loc := storage.Location{Remote: fixture.remote, Path: fixture.path}
			for _, limit := range []int64{1, 7, int64(len(fixture.data)), 65536} {
				got, err := reader.Read(ctx, loc, limit)
				want := []byte(fixture.data)[:min(int64(len(fixture.data)), limit)]
				if err != nil || !bytes.Equal(got, want) {
					t.Fatalf("limit %d: got %q err=%v, want %q; ensure make lab-up lab-seed", limit, got, err, want)
				}
			}
			canceled, stop := context.WithCancel(ctx)
			stop()
			if _, err := reader.Read(canceled, loc, 8); !errors.Is(err, context.Canceled) {
				t.Fatalf("canceled lab read: %v", err)
			}
		})
	}
}
