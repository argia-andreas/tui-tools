package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
)

func TestHighlightJSONOutput(t *testing.T) {
	profile := lipgloss.ColorProfile()
	t.Cleanup(func() { lipgloss.SetColorProfile(profile) })
	// Force color only in this test, so assertions exercise actual ANSI output
	// even when the test process has no terminal.
	lipgloss.SetColorProfile(termenv.TrueColor)
	paint := func(s, color string) string {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Render(s)
	}
	input := "\t" + `"escaped\"key\\"` + " \t: [" + `"雪\u001b[31m\\\"", -12.5e+10, true, false, null, {"n":0}]` + "\n"
	got := highlightJSON(input)
	if plain := ansi.Strip(got); plain != input {
		t.Fatalf("text changed:\n got %q\nwant %q", plain, input)
	}
	for _, token := range []struct{ text, color string }{
		{`"escaped\"key\\"`, "#7dcfff"},
		{`"雪\u001b[31m\\\""`, "#a6d7a5"},
		{"-12.5e+10", "#e5bd78"},
		{"true", "#bb9af7"},
		{"false", "#bb9af7"},
		{"null", "#df9ba9"},
		{`"n"`, "#7dcfff"},
		{"0", "#e5bd78"},
		{":", "#62758b"},
		{"[", "#62758b"},
		{"}", "#62758b"},
	} {
		if !strings.Contains(got, paint(token.text, token.color)) {
			t.Errorf("missing foreground-colored token %q in %q", token.text, got)
		}
	}
	// Check a formatted line, including untouched whitespace and exact token
	// boundaries, rather than requiring a standalone valid JSON document.
	line := "  " + `"value"` + " \t: " + `"true: 12"` + ","
	want := "  " + paint(`"value"`, "#7dcfff") + " \t" + paint(":", "#62758b") + " " + paint(`"true: 12"`, "#a6d7a5") + paint(",", "#62758b")
	if got := highlightJSON(line); got != want {
		t.Fatalf("token boundaries:\n got %q\nwant %q", got, want)
	}
	unknown := "trueish falsehood nullish 12oops 01 +1 1e 1. - \"unfinished"
	if got := highlightJSON(unknown); got != unknown {
		t.Fatalf("unrecognized spans were styled: %q", got)
	}
	lipgloss.SetColorProfile(termenv.Ascii)
	if got := highlightJSON(input); got != input {
		t.Fatalf("color-disabled output changed: %q", got)
	}
}
