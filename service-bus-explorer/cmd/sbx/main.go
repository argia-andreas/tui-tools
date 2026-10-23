package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"servicebusexplorer/internal/azure"
	"servicebusexplorer/internal/explorer"
	"servicebusexplorer/internal/fixture"
	"servicebusexplorer/internal/ui"
)

func main() {
	fixtureMode := flag.Bool("fixture", false, "browse deterministic offline sample data (no Azure connection)")
	emulatorMode := flag.Bool("emulator", false, "browse the fixed local lab on loopback ports 5673/5301 (no cloud access)")
	flag.Parse()
	if *fixtureMode == *emulatorMode || flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "Select exactly one explicit mode: sbx --fixture OR sbx --emulator")
		os.Exit(2)
	}
	var store explorer.Store
	var err error
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
	_, runErr := tea.NewProgram(ui.New(store, mode), tea.WithAltScreen()).Run()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	closeErr := store.Close(ctx)
	if runErr != nil || closeErr != nil {
		fmt.Fprintln(os.Stderr, "Explorer stopped:", runErr, closeErr)
		os.Exit(1)
	}
}
