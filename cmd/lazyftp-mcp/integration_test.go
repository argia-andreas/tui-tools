//go:build integration

package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"testing"
	"time"
)

func TestStdioRcloneLab(t *testing.T) {
	binary := buildServer(t)
	config, err := filepath.Abs("../../.lab/rclone.conf")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(config); err != nil {
		t.Fatal("local lab required: python3 scripts/lab.py up; python3 scripts/lab.py seed")
	}
	for _, remote := range []struct{ name, root string }{{"lab-ftp", "/incoming"}, {"lab-blob", "/files/assets"}, {"lab-s3", "/files/assets"}} {
		t.Run(remote.name, func(t *testing.T) {
			var nonce [12]byte
			if _, err := rand.Read(nonce[:]); err != nil {
				t.Fatal(err)
			}
			prefix := path.Join(remote.root, "lazyftp-mcp-test-"+hex.EncodeToString(nonce[:]))
			// Only this cryptographically unique test prefix is removed, never
			// seeded/shared roots. Cleanup is not an MCP capability.
			t.Cleanup(func() {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				if err := exec.CommandContext(ctx, "rclone", "purge", "--config", config, "--", remote.name+":"+prefix).Run(); err != nil {
					t.Errorf("unique test prefix cleanup: %v", err)
				}
			})
			local := t.TempDir()
			policy := filepath.Join(t.TempDir(), "policy.json")
			grants := []map[string]any{}
			for _, op := range []string{"list", "upload", "download"} {
				g := map[string]any{"operation": op, "remote": remote.name, "root": prefix}
				if op != "list" {
					g["local_roots"] = []string{local}
				}
				grants = append(grants, g)
			}
			data, err := json.Marshal(map[string]any{"grants": grants})
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(policy, data, 0600); err != nil {
				t.Fatal(err)
			}
			c := startServer(t, binary, "--policy", policy, "--rclone-config", config)
			c.tool("list_files", map[string]any{"remote": remote.name, "path": remote.root}, false)
			c.tool("list_files", map[string]any{"remote": remote.name, "path": prefix + "-other"}, false)
			for _, empty := range []bool{false, true} {
				name := "雪 space.bin"
				payload := make([]byte, 4233)
				for i := range payload {
					payload[i] = byte(i*73 + i/251 + 19)
				}
				if empty {
					name, payload = "empty.bin", []byte{}
				}
				source := filepath.Join(local, name)
				if err := os.WriteFile(source, payload, 0600); err != nil {
					t.Fatal(err)
				}
				args := map[string]any{"remote": remote.name, "path": path.Join(prefix, name), "local_path": source}
				c.tool("upload_file", args, true)
				c.tool("upload_file", args, false)
				listing := c.tool("list_files", map[string]any{"remote": remote.name, "path": prefix}, true)
				if !bytes.Contains(listing, []byte(name)) {
					t.Fatalf("missing uploaded filename: %s", listing)
				}
				destination := filepath.Join(local, "download-"+name)
				args["local_path"] = destination
				c.tool("download_file", args, true)
				c.tool("download_file", args, false)
				args["overwrite"] = true
				c.tool("download_file", args, false)
				got, err := os.ReadFile(destination)
				if err != nil || !bytes.Equal(got, payload) {
					t.Fatalf("download bytes: len=%d want=%d err=%v", len(got), len(payload), err)
				}
				original, err := os.ReadFile(source)
				if err != nil || !bytes.Equal(original, payload) {
					t.Fatal("source altered")
				}
			}
			c.request("ping", map[string]any{}, new(any))
		})
	}
}
