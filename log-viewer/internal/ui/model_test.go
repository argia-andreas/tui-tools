package ui_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"logviewer/internal/fixture"
	"logviewer/internal/logs"
	"logviewer/internal/theme"
	"logviewer/internal/ui"
)

type multilineProvider struct{}

func (multilineProvider) Search(context.Context, logs.Query, string) (logs.NativePage, error) {
	return logs.NativePage{Events: []logs.Event{{
		Time:  fixture.Query().Window.Start,
		Level: "ERROR\nnot another event", Service: "checkout-api\nmetadata",
		Message: "First message line\nSecond message line\n" + strings.Repeat("long UTF-8 Åsa ", 100),
	}}}, nil
}

func TestMultilineDataCannotBreakLayout(t *testing.T) {
	s, err := logs.NewService(multilineProvider{}, fixture.Policy())
	if err != nil {
		t.Fatal(err)
	}
	m := ui.New(ui.Options{Service: s, Query: fixture.Query(), Theme: theme.Midnight(), Context: context.Background()})
	apply := func(msg tea.Msg) { next, _ := m.Update(msg); m = next.(ui.Model) }
	apply(m.Init()())
	for _, width := range []int{60, 80, 119, 120, 139, 140, 160} {
		for _, height := range []int{16, 24, 48} {
			apply(tea.WindowSizeMsg{Width: width, Height: height})
			for _, key := range []tea.KeyMsg{{Type: tea.KeyEsc}, {Type: tea.KeyEnter}, {Type: tea.KeyRunes, Runes: []rune{'G'}}, {Type: tea.KeyEsc}} {
				apply(key)
				lines := strings.Split(ansi.Strip(m.View()), "\n")
				if len(lines) > height {
					t.Fatalf("%dx%d: data expanded frame to %d lines", width, height, len(lines))
				}
				for _, line := range lines {
					if ansi.StringWidth(line) > width {
						t.Fatalf("%dx%d: row escaped frame", width, height)
					}
				}
			}
		}
	}
}

type yankProvider struct{ incomplete bool }

func (p yankProvider) Search(context.Context, logs.Query, string) (logs.NativePage, error) {
	return logs.NativePage{Events: []logs.Event{{Time: fixture.Query().Window.Start,
		Message:    strings.Repeat("Åsa $(touch NEVER_EXECUTE); ", 80) + "END",
		Fields:     map[string]json.RawMessage{"a/b~c": json.RawMessage(`[9007199254740993,{"z":true,"a":null}]`), "auth": json.RawMessage(`{"token":"fixture-secret-canary"}`)},
		Incomplete: p.incomplete,
	}}}, nil
}

func TestInspectorSelectionCopiesFullSafeTypedValuesAndRejectsFailures(t *testing.T) {
	bin := t.TempDir()
	capture := filepath.Join(t.TempDir(), "capture")
	for _, name := range []string{"wl-copy", "pbcopy"} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte("#!/bin/sh\nif [ \"$CLIP_EXIT\" = timeout ]; then exec /bin/sleep 10; fi\n/bin/cat > \"$CLIP_CAPTURE\"\nexit \"${CLIP_EXIT:-0}\"\n"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin)
	t.Setenv("WAYLAND_DISPLAY", "test-only")
	t.Setenv("CLIP_CAPTURE", capture)
	for _, incomplete := range []bool{false, true} {
		s, err := logs.NewService(yankProvider{incomplete}, fixture.Policy())
		if err != nil {
			t.Fatal(err)
		}
		d := &driver{t: t, m: ui.New(ui.Options{Service: s, Query: fixture.Query(), Theme: theme.Midnight()})}
		d.apply(tea.WindowSizeMsg{Width: 80, Height: 24})
		d.run(d.m.Init())
		d.key("enter")
		for n := 0; n < 5; n++ {
			d.key("j")
		}
		if !strings.Contains(d.view(), "/message · safe") && !incomplete {
			t.Fatal("field cursor did not navigate", d.view())
		}
		d.key("y")
		if incomplete {
			if !strings.Contains(d.view(), "Copy refused · incomplete") {
				t.Fatal(d.view())
			}
			_ = s.Shutdown(context.Background())
			continue
		}
		assertPayload := func(want string) {
			t.Helper()
			b, err := os.ReadFile(capture)
			if err != nil || string(b) != want {
				t.Fatalf("payload = %q, %v; want %q", b, err, want)
			}
		}
		assertPayload(strings.Repeat("Åsa $(touch NEVER_EXECUTE); ", 80) + "END")
		if !strings.Contains(d.view(), "Copy command completed") || strings.Contains(d.view(), "END") {
			t.Fatal("preview must shorten without shortening copy", d.view())
		}
		d.key("enter")
		expanded := d.view()
		d.key("pgdown")
		if d.view() == expanded || !strings.Contains(d.view(), "Åsa") || !strings.Contains(d.view(), "/message · safe") {
			t.Fatal("expanded field cannot scroll", d.view())
		}
		d.key("y")
		assertPayload(strings.Repeat("Åsa $(touch NEVER_EXECUTE); ", 80) + "END")
		d.key("pgup")
		if d.view() != expanded {
			t.Fatal("page up lost selected field or position", d.view())
		}
		d.key("g")
		for n := 0; n < 7; n++ {
			d.key("j")
		}
		d.key("Y")
		assertPayload(`/fields/a~1b~0c=[9007199254740993,{"a":null,"z":true}]`)
		d.key("enter")
		d.key("j")
		d.key("y")
		assertPayload(`{"token":"[REDACTED]"}`)
		d.key("j")
		d.key("y")
		assertPayload("[REDACTED]")
		d.key("f")
		if !strings.Contains(d.view(), "› /fields/auth/token") {
			t.Fatal("f lost the inspector cursor", d.view())
		}
		d.key("enter")
		if !strings.Contains(d.view(), "redacted value") {
			t.Fatal(d.view())
		}
		d.key("esc")
		t.Setenv("CLIP_EXIT", "1")
		d.key("y")
		if !strings.Contains(d.view(), "Copy failed") || strings.Contains(d.view(), "Copy command completed") {
			t.Fatal("failure claimed success", d.view())
		}
		t.Setenv("CLIP_EXIT", "timeout")
		d.key("y")
		if !strings.Contains(d.view(), "Copy failed") {
			t.Fatal("timeout claimed success", d.view())
		}
		t.Setenv("CLIP_EXIT", "0")
		d.apply(tea.WindowSizeMsg{Width: 160, Height: 48})
		if !strings.Contains(d.view(), "/fields/auth/token · safe") {
			t.Fatal("resize lost selected field", d.view())
		}
		_ = s.Shutdown(context.Background())
	}
}
