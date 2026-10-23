// Package theme provides semantic terminal colors and strict JSON configuration.
package theme

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
)

// Theme is a value type; callers can customize a copy without changing presets.
// Every color is a six-digit RGB hex string, including its leading #.
type Theme struct {
	Name                                    string
	Background, Surface, Foreground, Muted  string
	Border, Focus, Selection, SelectionText string
	Local, Remote, Success, Warning, Error  string
	Folder, Markdown, Data, Image           string
}

// Names returns a new slice in stable preset order.
func Names() []string {
	return []string{"catppuccin-mocha", "tokyo-night", "gruvbox-dark", "dracula"}
}

// Get returns the named preset. An empty name selects Catppuccin Mocha.
func Get(name string) (Theme, error) {
	if name == "" {
		name = "catppuccin-mocha"
	}
	var colors []string
	switch name {
	case "catppuccin-mocha":
		colors = []string{"#1e1e2e", "#313244", "#cdd6f4", "#a6adc8", "#585b70", "#89b4fa", "#45475a", "#cdd6f4", "#a6e3a1", "#cba6f7", "#a6e3a1", "#f9e2af", "#f38ba8", "#89b4fa", "#a6e3a1", "#fab387", "#f5c2e7"}
	case "tokyo-night":
		colors = []string{"#1a1b26", "#24283b", "#c0caf5", "#a9b1d6", "#414868", "#7aa2f7", "#283457", "#c0caf5", "#9ece6a", "#bb9af7", "#9ece6a", "#e0af68", "#f7768e", "#7aa2f7", "#9ece6a", "#ff9e64", "#bb9af7"}
	case "gruvbox-dark":
		colors = []string{"#282828", "#3c3836", "#ebdbb2", "#bdae93", "#665c54", "#83a598", "#504945", "#fbf1c7", "#b8bb26", "#d3869b", "#b8bb26", "#fabd2f", "#fb4934", "#83a598", "#b8bb26", "#fe8019", "#d3869b"}
	case "dracula":
		colors = []string{"#282a36", "#44475a", "#f8f8f2", "#6272a4", "#6272a4", "#bd93f9", "#44475a", "#f8f8f2", "#50fa7b", "#ff79c6", "#50fa7b", "#f1fa8c", "#ff5555", "#8be9fd", "#50fa7b", "#ffb86c", "#ff79c6"}
	default:
		return Theme{}, fmt.Errorf("unknown theme preset %q (choose one of %v)", name, Names())
	}
	return Theme{name, colors[0], colors[1], colors[2], colors[3], colors[4], colors[5], colors[6], colors[7], colors[8], colors[9], colors[10], colors[11], colors[12], colors[13], colors[14], colors[15], colors[16]}, nil
}

var hexColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// Load reads an optional explicit JSON config. The explicit preset wins over
// the config's theme, but config colors override either preset. An empty path
// reads no files. Icons is accepted for the executable to handle separately.
// Unknown fields, roles, presets, trailing JSON, and non-#RRGGBB colors fail.
func Load(path, preset string) (Theme, error) {
	type configFile struct {
		Theme  string            `json:"theme"`
		Colors map[string]string `json:"colors"`
		Icons  *bool             `json:"icons"`
	}
	config := new(configFile)
	if path != "" {
		f, err := os.Open(path)
		if err != nil {
			return Theme{}, fmt.Errorf("open theme config: %w", err)
		}
		defer f.Close()
		decoder := json.NewDecoder(f)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&config); err != nil {
			return Theme{}, fmt.Errorf("decode theme config: %w", err)
		}
		if config == nil {
			return Theme{}, fmt.Errorf("theme config must be a JSON object, not null")
		}
		var extra any
		if err := decoder.Decode(&extra); err != io.EOF {
			return Theme{}, fmt.Errorf("theme config must contain exactly one JSON object")
		}
	}
	// Validate even an overridden config preset so typos never go unnoticed.
	if _, err := Get(config.Theme); err != nil {
		return Theme{}, err
	}
	if preset == "" {
		preset = config.Theme
	}
	t, err := Get(preset)
	if err != nil {
		return Theme{}, err
	}
	roles := map[string]*string{
		"background": &t.Background, "surface": &t.Surface,
		"foreground": &t.Foreground, "muted": &t.Muted,
		"border": &t.Border, "focus": &t.Focus,
		"selection": &t.Selection, "selection_text": &t.SelectionText,
		"local": &t.Local, "remote": &t.Remote, "success": &t.Success,
		"warning": &t.Warning, "error": &t.Error, "folder": &t.Folder,
		"markdown": &t.Markdown, "data": &t.Data, "image": &t.Image,
	}
	for role, color := range config.Colors {
		target, ok := roles[role]
		if !ok {
			return Theme{}, fmt.Errorf("unknown theme color role %q", role)
		}
		if !hexColor.MatchString(color) {
			return Theme{}, fmt.Errorf("color %q for %s must be #RRGGBB", color, role)
		}
		*target = color
	}
	return t, nil
}
