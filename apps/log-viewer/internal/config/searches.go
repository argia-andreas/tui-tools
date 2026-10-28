package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"logviewer/internal/logs"
)

type SavedSearch struct {
	Name  string
	Query logs.Query
}

// ParseTime requires an explicit UTC Z or numeric offset. An offset-free local
// time (including an ambiguous Stockholm DST overlap) is never guessed.
func ParseTime(text string) (time.Time, error) {
	text = strings.TrimSpace(text)
	if !regexp.MustCompile(`(Z|[+-](0[0-9]|1[0-9]|2[0-3]):[0-5][0-9])$`).MatchString(text) {
		return time.Time{}, fmt.Errorf("use RFC3339 time with Z or explicit offset")
	}
	t, err := time.Parse(time.RFC3339Nano, text)
	if err != nil {
		return time.Time{}, fmt.Errorf("use RFC3339 time with Z or explicit offset")
	}
	return t.UTC(), nil
}

func validateSaved(s SavedSearch) error {
	if len(s.Name) < 1 || len(s.Name) > 80 || logs.Clean(s.Name) != s.Name || strings.ContainsRune(s.Name, '\n') || strings.TrimSpace(s.Name) != s.Name {
		return fmt.Errorf("invalid saved search name")
	}
	if s.Query.Scope.Source != "fixture" && s.Query.Scope.Source != "local-elastic" {
		return fmt.Errorf("invalid saved source")
	}
	return logs.ValidateQuery(s.Query)
}

func private(info os.FileInfo) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && stat.Uid == uint32(os.Getuid()) && info.Mode().Perm()&0077 == 0
}

// LoadSearches is read-only. Neither startup nor ordinary searches create a file.
func LoadSearches(path string) ([]SavedSearch, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil || !info.Mode().IsRegular() || !private(info) {
		return nil, fmt.Errorf("saved searches must be a private owned regular file")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read saved searches")
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, (256<<10)+1))
	if err != nil || len(b) > 256<<10 {
		return nil, fmt.Errorf("saved searches exceed read budget")
	}
	d := json.NewDecoder(bytes.NewReader(b))
	if unique(d) != nil {
		return nil, fmt.Errorf("invalid saved searches")
	}
	if _, err := d.Token(); err != io.EOF {
		return nil, fmt.Errorf("trailing saved search data")
	}
	var saved []SavedSearch
	d = json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if d.Decode(&saved) != nil || len(saved) > 50 {
		return nil, fmt.Errorf("invalid saved searches")
	}
	names := map[string]bool{}
	for _, entry := range saved {
		if validateSaved(entry) != nil || names[entry.Name] {
			return nil, fmt.Errorf("invalid or duplicate saved search")
		}
		names[entry.Name] = true
	}
	return saved, nil
}

// SaveSearch runs only on an explicit named save. The schema contains predicates
// and absolute times, never event bodies, endpoints, credentials or native cursors.
func SaveSearch(path string, entry SavedSearch) error {
	if err := validateSaved(entry); err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("cannot create private searches directory")
	}
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() || !private(info) {
		return fmt.Errorf("searches directory must be private and owned")
	}
	saved, err := LoadSearches(path)
	if err != nil {
		return err
	}
	replaced := false
	for i := range saved {
		if saved[i].Name == entry.Name {
			saved[i] = entry
			replaced = true
		}
	}
	if !replaced {
		saved = append(saved, entry)
	}
	if len(saved) > 50 {
		return fmt.Errorf("saved search limit reached")
	}
	b, err := json.MarshalIndent(saved, "", "  ")
	if err != nil || len(b)+1 > 256<<10 {
		return fmt.Errorf("saved searches exceed write budget")
	}
	f, err := os.CreateTemp(dir, ".searches-*")
	if err != nil {
		return fmt.Errorf("cannot create private saved searches")
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err = f.Write(append(b, '\n')); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil || closeErr != nil {
		return fmt.Errorf("cannot write saved searches")
	}
	if os.Rename(name, path) != nil {
		return fmt.Errorf("cannot replace saved searches")
	}
	return nil
}
