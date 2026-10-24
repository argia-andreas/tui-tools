package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"

	"logviewer/internal/config"
	"logviewer/internal/elastic"
	"logviewer/internal/fixture"
	"logviewer/internal/logs"
	"logviewer/internal/theme"
	"logviewer/internal/ui"
)

func main() { os.Exit(run()) }

func run() int {
	flags := flag.NewFlagSet("logview", flag.ContinueOnError)
	useFixture := flags.Bool("fixture", false, "open fictional offline logs; makes no network connections")
	configPath := flags.String("config", "", "open an explicit local Elasticsearch lab source configuration")
	if err := flags.Parse(os.Args[1:]); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if *useFixture == (*configPath != "") || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "Choose --fixture or --config <local lab source file>.")
		return 2
	}
	var provider logs.Provider
	var err error
	query, policy := fixture.Query(), fixture.Policy()
	localLab := false
	if *useFixture {
		provider, err = fixture.New()
	} else {
		var c config.Config
		c, err = config.Load(*configPath)
		if err == nil {
			var client *elastic.Client
			client, err = elastic.New(c)
			if err == nil {
				defer client.Close()
				provider = client
				query, policy, localLab = c.Query(), c.Policy(), true
			}
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "Could not initialize the selected source; check private configuration, CA and credential files.")
		return 1
	}
	service, err := logs.NewService(provider, policy)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Invalid source output policy.")
		return 1
	}
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM)
	defer cancel()
	_, err = tea.NewProgram(ui.New(ui.Options{Service: service, Query: query, Theme: theme.Midnight(), Context: ctx, LocalLab: localLab}), tea.WithAltScreen(), tea.WithContext(ctx)).Run()
	if err != nil && ctx.Err() == nil {
		fmt.Fprintln(os.Stderr, "Terminal viewer failed.")
		return 1
	}
	return 0
}
