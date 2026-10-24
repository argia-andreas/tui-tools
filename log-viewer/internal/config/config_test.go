package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"logviewer/internal/config"
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
