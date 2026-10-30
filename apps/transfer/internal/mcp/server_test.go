package mcp_test

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"lazyftp/internal/mcp"
	"lazyftp/internal/storage"
)

type backend struct {
	calls int
	err   error
	files map[storage.Location][]byte
}

func (b *backend) List(context.Context, storage.Location) ([]storage.Entry, error) {
	b.calls++
	return []storage.Entry{{Name: "雪 space.bin", Size: 7}}, b.err
}
func (b *backend) Stat(context.Context, storage.Location) (storage.Entry, error) {
	b.calls++
	return storage.Entry{}, errors.New("unexpected stat")
}
func (b *backend) Copy(_ context.Context, src, dst storage.Location, overwrite bool) error {
	b.calls++
	if overwrite {
		panic("overwrite must never reach backend")
	}
	if b.err != nil {
		return b.err
	}
	if _, exists := b.files[dst]; exists {
		return fs.ErrExist
	}
	b.files[dst] = append([]byte(nil), b.files[src]...)
	return nil
}

func session(t *testing.T, p mcp.Policy, b mcp.Backend) *sdk.ClientSession {
	t.Helper()
	s, err := mcp.New(p, b)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)
	a, z := sdk.NewInMemoryTransports()
	ss, err := s.Connect(ctx, a, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ss.Close() })
	c, err := sdk.NewClient(&sdk.Implementation{Name: "test", Version: "1"}, nil).Connect(ctx, z, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	return c
}

func call(t *testing.T, s *sdk.ClientSession, name string, args any, allowed bool) string {
	t.Helper()
	r, err := s.CallTool(context.Background(), &sdk.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		if allowed {
			t.Fatal(err)
		}
		return err.Error()
	}
	if r.IsError == allowed {
		t.Fatalf("%s: isError=%v, want allowed=%v; %+v", name, r.IsError, allowed, r.Content)
	}
	data, _ := json.Marshal(r)
	return string(data)
}

func TestPublicPolicyBoundary(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "雪 space.bin")
	if err := os.WriteFile(file, []byte("source"), 0600); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(file, filepath.Join(root, "leaf")); err != nil {
		t.Fatal(err)
	}
	b := &backend{files: map[storage.Location][]byte{}}
	p := mcp.Policy{Grants: []mcp.Grant{
		{Operation: "list", Remote: "lab-ftp", Root: "/safe"},
		{Operation: "upload", Remote: "lab-ftp", Root: "/safe", LocalRoots: []string{root}},
		{Operation: "download", Remote: "lab-ftp", Root: "/safe", LocalRoots: []string{root}},
	}}
	s := session(t, p, b)
	// Mutating the caller's policy cannot broaden a running server's authority.
	p.Grants[0].Root = "/"
	p.Grants[1].LocalRoots[0] = "/"
	for _, remote := range []string{"", "other", ":local:", "lab-ftp:", "lab-ftp,pass=secret", "--config", "C", "lab-ftp\n"} {
		call(t, s, "list_files", mcp.ListInput{Remote: remote, Path: "/safe"}, false)
		for _, tool := range []string{"upload_file", "download_file"} {
			call(t, s, tool, mcp.TransferInput{Remote: remote, Path: "/safe/file", LocalPath: file}, false)
		}
	}
	for _, name := range []string{"/", "/safe-other", "/safe/../outside", "/safe/./file", "/safe//file", "/safe/", "safe/file", "/safe/with:colon", "/safe/a\\b", "/safe/\x00x"} {
		call(t, s, "list_files", mcp.ListInput{Remote: "lab-ftp", Path: name}, false)
		for _, tool := range []string{"upload_file", "download_file"} {
			call(t, s, tool, mcp.TransferInput{Remote: "lab-ftp", Path: name, LocalPath: file}, false)
		}
	}
	for _, name := range []string{root, root + "-other/file", root + "/../outside", root + "/escape/new", root + "/leaf", root + "/missing/child", "/etc/passwd", "relative"} {
		for _, tool := range []string{"upload_file", "download_file"} {
			call(t, s, tool, mcp.TransferInput{Remote: "lab-ftp", Path: "/safe/file", LocalPath: name}, false)
		}
	}
	call(t, s, "upload_file", mcp.TransferInput{Remote: "lab-ftp", Path: "/safe/file", LocalPath: file, Overwrite: true}, false)
	call(t, s, "download_file", mcp.TransferInput{Remote: "lab-ftp", Path: "/safe/file", LocalPath: file, Overwrite: true}, false)
	if b.calls != 0 {
		t.Fatalf("denied inputs accessed backend %d times", b.calls)
	}
	if got := call(t, s, "list_files", mcp.ListInput{Remote: "lab-ftp", Path: "/safe/nested"}, true); !strings.Contains(got, "雪 space.bin") {
		t.Fatalf("listing missing filename: %s", got)
	}
	local := storage.Location{Path: file}
	remote := storage.Location{Remote: "lab-ftp", Path: "/safe/雪 space.bin"}
	b.files[local] = []byte{0, 73, 255, 19, 5}
	call(t, s, "upload_file", mcp.TransferInput{Remote: remote.Remote, Path: remote.Path, LocalPath: file}, true)
	if string(b.files[remote]) != string([]byte{0, 73, 255, 19, 5}) {
		t.Fatal("upload did not copy local to remote")
	}
	download := filepath.Join(root, "new.bin")
	call(t, s, "download_file", mcp.TransferInput{Remote: remote.Remote, Path: remote.Path, LocalPath: download}, true)
	if string(b.files[storage.Location{Path: download}]) != string(b.files[remote]) {
		t.Fatal("download did not copy remote to local")
	}
	if got := call(t, s, "upload_file", mcp.TransferInput{Remote: remote.Remote, Path: remote.Path, LocalPath: file}, false); !strings.Contains(got, "destination exists") {
		t.Fatalf("overwrite refusal: %s", got)
	}
	b.err = errors.New("https://user:SECRET@example.test/private?token=SECRET")
	for _, tool := range []string{"list_files", "upload_file", "download_file"} {
		var args any = mcp.TransferInput{Remote: remote.Remote, Path: remote.Path, LocalPath: file}
		if tool == "list_files" {
			args = mcp.ListInput{Remote: remote.Remote, Path: remote.Path}
		}
		got := call(t, s, tool, args, false)
		if strings.Contains(got, "SECRET") || strings.Contains(got, "example.test") {
			t.Fatal("backend credentials escaped in tool result")
		}
	}
}

func TestDefaultDenyAndIndependentGrants(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "source")
	if err := os.WriteFile(file, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, operation := range []string{"", "list", "upload", "download"} {
		t.Run(operation, func(t *testing.T) {
			p := mcp.Policy{}
			if operation != "" {
				g := mcp.Grant{Operation: operation, Remote: "lab-ftp", Root: "/safe"}
				if operation != "list" {
					g.LocalRoots = []string{root}
				}
				p.Grants = []mcp.Grant{g}
			}
			b := &backend{files: map[storage.Location][]byte{}}
			s := session(t, p, b)
			for _, op := range []string{"list", "upload", "download"} {
				tool := op + "_file"
				var args any = mcp.TransferInput{Remote: "lab-ftp", Path: "/safe/file", LocalPath: file}
				if op == "list" {
					tool = "list_files"
					args = mcp.ListInput{Remote: "lab-ftp", Path: "/safe/file"}
				}
				call(t, s, tool, args, op == operation)
			}
			want := 1
			if operation == "" {
				want = 0
			}
			if b.calls != want {
				t.Fatalf("backend calls=%d, want %d", b.calls, want)
			}
		})
	}
}

func TestPolicyLoadingAndCredentialProtection(t *testing.T) {
	for _, raw := range []string{"", "{", `{"grant":[]}`, `{"grants":[{"operation":"delete","remote":"lab-ftp","root":"/"}]}`, `{"grants":[]} {}`, `{"grants":[{"operation":"upload","remote":"lab-ftp","root":"/safe"}]}`, `{}` + strings.Repeat(" ", 1<<20)} {
		if _, err := mcp.ReadPolicy(strings.NewReader(raw)); err == nil {
			t.Fatalf("accepted invalid policy (length %d)", len(raw))
		}
	}
	for _, raw := range []string{`{}`, `{"grants":[]}`, `{"grants":null}`} {
		p, err := mcp.ReadPolicy(strings.NewReader(raw))
		if err != nil || len(p.Grants) != 0 {
			t.Fatalf("default deny %q: %+v %v", raw, p, err)
		}
	}
	root := t.TempDir()
	config := filepath.Join(root, "credentials.conf")
	if err := os.WriteFile(config, []byte("not-a-real-secret"), 0600); err != nil {
		t.Fatal(err)
	}
	p := mcp.Policy{Grants: []mcp.Grant{{Operation: "upload", Remote: "lab-ftp", Root: "/safe", LocalRoots: []string{root}}}}
	if p.ProtectConfig(config) == nil {
		t.Fatal("credential file under granted root accepted")
	}
	if (mcp.Policy{}).ProtectConfig(config) != nil {
		t.Fatal("separate config rejected")
	}
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(root, link); err != nil {
		t.Fatal(err)
	}
	p.Grants[0].LocalRoots = []string{link}
	if _, err := mcp.New(p, &backend{}); err == nil {
		t.Fatal("symlink grant root accepted")
	}
}

func TestGrantsCannotBeCombined(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	file := filepath.Join(b, "source")
	if err := os.WriteFile(file, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	backend := &backend{files: map[storage.Location][]byte{}}
	s := session(t, mcp.Policy{Grants: []mcp.Grant{
		{Operation: "upload", Remote: "lab-ftp", Root: "/alpha", LocalRoots: []string{a}},
		{Operation: "upload", Remote: "lab-ftp", Root: "/beta", LocalRoots: []string{b}},
	}}, backend)
	call(t, s, "upload_file", mcp.TransferInput{Remote: "lab-ftp", Path: "/alpha/file", LocalPath: file}, false)
	if backend.calls != 0 {
		t.Fatal("combined parts of different grants")
	}
	call(t, s, "upload_file", mcp.TransferInput{Remote: "lab-ftp", Path: "/beta/file", LocalPath: file}, true)
}
