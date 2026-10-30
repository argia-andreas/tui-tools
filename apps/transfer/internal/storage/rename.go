package storage

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

var _ Renamer = (*Rclone)(nil)

func (r *Rclone) Rename(ctx context.Context, source, destination Location) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if source.Remote != destination.Remote {
		return fmt.Errorf("rename requires the same remote")
	}
	// Validate before cleaning: a trailing slash, '.' or '..' is not a file
	// basename, even if cleaning would turn it into one.
	for _, loc := range []Location{source, destination} {
		name := path.Base(loc.Path)
		if loc.Path == "" || strings.HasSuffix(loc.Path, "/") || name == "." || name == ".." || strings.ContainsAny(name, "\\\x00") || strings.ContainsRune(loc.Path, '\x00') {
			return fmt.Errorf("rename requires valid file basenames")
		}
	}
	if source.Remote == "" {
		var err error
		source.Path, err = filepath.Abs(source.Path)
		if err != nil {
			return err
		}
		destination.Path, err = filepath.Abs(destination.Path)
		if err != nil {
			return err
		}
	} else {
		source.Path = path.Clean("/" + source.Path)
		destination.Path = path.Clean("/" + destination.Path)
	}
	if path.Dir(source.Path) != path.Dir(destination.Path) {
		return fmt.Errorf("rename requires the same directory")
	}
	if source == destination {
		return fmt.Errorf("source and destination are the same: %w", fs.ErrExist)
	}
	src, err := r.Stat(ctx, source)
	if err != nil {
		return err
	}
	if src.IsDir {
		return fmt.Errorf("only regular files can be renamed")
	}
	if destination.Remote == "" {
		// Lstat also detects dangling symlinks, which must count as collisions.
		_, err = os.Lstat(destination.Path)
	} else {
		_, err = r.Stat(ctx, destination)
	}
	if err == nil {
		return fs.ErrExist
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if source.Remote == "" {
		// Unlike os.Rename, link creation cannot replace a concurrently created
		// destination. Both names are in the same directory/filesystem. A remove
		// failure leaves both names intact rather than losing file contents.
		if err := os.Link(source.Path, destination.Path); err != nil {
			return err
		}
		if err := os.Remove(source.Path); err != nil {
			return fmt.Errorf("rename may have partly succeeded; refresh both paths before retrying: %w", err)
		}
		return nil
	}
	// --immutable is insufficient: moveto can delete the source when the
	// destination has identical bytes. --ignore-existing explicitly preserves
	// the source on a skipped move (rclone operations.moveOrCopyFile).
	_, err = r.run(ctx, 10*time.Minute, "moveto", []string{"--ignore-existing"}, source, destination)
	if err != nil {
		return fmt.Errorf("rename may have partly succeeded; refresh both paths before retrying: %w", err)
	}
	// A skipped --ignore-existing move exits successfully. Never report it as
	// a successful rename. Also detect incomplete copy/delete backend moves.
	_, sourceErr := r.Stat(ctx, source)
	dst, destinationErr := r.Stat(ctx, destination)
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("rename outcome is uncertain; it may have partly succeeded; refresh both paths before retrying: %w", err)
	}
	if sourceErr == nil && destinationErr == nil {
		return fmt.Errorf("rename destination exists; move skipped or partly succeeded; refresh both paths before retrying: %w", fs.ErrExist)
	}
	if !errors.Is(sourceErr, fs.ErrNotExist) || destinationErr != nil || dst.IsDir || dst.Size != src.Size {
		return fmt.Errorf("rename outcome is uncertain; it may have partly succeeded; refresh both paths before retrying")
	}
	return nil
}
