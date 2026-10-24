package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"

	"logviewer/internal/fixture"
	"logviewer/internal/logs"
	"logviewer/internal/theme"
	"logviewer/internal/ui"
)

func main() { os.Exit(run()) }

func run() int {
	flags := flag.NewFlagSet("logview", flag.ContinueOnError)
	useFixture := flags.Bool("fixture", false, "open fictional offline logs; makes no network connections")
	if err := flags.Parse(os.Args[1:]); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if !*useFixture || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "Choose --fixture. Live sources are not implemented yet.")
		return 2
	}
	provider, err := fixture.New()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Could not load offline fixtures.")
		return 1
	}
	service, err := logs.NewService(provider, fixture.Policy())
	if err != nil {
		fmt.Fprintln(os.Stderr, "Invalid fixture output policy.")
		return 1
	}
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM)
	defer cancel()
	_, err = tea.NewProgram(ui.New(ui.Options{Service: service, Query: fixture.Query(), Theme: theme.Midnight(), Context: ctx}), tea.WithAltScreen(), tea.WithContext(ctx)).Run()
	if err != nil && ctx.Err() == nil {
		fmt.Fprintln(os.Stderr, "Terminal viewer failed.")
		return 1
	}
	return 0
}
