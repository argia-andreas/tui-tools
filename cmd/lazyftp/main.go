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
	"lazyftp/internal/connections"
	"lazyftp/internal/storage"
	"lazyftp/internal/theme"
	"lazyftp/internal/ui"
)

func main() {
	demo := flag.Bool("demo", false, "Run the read-only fixture workspace (default without --live)")
	live := flag.Bool("live", false, "Use real filesystems and enable confirmed file/folder copies")
	left := flag.String("left", ".", "Local starting directory in live mode")
	right := flag.String("right", "", "Starting rclone remote:path in live mode")
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
		var extra []storage.Location
		for _, value := range append([]string{*blob}, remotes...) {
			if value == "" {
				continue
			}
			loc, err := remoteLocation(value)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			extra = append(extra, loc)
		}
		if *right == "" {
			if *bookmarks == "" {
				*bookmarks, err = connections.DefaultPath()
				if err != nil {
					fmt.Fprintln(os.Stderr, err)
					os.Exit(1)
				}
			}
			saved, err := connections.Load(*bookmarks)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			names, discoveryErr := connections.Discover(ctx, *rcloneConfig)
			leftOverride := ""
			flag.Visit(func(f *flag.Flag) {
				if f.Name == "left" {
					leftOverride = local
				}
			})
			startup := newPicker(names, saved, local, leftOverride, *bookmarks, t)
			if discoveryErr != nil {
				startup.message = discoveryErr.Error()
			}
			result, err := tea.NewProgram(startup, tea.WithAltScreen(), tea.WithMouseCellMotion()).Run()
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			choice := result.(picker).selected
			if choice == nil {
				return
			}
			*right, local = choice.Remote+":"+choice.Root, choice.Local
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
		options.Locations = append(options.Locations, extra...)
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
