//go:build integration

package ui_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"lazyftp/internal/storage"
	"lazyftp/internal/theme"
	"lazyftp/internal/ui"
)

func TestLiveFolderRoundTrips(t *testing.T) {
	config, err := filepath.Abs("../../.lab/rclone.conf")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(config); err != nil {
		t.Fatal("run make lab-up lab-seed first:", err)
	}
	for _, root := range []storage.Location{{Remote: "lab-ftp", Path: "/incoming"}, {Remote: "lab-blob", Path: "/files/assets"}, {Remote: "lab-s3", Path: "/files/assets"}} {
		t.Run(root.Remote, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
			defer cancel()
			backend := storage.NewRclone(config)
			if _, err := backend.List(ctx, root); err != nil {
				t.Fatal(err)
			}
			name := fmt.Sprintf("lazyftp-ui-%d", time.Now().UnixNano())
			upload, download := t.TempDir(), t.TempDir()
			bodies := map[string]string{"a.txt": "asymmetric original\n", "nested dir/雪.txt": "different bytes 日本語\n", "nested dir/zero.bin": ""}
			for rel, body := range bodies {
				file := filepath.Join(upload, name, rel)
				if err := os.MkdirAll(filepath.Dir(file), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(file, []byte(body), 0600); err != nil {
					t.Fatal(err)
				}
			}
			t.Cleanup(func() {
				cleanup, stop := context.WithTimeout(context.Background(), 20*time.Second)
				defer stop()
				if err := exec.CommandContext(cleanup, "rclone", "purge", "--config", config, "--", root.String()+"/"+name).Run(); err != nil {
					t.Errorf("owned remote cleanup: %v", err)
				}
			})
			th, _ := theme.Get("")
			m := ui.New(ui.Options{Browser: backend, Transferer: backend, Context: ctx, Left: storage.Location{Path: upload}, Right: root, Theme: th})
			d := &driver{model: m}
			d.deliver(tea.WindowSizeMsg{Width: 160, Height: 40})
			d.run(m.Init())
			d.key("c")
			requireText(t, d, "3 captured file jobs")
			d.key("tab")
			d.key("enter")
			requireText(t, d, "3 completed")
			// Download the uploaded folder into a different, initially empty root.
			m = ui.New(ui.Options{Browser: backend, Transferer: backend, Context: ctx, Left: storage.Location{Path: download}, Right: storage.Location{Remote: root.Remote, Path: root.Path + "/" + name}, Theme: th})
			d = &driver{model: m}
			d.deliver(tea.WindowSizeMsg{Width: 160, Height: 40})
			d.run(m.Init())
			d.key("tab")
			d.key(" ")
			d.key("j")
			d.key(" ")
			d.key("c")
			requireText(t, d, "3 captured file jobs")
			d.key("tab")
			d.key("enter")
			requireText(t, d, "3 completed")
			for rel, want := range bodies {
				got, err := os.ReadFile(filepath.Join(download, rel))
				if err != nil {
					t.Fatal(err)
				}
				if string(got) != want {
					t.Fatalf("wrong bytes for %s", rel)
				}
			}
		})
	}
}
