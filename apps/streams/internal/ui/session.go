package ui

import (
	"context"
	"sync"

	"streams/internal/stream"
)

// session is shared across Bubble Tea's model copies. Polls never hold mu,
// so Pause can cancel/close an in-flight poll without waiting for its deadline.
type session struct {
	mu               sync.Mutex
	op               sync.Mutex
	gen              uint64
	reader           stream.Reader
	cancel           context.CancelFunc
	current, pending stream.Backend
	connectionGen    uint64
	connectionCancel context.CancelFunc
	closed           bool
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

// beginConnection retires an unconsumed result, but keeps current usable until
// a new connection has opened successfully and its message is accepted.
func (s *session) beginConnection(gen uint64, cancel context.CancelFunc) {
	s.mu.Lock()
	previous, pending := s.connectionCancel, s.pending
	s.connectionGen, s.connectionCancel, s.pending = gen, cancel, nil
	closed := s.closed
	s.mu.Unlock()
	if previous != nil {
		previous()
	}
	if pending != nil {
		pending.Close()
	}
	if closed {
		cancel()
	}
}

// stageConnection transfers ownership before delivering the message: quit may
// happen after OpenConnection returns but before Bubble Tea consumes its result.
func (s *session) stageConnection(gen uint64, b stream.Backend) bool {
	s.mu.Lock()
	if s.closed || gen != s.connectionGen {
		s.mu.Unlock()
		b.Close()
		return false
	}
	s.pending = b
	s.mu.Unlock()
	return true
}

func (s *session) commitConnection(gen uint64) (stream.Backend, bool) {
	s.mu.Lock()
	if s.closed || gen != s.connectionGen || s.pending == nil {
		s.mu.Unlock()
		return nil, false
	}
	previous, current := s.current, s.pending
	s.current, s.pending = current, nil
	s.mu.Unlock()
	previous.Close()
	return current, true
}

func (s *session) close() {
	s.mu.Lock()
	s.closed = true
	current, pending, cancel := s.current, s.pending, s.connectionCancel
	s.current, s.pending, s.connectionCancel = nil, nil, nil
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	s.invalidate()
	s.op.Lock()
	defer s.op.Unlock()
	if current != nil {
		current.Close()
	}
	if pending != nil {
		pending.Close()
	}
}
