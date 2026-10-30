// Package editor runs an unmodified Neovim and synchronizes remote working copies.
package editor

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"time"

	"lazyftp/internal/storage"
)

// Command implements Bubble Tea's ExecCommand. Run owns the working directory
// until all uploads finish; failed sessions retain it for manual recovery.
type Command struct {
	ctx            context.Context
	transferer     storage.Transferer
	location       storage.Location
	stdin          io.Reader
	stdout, stderr io.Writer
}

func New(ctx context.Context, transferer storage.Transferer, location storage.Location) *Command {
	return &Command{ctx: ctx, transferer: transferer, location: location, stdin: os.Stdin, stdout: os.Stdout, stderr: os.Stderr}
}

func (c *Command) SetStdin(r io.Reader)  { c.stdin = r }
func (c *Command) SetStdout(w io.Writer) { c.stdout = w }
func (c *Command) SetStderr(w io.Writer) { c.stderr = w }

func (c *Command) Run() error {
	bin, err := exec.LookPath("nvim")
	if err != nil {
		return fmt.Errorf("Neovim is not installed or nvim is not on PATH")
	}
	if c.location.Remote == "" {
		file, err := filepath.Abs(c.location.Path)
		if err != nil {
			return err
		}
		info, err := os.Stat(file)
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("only regular files can be edited")
		}
		return c.process(bin, file).Run()
	}

	baseline, err := c.transferer.Stat(c.ctx, c.location)
	if err != nil {
		return err
	}
	if baseline.IsDir {
		return fmt.Errorf("only files can be edited")
	}
	dir, err := os.MkdirTemp("", "lazyftp-edit-*")
	if err != nil {
		return err
	}
	keep := false
	defer func() {
		if !keep {
			os.RemoveAll(dir)
		}
	}()
	// Keep Neovim swap/backup files beside the working copy, and upload only
	// snapshots of this exact filename (never swap files or an entire folder).
	work := filepath.Join(dir, path.Base(c.location.Path))
	if path.Base(c.location.Path) == "." || path.Base(c.location.Path) == ".." || path.Base(c.location.Path) == "/" {
		return fmt.Errorf("choose a file to edit")
	}
	if err := c.transferer.Copy(c.ctx, c.location, storage.Location{Path: work}, false); err != nil {
		return err
	}
	if err := os.Chmod(work, 0600); err != nil {
		return err
	}
	saved, err := os.ReadFile(work)
	if err != nil {
		return err
	}
	current, err := c.transferer.Stat(c.ctx, c.location)
	if err != nil {
		return err
	}
	if !sameVersion(baseline, current) {
		return fmt.Errorf("remote changed during download; reopen the file")
	}

	cmd := c.process(bin, work)
	if err := cmd.Start(); err != nil {
		return err
	}
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()
	ticker := time.NewTicker(150 * time.Millisecond)
	defer ticker.Stop()
	var pending []byte
	var syncErr error
	// A snapshot stays unchanged throughout Copy, even if another :w occurs.
	snapshot := filepath.Join(dir, ".upload")
	if work == snapshot {
		snapshot = filepath.Join(dir, ".upload-snapshot")
	}
	sync := func(final bool) error {
		data, err := os.ReadFile(work)
		if err != nil {
			// Atomic saves can briefly remove the pathname. Retry on the next
			// sample, but never silently ignore an unreadable file on exit.
			if !final {
				pending = nil
				return nil
			}
			return err
		}
		if bytes.Equal(data, saved) {
			pending = nil
			return nil
		}
		if !final && (pending == nil || !bytes.Equal(data, pending)) {
			pending = data
			return nil
		}
		current, err := c.transferer.Stat(c.ctx, c.location)
		if err != nil {
			return err
		}
		if !sameVersion(baseline, current) {
			return fmt.Errorf("remote changed; automatic upload stopped to avoid overwriting it")
		}
		if err := os.WriteFile(snapshot, data, 0600); err != nil {
			return err
		}
		if err := c.transferer.Copy(c.ctx, storage.Location{Path: snapshot}, c.location, true); err != nil {
			return err
		}
		baseline, err = c.transferer.Stat(c.ctx, c.location)
		if err != nil {
			return err
		}
		if baseline.IsDir || baseline.Size != int64(len(data)) {
			return fmt.Errorf("remote changed during upload")
		}
		saved, pending = data, nil
		return nil
	}
	for {
		select {
		case <-ticker.C:
			if syncErr == nil {
				syncErr = sync(false)
			}
		case editorErr := <-exited:
			if syncErr == nil {
				syncErr = sync(true)
			}
			if syncErr != nil || editorErr != nil {
				keep = true
				if syncErr != nil {
					return fmt.Errorf("working copy kept at %s; remote save failed: %w", work, syncErr)
				}
				return fmt.Errorf("working copy kept at %s; Neovim failed: %w", work, editorErr)
			}
			return nil
		}
	}
}

func (c *Command) process(bin, file string) *exec.Cmd {
	cmd := exec.CommandContext(c.ctx, bin, "--", file)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = c.stdin, c.stdout, c.stderr
	return cmd
}

// FTP has no atomic compare-and-swap. This is best-effort conflict detection,
// not protection against a writer racing the check or matching these metadata.
func sameVersion(a, b storage.Entry) bool {
	return a.IsDir == b.IsDir && a.Size == b.Size && a.Modified.Equal(b.Modified)
}
