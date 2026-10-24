package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"servicebusexplorer/internal/azure"
	"servicebusexplorer/internal/config"
	"servicebusexplorer/internal/explorer"
	"servicebusexplorer/internal/fixture"
	"servicebusexplorer/internal/ui"
)

func main() {
	fixtureMode := flag.Bool("fixture", false, "browse deterministic offline sample data (no Azure connection)")
	emulatorMode := flag.Bool("emulator", false, "browse the fixed local lab on loopback ports 5673/5301 (no cloud access)")
	configPath := flag.String("config", "", "nonsecret preferences file (default: user config directory)")
	ascii := flag.Bool("ascii", false, "use ASCII borders and UI indicators")
	flag.Parse()
	if *fixtureMode == *emulatorMode || flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "Select exactly one explicit mode: sbx --fixture OR sbx --emulator")
		os.Exit(2)
	}
	if *configPath == "" {
		path, err := config.Path()
		if err != nil {
			fmt.Fprintln(os.Stderr, "Cannot locate preferences:", err)
			os.Exit(1)
		}
		*configPath = path
	}
	prefs, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Cannot load preferences (file left unchanged):", err)
		os.Exit(1)
	}
	var store explorer.Store
	mode := "fixture"
	if *emulatorMode {
		mode = "emulator"
		store, err = azure.NewEmulator()
	} else {
		store, err = fixture.New()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	options := ui.Options{Mode: mode, Preferences: prefs, ConfigPath: *configPath, ASCII: *ascii}
	final, runErr := tea.NewProgram(ui.New(store, options), tea.WithAltScreen()).Run()
	if summary, ok := final.(interface{ ExitSummary() string }); ok && summary.ExitSummary() != "" {
		fmt.Fprintln(os.Stdout, summary.ExitSummary())
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	closeErr := store.Close(ctx)
	if runErr != nil || closeErr != nil {
		fmt.Fprintln(os.Stderr, "Explorer stopped:", runErr, closeErr)
		os.Exit(1)
	}
}
