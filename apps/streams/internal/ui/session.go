package ui

import (
	"context"
	"sync"

	"streams/internal/stream"
)

// session is shared across Bubble Tea's model copies. Polls never hold mu,
// so Pause can cancel/close an in-flight poll without waiting for its deadline.
type session struct {
	mu       sync.Mutex
	op       sync.Mutex
	gen      uint64
	reader   stream.Reader
	cancel   context.CancelFunc
	backends []stream.Backend
	closed   bool
}

func (s *session) invalidate() uint64 {
	s.mu.Lock()
	s.gen++
	gen, cancel, reader := s.gen, s.cancel, s.reader
	s.reader, s.cancel = nil, nil
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if reader != nil {
		reader.Close()
	}
	return gen
}

func (s *session) open(ctx context.Context, backend stream.Backend, gen uint64, q stream.ReadRequest) (stream.Batch, error) {
	s.op.Lock()
	defer s.op.Unlock()
	ctx, cancel := context.WithCancel(ctx)
	s.mu.Lock()
	if s.closed || s.gen != gen {
		s.mu.Unlock()
		cancel()
		return stream.Batch{}, stream.ErrClosed
	}
	s.cancel = cancel
	s.mu.Unlock()
	r, err := backend.Open(ctx, q)
	s.mu.Lock()
	if s.closed || s.gen != gen || err != nil {
		s.mu.Unlock()
		cancel()
		if r != nil {
			r.Close()
		}
		return stream.Batch{}, err
	}
	s.reader = r
	s.mu.Unlock()
	return r.Next(ctx), nil
}

func (s *session) next(ctx context.Context, gen uint64) stream.Batch {
	s.op.Lock()
	defer s.op.Unlock()
	s.mu.Lock()
	r := s.reader
	valid := s.gen == gen && !s.closed
	s.mu.Unlock()
	if !valid || r == nil {
		return stream.Batch{Stop: "error"}
	}
	return r.Next(ctx)
}

func (s *session) add(b stream.Backend) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		b.Close()
		return false
	}
	s.backends = append(s.backends, b)
	return true
}

func (s *session) close() {
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()
	s.invalidate()
	s.op.Lock()
	defer s.op.Unlock()
	s.mu.Lock()
	backends := s.backends
	s.backends = nil
	s.mu.Unlock()
	for _, b := range backends {
		b.Close()
	}
}
