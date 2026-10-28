// Package mcp exposes a deliberately small, default-deny LazyFTP MCP server.
package mcp

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

var errDenied = errors.New("operation denied by MCP policy")

// Policy is independent of rclone credentials and the interactive application's
// state. One complete grant must authorize an operation; grants are not combined.
type Policy struct {
	Grants []Grant `json:"grants"`
}

type Grant struct {
	Operation  string   `json:"operation"`
	Remote     string   `json:"remote"`
	Root       string   `json:"root"`
	LocalRoots []string `json:"local_roots,omitempty"`
}

// ReadPolicy rejects misspelled fields and trailing JSON. Empty grants deny all.
func ReadPolicy(r io.Reader) (Policy, error) {
	var p Policy
	data, err := io.ReadAll(io.LimitReader(r, (1<<20)+1))
	if err != nil || len(data) > 1<<20 {
		return Policy{}, errors.New("cannot read MCP policy (maximum 1 MiB)")
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(&p); err != nil {
		return Policy{}, errors.New("invalid MCP policy JSON")
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return Policy{}, errors.New("invalid trailing MCP policy data")
	}
	if err := p.validate(); err != nil {
		return Policy{}, err
	}
	return p, nil
}

// Restrict names to ordinary named rclone remotes, never connection strings,
// on-the-fly backends, flags, paths, or drive letters.
var remoteName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{1,63}$`)

func safeText(s string) bool {
	return !strings.ContainsAny(s, `\:`) && !strings.ContainsFunc(s, unicode.IsControl)
}

func remotePath(s string) bool {
	return strings.HasPrefix(s, "/") && safeText(s) && path.Clean(s) == s
}

func localPath(s string) bool {
	return filepath.IsAbs(s) && safeText(s) && filepath.Clean(s) == s
}

func within(root, name string) bool {
	return name == root || strings.HasPrefix(name, strings.TrimSuffix(root, "/")+"/")
}

func (p Policy) validate() error {
	for _, g := range p.Grants {
		if (g.Operation != "list" && g.Operation != "upload" && g.Operation != "download") ||
			!remoteName.MatchString(g.Remote) || !remotePath(g.Root) ||
			(g.Operation == "list" && len(g.LocalRoots) != 0) ||
			(g.Operation != "list" && len(g.LocalRoots) == 0) {
			return errors.New("invalid MCP grant: require operation, named remote, canonical absolute root and transfer local_roots")
		}
		for _, root := range g.LocalRoots {
			if !localPath(root) || checkLocal(root, false, true) != nil {
				return errors.New("invalid MCP local root: require existing absolute directory without symlink components")
			}
		}
	}
	return nil
}

func (p Policy) allows(operation, remote, name, local string) bool {
	if !remoteName.MatchString(remote) || !remotePath(name) || (operation != "list" && !localPath(local)) {
		return false
	}
	for _, g := range p.Grants {
		if g.Operation != operation || g.Remote != remote || !within(g.Root, name) {
			continue
		}
		if operation == "list" {
			return true
		}
		for _, root := range g.LocalRoots {
			if local != root && within(root, local) {
				return true
			}
		}
	}
	return false
}

// checkLocal checks every component, including ancestors above the grant root.
// Only a missing final download filename is allowed. This is a snapshot check,
// NOT race-proof confinement: rclone later reopens paths by name.
func checkLocal(name string, missingLeaf, directory bool) error {
	if !localPath(name) {
		return errDenied
	}
	current := string(filepath.Separator)
	parts := strings.Split(strings.TrimPrefix(name, current), current)
	if name == current {
		if !directory {
			return errDenied
		}
		parts = nil
	}
	for i, part := range parts {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		last := i == len(parts)-1
		if errors.Is(err, os.ErrNotExist) && last && missingLeaf {
			return nil
		}
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			return errDenied
		}
		if !last || directory {
			if !info.IsDir() {
				return errDenied
			}
		} else if !info.Mode().IsRegular() {
			return errDenied
		}
	}
	return nil
}

// ProtectConfig prevents granting access to the credential file or a containing
// directory. Call before constructing the backend; symlinked config is rejected.
func (p Policy) ProtectConfig(name string) error {
	if !localPath(name) || checkLocal(name, false, false) != nil {
		return errors.New("rclone config must be an existing absolute regular file without symlinks")
	}
	for _, g := range p.Grants {
		for _, root := range g.LocalRoots {
			if within(root, name) {
				return errors.New("rclone credential config must be outside all MCP local roots")
			}
		}
	}
	return nil
}
