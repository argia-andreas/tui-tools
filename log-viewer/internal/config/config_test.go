package config_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"logviewer/internal/config"
	"logviewer/internal/fixture"
	"logviewer/internal/logs"
)

func TestExplicitLocalSource(t *testing.T) {
	valid := `{"source":"local-elastic","endpoint":"https://127.0.0.1:19200","target":"logview-lab-events","ca_file":"ca","api_key_file":"key","window":{"start":"2026-10-23T14:15:00Z","end":"2026-10-23T14:30:00Z"}}`
	p := filepath.Join(t.TempDir(), "source.json")
	for _, input := range []string{valid, strings.Replace(valid, `"source":`, `"source":"other","source":`, 1), strings.Replace(valid, `"start":`, `"start":"2020-01-01T00:00:00Z","start":`, 1), valid + `{}`, strings.Replace(valid, `"target":`, `"unknown":true,"target":`, 1), strings.Replace(valid, "127.0.0.1", "company.example", 1), strings.Repeat(" ", 65537)} {
		if err := os.WriteFile(p, []byte(input), 0600); err != nil {
			t.Fatal(err)
		}
		_, err := config.Load(p)
		if (err == nil) != (input == valid) {
			t.Fatalf("strict configuration acceptance: %v", err)
		}
	}
}

func TestSavedPredicatesAndOffsetTimes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "private", "searches.json")
	if saved, err := config.LoadSearches(path); err != nil || len(saved) != 0 {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Dir(path)); !os.IsNotExist(err) {
		t.Fatal("read created configuration")
	}
	q := fixture.Query()
	q.Scope.Services = []string{"checkout-api", "payment-worker"}
	q.Levels = []string{"WARN", "ERROR", "FATAL"}
	q.TraceIDs = []string{"8c3e7a21b9f04450"}
	q.Fields = []logs.FieldFilter{{Path: "/fields/order/id", Value: json.RawMessage(`8042`)}}
	entry := config.SavedSearch{Name: "Checkout trace errors", Query: q}
	if err := config.SaveSearch(path, entry); err != nil {
		t.Fatal(err)
	}
	saved, err := config.LoadSearches(path)
	if err != nil || len(saved) != 1 || !reflect.DeepEqual(saved[0], entry) {
		t.Fatal("saved predicate semantics changed", err)
	}
	info, _ := os.Stat(path)
	dir, _ := os.Stat(filepath.Dir(path))
	if info.Mode().Perm() != 0600 || dir.Mode().Perm() != 0700 {
		t.Fatal("searches not private")
	}
	entry.Query.Levels = []string{"UNKNOWN"}
	if err := config.SaveSearch(path, entry); err != nil {
		t.Fatal(err)
	}
	saved, _ = config.LoadSearches(path)
	if len(saved) != 1 || saved[0].Query.Levels[0] != "UNKNOWN" {
		t.Fatal("explicit replace failed")
	}
	b, _ := os.ReadFile(path)
	if strings.Contains(string(b), "Values") || strings.Contains(string(b), "APIKey") || strings.Contains(string(b), "Cursor") {
		t.Fatal("persisted event/auth/session state")
	}
	if err := os.Chmod(path, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := config.LoadSearches(path); err == nil {
		t.Fatal("public searches accepted")
	}
	_ = os.Chmod(path, 0600)
	_ = os.WriteFile(path, []byte(`[ {"Name":"x","Name":"y"} ]`), 0600)
	if _, err := config.LoadSearches(path); err == nil {
		t.Fatal("duplicate saved keys accepted")
	}
	if _, err := config.ParseTime("2026-10-25T02:30:00"); err == nil {
		t.Fatal("ambiguous offset-free Stockholm time accepted")
	}
	first, err := config.ParseTime("2026-10-25T02:30:00+02:00")
	if err != nil {
		t.Fatal(err)
	}
	second, err := config.ParseTime("2026-10-25T02:30:00+01:00")
	if err != nil || second.Sub(first) != time.Hour || first.Location() != time.UTC {
		t.Fatal("explicit DST offsets conflated")
	}
}
