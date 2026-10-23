package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"lazyftp/internal/storage"
	"lazyftp/internal/theme"
	"lazyftp/internal/ui"
)

func main() {
	flag.Bool("demo", true, "Run the read-only fixture workspace (only mode currently available)")
	preset := flag.String("theme", "", "Theme preset (default catppuccin-mocha)")
	config := flag.String("config", "", "Optional JSON theme configuration")
	icons := flag.Bool("icons", false, "Enable Nerd Font filetype icons")
	flag.Parse()
	t, err := theme.Load(*config, *preset)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if *config != "" {
		data, err := os.ReadFile(*config)
		var preferences struct {
			Icons bool `json:"icons"`
		}
		if err == nil {
			err = json.Unmarshal(data, &preferences)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		explicitIcons := false
		flag.Visit(func(f *flag.Flag) {
			if f.Name == "icons" {
				explicitIcons = true
			}
		})
		if !explicitIcons {
			*icons = preferences.Icons
		}
	}
	m := ui.New(ui.Options{Browser: storage.NewDemo(), Left: storage.Location{Path: "/project"}, Right: storage.Location{Remote: "demo-ftp", Path: "/incoming"}, Theme: t, Icons: *icons})
	if _, err = tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion()).Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
