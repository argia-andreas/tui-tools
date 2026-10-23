package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"time"
)

// Reader optionally supplies raw file prefixes. Read returns at most limit
// bytes, without decoding or sanitizing them. Zero returns empty without I/O;
// negative limits fail. Callers should choose a small preview-sized limit.
type Reader interface {
	Read(context.Context, Location, int64) ([]byte, error)
}

var _ Reader = (*Rclone)(nil)

// Read bounds both captured output and memory to O(limit), independently of
// whether a remote backend honors --head. It never relays backend stderr.
func (r *Rclone) Read(ctx context.Context, loc Location, limit int64) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if limit < 0 {
		return nil, fmt.Errorf("read limit must not be negative")
	}
	if limit == 0 {
		return []byte{}, nil
	}
	if loc.Remote == "" {
		entry, err := r.Stat(ctx, loc)
		if err != nil {
			return nil, err
		}
		if entry.IsDir {
			return nil, fmt.Errorf("only regular files can be read")
		}
		file, err := os.Open(loc.Path)
		if err != nil {
			return nil, err
		}
		defer file.Close()
		info, err := file.Stat()
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("only regular files can be read")
		}
		return readPrefix(ctx, file, limit)
	}

	caller := ctx
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	args := []string{"cat", "--head", strconv.FormatInt(limit, 10), "--retries", "1", "--low-level-retries", "1", "--contimeout", "5s", "--timeout", "30s"}
	if r.config != "" {
		args = append(args, "--config", r.config)
	}
	args = append(args, "--", loc.String())
	cmd := exec.CommandContext(ctx, "rclone", args...)
	// Stderr defaults to the null device, not an unbounded capture buffer.
	cmd.WaitDelay = time.Second
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("cannot open rclone preview pipe")
	}
	if err := cmd.Start(); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("cannot start rclone preview (check rclone installation)")
	}
	// Closing the pipe also unblocks Read if a backend child inherits stdout.
	stop := context.AfterFunc(ctx, func() { stdout.Close() })
	defer stop()
	data, readErr := readPrefix(ctx, stdout, limit)
	contextErr := ctx.Err()
	// A full prefix is successful even if the backend keeps producing bytes.
	// Stop it before Wait so neither output nor process lifetime is unbounded.
	full := int64(len(data)) == limit && readErr == nil
	if full || readErr != nil {
		cancel()
		stdout.Close()
	}
	waitErr := cmd.Wait()
	if caller.Err() != nil {
		return nil, caller.Err()
	}
	if contextErr != nil {
		return nil, contextErr
	}
	if !full && readErr == nil && ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if readErr != nil || (!full && waitErr != nil) {
		return nil, fmt.Errorf("rclone preview failed (check connection, file path and permissions)")
	}
	return data, nil
}

func readPrefix(ctx context.Context, source io.Reader, limit int64) ([]byte, error) {
	buffer := make([]byte, min(limit, 32*1024))
	data := []byte{}
	for int64(len(data)) < limit {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		n, err := source.Read(buffer[:min(int64(len(buffer)), limit-int64(len(data)))])
		data = append(data, buffer[:n]...)
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if err == io.EOF {
			return data, nil
		}
		if err != nil {
			return nil, err
		}
	}
	return data, nil
}
