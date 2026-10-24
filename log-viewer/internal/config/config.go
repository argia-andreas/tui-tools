// Package config loads explicit source configuration, never ambient credentials.
package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"logviewer/internal/logs"
)

type Config struct {
	Source     string `json:"source"`
	Endpoint   string `json:"endpoint"`
	Target     string `json:"target"`
	CAFile     string `json:"ca_file"`
	APIKeyFile string `json:"api_key_file"`
	Window     struct {
		Start time.Time `json:"start"`
		End   time.Time `json:"end"`
	} `json:"window"`
}

// Load rejects duplicate keys as well as unknown fields and trailing values.
func Load(path string) (Config, error) {
	var c Config
	f, err := os.Open(path)
	if err != nil {
		return c, fmt.Errorf("cannot open source configuration")
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, (64<<10)+1))
	if err != nil || len(b) > 64<<10 {
		return c, fmt.Errorf("source configuration exceeds read limit")
	}
	d := json.NewDecoder(bytes.NewReader(b))
	if err := unique(d); err != nil {
		return c, fmt.Errorf("invalid source configuration")
	}
	if _, err := d.Token(); err != io.EOF {
		return c, fmt.Errorf("trailing source configuration")
	}
	d = json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err := d.Decode(&c); err != nil {
		return Config{}, fmt.Errorf("invalid source configuration")
	}
	// This slice deliberately supports only the owned local lab. Do not apply
	// fictional-data policy to an unidentified production source.
	if c.Source != "local-elastic" || c.Endpoint != "https://127.0.0.1:19200" || c.Target != "logview-lab-events" || c.CAFile == "" || c.APIKeyFile == "" || !c.Window.End.After(c.Window.Start) {
		return Config{}, fmt.Errorf("invalid source scope, trust or window")
	}
	return c, nil
}

func unique(d *json.Decoder) error {
	t, err := d.Token()
	if err != nil {
		return err
	}
	if delim, ok := t.(json.Delim); ok {
		switch delim {
		case '{':
			seen := map[string]bool{}
			for d.More() {
				key, err := d.Token()
				if err != nil {
					return err
				}
				k, ok := key.(string)
				if !ok || seen[k] {
					return fmt.Errorf("duplicate key")
				}
				seen[k] = true
				if err := unique(d); err != nil {
					return err
				}
			}
		case '[':
			for d.More() {
				if err := unique(d); err != nil {
					return err
				}
			}
		default:
			return fmt.Errorf("unexpected delimiter")
		}
		_, err = d.Token()
	}
	return err
}

func (c Config) Query() logs.Query {
	return logs.Query{Scope: logs.Scope{Source: c.Source}, Window: logs.Window{Start: c.Window.Start, End: c.Window.End}, Limits: logs.Limits{Events: 500, Bytes: 32 << 20}}
}

// The local lab uses the same explicit fictional-data output policy as fixtures.
// Production policy configuration and schema discovery are not this slice.
func (c Config) Policy() logs.OutputPolicy {
	return logs.OutputPolicy{Source: c.Source, Fields: []string{"time", "level", "service", "instance", "message", "trace.id", "fields"}, RedactPaths: []string{"/fields/auth/token"}, Patterns: []string{`fixture-secret-[A-Za-z0-9-]+`}}
}
