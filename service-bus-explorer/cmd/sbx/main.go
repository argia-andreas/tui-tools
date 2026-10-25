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
	azureMode := flag.Bool("azure", false, "read-only Azure connection using an existing Azure CLI login")
	tenant := flag.String("tenant", "", "explicit Azure tenant ID (required with --azure)")
	configPath := flag.String("config", "", "nonsecret preferences file (default: user config directory)")
	ascii := flag.Bool("ascii", false, "use ASCII borders and UI indicators")
	flag.Parse()
	modes := 0
	for _, selected := range []bool{*fixtureMode, *emulatorMode, *azureMode} {
		if selected {
			modes++
		}
	}
	if modes != 1 || flag.NArg() != 0 || *azureMode && *tenant == "" || !*azureMode && *tenant != "" {
		fmt.Fprintln(os.Stderr, "Select exactly one explicit mode: sbx --fixture OR sbx --emulator OR sbx --azure --tenant <tenant-id>")
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
	var provider explorer.ScopeProvider
	mode := "fixture"
	connection, label := "fixture", "Offline sample catalog · no Azure connection"
	if *azureMode {
		mode = "azure"
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		cloud, cloudErr := azure.NewCloud(ctx, *tenant)
		cancel()
		err = cloudErr
		if err == nil {
			store, provider, connection, label = cloud, cloud, cloud.Profile(), cloud.ConnectionLabel()
		}
	} else if *emulatorMode {
		mode = "emulator"
		local, localErr := azure.NewEmulator()
		store, provider, err = local, local, localErr
		connection, label = "emulator", "Local lab · fixed loopback emulator"
		prefs = config.EnsureLocal(prefs, connection, "Local lab", azure.LocalNamespace())
	} else {
		local, localErr := fixture.NewScoped()
		store, provider, err = local, local, localErr
		prefs = config.EnsureLocal(prefs, connection, "Offline fixture", explorer.Namespace{Profile: "fixture", Address: "local-lab", Name: "local-lab"})
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	options := ui.Options{Mode: mode, Preferences: prefs, ConfigPath: *configPath, ASCII: *ascii,
		ScopeProvider: provider, Connection: connection, ConnectionLabel: label}
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
