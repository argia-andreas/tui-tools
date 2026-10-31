// Package config loads explicit, non-interpolated connection profiles.
package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const maxConfig = 1 << 20

type Config struct {
	Connections []Profile `json:"connections"`
}

type Profile struct {
	Name           string   `json:"name"`
	Brokers        []string `json:"brokers"`
	TLS            bool     `json:"tls"`
	LocalPlaintext bool     `json:"local_plaintext"`
	CAFile         string   `json:"ca_file"`
	CertFile       string   `json:"cert_file"`
	KeyFile        string   `json:"key_file"`
	SASL           string   `json:"sasl"`
	Username       string   `json:"username"`
	PasswordFile   string   `json:"password_file"`
}

func Load(path string) (Config, error) {
	var c Config
	f, err := os.Open(path)
	if err != nil {
		return c, errors.New("cannot open connection configuration")
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, maxConfig+1))
	if err != nil || len(b) > maxConfig {
		return c, errors.New("connection configuration exceeds size limit or cannot be read")
	}
	// Token validation rejects duplicate keys at every nesting level. Decoder's
	// DisallowUnknownFields alone does not reject duplicates.
	d := json.NewDecoder(bytes.NewReader(b))
	if err := uniqueValue(d, 0); err != nil {
		return c, errors.New("invalid or duplicate configuration keys")
	}
	if _, err := d.Token(); err != io.EOF {
		return c, errors.New("configuration must contain one JSON object")
	}
	d = json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err := d.Decode(&c); err != nil || len(c.Connections) == 0 {
		return Config{}, errors.New("invalid connection configuration schema")
	}
	dir, err := filepath.Abs(filepath.Dir(path))
	if err != nil {
		return Config{}, errors.New("invalid configuration directory")
	}
	names := map[string]bool{}
	for i := range c.Connections {
		p := &c.Connections[i]
		if names[p.Name] {
			return Config{}, errors.New("duplicate connection name")
		}
		names[p.Name] = true
		for _, ref := range []*string{&p.CAFile, &p.CertFile, &p.KeyFile, &p.PasswordFile} {
			if *ref != "" && !filepath.IsAbs(*ref) {
				*ref = filepath.Join(dir, *ref)
			}
		}
		if err := p.Validate(); err != nil {
			return Config{}, err
		}
		for _, ref := range []string{p.CAFile, p.CertFile, p.KeyFile, p.PasswordFile} {
			if ref != "" {
				if _, err := ReadPrivateFile(ref); err != nil {
					return Config{}, err
				}
			}
		}
	}
	return c, nil
}

func uniqueValue(d *json.Decoder, depth int) error {
	if depth > 8 {
		return errors.New("configuration nesting too deep")
	}
	t, err := d.Token()
	if err != nil {
		return err
	}
	if t == nil {
		return errors.New("null configuration value")
	}
	switch t {
	case json.Delim('{'):
		seen := map[string]bool{}
		for d.More() {
			k, err := d.Token()
			if err != nil {
				return err
			}
			key, ok := k.(string)
			if !ok || seen[key] {
				return errors.New("duplicate key")
			}
			switch key {
			case "connections", "name", "brokers", "tls", "local_plaintext", "ca_file", "cert_file", "key_file", "sasl", "username", "password_file":
			default:
				return errors.New("unknown key")
			}
			seen[key] = true
			if err := uniqueValue(d, depth+1); err != nil {
				return err
			}
		}
		_, err = d.Token()
	case json.Delim('['):
		for d.More() {
			if err := uniqueValue(d, depth+1); err != nil {
				return err
			}
		}
		_, err = d.Token()
	}
	return err
}

func (c Config) Connection(name string) (Profile, error) {
	for _, p := range c.Connections {
		if p.Name == name {
			p.Brokers = append([]string(nil), p.Brokers...)
			return p, nil
		}
	}
	return Profile{}, errors.New("unknown connection profile")
}

func (c Config) Names() []string {
	names := make([]string, 0, len(c.Connections))
	for _, p := range c.Connections {
		names = append(names, p.Name)
	}
	sort.Strings(names)
	return names
}

func (p Profile) Validate() error {
	if strings.TrimSpace(p.Name) == "" || len(p.Brokers) == 0 || len(p.Brokers) > 32 {
		return errors.New("profile requires a name and 1–32 brokers")
	}
	if p.TLS == p.LocalPlaintext {
		return errors.New("choose verified TLS or explicit local plaintext")
	}
	for _, b := range p.Brokers {
		host, port, err := net.SplitHostPort(b)
		if err != nil || host == "" || port == "" {
			return errors.New("broker must be a host:port address")
		}
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return errors.New("broker port must be 1–65535")
		}
		if p.LocalPlaintext && host != "localhost" && !net.ParseIP(host).IsLoopback() {
			return errors.New("local plaintext requires loopback brokers")
		}
	}
	if (p.CertFile == "") != (p.KeyFile == "") {
		return errors.New("client certificate and key must be supplied together")
	}
	if !p.TLS && (p.CAFile != "" || p.CertFile != "" || p.SASL != "" || p.Username != "" || p.PasswordFile != "") {
		return errors.New("credentials and certificates require TLS")
	}
	switch p.SASL {
	case "":
		if p.Username != "" || p.PasswordFile != "" {
			return errors.New("SASL credentials require an explicit mechanism")
		}
	case "PLAIN", "SCRAM-SHA-256", "SCRAM-SHA-512":
		if p.Username == "" || p.PasswordFile == "" {
			return errors.New("SASL requires username and private password file")
		}
	default:
		return errors.New("unsupported SASL mechanism")
	}
	return nil
}

// ReadPrivateFile never includes a path or file contents in an error. All path
// components must be nonsymlinks, and the final regular file must be owner-only.
func ReadPrivateFile(path string) ([]byte, error) {
	fail := errors.New("credential file must be private, regular, nonsymlink and at most 1 MiB")
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fail
	}
	for p := abs; ; p = filepath.Dir(p) {
		st, err := os.Lstat(p)
		if err != nil || st.Mode()&os.ModeSymlink != 0 {
			return nil, fail
		}
		if p == filepath.Dir(p) {
			break
		}
	}
	before, err := os.Lstat(abs)
	if err != nil || !before.Mode().IsRegular() || before.Mode().Perm()&0077 != 0 {
		return nil, fail
	}
	f, err := os.Open(abs)
	if err != nil {
		return nil, fail
	}
	defer f.Close()
	after, err := f.Stat()
	if err != nil || !os.SameFile(before, after) || !after.Mode().IsRegular() || after.Mode().Perm()&0077 != 0 {
		return nil, fail
	}
	b, err := io.ReadAll(io.LimitReader(f, maxConfig+1))
	if err != nil || len(b) > maxConfig {
		return nil, fail
	}
	return b, nil
}
