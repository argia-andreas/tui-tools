package terminalui_test

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
	ui "terminalui"
)

func checkFrame(t *testing.T, f ui.Frame) {
	t.Helper()
	if len(f.Lines) != f.Bounds.H {
		t.Fatalf("height %d != %d", len(f.Lines), f.Bounds.H)
	}
	for _, line := range f.Lines {
		if !utf8.ValidString(line) || ansi.StringWidth(line) != f.Bounds.W {
			t.Fatalf("bad line (%d, want %d): %q", ansi.StringWidth(line), f.Bounds.W, line)
		}
	}
	for _, r := range f.Regions {
		b := r.Bounds
		if b.W <= 0 || b.H <= 0 || b.X < f.Bounds.X || b.Y < f.Bounds.Y || b.X+b.W > f.Bounds.X+f.Bounds.W || b.Y+b.H > f.Bounds.Y+f.Bounds.H {
			t.Fatalf("region %+v outside %+v", r, f.Bounds)
		}
	}
}

func TestPanelPresentationAndCellBounds(t *testing.T) {
	for _, tc := range []struct {
		name, title, body, marker, top, middle, bottom string
		rule, ascii                                    bool
	}{
		{"rule", " A ", "body", "…", "┌ A ───┐", "│body  │", "└──────┘", true, false},
		{"spaces", " B ", "body", "", "┌ B    ┐", "│body  │", "└──────┘", false, false},
		{"ascii", " C ", "body", "", "+ C ---+", "|body  |", "+------+", true, true},
		{"ellipsis", "long title", "abcdefghi", "…", "┌long …┐", "│abcde…│", "└──────┘", false, false},
		{"hard clip", "long title", "abcdefghi", "", "┌long t┐", "│abcdef│", "└──────┘", false, false},
		{"graphemes", "界é", "界é👩‍💻xy", "", "┌界é   ┐", "│界é👩‍💻x│", "└──────┘", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			lines := ui.RenderPanel(ui.PanelSpec{Title: tc.title, Lines: []string{tc.body, "ignored"}, TitleRule: tc.rule, Ellipsis: tc.marker}, 8, 3, ui.Style{ASCII: tc.ascii})
			want := []string{tc.top, tc.middle, tc.bottom}
			for i := range want {
				if got := ansi.Strip(lines[i]); got != want[i] {
					t.Fatalf("row %d = %q; want %q", i, got, want[i])
				}
			}
		})
	}
	for _, width := range []int{-1, 0, 1, 2, 3, 11, 24} {
		for _, height := range []int{-1, 0, 1, 2, 5} {
			lines := ui.RenderPanel(ui.PanelSpec{Title: "界é👩‍💻", Lines: []string{"界é👩‍💻"}, Ellipsis: "…"}, width, height, ui.Style{})
			if len(lines) != max(0, height) {
				t.Fatalf("%dx%d: wrong height %d", width, height, len(lines))
			}
			if width < 2 || height < 2 {
				for _, line := range lines {
					if line != "" {
						t.Fatalf("%dx%d: tiny panel has content", width, height)
					}
				}
				continue
			}
			checkFrame(t, ui.Frame{Bounds: ui.Rect{W: width, H: height}, Lines: lines})
			if height == 5 && ansi.Strip(lines[3]) != "│"+strings.Repeat(" ", width-2)+"│" {
				t.Fatal("missing body row was not padded")
			}
		}
	}
	t.Run("style roles", func(t *testing.T) {
		oldProfile := lipgloss.ColorProfile()
		lipgloss.SetColorProfile(termenv.TrueColor)
		t.Cleanup(func() { lipgloss.SetColorProfile(oldProfile) })
		body := "\x1b[38;2;170;187;204;48;2;221;238;255mxy\x1b[0m"
		lines := ui.RenderPanel(ui.PanelSpec{Title: "title", TitleColor: "#112233", Lines: []string{body}}, 11, 3,
			ui.Style{Border: "#445566", Foreground: "#778899", Background: "#010203", Surface: "#aabbcc"})
		for _, tc := range []struct{ text, color string }{
			{ansi.Cut(lines[0], 0, 1), "38;2;68;85;102"},
			{ansi.Cut(lines[0], 1, 6), "38;2;17;34;51"},
			{ansi.Cut(lines[1], 3, 9), "38;2;119;136;153"},
			{ansi.Cut(lines[1], 3, 9), "48;2;1;2;3"},
		} {
			if !strings.Contains(tc.text, tc.color) {
				t.Fatalf("missing style role %s in %q", tc.color, tc.text)
			}
		}
		if !strings.Contains(lines[1], body) {
			t.Fatal("supplied body style was changed")
		}
	})
}

func TestFrameCellBoundsAndClipping(t *testing.T) {
	actions := []ui.Action{
		{ID: "one", Label: "\x1b[31m界 e\u0301 👩‍💻\x1b[0m", Keys: []string{"ctrl+p"}},
		{ID: "two", Label: "Delete", Unavailable: "read only", Dangerous: true},
	}
	for _, ascii := range []bool{false, true} {
		for _, w := range []int{0, 1, 2, 3, 10, 20, 40, 82} {
			for _, h := range []int{0, 1, 2, 4, 5, 6, 18} {
				t.Run(fmt.Sprintf("%t/%d/%d", ascii, w, h), func(t *testing.T) {
					style := ui.Style{ASCII: ascii}
					bounds := ui.Rect{X: 2, Y: 5, W: w, H: h}
					bar := ui.RenderActionBar(actions, bounds, "one", style)
					palette := (ui.Palette{}).View(actions, bounds, style)
					checkFrame(t, bar)
					checkFrame(t, palette)
					checkFrame(t, ui.Overlay(bar, palette, style))
					fit := ui.Fit(actions[0].Label, w)
					if !utf8.ValidString(fit) || ansi.StringWidth(fit) != w {
						t.Fatalf("Fit broke cells: %q", fit)
					}
				})
			}
		}
	}
}

func TestPalettePresentationAndStateMarkers(t *testing.T) {
	actions := []ui.Action{
		{ID: "inspect", Label: "Inspect", Keys: []string{"i", "enter"}},
		{ID: "remove", Label: "Delete", Keys: []string{"d"}, Dangerous: true, Unavailable: "read only"},
		{ID: "load", Label: "Load", Busy: true},
	}
	for _, ascii := range []bool{false, true} {
		style := ui.Style{ASCII: ascii}
		f := (ui.Palette{SelectedID: "remove"}).View(actions, ui.Rect{W: 82, H: 12}, style)
		s := ansi.Strip(strings.Join(f.Lines, "\n"))
		for _, want := range []string{"ACTIONS", "Ctrl+P", "[x]", "Query:", "i/enter", "read only", "[unavailable]", "[busy]", "! Delete", "Esc close"} {
			if !strings.Contains(s, want) {
				t.Errorf("missing %q in %s", want, s)
			}
		}
		bar := ui.RenderActionBar(actions, ui.Rect{W: 180, H: 1}, "inspect", style)
		for _, want := range []string{"[ Inspect i/enter ]", "[busy]", "read only", "! Delete"} {
			if !strings.Contains(ansi.Strip(bar.Lines[0]), want) {
				t.Errorf("bar missing %q: %s", want, bar.Lines[0])
			}
		}
		no := (ui.Palette{Query: "zzzz"}).View(actions, ui.Rect{W: 82, H: 12}, style)
		if len(no.Regions) != 2 || no.Regions[0].Kind != ui.ControlRegion || no.Regions[1].Kind != ui.ControlRegion || !strings.Contains(strings.Join(no.Lines, ""), "No results") {
			t.Fatal("no-results presentation retained interactive row")
		}
		if ascii {
			for _, r := range s {
				if r > 127 {
					t.Fatalf("non-ASCII chrome: %q", r)
				}
			}
		}
	}
}

func TestPaletteSelectedReasonAndAlignedColumns(t *testing.T) {
	reason := "No file path selected; choose a file to continue"
	actions := []ui.Action{
		{ID: "save", Label: "Edit selected file in Neovim", Keys: []string{"ctrl+s"}, Unavailable: reason},
		{ID: "inspect", Label: "Inspect", Keys: []string{"i"}},
	}
	for _, width := range []int{54, 82} {
		f := (ui.Palette{SelectedID: "save"}).View(actions, ui.Rect{W: width, H: 12}, ui.Style{})
		checkFrame(t, f)
		if width == 82 && !strings.Contains(ansi.Strip(f.Lines[3]), "Edit selected file in Neovim") {
			t.Fatal("standard-width unavailable marker hides the action identity")
		}
		if got := strings.TrimSpace(ansi.Strip(f.Lines[9])); !strings.Contains(got, reason) {
			t.Fatalf("width %d selected reason clipped: %q", width, got)
		}
		if !strings.Contains(ansi.Strip(f.Lines[10]), "F1 Help") {
			t.Fatal("footer does not advertise Help")
		}
		for _, y := range []int{2, 3, 4} {
			line := ansi.Strip(f.Lines[y])
			column := ansi.StringWidth(line[:strings.Index(line, "|")])
			if column != map[int]int{54: 34, 82: 55}[width] {
				t.Fatalf("width %d row %d misaligned column %d: %q", width, y, column, line)
			}
		}
		if !strings.Contains(ansi.Strip(f.Lines[3]), "›") || strings.Contains(ansi.Strip(f.Lines[4]), "›") {
			t.Fatal("selected state lacks distinct text marker")
		}
	}
}

func TestActionBarFocusMarkerAndCompleteLayout(t *testing.T) {
	actions := []ui.Action{{ID: "cancel", Label: "Cancel"}, {ID: "apply", Label: "Apply"}}
	for _, ascii := range []bool{false, true} {
		marker := "›"
		if ascii {
			marker = ">"
		}
		for _, focus := range []string{"", "cancel", "apply", "body-field"} {
			for _, width := range []int{0, 9, 10, 11, 19, 20, 21, 22, 23} {
				t.Run(fmt.Sprintf("%t/%s/%d", ascii, focus, width), func(t *testing.T) {
					f := ui.RenderActionBar(actions, ui.Rect{X: 3, Y: 4, W: width, H: 1}, focus, ui.Style{ASCII: ascii})
					checkFrame(t, f)
					firstWidth, fullWidth, firstX, secondX := 10, 20, 3, 14
					first, second := "[ Cancel ]", "[ Apply ]"
					if focus != "" {
						firstWidth, fullWidth, firstX, secondX = 11, 22, 4, 16
						first, second = " "+first, " "+second
						if focus == "cancel" {
							first = marker + "[ Cancel ]"
						}
						if focus == "apply" {
							second = marker + "[ Apply ]"
						}
					}
					want, count := "", 0
					if width >= firstWidth {
						want, count = first, 1
					}
					if width >= fullWidth {
						want, count = first+" "+second, 2
					}
					if got := ansi.Strip(f.Lines[0]); got != ui.Fit(want, width) {
						t.Fatalf("incomplete control or wrong marker: %q, want %q", got, ui.Fit(want, width))
					}
					if len(f.Regions) != count {
						t.Fatalf("regions %+v, want %d complete controls", f.Regions, count)
					}
					if count >= 1 && f.Regions[0].Bounds != (ui.Rect{X: firstX, Y: 4, W: 10, H: 1}) {
						t.Fatalf("Cancel geometry: %+v", f.Regions[0])
					}
					if count == 2 && f.Regions[1].Bounds != (ui.Rect{X: secondX, Y: 4, W: 9, H: 1}) {
						t.Fatalf("Apply geometry: %+v", f.Regions[1])
					}
				})
			}
		}
	}
	// Availability never hides the non-color focus indicator.
	for _, action := range []ui.Action{
		{ID: "cancel", Label: "Cancel", Unavailable: "running"},
		{ID: "cancel", Label: "Cancel", Busy: true},
	} {
		f := ui.RenderActionBar([]ui.Action{action}, ui.Rect{W: 80, H: 1}, "cancel", ui.Style{})
		if !strings.HasPrefix(ansi.Strip(f.Lines[0]), "›[ ") {
			t.Fatal("unavailable/busy focus marker missing")
		}
	}
}

func TestOverlayOnlyExposesForegroundRegions(t *testing.T) {
	base := ui.Frame{Bounds: ui.Rect{X: 2, Y: 3, W: 8, H: 3}, Lines: []string{"abcdefgh", "ijklmnop", "qrstuvwx"}, Regions: []ui.Region{{ID: "delete", Kind: ui.ControlRegion, Bounds: ui.Rect{X: 2, Y: 3, W: 8, H: 1}}}}
	foreground := ui.Frame{Bounds: ui.Rect{X: 4, Y: 4, W: 4, H: 1}, Lines: []string{"TEST"}, Regions: []ui.Region{{ID: "safe", Kind: ui.RowRegion, Bounds: ui.Rect{X: 4, Y: 4, W: 4, H: 1}}}}
	f := ui.Overlay(base, foreground, ui.Style{})
	checkFrame(t, f)
	if ansi.Strip(f.Lines[1]) != "ijTESTop" || len(f.Regions) != 1 || f.Regions[0].ID != "safe" {
		t.Fatalf("bad composition: %+v", f)
	}
	foreground.Bounds.X, foreground.Regions[0].Bounds.X = 8, 8
	f = ui.Overlay(base, foreground, ui.Style{})
	checkFrame(t, f)
	if ansi.Strip(f.Lines[1]) != "ijklmnTE" || len(f.Regions) != 1 || f.Regions[0].Bounds.W != 2 {
		t.Fatalf("bad partial overlay: %+v", f)
	}
	foreground.Bounds.X = 100
	f = ui.Overlay(base, foreground, ui.Style{})
	checkFrame(t, f)
	if len(f.Regions) != 0 || ansi.Strip(f.Lines[1]) != "ijklmnop" {
		t.Fatal("off-screen foreground retained regions or changed base")
	}
}
