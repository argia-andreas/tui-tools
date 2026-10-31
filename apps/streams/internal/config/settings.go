package config

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type Settings struct {
	Sidebar     bool `json:"sidebar"`
	Inspector   bool `json:"inspector"`
	ASCII       bool `json:"ascii"`
	Comfortable bool `json:"comfortable"`
}

func DefaultSettings() Settings { return Settings{Sidebar: true, Inspector: true} }

func LoadSettings(path string) (Settings, error) {
	s := DefaultSettings()
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return s, fmt.Errorf("settings unavailable")
	}
	defer f.Close()
	d := json.NewDecoder(io.LimitReader(f, 4097))
	d.DisallowUnknownFields()
	if d.Decode(&s) != nil {
		return DefaultSettings(), fmt.Errorf("invalid settings")
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return DefaultSettings(), fmt.Errorf("invalid trailing settings")
	}
	return s, nil
}

func SaveSettings(path string, s Settings) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("cannot create settings directory")
	}
	if info, err := os.Lstat(path); err == nil && (!info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0) {
		return fmt.Errorf("settings file must be private and regular")
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".streams-settings-")
	if err != nil {
		return fmt.Errorf("cannot save settings")
	}
	defer os.Remove(f.Name())
	err = json.NewEncoder(f).Encode(s)
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(f.Name(), path)
	}
	if err != nil {
		return fmt.Errorf("cannot save settings")
	}
	return nil
}
