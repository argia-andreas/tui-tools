//go:build integration

package storage_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"testing"
	"time"

	"lazyftp/internal/storage"
)

func TestRcloneIntegration(t *testing.T) {
	if _, err := exec.LookPath("rclone"); err != nil {
		t.Fatalf("integration prerequisite unavailable: install rclone: %v", err)
	}
	config, err := filepath.Abs("../../.lab/rclone.conf")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(config); err != nil {
		t.Fatalf("integration prerequisite unavailable: prepare .lab/rclone.conf and start the lab services: %v", err)
	}
	client := storage.NewRclone(config)
	for _, remote := range []struct{ name, root string }{
		{"lab-ftp", "/incoming"},
		{"lab-blob", "/files/assets"},
		{"lab-s3", "/files/assets"},
	} {
		t.Run(remote.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			if _, err := client.List(ctx, storage.Location{Remote: remote.name, Path: remote.root}); err != nil {
				t.Fatalf("integration prerequisite unavailable: %s lab service/root: %v", remote.name, err)
			}
			var nonce [12]byte
			if _, err := rand.Read(nonce[:]); err != nil {
				t.Fatal(err)
			}
			prefix := path.Join(remote.root, "lazyftp-integration-"+hex.EncodeToString(nonce[:]))
			remoteFile := func(name string) storage.Location {
				return storage.Location{Remote: remote.name, Path: path.Join(prefix, name)}
			}
			// Never purge a seed directory or shared root, even after a failed test.
			t.Cleanup(func() {
				cleanupCtx, stop := context.WithTimeout(context.Background(), 30*time.Second)
				defer stop()
				if err := exec.CommandContext(cleanupCtx, "rclone", "purge", "--config", config,
					"--retries", "1", "--low-level-retries", "1", "--", remote.name+":"+prefix).Run(); err != nil {
					t.Errorf("cleanup of unique test prefix %s:%s failed: %v", remote.name, prefix, err)
				}
			})
			localDir := t.TempDir()
			localFile := func(name string) storage.Location {
				return storage.Location{Path: filepath.Join(localDir, name)}
			}
			write := func(loc storage.Location, data []byte) {
				t.Helper()
				if err := os.WriteFile(loc.Path, data, 0600); err != nil {
					t.Fatal(err)
				}
			}
			copyFile := func(src, dst storage.Location, overwrite bool) {
				t.Helper()
				if err := client.Copy(ctx, src, dst, overwrite); err != nil {
					t.Fatalf("Copy(%+v, %+v, overwrite=%v): %v", src, dst, overwrite, err)
				}
			}
			checkBytes := func(loc storage.Location, want []byte) {
				t.Helper()
				file := loc.Path
				if loc.Remote != "" {
					file = filepath.Join(t.TempDir(), "verification.bin")
					copyFile(loc, storage.Location{Path: file}, false)
				}
				got, err := os.ReadFile(file)
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(got, want) {
					t.Fatalf("bytes differ at %+v: got %d bytes, want %d", loc, len(got), len(want))
				}
			}

			// Distinct, non-palindromic binary payloads catch truncation, text
			// conversion, stale copies and accidentally reversed transfers.
			names := make([]string, 10)
			payloads := make([][]byte, 10)
			for i := range names {
				names[i] = fmt.Sprintf("%02d space 雪-é.bin", i)
				if i != 0 {
					payloads[i] = make([]byte, 4096+i*137)
					for j := range payloads[i] {
						payloads[i][j] = byte(j*73 + j/251 + i*19)
					}
				}
				source := localFile(names[i])
				write(source, payloads[i])
				copyFile(source, remoteFile(names[i]), false)
				entry, err := client.Stat(ctx, remoteFile(names[i]))
				if err != nil || entry.IsDir || entry.Name != names[i] || entry.Size != int64(len(payloads[i])) {
					t.Fatalf("uploaded Stat: entry=%+v err=%v", entry, err)
				}
				download := localFile("download-" + names[i])
				copyFile(remoteFile(names[i]), download, false)
				checkBytes(download, payloads[i])
				checkBytes(source, payloads[i])
			}
			entries, err := client.List(ctx, remoteFile(""))
			if err != nil {
				t.Fatalf("List test prefix: %v", err)
			}
			if len(entries) != len(names) {
				t.Fatalf("List returned %d entries, want %d", len(entries), len(names))
			}
			seen := make(map[string]int64)
			for _, entry := range entries {
				if entry.IsDir {
					t.Fatalf("unexpected directory: %+v", entry)
				}
				seen[entry.Name] = entry.Size
			}
			for i, name := range names {
				if size, ok := seen[name]; !ok || size != int64(len(payloads[i])) {
					t.Errorf("List missing or incorrect metadata for %q", name)
				}
			}

			// Rename through the optional public contract used by the UI. Test
			// empty and binary files, Unicode/spaces and both kinds of collision.
			var renamer storage.Renamer = client
			for _, i := range []int{0, 4} {
				source := remoteFile(names[i])
				destination := remoteFile(fmt.Sprintf("renamed %d 雪.bin", i))
				if err := renamer.Rename(ctx, source, destination); err != nil {
					t.Fatalf("Rename: %v", err)
				}
				if _, err := client.Stat(ctx, source); !errors.Is(err, fs.ErrNotExist) {
					t.Fatalf("renamed source still exists: %v", err)
				}
				checkBytes(destination, payloads[i])
				if err := renamer.Rename(ctx, destination, source); err != nil {
					t.Fatalf("rename back: %v", err)
				}
			}
			for _, i := range []int{4, 5} {
				destination := remoteFile(fmt.Sprintf("rename collision %d.bin", i))
				copyFile(localFile(names[i]), destination, false)
				if err := renamer.Rename(ctx, remoteFile(names[4]), destination); !errors.Is(err, fs.ErrExist) {
					t.Fatalf("rename collision: %v", err)
				}
				checkBytes(remoteFile(names[4]), payloads[4])
				checkBytes(destination, payloads[i])
			}
			if err := renamer.Rename(ctx, remoteFile(""), storage.Location{Remote: remote.name, Path: prefix + "-renamed"}); err == nil {
				t.Fatal("directory rename succeeded")
			}
			canceledRename, stopRename := context.WithCancel(ctx)
			stopRename()
			if err := renamer.Rename(canceledRename, remoteFile(names[4]), remoteFile("canceled-rename")); !errors.Is(err, context.Canceled) {
				t.Fatalf("canceled rename: %v", err)
			}
			checkBytes(remoteFile(names[4]), payloads[4])

			// Object-store prefixes must behave as navigable directories, not
			// flatten keys or lose Unicode names on a nested round trip.
			nested := remoteFile("nested dir/deeper/雪 café.bin")
			copyFile(localFile(names[2]), nested, false)
			checkBytes(nested, payloads[2])
			nestedEntries, err := client.List(ctx, remoteFile("nested dir/deeper"))
			if err != nil || len(nestedEntries) != 1 || nestedEntries[0].Name != "雪 café.bin" || nestedEntries[0].Size != int64(len(payloads[2])) {
				t.Fatalf("nested List: entries=%+v err=%v", nestedEntries, err)
			}

			original := payloads[1]
			replacement := make([]byte, len(original))
			for i, b := range original {
				replacement[i] = b ^ 0xa7
			}
			newLocal := localFile("replacement.bin")
			write(newLocal, replacement)
			for _, direction := range []string{"upload", "download"} {
				src, dst := newLocal, remoteFile(names[1])
				if direction == "download" {
					// Match sizes while retaining different contents.
					copyFile(newLocal, remoteFile("replacement.bin"), false)
					src, dst = remoteFile("replacement.bin"), localFile("existing.bin")
					write(dst, original)
				}
				if err := client.Copy(ctx, src, dst, false); !errors.Is(err, fs.ErrExist) {
					t.Fatalf("existing destination: want fs.ErrExist, got %v", err)
				}
				checkBytes(dst, original)
				copyFile(src, dst, true)
				checkBytes(dst, replacement)
			}

			// A renamed destination must not turn a copy into a move.
			copyFile(localFile(names[3]), remoteFile("renamed 雪.bin"), false)
			checkBytes(remoteFile("renamed 雪.bin"), payloads[3])
			checkBytes(remoteFile(names[3]), payloads[3])
			checkBytes(localFile(names[3]), payloads[3])
			copyFile(remoteFile(names[3]), localFile("renamed download.bin"), false)
			checkBytes(localFile("renamed download.bin"), payloads[3])
			checkBytes(remoteFile(names[3]), payloads[3])

			for _, loc := range []storage.Location{localFile("missing.bin"), remoteFile("missing.bin")} {
				if _, err := client.Stat(ctx, loc); !errors.Is(err, fs.ErrNotExist) {
					t.Errorf("missing Stat(%+v): want fs.ErrNotExist, got %v", loc, err)
				}
			}
			for _, pair := range [][2]storage.Location{
				{{Path: localDir}, remoteFile("directory-upload")},
				{remoteFile(""), localFile("directory-download")},
				{localFile(names[3]), remoteFile("")},
				{remoteFile(names[3]), {Path: localDir}},
			} {
				for _, overwrite := range []bool{false, true} {
					if err := client.Copy(ctx, pair[0], pair[1], overwrite); err == nil {
						t.Errorf("directory Copy(%+v, %+v, %v) unexpectedly succeeded", pair[0], pair[1], overwrite)
					}
				}
			}
			canceled, stop := context.WithCancel(ctx)
			stop()
			if _, err := client.List(canceled, remoteFile("")); err == nil {
				t.Error("canceled List reported success")
			}
			if _, err := client.Stat(canceled, remoteFile(names[3])); err == nil {
				t.Error("canceled Stat reported success")
			}
			for _, pair := range [][2]storage.Location{
				{localFile(names[3]), remoteFile("canceled.bin")},
				{remoteFile(names[3]), localFile("canceled.bin")},
			} {
				if err := client.Copy(canceled, pair[0], pair[1], false); err == nil {
					t.Error("canceled Copy reported success")
				}
			}
			for _, loc := range []storage.Location{localFile(names[3]), remoteFile(names[3])} {
				if err := client.Delete(canceled, loc); !errors.Is(err, context.Canceled) {
					t.Fatalf("canceled Delete: %v", err)
				}
				checkBytes(loc, payloads[3])
				parent := loc
				parent.Path = path.Dir(parent.Path)
				if err := client.Delete(ctx, parent); err == nil {
					t.Fatal("Delete accepted a directory/prefix")
				}
				checkBytes(loc, payloads[3])
				if err := client.Delete(ctx, loc); err != nil {
					t.Fatalf("Delete(%v): %v", loc, err)
				}
				if _, err := client.Stat(ctx, loc); !errors.Is(err, fs.ErrNotExist) {
					t.Fatalf("deleted file still exists: %v", err)
				}
				if err := client.Delete(ctx, loc); !errors.Is(err, fs.ErrNotExist) {
					t.Fatalf("missing Delete: %v", err)
				}
			}
			checkBytes(localFile(names[4]), payloads[4])
			checkBytes(remoteFile(names[4]), payloads[4])
		})
	}
}
