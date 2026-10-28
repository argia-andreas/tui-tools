package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
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
	mouse := flags.Bool("mouse", true, "enable application mouse reporting; use --mouse=false for terminal-native selection")
	configPath := flags.String("config", "", "open an explicit local Elasticsearch lab source configuration")
	searchesPath := flags.String("searches", "", "private saved-search file; written only by explicit named save")
	trustOSC52 := flags.Bool("trust-osc52", false, "explicitly trust terminal clipboard transmission when native copy is unavailable")
	settingsPath := flags.String("settings", "", "private user settings file; written only on preference changes")
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
	if *searchesPath == "" {
		dir, err := os.UserConfigDir()
		if err != nil {
			fmt.Fprintln(os.Stderr, "Cannot determine saved-search location; choose --searches explicitly.")
			return 1
		}
		*searchesPath = filepath.Join(dir, "logview", "searches.json")
	}
	if *settingsPath == "" {
		dir, err := os.UserConfigDir()
		if err != nil {
			fmt.Fprintln(os.Stderr, "Cannot determine settings location; choose --settings explicitly.")
			return 1
		}
		*settingsPath = filepath.Join(dir, "logview", "settings.json")
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
				defer client.Shutdown(context.Background())
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
	defer service.Shutdown(context.Background())
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM)
	defer cancel()
	programOptions := []tea.ProgramOption{tea.WithAltScreen(), tea.WithContext(ctx)}
	if *mouse {
		programOptions = append(programOptions, tea.WithMouseCellMotion())
	}
	_, err = tea.NewProgram(ui.New(ui.Options{Service: service, Query: query, Theme: theme.Midnight(), Context: ctx, LocalLab: localLab, SearchesPath: *searchesPath, TrustOSC52: *trustOSC52, SettingsPath: *settingsPath}), programOptions...).Run()
	if err != nil && ctx.Err() == nil {
		fmt.Fprintln(os.Stderr, "Terminal viewer failed.")
		return 1
	}
	return 0
}
