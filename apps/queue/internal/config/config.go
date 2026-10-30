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
	"unicode/utf8"

	"servicebusexplorer/internal/explorer"
	"servicebusexplorer/internal/theme"
)

// Preferences is the version 1 on-disk contract. An absent profile watches all
// discovered resources; a present, empty list explicitly watches none.
type Preferences struct {
	Version                int                            `json:"version"`
	Theme                  string                         `json:"theme"`
	Sidebar                bool                           `json:"sidebar"`
	Inspector              bool                           `json:"inspector"`
	Table                  bool                           `json:"table"`
	Watchlists             map[string][]explorer.Resource `json:"watchlists"`
	Environments           []Environment                  `json:"environments,omitzero"`
	ActiveEnvironment      string                         `json:"activeEnvironment,omitempty"`
	InitializedConnections []string                       `json:"initializedConnections,omitempty"`
}

// Environment saves explicit discovery scope separately from dashboard inclusion.
type Environment struct {
	ID         string               `json:"id"`
	Name       string               `json:"name"`
	Connection string               `json:"connection"`
	Namespaces []explorer.Namespace `json:"namespaces"`
	All        bool                 `json:"all"`
	Watchlist  []explorer.Resource  `json:"watchlist"`
}

// Clone isolates saved snapshots, including absent versus explicitly empty lists.
func (p Preferences) Clone() Preferences {
	if p.Watchlists != nil {
		lists := make(map[string][]explorer.Resource, len(p.Watchlists))
		for key, list := range p.Watchlists {
			lists[key] = cloneSlice(list)
		}
		p.Watchlists = lists
	}
	p.Environments = cloneSlice(p.Environments)
	for i := range p.Environments {
		p.Environments[i].Namespaces = cloneSlice(p.Environments[i].Namespaces)
		p.Environments[i].Watchlist = cloneSlice(p.Environments[i].Watchlist)
	}
	p.InitializedConnections = cloneSlice(p.InitializedConnections)
	return p
}

func cloneSlice[T any](in []T) []T {
	if in == nil {
		return nil
	}
	return append(make([]T, 0, len(in)), in...)
}

// EnsureLocal migrates legacy local inclusion once per connection. A persisted
// empty environment list is deliberate, not a request to recreate defaults.
func EnsureLocal(p Preferences, connection, name string, namespace explorer.Namespace) Preferences {
	p = p.Clone()
	for _, migrated := range p.InitializedConnections {
		if migrated == connection {
			return p
		}
	}
	for _, e := range p.Environments {
		if e.Connection == connection {
			p.InitializedConnections = append(p.InitializedConnections, connection)
			return p
		}
	}
	if p.Environments != nil && len(p.Environments) == 0 {
		return p
	}
	list, selected := p.Watchlists[connection]
	p.Environments = append(p.Environments, Environment{
		ID: connection, Name: strings.TrimSpace(name), Connection: connection,
		Namespaces: []explorer.Namespace{namespace}, All: !selected,
		Watchlist: cloneSlice(list),
	})
	p.InitializedConnections = append(p.InitializedConnections, connection)
	if p.ActiveEnvironment == "" {
		p.ActiveEnvironment = connection
	}
	return p
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
			if !validManagementIdentity(r.AzureSubscriptionID, r.ResourceGroup) ||
				(profile == "emulator" && r.AzureSubscriptionID != "") ||
				(profile == "fixture" && strings.Contains(strings.ToLower(r.Namespace), ".servicebus.")) {
				return errors.New("invalid watchlist management identity")
			}
			if seen[r] {
				return errors.New("duplicate watchlist resource")
			}
			seen[r] = true
		}
	}
	ids := make(map[string]bool)
	for i, e := range p.Environments {
		if !identityPart(e.ID) || ids[e.ID] || !identityPart(e.Name) ||
			utf8.RuneCountInString(e.Name) > 40 ||
			!identityPart(e.Connection) || len(e.Namespaces) == 0 {
			return errors.New("invalid environment identity or name")
		}
		for _, previous := range p.Environments[:i] {
			if strings.EqualFold(e.Name, previous.Name) {
				return errors.New("duplicate environment name")
			}
		}
		ids[e.ID] = true
		addresses := make(map[string]bool)
		for _, ns := range e.Namespaces {
			address := strings.ToLower(ns.Address)
			if ns.Profile != e.Connection || !validNamespace(ns) || addresses[address] {
				return errors.New("invalid or duplicate environment namespace")
			}
			addresses[address] = true
		}
		// Reuse the legacy entity identity rules, but nil is a valid empty
		// Selected list and excluded namespaces retain their selections.
		if len(e.Watchlist) > 0 {
			legacy := Default()
			legacy.Watchlists = map[string][]explorer.Resource{e.Connection: e.Watchlist}
			if err := validate(legacy); err != nil {
				return errors.New("invalid environment watchlist")
			}
		}
	}
	for _, connection := range p.InitializedConnections {
		if !identityPart(connection) {
			return errors.New("invalid initialized connection")
		}
	}
	if p.ActiveEnvironment != "" && !identityPart(p.ActiveEnvironment) {
		return errors.New("invalid active environment identity")
	}
	return nil
}

func validNamespace(ns explorer.Namespace) bool {
	if !safeNamespacePart(ns.Address, true) || !identityPart(ns.Name) {
		return false
	}
	if !validManagementIdentity(ns.AzureSubscriptionID, ns.ResourceGroup) ||
		(ns.AzureSubscriptionName != "" && (ns.AzureSubscriptionID == "" || !identityPart(ns.AzureSubscriptionName))) {
		return false
	}
	local := ns.Profile == "fixture" || ns.Profile == "emulator"
	cloud := strings.Contains(strings.ToLower(ns.Address), ".servicebus.")
	if local {
		// Offline fixture catalogs may carry fictional management metadata;
		// they still cannot address a real cloud messaging endpoint.
		return !cloud && (ns.Profile == "fixture" || ns.AzureSubscriptionID == "")
	}
	if !safeNamespacePart(ns.Name, false) || strings.ContainsAny(ns.Name, "._") {
		return false
	}
	for _, suffix := range []string{".servicebus.windows.net", ".servicebus.usgovcloudapi.net", ".servicebus.chinacloudapi.cn", ".servicebus.cloudapi.de"} {
		if strings.EqualFold(ns.Address, ns.Name+suffix) {
			return true
		}
	}
	return false
}

func validManagementIdentity(subscription, group string) bool {
	if subscription == "" && group == "" {
		return true
	}
	if !safeNamespacePart(subscription, false) || !identityPart(group) || len(group) > 90 || group == "." || group == ".." {
		return false
	}
	for _, r := range group {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || strings.ContainsRune("_-().", r) {
			continue
		}
		return false
	}
	return true
}

func safeNamespacePart(s string, address bool) bool {
	if !identityPart(s) {
		return false
	}
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' ||
			(address && (r == ':' || r == '[' || r == ']')) {
			continue
		}
		return false
	}
	return true
}

func identityPart(s string) bool {
	return s != "" && strings.TrimSpace(s) == s && !strings.ContainsFunc(s, unicode.IsControl)
}
