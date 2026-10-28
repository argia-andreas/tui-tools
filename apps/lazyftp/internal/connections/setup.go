package connections

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
)

// Draft contains plaintext input. Prepare obscures FTP passwords; rclone's
// obscuring is reversible, not encryption. No values are persisted until Save.
type Draft struct {
	Name, Backend string
	Values        map[string]string
	// SourceConfig optionally supplies the active session snapshot. It is never
	// the save target, and its other session-only remotes are never persisted.
	SourceConfig string
}

// Candidate owns a private session config. Keep it alive while the app uses
// ConfigPath, even after Save, and call Close when the session ends.
type Candidate struct {
	ConfigPath string
	Remote     string
	mu         sync.Mutex
	target     string
	path       string
	remote     string
	stanza     []byte
	closed     bool
}

var setupName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)

// These config keys are taken from rclone help backend ftp/azureblob/s3.
var setupOptions = map[string]map[string]bool{
	"ftp":       {"host": true, "user": true, "pass": true, "port": true, "tls": true, "explicit_tls": true, "disable_tls13": true},
	"azureblob": {"account": true, "key": true, "sas_url": true, "use_az": true, "use_msi": true, "endpoint": true},
	"s3":        {"provider": true, "env_auth": true, "profile": true, "region": true, "endpoint": true, "access_key_id": true, "secret_access_key": true, "force_path_style": true},
}

func setupError(message string) error { return errors.New(message) }

// Prepare reads the existing config without rewriting it, and creates a 0600
// session config containing all existing remotes plus the proposed remote.
func Prepare(ctx context.Context, configPath string, draft Draft) (*Candidate, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if !setupName.MatchString(draft.Name) {
		return nil, setupError("remote name must be 1-64 ASCII letters, digits, underscores or hyphens, starting with a letter or digit")
	}
	allowed, ok := setupOptions[draft.Backend]
	if !ok {
		return nil, setupError("unsupported connection backend")
	}
	values := make(map[string]string, len(draft.Values))
	for k, v := range draft.Values {
		if !allowed[k] {
			return nil, setupError("unsupported connection option")
		}
		// FTP passwords are obscured before INI serialization, so leading or
		// trailing spaces in a password are meaningful and safe to preserve.
		if strings.IndexFunc(v, unicode.IsControl) >= 0 || (strings.TrimSpace(v) != v && !(draft.Backend == "ftp" && k == "pass")) {
			return nil, setupError("connection values cannot contain control characters or surrounding whitespace")
		}
		values[k] = v
	}
	for _, k := range []string{"tls", "explicit_tls", "disable_tls13", "use_az", "use_msi", "env_auth", "force_path_style"} {
		if v := values[k]; v != "" && v != "true" && v != "false" {
			return nil, setupError("connection boolean options must be true or false")
		}
	}
	if draft.Backend == "ftp" {
		if values["host"] == "" {
			return nil, setupError("FTP host is required")
		}
		if values["tls"] == "true" && values["explicit_tls"] == "true" {
			return nil, setupError("choose either implicit or explicit FTPS, not both")
		}
		if v := values["port"]; v != "" {
			n, err := strconv.Atoi(v)
			if err != nil || n < 1 || n > 65535 {
				return nil, setupError("FTP port must be between 1 and 65535")
			}
		}
		if password := values["pass"]; password != "" {
			cmd := setupCommand(ctx, "obscure", "-")
			cmd.Stdin = strings.NewReader(password + "\n")
			out := &setupOutput{limit: 8192}
			cmd.Stdout = out
			if err := cmd.Run(); err != nil {
				return nil, setupError("could not protect FTP password; check rclone installation")
			}
			obscured := strings.TrimSpace(out.String())
			if obscured == "" || strings.ContainsAny(obscured, "\r\n") {
				return nil, setupError("invalid password response from rclone")
			}
			values["pass"] = obscured
		}
	}
	if configPath == "" {
		cmd := setupCommand(ctx, "config", "file")
		out := &setupOutput{limit: 16384}
		cmd.Stdout = out
		if err := cmd.Run(); err != nil {
			return nil, setupError("could not locate rclone config; choose a dedicated config file")
		}
		lines := strings.Split(strings.TrimSpace(out.String()), "\n")
		configPath = strings.TrimSpace(lines[len(lines)-1])
		if !filepath.IsAbs(configPath) {
			return nil, setupError("could not locate rclone config; choose a dedicated config file")
		}
	}
	target, err := filepath.Abs(configPath)
	if err != nil {
		return nil, setupError("invalid config path")
	}
	original, err := readSetupConfig(target, draft.Name)
	if err != nil {
		return nil, err
	}
	if draft.SourceConfig != "" {
		source, pathErr := filepath.Abs(draft.SourceConfig)
		if pathErr != nil {
			return nil, setupError("invalid session config path")
		}
		if _, err := os.Lstat(source); err != nil {
			return nil, setupError("could not read source session config")
		}
		original, err = readSetupConfig(source, draft.Name)
		if err != nil {
			return nil, err
		}
	}
	var stanza strings.Builder
	fmt.Fprintf(&stanza, "[%s]\ntype = %s\n", draft.Name, draft.Backend)
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(&stanza, "%s = %s\n", k, values[k])
	}
	f, err := os.CreateTemp("", "lazyftp-session-*.conf")
	if err != nil {
		return nil, setupError("could not create private session config")
	}
	c := &Candidate{ConfigPath: f.Name(), Remote: draft.Name, target: target, path: f.Name(), remote: draft.Name, stanza: []byte(stanza.String())}
	_, writeErr := f.Write(appendSetupStanza(original, c.stanza))
	closeErr := f.Close()
	if writeErr != nil || closeErr != nil || ctx.Err() != nil {
		c.Close()
		return nil, setupError("could not prepare private session config")
	}
	return c, nil
}

// Override inherited diagnostic destinations/dumps: backend errors may contain
// credential-bearing URLs. Stderr is also discarded by exec's default wiring.
func setupCommand(ctx context.Context, args ...string) *exec.Cmd {
	flags := []string{"--log-file", os.DevNull, "--log-level", "ERROR", "--syslog=false", "--dump", ""}
	// Global flags must precede the positional-argument separator.
	for i, arg := range args {
		if arg == "--" {
			all := append([]string(nil), args[:i]...)
			all = append(all, flags...)
			all = append(all, args[i:]...)
			cmd := exec.CommandContext(ctx, "rclone", all...)
			cmd.WaitDelay = time.Second
			return cmd
		}
	}
	cmd := exec.CommandContext(ctx, "rclone", append(args, flags...)...)
	cmd.WaitDelay = time.Second
	return cmd
}

// setupOutput bounds captured process output, including pathological responses.
type setupOutput struct {
	buffer bytes.Buffer
	limit  int
}

func (b *setupOutput) String() string { return b.buffer.String() }

func (b *setupOutput) Write(p []byte) (int, error) {
	if len(p) > b.limit-b.buffer.Len() {
		return 0, setupError("rclone output limit exceeded")
	}
	return b.buffer.Write(p)
}

// Test performs only a shallow listing. It never creates buckets, directories,
// or objects. Output and backend diagnostics are deliberately not returned.
func (c *Candidate) Test(ctx context.Context, root string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return setupError("connection session is closed")
	}
	if strings.IndexFunc(root, unicode.IsControl) >= 0 {
		return setupError("connection root cannot contain control characters")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	cmd := setupCommand(ctx, "lsf", "--config", c.path, "--max-depth", "1", "--files-only", "--retries", "1", "--low-level-retries", "1", "--contimeout", "10s", "--timeout", "20s", "--", c.remote+":"+root)
	cmd.Stdout = &setupOutput{limit: 64 * 1024}
	if err := cmd.Run(); err != nil {
		return setupError("connection test failed or listing limit reached; check credentials, root and network access")
	}
	return nil
}

func appendSetupStanza(original, stanza []byte) []byte {
	out := append([]byte(nil), original...)
	if len(out) > 0 && out[len(out)-1] != '\n' {
		out = append(out, '\n')
	}
	out = append(out, '\n')
	return append(out, stanza...)
}

// Reject symlinks in the config path (including parent directories). This also
// avoids following a config symlink during atomic replacement.
func checkSetupPath(path string) error {
	for p := path; ; p = filepath.Dir(p) {
		info, err := os.Lstat(p)
		if err != nil && !os.IsNotExist(err) {
			return setupError("could not inspect config path")
		}
		if err == nil && info.Mode()&os.ModeSymlink != 0 {
			return setupError("symlink config paths are not supported; choose a dedicated config file")
		}
		if p == filepath.Dir(p) {
			return nil
		}
	}
}

func readSetupConfig(path, remote string) ([]byte, error) {
	if err := checkSetupPath(path); err != nil {
		return nil, err
	}
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil || !info.Mode().IsRegular() {
		return nil, setupError("config must be a regular readable file")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, setupError("could not read config")
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return nil, setupError("config changed while reading; retry")
	}
	data, err := io.ReadAll(io.LimitReader(f, 8*1024*1024+1))
	if err != nil || len(data) > 8*1024*1024 {
		return nil, setupError("could not read config or config exceeds 8 MiB")
	}
	if bytes.Contains(data, []byte("RCLONE_ENCRYPT_V")) {
		return nil, setupError("encrypted config is not supported; unlock it outside the app or use a dedicated unencrypted config file")
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[") {
			if end := strings.IndexByte(line, ']'); end >= 0 && strings.EqualFold(strings.TrimSpace(line[1:end]), remote) {
				return nil, setupError("a remote with this name already exists; choose another name")
			}
		}
	}
	return data, nil
}

// Save is opt-in. It merges with the current original config, never the stale
// session snapshot. A sidecar exclusive lock coordinates other setup saves;
// external writers must also cooperate to eliminate the final rename race.
func (c *Candidate) Save(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return setupError("connection session is closed")
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := checkSetupPath(c.target); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(c.target), 0700); err != nil {
		return setupError("could not create config directory")
	}
	lockPath := c.target + ".lazyftp.lock"
	for {
		if ctx.Err() != nil {
			return setupError("config save cancelled or lock unavailable")
		}
		lock, err := os.OpenFile(lockPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err == nil {
			lock.Close()
			defer os.Remove(lockPath)
			break
		}
		if !os.IsExist(err) {
			return setupError("could not lock config")
		}
		select {
		case <-ctx.Done():
			return setupError("config save cancelled or lock unavailable")
		case <-time.After(20 * time.Millisecond):
		}
	}
	original, err := readSetupConfig(c.target, c.remote)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(c.target), ".lazyftp-config-*")
	if err != nil {
		return setupError("could not create config save file")
	}
	defer os.Remove(f.Name())
	_, writeErr := f.Write(appendSetupStanza(original, c.stanza))
	syncErr := f.Sync()
	closeErr := f.Close()
	if writeErr != nil || syncErr != nil || closeErr != nil {
		return setupError("could not write config")
	}
	latest, err := readSetupConfig(c.target, c.remote)
	if err != nil {
		return err
	}
	if !bytes.Equal(original, latest) || ctx.Err() != nil {
		return setupError("config changed or save cancelled; retry")
	}
	if err := os.Rename(f.Name(), c.target); err != nil {
		return setupError("could not replace config atomically")
	}
	return nil
}

// Close removes only the session file, never the user's original config.
func (c *Candidate) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	if err := os.Remove(c.path); err != nil && !os.IsNotExist(err) {
		return setupError("could not remove private session config")
	}
	c.closed = true
	return nil
}
