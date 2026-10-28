package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Settings contains preferences only, never events, credentials or session state.
type Settings struct {
	VimMode bool `json:"vim_mode"`
}

func DefaultSettings() Settings { return Settings{VimMode: true} }

// LoadSettings is read-only, including when the file does not exist.
func LoadSettings(path string) (Settings, error) {
	settings := DefaultSettings()
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return settings, nil
	}
	if err != nil || !info.Mode().IsRegular() || !private(info) {
		return settings, fmt.Errorf("settings must be a private owned regular file")
	}
	f, err := os.Open(path)
	if err != nil {
		return settings, fmt.Errorf("cannot read settings")
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, (64<<10)+1))
	if err != nil || len(b) > 64<<10 {
		return settings, fmt.Errorf("settings exceed read budget")
	}
	d := json.NewDecoder(bytes.NewReader(b))
	if unique(d) != nil {
		return settings, fmt.Errorf("invalid settings")
	}
	if _, err := d.Token(); err != io.EOF {
		return settings, fmt.Errorf("trailing settings data")
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(b, &fields) != nil || fields == nil {
		return settings, fmt.Errorf("invalid settings object")
	}
	for key, value := range fields {
		if key != "vim_mode" || (!bytes.Equal(value, []byte("true")) && !bytes.Equal(value, []byte("false"))) {
			return DefaultSettings(), fmt.Errorf("invalid settings field")
		}
		settings.VimMode = bytes.Equal(value, []byte("true"))
	}
	return settings, nil
}

// SaveSettings atomically replaces a private file on an explicit preference save.
func SaveSettings(path string, settings Settings) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("cannot create private settings directory")
	}
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() || !private(info) {
		return fmt.Errorf("settings directory must be private and owned")
	}
	info, err = os.Lstat(path)
	if !os.IsNotExist(err) && (err != nil || !info.Mode().IsRegular() || !private(info)) {
		return fmt.Errorf("settings must be a private owned regular file")
	}
	b, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("cannot encode settings")
	}
	f, err := os.CreateTemp(dir, ".settings-*")
	if err != nil {
		return fmt.Errorf("cannot create private settings")
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err = f.Write(append(b, '\n')); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil || closeErr != nil {
		return fmt.Errorf("cannot write settings")
	}
	if os.Rename(name, path) != nil {
		return fmt.Errorf("cannot replace settings")
	}
	return nil
}
