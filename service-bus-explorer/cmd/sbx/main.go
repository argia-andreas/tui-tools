package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"servicebusexplorer/internal/fixture"
	"servicebusexplorer/internal/ui"
)

func main() {
	fixtureMode := flag.Bool("fixture", false, "browse deterministic offline sample data (no Azure connection)")
	flag.Parse()
	if !*fixtureMode || flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "This first slice supports offline fixtures only. Run: sbx --fixture")
		os.Exit(2)
	}
	store, err := fixture.New()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	_, runErr := tea.NewProgram(ui.New(store), tea.WithAltScreen()).Run()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	closeErr := store.Close(ctx)
	if runErr != nil || closeErr != nil {
		fmt.Fprintln(os.Stderr, "Explorer stopped:", runErr, closeErr)
		os.Exit(1)
	}
}
