package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"streams/internal/config"
)

func TestExplicitSettingsSave(t *testing.T) {
	p := filepath.Join(t.TempDir(), "settings.json")
	s, err := config.LoadSettings(p)
	if err != nil || !s.Sidebar || !s.Inspector || s.Comfortable {
		t.Fatal(s, err)
	}
	if _, err = os.Stat(p); !os.IsNotExist(err) {
		t.Fatal("load wrote defaults")
	}
	s.Sidebar = false
	s.ASCII = true
	s.Comfortable = true
	if err = config.SaveSettings(p, s); err != nil {
		t.Fatal(err)
	}
	got, err := config.LoadSettings(p)
	if err != nil || got != s {
		t.Fatal(got, err)
	}
	info, _ := os.Stat(p)
	if info.Mode().Perm() != 0600 {
		t.Fatal("public settings")
	}
}
