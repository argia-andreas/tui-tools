package theme_test

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"lazyftp/internal/theme"
)

func configPath(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "theme.json")
	if err := os.WriteFile(p, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestPresets(t *testing.T) {
	names := theme.Names()
	wantNames := []string{"midnight", "catppuccin-mocha", "tokyo-night", "gruvbox-dark", "dracula"}
	if !reflect.DeepEqual(names, wantNames) {
		t.Fatalf("got presets %v", names)
	}
	valid := regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
	for _, name := range names {
		preset, err := theme.Get(name)
		if err != nil || preset.Name != name {
			t.Fatalf("%s: %+v %v", name, preset, err)
		}
		v := reflect.ValueOf(preset)
		for i := 1; i < v.NumField(); i++ {
			if !valid.MatchString(v.Field(i).String()) {
				t.Errorf("%s: invalid %s", name, v.Type().Field(i).Name)
			}
		}
	}
	names[0] = "changed"
	if theme.Names()[0] != "midnight" {
		t.Fatal("Names aliases shared data")
	}
	if _, err := theme.Get("unknown"); err == nil {
		t.Fatal("unknown preset accepted")
	}
	want, _ := theme.Get("midnight")
	if got, err := theme.Get(""); err != nil || got != want {
		t.Fatalf("default preset: %+v %v", got, err)
	}
	got, err := theme.Load("", "")
	if err != nil || got != want {
		t.Fatalf("default: %+v %v", got, err)
	}
}

func TestConfigPrecedence(t *testing.T) {
	p := configPath(t, `{"theme":"dracula","colors":{"focus":"#AbC123","selection_text":"#123456"},"icons":false}`)
	got, err := theme.Load(p, "tokyo-night")
	want, _ := theme.Get("tokyo-night")
	want.Focus, want.SelectionText = "#AbC123", "#123456"
	if err != nil || got != want {
		t.Fatalf("explicit preset/overrides: %+v %v", got, err)
	}
	got, err = theme.Load(p, "")
	if err != nil || got.Name != "dracula" || got.Focus != "#AbC123" {
		t.Fatalf("config preset: %+v %v", got, err)
	}
	got, _ = theme.Get("tokyo-night")
	if got.Focus == "#AbC123" {
		t.Fatal("Load mutated preset")
	}
	if _, err := theme.Load(configPath(t, `{"icons":true}`), ""); err != nil {
		t.Fatal(err)
	}
	got, err = theme.Load(configPath(t, `{"colors":{"selection":"#AbC123"}}`), "")
	want, _ = theme.Get("midnight")
	want.Selection = "#AbC123"
	if err != nil || got != want {
		t.Fatalf("default preset/overrides: %+v %v", got, err)
	}
}

func TestConfigRejectsInvalidInput(t *testing.T) {
	for _, body := range []string{
		``, `null`, `[]`, `{`, `{} {}`, `{} trailing`,
		`{"unknown":true}`, `{"theme":"unknown"}`, `{"theme":2}`,
		`{"colors":{"unknown":"#123456"}}`, `{"colors":{"selectionText":"#123456"}}`,
		`{"colors":{"focus":"red"}}`, `{"colors":{"focus":"#123"}}`,
		`{"colors":{"focus":"#gggggg"}}`, `{"colors":{"focus":"#12345678"}}`,
		`{"colors":{"focus":"123456"}}`, `{"colors":{"focus":42}}`, `{"icons":"yes"}`,
	} {
		t.Run(body, func(t *testing.T) {
			if _, err := theme.Load(configPath(t, body), ""); err == nil {
				t.Fatalf("accepted %q", body)
			}
		})
	}
	if _, err := theme.Load(configPath(t, `{"theme":"typo"}`), "dracula"); err == nil {
		t.Fatal("invalid config preset hidden by flag")
	}
	if _, err := theme.Load("", "typo"); err == nil {
		t.Fatal("invalid explicit preset accepted")
	}
	if _, err := theme.Load(filepath.Join(t.TempDir(), "missing.json"), ""); err == nil || !strings.Contains(err.Error(), "open theme config") {
		t.Fatalf("missing config: %v", err)
	}
}
