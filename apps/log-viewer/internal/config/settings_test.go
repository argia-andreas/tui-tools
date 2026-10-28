package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"logviewer/internal/config"
)

func TestSettingsDefaultsAndRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "private", "settings.json")
	if !config.DefaultSettings().VimMode {
		t.Fatal("Vim mode must default on")
	}
	if settings, err := config.LoadSettings(path); err != nil || !settings.VimMode {
		t.Fatalf("missing settings: %+v, %v", settings, err)
	}
	if _, err := os.Stat(filepath.Dir(path)); !os.IsNotExist(err) {
		t.Fatal("load created configuration")
	}
	for _, enabled := range []bool{false, true, false} {
		want := config.Settings{VimMode: enabled}
		if err := config.SaveSettings(path, want); err != nil {
			t.Fatal(err)
		}
		if got, err := config.LoadSettings(path); err != nil || got != want {
			t.Fatalf("round trip: %+v, %v", got, err)
		}
	}
	for p, mode := range map[string]os.FileMode{path: 0600, filepath.Dir(path): 0700} {
		info, err := os.Stat(p)
		if err != nil || info.Mode().Perm() != mode {
			t.Fatalf("incorrect private permissions for %s: %v", p, err)
		}
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil || len(entries) != 1 {
		t.Fatalf("temporary settings left behind: %v", err)
	}
}

func TestSettingsStrictJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	for _, input := range []string{
		`{}`, `{"vim_mode": true}`, `{"vim_mode": false}`,
		``, `{`, `null`, `[]`, `true`, `{"vim_mode":null}`,
		`{"vim_mode":"false"}`, `{"vim_mode":0}`, `{"other":true}`,
		`{"VIM_MODE":true}`, `{"vim_mode":true,"vim_mode":false}`,
		`{"vim_mode":true,"vim_\u006dode":false}`,
		`{} {}`, `{} trailing`, `{"vim_mode":false,}`, strings.Repeat(" ", 65537),
	} {
		t.Run(input[:min(len(input), 60)], func(t *testing.T) {
			if err := os.WriteFile(path, []byte(input), 0600); err != nil {
				t.Fatal(err)
			}
			got, err := config.LoadSettings(path)
			valid := input == `{}` || input == `{"vim_mode": true}` || input == `{"vim_mode": false}`
			if (err == nil) != valid {
				t.Fatalf("strict JSON acceptance: %v", err)
			}
			if valid && got.VimMode != (input != `{"vim_mode": false}`) {
				t.Fatalf("unexpected defaults: %+v", got)
			}
		})
	}
}

func TestSettingsUnsafeDestinations(t *testing.T) {
	for _, kind := range []string{"public-file", "directory", "symlink", "dangling-symlink", "public-directory", "symlink-directory", "unowned-file", "unowned-directory"} {
		t.Run(kind, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "settings.json")
			if err := os.WriteFile(path, []byte(`{}`), 0600); err != nil {
				t.Fatal(err)
			}
			var err error
			switch kind {
			case "public-file":
				err = os.Chmod(path, 0644)
			case "directory":
				err = os.Remove(path)
				if err == nil {
					err = os.Mkdir(path, 0700)
				}
			case "symlink", "dangling-symlink":
				target := path
				if kind == "dangling-symlink" {
					target += ".missing"
				}
				path += ".link"
				err = os.Symlink(target, path)
			case "public-directory":
				err = os.Chmod(dir, 0755)
			case "symlink-directory":
				link := filepath.Join(t.TempDir(), "link")
				err = os.Symlink(dir, link)
				path = filepath.Join(link, "settings.json")
			case "unowned-file", "unowned-directory":
				target := path
				if kind == "unowned-directory" {
					target = dir
				}
				err = os.Chown(target, os.Getuid()+1, -1)
				if os.IsPermission(err) {
					t.Skip("changing ownership requires privilege")
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			if kind != "public-directory" && kind != "symlink-directory" && kind != "unowned-directory" {
				if _, err := config.LoadSettings(path); err == nil {
					t.Fatal("unsafe settings loaded")
				}
			}
			if err := config.SaveSettings(path, config.DefaultSettings()); err == nil {
				t.Fatal("unsafe destination saved")
			}
		})
	}
}
