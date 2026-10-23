// Package connections discovers remote names and persists opt-in, non-secret bookmarks.
// It never reads rclone's configuration contents.
package connections

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"lazyftp/internal/storage"
)

type Bookmark struct {
	Name   string `json:"name"`
	Remote string `json:"remote"`
	Root   string `json:"root"`
	Local  string `json:"local"`
}

// ParseRemote accepts named remotes, not inline backend definitions or URLs.
func ParseRemote(value string) (storage.Location, error) {
	remote, root, ok := strings.Cut(value, ":")
	if !ok || strings.TrimSpace(remote) == "" || strings.HasPrefix(remote, "-") || strings.ContainsAny(remote, "/\\:") {
		return storage.Location{}, fmt.Errorf("use an existing rclone remote:path (for example lab-ftp:/incoming)")
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return storage.Location{}, fmt.Errorf("connection paths cannot contain control characters")
		}
	}
	return storage.Location{Remote: remote, Path: "/" + strings.TrimPrefix(root, "/")}, nil
}

// Discover asks rclone only for remote names. Config contents and backend stderr
// are never read or returned; environment-based rclone configuration still works.
func Discover(ctx context.Context, config string) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	args := []string{"listremotes"}
	if config != "" {
		args = append(args, "--config", config)
	}
	data, err := exec.CommandContext(ctx, "rclone", args...).Output()
	if err != nil {
		return nil, fmt.Errorf("could not discover rclone remotes; check rclone configuration/unlock settings")
	}
	var names []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		loc, err := ParseRemote(line)
		if err != nil || loc.Path != "/" {
			return nil, fmt.Errorf("invalid remote-name response from rclone")
		}
		names = append(names, loc.Remote)
	}
	sort.Strings(names)
	return names, nil
}

func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	return filepath.Join(dir, "lazyftp", "connections.json"), err
}

func (b Bookmark) Validate() error {
	if strings.TrimSpace(b.Name) == "" {
		return fmt.Errorf("enter a bookmark name before saving")
	}
	loc, err := ParseRemote(b.Remote + ":" + b.Root)
	if err != nil {
		return err
	}
	if loc.Remote != b.Remote {
		return fmt.Errorf("bookmark remote must be a name, not a path")
	}
	for _, s := range []string{b.Name, b.Local} {
		for _, r := range s {
			if unicode.IsControl(r) {
				return fmt.Errorf("bookmark fields cannot contain control characters")
			}
		}
	}
	if !filepath.IsAbs(b.Local) {
		return fmt.Errorf("bookmark local folder must be absolute")
	}
	return nil
}

func Load(path string) ([]Bookmark, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read bookmarks: %w", err)
	}
	var bookmarks []Bookmark
	if err := json.Unmarshal(data, &bookmarks); err != nil {
		return nil, fmt.Errorf("invalid bookmarks JSON")
	}
	names := map[string]bool{}
	for _, b := range bookmarks {
		if err := b.Validate(); err != nil {
			return nil, fmt.Errorf("invalid bookmark: %w", err)
		}
		if names[b.Name] {
			return nil, fmt.Errorf("duplicate bookmark name")
		}
		names[b.Name] = true
	}
	return bookmarks, nil
}

// Save explicitly replaces the same-named bookmark, retaining the others. A
// same-directory rename prevents a partial write from destroying the old file.
func Save(path string, bookmark Bookmark) error {
	if err := bookmark.Validate(); err != nil {
		return err
	}
	bookmarks, err := Load(path)
	if err != nil {
		return err
	}
	found := false
	for i := range bookmarks {
		if bookmarks[i].Name == bookmark.Name {
			bookmarks[i], found = bookmark, true
		}
	}
	if !found {
		bookmarks = append(bookmarks, bookmark)
	}
	data, err := json.MarshalIndent(bookmarks, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("create bookmark directory: %w", err)
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".connections-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(append(data, '\n')); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
