package connections_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"lazyftp/internal/connections"
)

func setupWrite(t *testing.T, path, data string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
}

func setupRead(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func setupCandidate(t *testing.T, path, name string) *connections.Candidate {
	t.Helper()
	c, err := connections.Prepare(context.Background(), path, connections.Draft{Name: name, Backend: "s3", Values: map[string]string{"provider": "AWS", "secret_access_key": "private-" + name}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := c.Close(); err != nil {
			t.Error(err)
		}
	})
	return c
}

// The fixture is an executable, not a shell invoked by the production API.
func setupFake(t *testing.T, body string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "rclone"), []byte("#!/bin/sh\n"+body), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestSetupPrepareSaveAndClose(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config")
	original := "# original bytes\r\n[old]\r\ntype = local\r\n# last comment"
	setupWrite(t, path, original)
	c := setupCandidate(t, path, "new")
	if got := setupRead(t, path); got != original {
		t.Fatal("Prepare persisted the draft")
	}
	if got := setupRead(t, c.ConfigPath); !strings.HasPrefix(got, original+"\n") || !strings.Contains(got, "[new]\ntype = s3\n") {
		t.Fatal("session does not preserve original bytes and append draft")
	}
	if info, err := os.Stat(c.ConfigPath); err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("session permissions: %v %v", info, err)
	}
	concurrent := original + "\n[external]\ntype = local\n"
	setupWrite(t, path, concurrent)
	if err := c.Save(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := setupRead(t, path); !strings.HasPrefix(got, concurrent) || strings.Count(got, "[new]") != 1 {
		t.Fatal("Save erased current disk bytes or duplicated remote")
	}
	if info, _ := os.Stat(path); info.Mode().Perm() != 0600 {
		t.Fatal("saved config permissions are not private")
	}
	before := setupRead(t, path)
	if err := c.Save(context.Background()); err == nil {
		t.Fatal("second Save must reject existing remote")
	}
	if setupRead(t, path) != before {
		t.Fatal("duplicate save changed config")
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(c.ConfigPath); !os.IsNotExist(err) {
		t.Fatal("Close did not remove session config")
	}
	if err := c.Save(context.Background()); err == nil {
		t.Fatal("Save accepted closed session")
	}
	if err := c.Test(context.Background(), ""); err == nil {
		t.Fatal("Test accepted closed session")
	}
}

func TestSetupSourceSnapshotNeverPersisted(t *testing.T) {
	path := filepath.Join(t.TempDir(), "new-directory", "config")
	first := setupCandidate(t, path, "session_only")
	second, err := connections.Prepare(context.Background(), path, connections.Draft{Name: "remembered", Backend: "azureblob", SourceConfig: first.ConfigPath, Values: map[string]string{"account": "account", "key": "second-secret"}})
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	if !strings.Contains(setupRead(t, second.ConfigPath), "[session_only]") {
		t.Fatal("source session was lost")
	}
	if err := second.Save(context.Background()); err != nil {
		t.Fatal(err)
	}
	got := setupRead(t, path)
	if strings.Contains(got, "session_only") || !strings.Contains(got, "[remembered]") {
		t.Fatal("saving second draft persisted first session-only remote")
	}
}

func TestSetupRejectUnsafeDrafts(t *testing.T) {
	for _, draft := range []connections.Draft{
		{Name: "bad\n[evil]", Backend: "s3"},
		{Name: "C:", Backend: "s3"},
		{Name: "-flag", Backend: "s3"},
		{Name: "good", Backend: "local"},
		{Name: "good", Backend: "s3", Values: map[string]string{"type": "local"}},
		{Name: "good", Backend: "s3", Values: map[string]string{"region\nkey": "secret"}},
		{Name: "good", Backend: "s3", Values: map[string]string{"secret_access_key": "secret\n[evil]"}},
		{Name: "good", Backend: "ftp", Values: map[string]string{"host": "host", "port": "65536"}},
		{Name: "good", Backend: "ftp", Values: map[string]string{"host": "host", "tls": "true", "explicit_tls": "true"}},
		{Name: "good", Backend: "ftp", Values: map[string]string{"host": "host", "tls": "secret-invalid"}},
	} {
		if c, err := connections.Prepare(context.Background(), filepath.Join(t.TempDir(), "config"), draft); err == nil {
			c.Close()
			t.Fatal("unsafe draft accepted")
		} else if strings.Contains(err.Error(), "secret") {
			t.Fatal("validation error echoed input")
		}
	}
}

func TestSetupRejectExistingEncryptedAndSymlink(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config")
	for _, data := range []string{"[NEW] # existing\ntype = local\n", "RCLONE_ENCRYPT_V0:\nsecretciphertext"} {
		setupWrite(t, path, data)
		if c, err := connections.Prepare(context.Background(), path, connections.Draft{Name: "new", Backend: "s3"}); err == nil {
			c.Close()
			t.Fatal("existing/encrypted config accepted")
		} else if strings.Contains(err.Error(), "secretciphertext") {
			t.Fatal("encrypted config leaked")
		}
		if setupRead(t, path) != data {
			t.Fatal("rejected config was changed")
		}
	}
	link := path + "-link"
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	if c, err := connections.Prepare(context.Background(), link, connections.Draft{Name: "new", Backend: "s3"}); err == nil {
		c.Close()
		t.Fatal("symlink accepted")
	}
	setupWrite(t, path, "[old]\ntype = local\n")
	c := setupCandidate(t, path, "new")
	setupWrite(t, path, "[new]\ntype = local\n")
	if err := c.Save(context.Background()); err == nil {
		t.Fatal("concurrent duplicate replaced")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(link, path); err != nil {
		t.Fatal(err)
	}
	if err := c.Save(context.Background()); err == nil {
		t.Fatal("Save accepted symlink introduced after Prepare")
	}
}

func TestSetupConcurrentSaves(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config")
	first := setupCandidate(t, path, "one")
	second := setupCandidate(t, path, "two")
	var wg sync.WaitGroup
	for _, c := range []*connections.Candidate{first, second} {
		wg.Add(1)
		go func(c *connections.Candidate) {
			defer wg.Done()
			if err := c.Save(context.Background()); err != nil {
				t.Error(err)
			}
		}(c)
	}
	wg.Wait()
	got := setupRead(t, path)
	if strings.Count(got, "[one]") != 1 || strings.Count(got, "[two]") != 1 {
		t.Fatal("concurrent Save erased a remote")
	}
}

func TestSetupProcessSecretsAndDefaultPath(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SETUP_TARGET", filepath.Join(dir, "config"))
	t.Setenv("SETUP_ARGV", filepath.Join(dir, "argv"))
	t.Setenv("SETUP_STDIN", filepath.Join(dir, "stdin"))
	setupFake(t, `printf '%s\n' "$@" >> "$SETUP_ARGV"
case "$1" in
config) [ "$2" = file ] || exit 9; printf 'Config file:\n%s\n' "$SETUP_TARGET";;
obscure) [ "$2" = - ] || exit 9; cat > "$SETUP_STDIN"; printf 'obscured-value\n';;
lsf) printf 'secret-backend-diagnostic\n' >&2; exit 1;;
*) exit 9;;
esac
`)
	c, err := connections.Prepare(context.Background(), "", connections.Draft{Name: "ftp", Backend: "ftp", Values: map[string]string{"host": "host", "pass": "unique-plaintext-password"}})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if setupRead(t, os.Getenv("SETUP_STDIN")) != "unique-plaintext-password\n" {
		t.Fatal("FTP password not sent on stdin")
	}
	if strings.Contains(setupRead(t, c.ConfigPath), "unique-plaintext-password") || !strings.Contains(setupRead(t, c.ConfigPath), "pass = obscured-value") {
		t.Fatal("FTP password was not obscured")
	}
	if err := c.Test(context.Background(), "root"); err == nil || strings.Contains(err.Error(), "secret-backend") {
		t.Fatal("Test returned unsafe diagnostics or ignored failure")
	}
	args := setupRead(t, os.Getenv("SETUP_ARGV"))
	if strings.Contains(args, "unique-plaintext") || strings.Contains(args, "config\ndump") || !strings.Contains(args, "lsf\n--config\n") || !strings.Contains(args, "--max-depth\n1\n--files-only") {
		t.Fatal("unsafe process contract")
	}
	if err := c.Save(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(setupRead(t, os.Getenv("SETUP_TARGET")), "[ftp]") {
		t.Fatal("default config target not resolved")
	}
}

func TestSetupCloudSecretsNeverReachArgsOrErrors(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SETUP_ARGV", filepath.Join(dir, "argv"))
	setupFake(t, `printf '%s\n' "$@" >> "$SETUP_ARGV"
printf 'unique-cloud-secret https://account/?sig=unique-sas-secret\n' >&2
exit 1
`)
	for _, draft := range []connections.Draft{
		{Name: "blob", Backend: "azureblob", Values: map[string]string{"account": "account", "key": "unique-cloud-secret", "sas_url": "https://account/?sig=unique-sas-secret"}},
		{Name: "s3", Backend: "s3", Values: map[string]string{"provider": "AWS", "access_key_id": "unique-access-id", "secret_access_key": "unique-cloud-secret"}},
	} {
		c, err := connections.Prepare(context.Background(), filepath.Join(dir, "config"), draft)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { c.Close() })
		if err := c.Test(context.Background(), ""); err == nil || strings.Contains(err.Error(), "unique-") {
			t.Fatal("cloud Test leaked backend diagnostics or ignored failure")
		}
	}
	if strings.Contains(setupRead(t, os.Getenv("SETUP_ARGV")), "unique-") {
		t.Fatal("cloud credentials appeared in argv")
	}
}

func TestSetupListingBoundAndCancellation(t *testing.T) {
	c := setupCandidate(t, filepath.Join(t.TempDir(), "config"), "new")
	setupFake(t, `case "$1" in lsf) head -c 131072 /dev/zero;; *) exit 9;; esac`)
	if err := c.Test(context.Background(), ""); err == nil {
		t.Fatal("oversize listing accepted")
	}
	setupFake(t, `exec sleep 60`)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	if err := c.Test(ctx, ""); err == nil || time.Since(start) > 2*time.Second {
		t.Fatal("Test cancellation did not stop process promptly")
	}
	ctx, cancelSave := context.WithCancel(context.Background())
	cancelSave()
	if err := c.Save(ctx); err == nil {
		t.Fatal("cancelled Save succeeded")
	}
}

func TestSetupRealRcloneLocalReadOnly(t *testing.T) {
	if _, err := exec.LookPath("rclone"); err != nil {
		t.Skip("rclone not installed")
	}
	path := filepath.Join(t.TempDir(), "config")
	setupWrite(t, path, "[existing]\ntype = local\n")
	// Prepare only offers approved providers. Use the retained local remote to
	// prove that the generated config works with real rclone, then exercise
	// Candidate.Test against an environment-configured local test remote.
	c := setupCandidate(t, path, "probe")
	root := t.TempDir()
	setupWrite(t, filepath.Join(root, "retained.txt"), "unchanged")
	cmd := exec.Command("rclone", "lsf", "--config", c.ConfigPath, "existing:"+root)
	if output, err := cmd.Output(); err != nil || !strings.Contains(string(output), "retained.txt") {
		t.Fatalf("existing remote unavailable in session: %v", err)
	}
	t.Setenv("RCLONE_CONFIG_PROBE_TYPE", "local")
	if err := c.Test(context.Background(), root); err != nil {
		t.Fatal(err)
	}
	if setupRead(t, filepath.Join(root, "retained.txt")) != "unchanged" {
		t.Fatal("read-only test changed data")
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 1 {
		t.Fatal("read-only test created data")
	}
}
