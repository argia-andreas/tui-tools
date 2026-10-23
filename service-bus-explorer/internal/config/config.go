// Package config persists only nonsecret explorer preferences.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"servicebusexplorer/internal/explorer"
	"servicebusexplorer/internal/theme"
)

// Preferences is the version 1 on-disk contract. An absent profile watches all
// discovered resources; a present, empty list explicitly watches none.
type Preferences struct {
	Version    int                            `json:"version"`
	Theme      string                         `json:"theme"`
	Sidebar    bool                           `json:"sidebar"`
	Inspector  bool                           `json:"inspector"`
	Table      bool                           `json:"table"`
	Watchlists map[string][]explorer.Resource `json:"watchlists"`
}

func Default() Preferences {
	return Preferences{Version: 1, Theme: "midnight", Sidebar: true, Inspector: true}
}

func Path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "service-bus-explorer", "preferences.json"), nil
}

// Load returns defaults only for a missing file, never for damaged preferences.
func Load(path string) (Preferences, error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return Default(), nil
	}
	if err != nil {
		return Preferences{}, err
	}
	defer f.Close()
	var p Preferences
	d := json.NewDecoder(f)
	d.DisallowUnknownFields()
	if err := d.Decode(&p); err != nil {
		// Do not echo unknown fields or file contents, which may contain secrets.
		return Preferences{}, errors.New("invalid preferences JSON")
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		return Preferences{}, errors.New("unexpected trailing preferences data")
	}
	if err := validate(p); err != nil {
		return Preferences{}, err
	}
	return p, nil
}

// Save validates both the new value and any existing file before replacing it.
// The temporary file is private and is removed on every unsuccessful write.
func Save(path string, p Preferences) error {
	if err := validate(p); err != nil {
		return err
	}
	if _, err := Load(path); err != nil {
		return fmt.Errorf("refusing to replace invalid preferences: %w", err)
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return errors.New("cannot encode preferences")
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".preferences-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if _, err := f.Write(append(data, '\n')); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}

func validate(p Preferences) error {
	if p.Version != 1 {
		return errors.New("unsupported preferences version")
	}
	if _, err := theme.Get(p.Theme); err != nil || p.Theme == "" {
		return errors.New("invalid preferences theme")
	}
	for profile, resources := range p.Watchlists {
		if !identityPart(profile) || resources == nil {
			return errors.New("invalid watchlist profile or null list")
		}
		seen := make(map[explorer.Resource]bool)
		for _, r := range resources {
			if r.Profile != profile || !identityPart(r.Namespace) ||
				(r.Queue == "" && r.Topic == "") ||
				(r.Queue != "" && (r.Topic != "" || r.Subscription != "")) {
				return errors.New("invalid watchlist resource identity")
			}
			for _, part := range []string{r.Queue, r.Topic, r.Subscription} {
				if part != "" && !identityPart(part) {
					return errors.New("invalid watchlist resource identity")
				}
			}
			if seen[r] {
				return errors.New("duplicate watchlist resource")
			}
			seen[r] = true
		}
	}
	return nil
}

func identityPart(s string) bool {
	return s != "" && strings.TrimSpace(s) == s && !strings.ContainsFunc(s, unicode.IsControl)
}
