package terminalui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// PanelSpec supplies safe, single-line title and pre-styled body content.
// Apps own title decoration, focus colors, layout, and all input regions.
type PanelSpec struct {
	Title      string
	Lines      []string
	TitleColor string
	TitleRule  bool
	Ellipsis   string
}

// RenderPanel draws ordinary panel chrome without creating an input scope.
// Background (not Surface) paints chrome and padding; supplied body styles are
// retained. Tiny panels return height empty lines, with negative height clamped.
func RenderPanel(spec PanelSpec, width, height int, style Style) []string {
	lines := make([]string, max(0, height))
	if width < 2 || height < 2 {
		return lines
	}
	v, h, tl, tr, bl, br := "│", "─", "┌", "┐", "└", "┘"
	if style.ASCII {
		v, h, tl, tr, bl, br = "|", "-", "+", "+", "+", "+"
	}
	border := func(s string) string { return paint(s, style.Border, style.Background) }
	inner := width - 2
	title := ansi.Truncate(spec.Title, inner, spec.Ellipsis)
	fill := " "
	if spec.TitleRule {
		fill = h
	}
	title += strings.Repeat(fill, max(0, inner-ansi.StringWidth(title)))
	titleColor := spec.TitleColor
	if titleColor == "" {
		titleColor = style.Border
	}
	lines[0] = border(tl) + paint(title, titleColor, style.Background) + border(tr)
	for row := 0; row < height-2; row++ {
		line := ""
		if row < len(spec.Lines) {
			line = spec.Lines[row]
		}
		line = ansi.Truncate(line, inner, spec.Ellipsis)
		padding := paint(strings.Repeat(" ", max(0, inner-ansi.StringWidth(line))), style.Foreground, style.Background)
		lines[row+1] = border(v) + line + padding + border(v)
	}
	lines[height-1] = border(bl + strings.Repeat(h, inner) + br)
	return lines
}
