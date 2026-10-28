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

func TestHelpReturnsToIntactOwnersAndConsumesInput(t *testing.T) {
	d, path := fixtureDriver(t, 22)
	click := func(label string) {
		t.Helper()
		rows := strings.Split(d.view(), "\n")
		for index := range rows {
			y := index
			if label == "[x]" { // Help's foreground close, not its parent Surface.
				y = len(rows) - 1 - index
			}
			line := rows[y]
			if x := strings.Index(line, label); x >= 0 {
				x = ansi.StringWidth(line[:x])
				d.run(d.apply(tea.MouseMsg{X: x, Y: y, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft}))
				d.run(d.apply(tea.MouseMsg{X: x, Y: y, Action: tea.MouseActionRelease, Button: tea.MouseButtonLeft}))
				return
			}
		}
		t.Fatalf("missing visible %q: %s", label, d.view())
	}
	f1 := func() { d.run(d.apply(tea.KeyMsg{Type: tea.KeyF1})) }
	workspace := d.view()
	click("Help f1")
	d.key("q")
	d.key("ctrl+p")
	click("[x]")
	if d.view() != workspace {
		t.Fatal("help input leaked into workspace", d.view())
	}
	d.key("?")
	f1()
	if d.view() != workspace {
		t.Fatal("? / F1 did not restore workspace")
	}
	d.key("ctrl+p")
	d.key("inspect")
	palette := d.view()
	click("Help F1")
	if strings.Contains(d.view(), "Reload current search") {
		t.Fatal("palette Help advertises workspace commands")
	}
	d.key("esc")
	if d.view() != palette {
		t.Fatal("palette query/focus lost after Help")
	}
	f1()
	click("[x]")
	if d.view() != palette {
		t.Fatal("F1 / pointer close lost palette")
	}
	d.key("esc")
	d.key("f")
	d.key("i")
	d.key("jk?")
	draft := d.view()
	if !strings.Contains(draft, "Draft owns input") || !strings.Contains(draft, "jk?") {
		t.Fatal("editor header/text ownership missing", draft)
	}
	click("Actions")
	d.key("ctrl+p")
	if d.view() != draft {
		t.Fatal("Actions opened over draft", d.view())
	}
	click("Help f1")
	if !strings.Contains(d.view(), "first Esc") || strings.Contains(d.view(), "Reload current search") {
		t.Fatal("wrong editor help scope", d.view())
	}
	// Resize and a background press remain captured, not a parent cancel.
	d.apply(tea.WindowSizeMsg{Width: 80, Height: 24})
	d.run(d.apply(tea.MouseMsg{X: 0, Y: 0, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft}))
	d.key("esc")
	d.apply(tea.WindowSizeMsg{Width: 160, Height: 48})
	if d.view() != draft {
		t.Fatal("help resize/return changed INSERT draft", d.view())
	}
	f1()
	click("[x]")
	d.key("j")
	if !strings.Contains(d.view(), "jk?j") {
		t.Fatal("Help return left INSERT")
	}
	d.key("esc")
	if !strings.Contains(d.view(), "FILTER EDITOR") {
		t.Fatal("first Esc discarded draft")
	}
	d.key("esc")
	if d.view() != workspace {
		t.Fatal("second Esc failed to cancel draft")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("Help wrote a saved query", err)
	}
	// A result that arrives behind Help must still finish the underlying read.
	read := d.apply(keyMsg("r"))
	f1()
	d.run(read)
	if !strings.Contains(d.view(), " HELP ") {
		t.Fatal("read completion dismissed Help")
	}
	f1()
	if strings.Contains(d.view(), "Reading…") || !strings.Contains(d.view(), "22 fictional events") {
		t.Fatal("Help swallowed read completion", d.view())
	}
	d.key(",")
	d.key("enter")
	settingsDraft := d.view()
	f1()
	if !strings.Contains(d.view(), "Save settings") || strings.Contains(d.view(), "Apply draft filters") {
		t.Fatal("settings Help confused save with filter Apply", d.view())
	}
	click("[x]")
	if d.view() != settingsDraft {
		t.Fatal("Help reset settings draft")
	}
	d.key("esc")
}

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
