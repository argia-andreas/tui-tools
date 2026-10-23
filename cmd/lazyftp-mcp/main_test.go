package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func buildServer(t *testing.T) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "lazyftp-mcp")
	if out, err := exec.Command("go", "build", "-o", binary, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v: %s", err, out)
	}
	return binary
}

type stdioClient struct {
	t      *testing.T
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	write  *json.Encoder
	read   *json.Decoder
	stderr bytes.Buffer
	id     int
}

func startServer(t *testing.T, binary string, args ...string) *stdioClient {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	t.Cleanup(cancel)
	c := &stdioClient{t: t, cmd: exec.CommandContext(ctx, binary, args...)}
	var err error
	c.stdin, err = c.cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := c.cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	c.cmd.Stderr = &c.stderr
	c.write, c.read = json.NewEncoder(c.stdin), json.NewDecoder(stdout)
	if err := c.cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		c.stdin.Close()
		if err := c.cmd.Wait(); err != nil {
			t.Errorf("stdio EOF lifecycle: %v; stderr=%s", err, c.stderr.String())
		}
		if c.stderr.Len() != 0 {
			t.Errorf("unexpected stderr: %s", c.stderr.String())
		}
	})
	var initialized struct {
		ProtocolVersion string `json:"protocolVersion"`
		ServerInfo      struct {
			Name string `json:"name"`
		} `json:"serverInfo"`
	}
	c.request("initialize", map[string]any{"protocolVersion": "2025-11-25", "capabilities": map[string]any{}, "clientInfo": map[string]string{"name": "raw-stdio-test", "version": "1"}}, &initialized)
	if initialized.ProtocolVersion != "2025-11-25" || initialized.ServerInfo.Name != "lazyftp" {
		t.Fatalf("initialize result: %+v", initialized)
	}
	if err := c.write.Encode(map[string]any{"jsonrpc": "2.0", "method": "notifications/initialized"}); err != nil {
		t.Fatal(err)
	}
	c.request("ping", map[string]any{}, new(any))
	return c
}

func (c *stdioClient) request(method string, params, result any) {
	c.t.Helper()
	c.id++
	if err := c.write.Encode(map[string]any{"jsonrpc": "2.0", "id": c.id, "method": method, "params": params}); err != nil {
		c.t.Fatal(err)
	}
	for {
		var response struct {
			ID     int             `json:"id"`
			Result json.RawMessage `json:"result"`
			Error  json.RawMessage `json:"error"`
		}
		if err := c.read.Decode(&response); err != nil {
			c.t.Fatalf("decode %s: %v", method, err)
		}
		if response.ID == 0 {
			continue
		}
		if response.ID != c.id || len(response.Error) != 0 {
			c.t.Fatalf("RPC %s: id=%d error=%s", method, response.ID, response.Error)
		}
		if err := json.Unmarshal(response.Result, result); err != nil {
			c.t.Fatal(err)
		}
		return
	}
}

func (c *stdioClient) tool(name string, args any, allowed bool) json.RawMessage {
	c.t.Helper()
	var result struct {
		IsError bool            `json:"isError"`
		Data    json.RawMessage `json:"structuredContent"`
	}
	c.request("tools/call", map[string]any{"name": name, "arguments": args}, &result)
	if result.IsError == allowed {
		c.t.Fatalf("tool %s: isError=%v, want allowed=%v", name, result.IsError, allowed)
	}
	return result.Data
}

func TestStdioLifecycleDefaultDeny(t *testing.T) {
	binary := buildServer(t)
	policy := filepath.Join(t.TempDir(), "deny.json")
	if err := os.WriteFile(policy, []byte(`{"grants":[]}`), 0600); err != nil {
		t.Fatal(err)
	}
	for _, explicit := range []bool{false, true} {
		t.Run(map[bool]string{false: "absent", true: "empty"}[explicit], func(t *testing.T) {
			var args []string
			if explicit {
				args = []string{"--policy", policy}
			}
			c := startServer(t, binary, args...)
			var tools struct {
				Tools []struct {
					Name string `json:"name"`
				} `json:"tools"`
			}
			c.request("tools/list", map[string]any{}, &tools)
			seen := map[string]bool{}
			for _, tool := range tools.Tools {
				seen[tool.Name] = true
			}
			if len(seen) != 3 || !seen["list_files"] || !seen["upload_file"] || !seen["download_file"] {
				t.Fatalf("unexpected tool surface: %+v", seen)
			}
			c.tool("list_files", map[string]any{"remote": "lab-ftp", "path": "/incoming"}, false)
			for _, name := range []string{"upload_file", "download_file"} {
				c.tool(name, map[string]any{"remote": "lab-ftp", "path": "/incoming/file", "local_path": "/tmp/file"}, false)
			}
			c.tool("list_files", map[string]any{"remote": ":local:", "path": "/incoming/../secret"}, false)
			c.request("ping", map[string]any{}, new(any))
		})
	}
}

func TestInvalidStartupFailsClosed(t *testing.T) {
	binary := buildServer(t)
	for _, policy := range []string{`{"grants":[{"operation":"list","remote":"lab-ftp","root":"/incoming"}]}`, `{"grants":[{"operation":"delete","remote":"lab-ftp","root":"/"}]}`, `{"unknown":true}`, `{`} {
		name := filepath.Join(t.TempDir(), "policy.json")
		if err := os.WriteFile(name, []byte(policy), 0600); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(binary, "--policy", name)
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		if err := cmd.Run(); err == nil || stdout.Len() != 0 || stderr.Len() == 0 {
			t.Fatalf("invalid startup: err=%v stdout=%s stderr=%s", err, stdout.String(), stderr.String())
		}
	}
}
