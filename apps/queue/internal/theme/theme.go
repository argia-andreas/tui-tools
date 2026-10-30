// Package theme provides semantic terminal colors for Service Bus Explorer.
package theme

import (
	"fmt"
	"terminalui"
)

// TerminalStyle is the single semantic mapping for shared controls.
func (t Theme) TerminalStyle(ascii bool) terminalui.Style {
	return terminalui.Style{Background: t.Background, Surface: t.Surface,
		Foreground: t.Foreground, Muted: t.Muted, Border: t.Border, Focus: t.Focus,
		Selection: t.Selection, SelectionText: t.SelectionText,
		Warning: t.Warning, Error: t.DeadLetter, ASCII: ascii}
}

// Theme is a value type; callers may customize a copy without changing presets.
// Colors are six-digit RGB hex strings, including the leading #. Queued,
// DeadLetter and Warning are foreground accents, not status backgrounds.
type Theme struct {
	Name                                    string
	Background, Surface, Foreground, Muted  string
	Border, Focus, Selection, SelectionText string
	Queued, DeadLetter, Warning             string
}

// Names returns a fresh slice in stable preset order.
func Names() []string {
	return []string{"midnight", "gruvbox", "light"}
}

// Get returns the named preset. An empty name selects midnight.
// Names are exact and case-sensitive; unknown names return an error.
func Get(name string) (Theme, error) {
	if name == "" {
		name = "midnight"
	}
	switch name {
	case "midnight":
		return Theme{
			Name:       name,
			Background: "#0b1423", Surface: "#101d2f",
			Foreground: "#dce8f4", Muted: "#91a6bb",
			Border: "#4b6a89", Focus: "#65c7f7",
			Selection: "#8af0ce", SelectionText: "#071a24",
			Queued: "#90b99b", DeadLetter: "#dc939b", Warning: "#efc46f",
		}, nil
	case "gruvbox":
		return Theme{
			Name:       name,
			Background: "#282828", Surface: "#3c3836",
			Foreground: "#ebdbb2", Muted: "#bdae93",
			Border: "#928374", Focus: "#83a598",
			Selection: "#504945", SelectionText: "#fbf1c7",
			Queued: "#a9b781", DeadLetter: "#e9a096", Warning: "#fabd2f",
		}, nil
	case "light":
		return Theme{
			Name:       name,
			Background: "#f6f8fb", Surface: "#ffffff",
			Foreground: "#243247", Muted: "#526277",
			Border: "#8793a3", Focus: "#235b95",
			Selection: "#d8e7f7", SelectionText: "#172d49",
			Queued: "#37684a", DeadLetter: "#a63f47", Warning: "#805b12",
		}, nil
	default:
		return Theme{}, fmt.Errorf("unknown theme preset %q (choose one of %v)", name, Names())
	}
}
