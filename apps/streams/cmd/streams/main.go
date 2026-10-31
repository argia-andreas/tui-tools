package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"streams/internal/config"
	"streams/internal/fixture"
	"streams/internal/kafka"
	"streams/internal/stream"
	"streams/internal/ui"
)

func main() { os.Exit(run(os.Args[1:], os.Stderr)) }
func run(args []string, stderr io.Writer) int {
	f := flag.NewFlagSet("streams", flag.ContinueOnError)
	f.SetOutput(stderr)
	fixtureMode := f.Bool("fixture", false, "fictional offline events; no network")
	configPath := f.String("config", "", "explicit connection profile file")
	connection := f.String("connection", "", "exact saved connection name")
	mouse := f.Bool("mouse", true, "enable mouse controls; --mouse=false for native selection")
	trust := f.Bool("trust-osc52", false, "permit explicit field copies through terminal OSC52; receipt unverified")
	settings := f.String("settings", "", "private appearance settings path; writes only on explicit Save")
	if err := f.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if f.NArg() != 0 || *fixtureMode == (*configPath != "") || (*configPath != "" && *connection == "") || (*fixtureMode && *connection != "") {
		fmt.Fprintln(stderr, "Choose --fixture OR --config <file> --connection <name>.")
		return 2
	}
	if *settings == "" {
		dir, err := os.UserConfigDir()
		if err != nil {
			fmt.Fprintln(stderr, "Choose an explicit --settings path.")
			return 2
		}
		*settings = filepath.Join(dir, "streams", "settings.json")
	}
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()
	o := ui.Options{Context: ctx, Fixture: *fixtureMode, TrustOSC52: *trust, SettingsPath: *settings, Connection: "fixture"}
	if *fixtureMode {
		o.Backend = fixture.New()
	} else {
		c, err := config.Load(*configPath)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		o.OpenConnection = func(ctx context.Context, name string) (stream.Backend, error) {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			p, err := c.Connection(name)
			if err != nil {
				return nil, err
			}
			return kafka.New(p)
		}
		o.Connections = c.Names()
		o.Connection = *connection
		var err2 error
		o.Backend, err2 = o.OpenConnection(ctx, *connection)
		if err2 != nil {
			fmt.Fprintln(stderr, err2)
			return 1
		}
	}
	m := ui.New(o)
	defer m.Close()
	opts := []tea.ProgramOption{tea.WithAltScreen(), tea.WithContext(ctx)}
	if *mouse {
		opts = append(opts, tea.WithMouseCellMotion())
	}
	if _, err := tea.NewProgram(m, opts...).Run(); err != nil && ctx.Err() == nil {
		fmt.Fprintln(stderr, "Streams terminal failed.")
		return 1
	}
	return 0
}
