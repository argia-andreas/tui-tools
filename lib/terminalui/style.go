package terminalui

import "github.com/charmbracelet/lipgloss"

// Style maps application-owned theme colors to shared semantic roles.
type Style struct {
	Background, Surface, Foreground, Muted, Border, Focus string
	Selection, SelectionText, Warning, Error              string
	ASCII                                                 bool
}

func paint(text, foreground, background string) string {
	s := lipgloss.NewStyle()
	if foreground != "" {
		s = s.Foreground(lipgloss.Color(foreground))
	}
	if background != "" {
		s = s.Background(lipgloss.Color(background))
	}
	return s.Render(text)
}

func actionColor(a Action, focused bool, s Style) string {
	switch {
	case a.Unavailable != "" || a.Busy:
		return s.Muted
	case focused:
		return s.Focus
	case a.Dangerous:
		return s.Warning
	default:
		return s.Foreground
	}
}
