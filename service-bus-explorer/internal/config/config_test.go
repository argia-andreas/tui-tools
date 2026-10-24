package config_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"servicebusexplorer/internal/config"
	"servicebusexplorer/internal/explorer"
)

func TestDefaultsAndRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "preferences.json")
	p, err := config.Load(path)
	if err != nil || !reflect.DeepEqual(p, config.Default()) {
		t.Fatalf("missing file: %+v, %v", p, err)
	}
	if p.Version != 1 || p.Theme != "midnight" || !p.Sidebar || !p.Inspector || p.Table {
		t.Fatalf("incorrect defaults: %+v", p)
	}
	p.Theme, p.Table, p.Sidebar, p.Inspector = "gruvbox", true, false, false
	p.Watchlists = map[string][]explorer.Resource{
		"fixture": {
			{Profile: "fixture", Namespace: "one", Queue: "orders"},
			{Profile: "fixture", Namespace: "two", Queue: "orders"},
			{Profile: "fixture", Namespace: "one", Topic: "events", Subscription: "billing"},
			{Profile: "fixture", Namespace: "one", Topic: "events"},
		},
		"emulator": {},
	}
	if err := config.Save(path, p); err != nil {
		t.Fatal(err)
	}
	got, err := config.Load(path)
	if err != nil || !reflect.DeepEqual(got, p) {
		t.Fatalf("roundtrip: %+v, %v", got, err)
	}
	if list, ok := got.Watchlists["emulator"]; !ok || list == nil || len(list) != 0 {
		t.Fatal("explicit empty list lost")
	}
	delete(p.Watchlists, "emulator")
	if err := config.Save(path, p); err != nil {
		t.Fatal(err)
	}
	got, err = config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got.Watchlists["emulator"]; ok {
		t.Fatal("absent profile became explicit empty list")
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("file permissions: %v, %v", info, err)
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil || len(entries) != 1 {
		t.Fatalf("temporary files left behind: %v, %v", entries, err)
	}
}

func TestRejectInvalidWithoutClobber(t *testing.T) {
	for name, raw := range map[string]string{
		"future":           `{"version":2,"theme":"midnight"}`,
		"missing version":  `{"theme":"midnight"}`,
		"theme":            `{"version":1,"theme":"unknown"}`,
		"corrupt":          `{`,
		"trailing":         `{"version":1,"theme":"midnight"} {}`,
		"credentials":      `{"version":1,"theme":"midnight","credentials":"secret"}`,
		"nested body":      `{"version":1,"theme":"midnight","watchlists":{"fixture":[{"Profile":"fixture","Namespace":"one","Queue":"orders","body":"secret"}]}}`,
		"profile mismatch": `{"version":1,"theme":"midnight","watchlists":{"fixture":[{"Profile":"emulator","Namespace":"one","Queue":"orders"}]}}`,
		"null list":        `{"version":1,"theme":"midnight","watchlists":{"fixture":null}}`,
		"invalid target":   `{"version":1,"theme":"midnight","watchlists":{"fixture":[{"Profile":"fixture","Namespace":"one","Queue":"orders","Topic":"events"}]}}`,
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "preferences.json")
			if err := os.WriteFile(path, []byte(raw), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := config.Load(path); err == nil {
				t.Fatal("invalid config accepted")
			}
			if err := config.Save(path, config.Default()); err == nil {
				t.Fatal("invalid existing config replaced")
			}
			got, err := os.ReadFile(path)
			if err != nil || string(got) != raw {
				t.Fatalf("existing file changed: %q, %v", got, err)
			}
		})
	}
	path := filepath.Join(t.TempDir(), "preferences.json")
	if err := config.Save(path, config.Default()); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(path)
	p := config.Default()
	p.Version = 2
	if err := config.Save(path, p); err == nil {
		t.Fatal("invalid new preferences accepted")
	}
	after, err := os.ReadFile(path)
	if err != nil || string(after) != string(before) {
		t.Fatal("invalid input clobbered valid file")
	}
}

func TestPath(t *testing.T) {
	dir, err := os.UserConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	got, err := config.Path()
	if err != nil || got != filepath.Join(dir, "service-bus-explorer", "preferences.json") {
		t.Fatalf("Path() = %q, %v", got, err)
	}
}
