package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"lazyftp/internal/storage"
	"lazyftp/internal/theme"
	"lazyftp/internal/ui"
)

func main() {
	demo := flag.Bool("demo", false, "Run the read-only fixture workspace (default without --live)")
	live := flag.Bool("live", false, "Use real filesystems and enable confirmed single-file copies")
	left := flag.String("left", ".", "Local starting directory in live mode")
	right := flag.String("right", "", "Starting rclone remote:path in live mode")
	blob := flag.String("blob", "", "Optional second rclone remote:path in navigation")
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
	options := ui.Options{Browser: storage.NewDemo(), Context: ctx, Left: storage.Location{Path: "/project"}, Right: storage.Location{Remote: "demo-ftp", Path: "/incoming"}, Theme: t, Icons: *icons}
	if *live {
		if *demo {
			fmt.Fprintln(os.Stderr, "choose either --demo or --live")
			os.Exit(1)
		}
		if _, err := exec.LookPath("rclone"); err != nil {
			fmt.Fprintln(os.Stderr, "live mode requires rclone; run ./.agents/setup")
			os.Exit(1)
		}
		local, err := filepath.Abs(*left)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		remote, err := remoteLocation(*right)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		backend := storage.NewRclone(*rcloneConfig)
		options.Browser, options.Transferer = backend, backend
		options.Left, options.Right = storage.Location{Path: local}, remote
		options.Locations = []storage.Location{remote}
		if *blob != "" {
			loc, err := remoteLocation(*blob)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			options.Locations = append(options.Locations, loc)
		}
		options.Locations = append(options.Locations, options.Left)
	}
	m := ui.New(options)
	if _, err = tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion()).Run(); err != nil {
		cancel()
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func remoteLocation(value string) (storage.Location, error) {
	remote, path, ok := strings.Cut(value, ":")
	if !ok || remote == "" || strings.ContainsAny(remote, "/\\") {
		return storage.Location{}, fmt.Errorf("live remotes must use an existing rclone remote:path (for example lab-ftp:/incoming)")
	}
	return storage.Location{Remote: remote, Path: "/" + strings.TrimPrefix(path, "/")}, nil
}
