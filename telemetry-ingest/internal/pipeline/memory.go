package pipeline

import (
	"context"
	"sync"
)

type MemoryWriter struct {
	mu       sync.Mutex
	Messages []ValidatedMessage
}

func (w *MemoryWriter) WriteLatest(_ context.Context, message ValidatedMessage) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.Messages = append(w.Messages, message)
	return nil
}

func (w *MemoryWriter) Count() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return len(w.Messages)
}

type MemoryFailureSink struct {
	mu       sync.Mutex
	Failures []IngestFailure
}

func (s *MemoryFailureSink) RecordFailure(_ context.Context, failure IngestFailure) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Failures = append(s.Failures, failure)
	return nil
}

func (s *MemoryFailureSink) Count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.Failures)
}
