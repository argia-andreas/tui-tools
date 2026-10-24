package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"lazyftp/internal/connections"
	"lazyftp/internal/storage"
	"lazyftp/internal/theme"
	"lazyftp/internal/ui"
)

func main() {
	demo := flag.Bool("demo", false, "Run the read-only fixture workspace")
	live := flag.Bool("live", false, "Compatibility flag: real local files are now the default")
	left := flag.String("left", ".", "Local starting directory")
	right := flag.String("right", "", "Connect immediately to rclone remote:path")
	blob := flag.String("blob", "", "Optional second rclone remote:path in navigation")
	var remotes remoteFlags
	flag.Var(&remotes, "remote", "Additional rclone remote:path in navigation (repeatable, any backend)")
	bookmarks := flag.String("connections", "", "Bookmark JSON file (default OS config directory/lazyftp/connections.json)")
	rcloneConfig := flag.String("rclone-config", "", "Existing rclone config file (defaults to rclone environment/config)")
	preset := flag.String("theme", "", "Theme preset (default midnight)")
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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if *demo && *live {
		fmt.Fprintln(os.Stderr, "choose either --demo or --live")
		os.Exit(1)
	}
	options, err := workspaceOptions(ctx, *demo, *left, *right, append([]string{*blob}, remotes...), *rcloneConfig, t, *icons)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	leftOverride := ""
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "left" {
			leftOverride = options.Left.Path
		}
	})
	var m tea.Model = ui.New(options)
	sessions := &connectionSessions{}
	defer func() { cancel(); sessions.Close() }()
	if !*demo {
		w := workspace{model: m, ctx: ctx, theme: t, bookmarkPath: *bookmarks, config: *rcloneConfig, persistentConfig: *rcloneConfig, sessions: sessions, leftOverride: leftOverride, extra: options.Locations}
		if options.Disconnected {
			w.connectOnStart = options.Left.Path
		}
		m = w
	}
	if _, err = tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion()).Run(); err != nil {
		cancel()
		sessions.Close()
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func workspaceOptions(ctx context.Context, demo bool, left, right string, roots []string, config string, t theme.Theme, icons bool) (ui.Options, error) {
	o := ui.Options{Context: ctx, Theme: t, Icons: icons}
	if demo {
		o.Browser = storage.NewDemo()
		o.Left, o.Right = storage.Location{Path: "/project"}, storage.Location{Remote: "demo-ftp", Path: "/incoming"}
		return o, nil
	}
	local, err := filepath.Abs(left)
	if err != nil {
		return o, err
	}
	backend := storage.NewRclone(config)
	o.Browser, o.Transferer = backend, backend
	o.Left, o.Disconnected = storage.Location{Path: local}, right == ""
	if right != "" {
		o.Right, err = remoteLocation(right)
		if err != nil {
			return o, err
		}
		o.Locations = append(o.Locations, o.Right)
	}
	for _, root := range roots {
		if root == "" {
			continue
		}
		loc, err := remoteLocation(root)
		if err != nil {
			return o, err
		}
		o.Locations = append(o.Locations, loc)
	}
	o.Locations = append(o.Locations, o.Left)
	return o, nil
}

func remoteLocation(value string) (storage.Location, error) {
	return connections.ParseRemote(value)
}

type remoteFlags []string

func (r *remoteFlags) String() string { return strings.Join(*r, ", ") }
func (r *remoteFlags) Set(value string) error {
	if _, err := remoteLocation(value); err != nil {
		return err
	}
	*r = append(*r, value)
	return nil
}
