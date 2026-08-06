package service

import (
	"context"

	"darow/internal/storage"
)

// DiagnosticsService exposes the storage engine's own operating state — the
// read that tells "the server is up" from "the server is healthy". It owns
// no cache: every call reads live, since a stale spike would misreport the
// one thing a caller is asking about.
type DiagnosticsService struct {
	reader storage.DiagnosticsReader
}

// NewDiagnosticsService builds a DiagnosticsService backed by reader.
func NewDiagnosticsService(reader storage.DiagnosticsReader) *DiagnosticsService {
	return &DiagnosticsService{reader: reader}
}

// Diagnostics returns the current snapshot.
func (s *DiagnosticsService) Diagnostics(ctx context.Context) (storage.Diagnostics, error) {
	d, err := s.reader.Diagnostics(ctx)
	if err != nil {
		return storage.Diagnostics{}, NewDisplayError("unable to read diagnostics", err)
	}
	return d, nil
}
