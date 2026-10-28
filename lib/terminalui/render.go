// Package terminalui renders stateless terminal controls and returns semantic
// requests. Applications own authorization, effects, lifecycle, and input routing.
package terminalui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// Rect is an absolute zero-based rectangle measured in terminal cells.
type Rect struct{ X, Y, W, H int }

type RegionKind uint8

const (
	ControlRegion RegionKind = iota + 1
	FieldRegion
	RowRegion
	DestinationRegion
	PaneRegion
)

type Region struct {
	ID     string
	Kind   RegionKind
	Bounds Rect
}

// Frame lines are local to Bounds; regions use absolute screen coordinates.
type Frame struct {
	Bounds  Rect
	Lines   []string
	Regions []Region
}

// Fit clips ANSI-styled text at grapheme/cell boundaries and pads to width.
// Like the renderers, it expects safe, single-line terminal content.
func Fit(styled string, width int) string {
	width = max(0, width)
	s := ansi.Truncate(styled, width, "")
	return s + strings.Repeat(" ", max(0, width-ansi.StringWidth(s)))
}

func blank(bounds Rect, style Style) Frame {
	bounds.W, bounds.H = max(0, bounds.W), max(0, bounds.H)
	f := Frame{Bounds: bounds, Lines: make([]string, bounds.H)}
	for i := range f.Lines {
		f.Lines[i] = paint(strings.Repeat(" ", bounds.W), style.Foreground, style.Surface)
	}
	return f
}

func contains(r Rect, x, y int) bool {
	return r.W > 0 && r.H > 0 && x >= r.X && x < r.X+r.W && y >= r.Y && y < r.Y+r.H
}

func intersection(a, b Rect) Rect {
	x, y := max(a.X, b.X), max(a.Y, b.Y)
	return Rect{x, y, max(0, min(a.X+a.W, b.X+b.W)-x), max(0, min(a.Y+a.H, b.Y+b.H)-y)}
}

// Overlay dims the base and exposes only visible foreground hit regions.
// The returned frame retains the base's bounds, including for partial overlays.
func Overlay(base, foreground Frame, style Style) Frame {
	f := blank(base.Bounds, style)
	for y := range f.Lines {
		if y < len(base.Lines) {
			f.Lines[y] = lipgloss.NewStyle().Faint(true).Render(paint(Fit(ansi.Strip(base.Lines[y]), f.Bounds.W), style.Muted, style.Background))
		}
	}
	visible := intersection(f.Bounds, foreground.Bounds)
	if visible.W == 0 || visible.H == 0 {
		return f
	}
	for y := visible.Y; y < visible.Y+visible.H; y++ {
		srcY, dstY := y-foreground.Bounds.Y, y-f.Bounds.Y
		if srcY >= len(foreground.Lines) {
			continue
		}
		left, right := visible.X-f.Bounds.X, visible.X+visible.W-f.Bounds.X
		middle := ansi.Cut(Fit(foreground.Lines[srcY], foreground.Bounds.W), visible.X-foreground.Bounds.X, visible.X-foreground.Bounds.X+visible.W)
		f.Lines[dstY] = Fit(ansi.Cut(f.Lines[dstY], 0, left), left) + Fit(middle, visible.W) + Fit(ansi.Cut(f.Lines[dstY], right, f.Bounds.W), f.Bounds.W-right)
	}
	for _, r := range foreground.Regions {
		r.Bounds = intersection(r.Bounds, visible)
		if r.Bounds.W > 0 && r.Bounds.H > 0 {
			f.Regions = append(f.Regions, r)
		}
	}
	return f
}
