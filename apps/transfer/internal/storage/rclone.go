package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"sort"
	"strings"
	"time"
)

// Rclone uses existing rclone remotes. Credentials stay in rclone's config,
// never command arguments or UI logs. No shell interprets paths or filenames.
type Rclone struct{ config string }

func NewRclone(config string) *Rclone { return &Rclone{config: config} }

func (l Location) String() string {
	if l.Remote != "" {
		return l.Remote + ":" + l.Path
	}
	return l.Path
}

func (r *Rclone) run(ctx context.Context, timeout time.Duration, command string, flags []string, locations ...Location) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	args := []string{command, "--retries", "1", "--low-level-retries", "1", "--contimeout", "5s", "--timeout", "30s"}
	if r.config != "" {
		args = append(args, "--config", r.config)
	}
	args = append(args, flags...)
	args = append(args, "--")
	for _, loc := range locations {
		args = append(args, loc.String())
	}
	data, err := exec.CommandContext(ctx, "rclone", args...).Output()
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if err != nil {
		var exit *exec.ExitError
		if command == "lsjson" && errors.As(err, &exit) && (exit.ExitCode() == 3 || exit.ExitCode() == 4) {
			return nil, fmt.Errorf("path not found: %w", fs.ErrNotExist)
		}
		// Backend stderr may contain credential-bearing URLs; do not relay it.
		return nil, fmt.Errorf("rclone %s failed: %w (check connection and permissions)", command, err)
	}
	return data, nil
}

type rcloneEntry struct {
	Name    string
	Size    int64
	ModTime time.Time
	IsDir   bool
}

func (e rcloneEntry) entry() Entry {
	return Entry{Name: e.Name, Size: e.Size, Modified: e.ModTime, IsDir: e.IsDir}
}

func (r *Rclone) List(ctx context.Context, loc Location) ([]Entry, error) {
	if loc.Remote == "" {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		items, err := os.ReadDir(loc.Path)
		if err != nil {
			return nil, err
		}
		entries := make([]Entry, 0, len(items))
		for _, item := range items {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			info, err := item.Info()
			if err != nil {
				return nil, err
			}
			entries = append(entries, Entry{Name: item.Name(), IsDir: info.IsDir(), Size: info.Size(), Modified: info.ModTime()})
		}
		sort.Slice(entries, func(i, j int) bool {
			if entries[i].IsDir != entries[j].IsDir {
				return entries[i].IsDir
			}
			return entries[i].Name < entries[j].Name
		})
		return entries, nil
	}
	data, err := r.run(ctx, time.Minute, "lsjson", nil, loc)
	if err != nil {
		return nil, err
	}
	var items []rcloneEntry
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, fmt.Errorf("invalid rclone listing: %w", err)
	}
	entries := make([]Entry, len(items))
	for i, e := range items {
		entries[i] = e.entry()
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir
		}
		return entries[i].Name < entries[j].Name
	})
	return entries, nil
}

func (r *Rclone) Stat(ctx context.Context, loc Location) (Entry, error) {
	if err := ctx.Err(); err != nil {
		return Entry{}, err
	}
	if loc.Remote == "" {
		e, err := os.Lstat(loc.Path)
		if err != nil {
			return Entry{}, err
		}
		if !e.IsDir() && !e.Mode().IsRegular() {
			return Entry{}, fmt.Errorf("only regular files and directories are supported")
		}
		return Entry{Name: e.Name(), Size: e.Size(), Modified: e.ModTime(), IsDir: e.IsDir()}, nil
	}
	// --stat reports nonexistent Blob paths as synthetic directories and FTP
	// missing parents as generic 501 errors. Walk actual listings instead.
	clean := path.Clean("/" + loc.Path)
	current := Location{Remote: loc.Remote, Path: "/"}
	if clean == "/" {
		_, err := r.List(ctx, current)
		return Entry{Name: "/", IsDir: true}, err
	}
	parts := strings.Split(strings.TrimPrefix(clean, "/"), "/")
	for i, name := range parts {
		entries, err := r.List(ctx, current)
		if err != nil {
			return Entry{}, err
		}
		var found *Entry
		for _, e := range entries {
			if e.Name == name {
				entry := e
				found = &entry
				break
			}
		}
		if found == nil {
			return Entry{}, fs.ErrNotExist
		}
		if i == len(parts)-1 {
			return *found, nil
		}
		if !found.IsDir {
			return Entry{}, fs.ErrNotExist
		}
		current.Path = path.Join(current.Path, name)
	}
	return Entry{}, fs.ErrNotExist
}

func (r *Rclone) Delete(ctx context.Context, loc Location) error {
	entry, err := r.Stat(ctx, loc)
	if err != nil {
		return err
	}
	if entry.IsDir {
		return fmt.Errorf("choose a file, not a directory, to delete")
	}
	// deletefile targets exactly one object; never use recursive delete or purge.
	_, err = r.run(ctx, time.Minute, "deletefile", nil, loc)
	return err
}

func (r *Rclone) Copy(ctx context.Context, source, destination Location, overwrite bool) error {
	if source == destination {
		return fmt.Errorf("source and destination are the same")
	}
	if source.Remote != "" && destination.Remote != "" {
		return fmt.Errorf("remote-to-remote copies are not supported yet")
	}
	src, err := r.Stat(ctx, source)
	if err != nil {
		return err
	}
	if src.IsDir {
		return fmt.Errorf("folder transfers are not supported yet")
	}
	dst, err := r.Stat(ctx, destination)
	if err == nil {
		if dst.IsDir {
			return fmt.Errorf("destination is a directory")
		}
		if !overwrite {
			return fs.ErrExist
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	flags := []string{"--ignore-times"}
	if !overwrite {
		flags = append(flags, "--immutable")
	}
	_, err = r.run(ctx, 10*time.Minute, "copyto", flags, source, destination)
	return err
}
