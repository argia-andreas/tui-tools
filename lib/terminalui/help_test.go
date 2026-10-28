package terminalui_test

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	ui "terminalui"
)

func TestPaletteVisibleHelpParity(t *testing.T) {
	actions := []ui.Action{{ID: "save", Label: "Save", Unavailable: "no path"}}
	p := ui.Palette{Query: "sav", SelectedID: "save"}
	f := p.View(actions, ui.Rect{X: 7, Y: 4, W: 82, H: 10}, ui.Style{})
	before := p
	var clicked bool
	for _, r := range f.Regions {
		if r.Kind != ui.ControlRegion {
			continue
		}
		result := p.Handle(press(r), actions, f)
		if result.Intent != nil && result.Intent.Kind == ui.Help {
			clicked = true
			start := r.Bounds.X - f.Bounds.X
			if got := ansi.Cut(f.Lines[0], start, start+r.Bounds.W); ansi.Strip(got) != "[Help F1]" {
				t.Fatalf("Help hitbox not on visible control: %q", got)
			}
			wantIntent(t, p.Handle(key(tea.KeyF1), actions, f), ui.Help, "")
			m := press(r)
			m.Action = tea.MouseActionRelease
			wantCaptured(t, p.Handle(m, actions, f))
		}
	}
	if !clicked || p != before {
		t.Fatalf("missing Help or changed parent palette: %t %+v", clicked, p)
	}
	if len(f.Regions) != 3 || f.Regions[0].Bounds.Y != f.Bounds.Y+3 {
		t.Fatalf("palette row geometry changed: %+v", f.Regions)
	}
}

func TestHelpCaptureScrollAndDismiss(t *testing.T) {
	actions := []ui.Action{{ID: "quit", Label: "Quit", Keys: []string{"q"}}, {ID: "save", Label: "Save", Unavailable: "no path"}}
	topics := []ui.HelpTopic{{Group: "Reader", Keys: "yy", Description: "Copy"}, {Group: "Editor", Keys: "Esc", Description: "Leave INSERT"}, {Group: "Reader", Keys: "j/k", Description: "Move"}}
	bounds := ui.Rect{X: 5, Y: 4, W: 70, H: 6} // 3 body rows, 9 total content rows
	offset := -10
	handle := func(msg tea.Msg) ui.Result {
		return ui.HandleHelp(msg, actions, topics, &offset, ui.RenderHelp(actions, topics, offset, bounds, ui.Style{}))
	}
	for _, msg := range []tea.Msg{text("qj?yy"), key(tea.KeyCtrlP), key(tea.KeyEnter), key(tea.KeyTab), key(tea.KeyCtrlC)} {
		wantCaptured(t, handle(msg))
	}
	if offset != 0 {
		t.Fatalf("negative offset not clamped: %d", offset)
	}
	wantCaptured(t, handle(key(tea.KeyPgDown)))
	if offset != 3 {
		t.Fatalf("page size differs from visible rows: %d", offset)
	}
	wantCaptured(t, handle(key(tea.KeyDown)))
	if offset != 4 {
		t.Fatal("down did not scroll one row")
	}
	wheel := tea.MouseMsg{X: 6, Y: 5, Action: tea.MouseActionPress, Button: tea.MouseButtonWheelDown}
	wantCaptured(t, handle(wheel))
	if offset != 5 {
		t.Fatalf("wheel did not scroll: %d", offset)
	}
	wantCaptured(t, handle(key(tea.KeyPgDown)))
	if offset != 6 {
		t.Fatalf("end did not clamp: %d", offset)
	}
	f := ui.RenderHelp(actions, topics, offset, bounds, ui.Style{})
	if !strings.Contains(ansi.Strip(f.Lines[3]), "Reason: no path") {
		t.Fatalf("last content row unreachable: %v", f.Lines)
	}
	wheel.X, wheel.Y, wheel.Button = 0, 0, tea.MouseButtonWheelUp
	wantCaptured(t, handle(wheel))
	wheel.Button = tea.MouseButtonLeft
	wantCaptured(t, handle(wheel)) // outside press cannot close the parent or Help
	if offset != 6 {
		t.Fatal("background mouse changed help scroll")
	}
	for _, action := range []tea.MouseAction{tea.MouseActionRelease, tea.MouseActionMotion} {
		m := press(f.Regions[0])
		m.Action = action
		wantCaptured(t, handle(m))
	}
	for _, msg := range []tea.Msg{key(tea.KeyF1), key(tea.KeyEsc), press(f.Regions[0])} {
		wantIntent(t, handle(msg), ui.Dismiss, "")
	}
	wantCaptured(t, handle(key(tea.KeyPgUp)))
	if offset != 3 {
		t.Fatal("page up did not scroll")
	}
	wheel.X, wheel.Y = 6, 5
	wheel.Button = tea.MouseButtonWheelUp
	wantCaptured(t, handle(wheel))
	if offset != 2 {
		t.Fatal("wheel up did not scroll")
	}
	// Current content and resize clamp the same offset as the renderer.
	offset = 999
	empty := ui.RenderHelp(nil, nil, offset, bounds, ui.Style{})
	wantCaptured(t, ui.HandleHelp(key(tea.KeyDown), nil, nil, &offset, empty))
	if offset != 0 || !strings.Contains(ansi.Strip(empty.Lines[1]), "No commands available") {
		t.Fatal("empty help did not clamp or show empty state")
	}
	if r := ui.HandleHelp(tea.WindowSizeMsg{Width: 80, Height: 24}, actions, topics, &offset, f); r.Consumed || r.Intent != nil {
		t.Fatal("help swallowed lifecycle message")
	}
}

func TestHelpPresentationAndCompactControls(t *testing.T) {
	actions := []ui.Action{
		{ID: "inspect", Label: "Inspect", Keys: []string{"i", "enter"}},
		{ID: "save", Label: "Save", Keys: []string{"ctrl+s"}, Unavailable: "no path"},
		{ID: "load", Label: "Load", Busy: true},
	}
	topics := []ui.HelpTopic{{Group: "Reader", Keys: "yy", Description: "Copy"}, {Group: "Editor", Keys: "Esc", Description: "Leave INSERT"}}
	f := ui.RenderHelp(actions, topics, 0, ui.Rect{W: 82, H: 14}, ui.Style{})
	s := ansi.Strip(strings.Join(f.Lines, "\n"))
	for _, want := range []string{"HELP", "[x]", "Actions", "i/enter", "available", "[unavailable]", "no path", "[busy]", "Reader", "yy - Copy", "Editor", "Leave INSERT"} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in %s", want, s)
		}
	}
	// Wide graphemes and ANSI must clip without changing size or leaking regions.
	topics = append(topics, ui.HelpTopic{Keys: "x", Description: "\x1b[31m界 e\u0301 👩‍💻\x1b[0m"})
	for _, ascii := range []bool{false, true} {
		for _, width := range []int{0, 1, 2, 3, 9, 10, 22, 23, 40, 82} {
			for _, height := range []int{0, 1, 3, 4, 6, 20} {
				t.Run(fmt.Sprintf("%t/%d/%d", ascii, width, height), func(t *testing.T) {
					style := ui.Style{ASCII: ascii, Foreground: "#eeeeee", Surface: "#111111", Focus: "#00ffff", Border: "#777777", Warning: "#ffff00"}
					bounds := ui.Rect{X: 3, Y: 2, W: width, H: height}
					f := ui.RenderHelp(actions, topics, 999, bounds, style)
					checkFrame(t, f)
					for _, r := range f.Regions {
						start := r.Bounds.X - bounds.X
						if r.Kind != ui.ControlRegion || ansi.Strip(ansi.Cut(f.Lines[0], start, start+r.Bounds.W)) != "[x]" {
							t.Fatal("help exposes hidden or action hitbox")
						}
					}
					if (width < 10 || height < 4) && len(f.Regions) != 0 {
						t.Fatal("compact help retains hidden close")
					}
					p := ui.Palette{}
					palette := p.View(nil, bounds, style)
					checkFrame(t, palette)
					for _, r := range palette.Regions {
						result := p.Handle(press(r), nil, palette)
						if result.Intent != nil && result.Intent.Kind == ui.Help && (width < 23 || height < 6) {
							t.Fatal("hidden palette help activated")
						}
					}
					if ascii {
						plain := ui.RenderHelp(actions, nil, 0, bounds, style)
						for _, r := range ansi.Strip(strings.Join(plain.Lines, "")) {
							if r > 127 {
								t.Fatal("non-ASCII help chrome")
							}
						}
					}
				})
			}
		}
	}
}

func TestHelpContextFirstAndWrappedReasons(t *testing.T) {
	reason := "No file path selected; choose a writable destination before saving. Preserve the prepared review and its authorization 界 e\u0301 👩‍💻 until the operation completes safely."
	label := "Save the selected Unicode 界 e\u0301 👩‍💻 document and preserve its original metadata"
	keys := strings.Repeat("ctrl+shift+long-key/", 7)
	actions := []ui.Action{{ID: "save", Label: label, Keys: []string{keys}, Unavailable: reason}}
	topics := []ui.HelpTopic{
		{Group: "Current reader", Keys: "yy", Description: "Copy the current field without leaving the prepared review"},
		{Group: "Editor", Keys: "Esc", Description: "Leave INSERT without discarding the draft"},
		{Group: "Current reader", Keys: "j/k", Description: "Move between fields"},
	}
	var endOffsets []int
	for _, width := range []int{54, 82} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			bounds := ui.Rect{X: 4, Y: 2, W: width, H: 4} // one content row
			offset := 0
			var content []string
			for steps := 0; steps < 200; steps++ {
				f := ui.RenderHelp(actions, topics, offset, bounds, ui.Style{})
				checkFrame(t, f)
				line := strings.TrimSpace(ansi.Strip(ansi.Cut(f.Lines[1], 1, width-1)))
				content = append(content, line)
				previous := offset
				wantCaptured(t, ui.HandleHelp(key(tea.KeyDown), actions, topics, &offset, f))
				if offset == previous {
					break
				}
				if steps == 199 {
					t.Fatal("scroll did not clamp")
				}
			}
			endOffsets = append(endOffsets, offset)
			if content[0] != "Current reader" {
				t.Fatalf("context not first: %q", content[0])
			}
			joined := strings.Join(content, " ")
			if strings.Index(joined, "Actions") < strings.Index(joined, "Editor") || strings.Count(joined, "Current reader") != 1 {
				t.Fatalf("topic grouping/order lost: %s", joined)
			}
			compact := func(s string) string { return strings.Join(strings.Fields(s), "") }
			for _, want := range []string{keys, label, reason, "[unavailable]"} {
				if !strings.Contains(compact(joined), compact(want)) {
					t.Errorf("scroll lost wrapped content %q in %s", want, joined)
				}
			}
			if strings.Count(joined, "[unavailable]") != 1 || !strings.Contains(joined, "Reason:") {
				t.Fatal("availability duplicated or standalone reason missing")
			}
			// A very tall render must contain exactly the same content rows as
			// reached by input, including the final wrapped reason line.
			tall := ui.RenderHelp(actions, topics, 0, ui.Rect{W: width, H: len(content) + 3}, ui.Style{})
			checkFrame(t, tall)
			for i, want := range content {
				got := strings.TrimSpace(ansi.Strip(ansi.Cut(tall.Lines[i+1], 1, width-1)))
				if got != want {
					t.Fatalf("renderer/handler row %d differs: %q != %q", i, got, want)
				}
			}
			// Resizing changes the wrap range and clamps an existing offset.
			offset = 999
			wantCaptured(t, ui.HandleHelp(key(tea.KeyDown), actions, topics, &offset, tall))
			if offset != 0 {
				t.Fatalf("resize did not clamp to full content viewport: %d", offset)
			}
		})
	}
	if endOffsets[0] <= endOffsets[1] {
		t.Fatalf("scroll range does not depend on wrapped width: %v", endOffsets)
	}
}
