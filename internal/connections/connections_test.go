package connections_test

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"lazyftp/internal/connections"
)

func TestBookmarkSaveReuseReplace(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app", "connections.json")
	first := connections.Bookmark{Name: "Incoming", Remote: "ftp-work", Root: "/incoming space/å", Local: t.TempDir()}
	second := connections.Bookmark{Name: "Assets", Remote: "azure", Root: "/container/assets", Local: t.TempDir()}
	for _, b := range []connections.Bookmark{first, second} {
		if err := connections.Save(path, b); err != nil {
			t.Fatal(err)
		}
	}
	first.Root = "/outgoing"
	if err := connections.Save(path, first); err != nil {
		t.Fatal(err)
	}
	got, err := connections.Load(path)
	if err != nil || !reflect.DeepEqual(got, []connections.Bookmark{first, second}) {
		t.Fatalf("reuse: %+v %v", got, err)
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0600 {
		t.Fatalf("permissions %v", info.Mode())
	}
	if err := os.WriteFile(path, []byte(`broken secret-looking data`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := connections.Save(path, first); err == nil || strings.Contains(err.Error(), "secret-looking") {
		t.Fatalf("unsafe malformed save: %v", err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "broken secret-looking data" {
		t.Fatal("corrupt bookmark file overwritten")
	}
}

func TestRemotePaths(t *testing.T) {
	for _, value := range []string{"ftp:/incoming", "azure:container/assets", "s3:bucket/a:b", "team space:/å file"} {
		loc, err := connections.ParseRemote(value)
		if err != nil || loc.Remote == "" || !strings.HasPrefix(loc.Path, "/") {
			t.Fatalf("%q: %+v %v", value, loc, err)
		}
	}
	for _, value := range []string{"", ":ftp,host=server:/", "local/path", "bad/name:/", "-flag:/", "remote:\x1b[31m"} {
		if _, err := connections.ParseRemote(value); err == nil {
			t.Fatalf("accepted %q", value)
		}
	}
}

func TestDiscoverNamesOnlyAndConfigPrecedence(t *testing.T) {
	// A fake executable exercises the public process contract without a network,
	// personal config, credentials or real rclone installation.
	dir := t.TempDir()
	script := "#!/bin/sh\n[ \"$1\" = listremotes ] || exit 3\n[ \"$2\" = --config ] && [ \"$3\" = '/explicit config' ] || exit 4\nprintf 'z-s3:\\na-ftp:\\nm-blob:\\n'\n"
	if err := os.WriteFile(filepath.Join(dir, "rclone"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	t.Setenv("RCLONE_CONFIG", "/environment config")
	got, err := connections.Discover(context.Background(), "/explicit config")
	if err != nil || !reflect.DeepEqual(got, []string{"a-ftp", "m-blob", "z-s3"}) {
		t.Fatalf("names: %v %v", got, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "rclone"), []byte("#!/bin/sh\necho 'credential-bearing URL' >&2\nexit 1\n"), 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := connections.Discover(context.Background(), ""); err == nil || strings.Contains(err.Error(), "credential-bearing") {
		t.Fatalf("stderr leaked: %v", err)
	}
}
