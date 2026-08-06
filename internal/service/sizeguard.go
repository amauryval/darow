package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"darow/internal/storage"
)

// QuotaExceededError marks a write refused because the database is at or
// over its configured size limit (duckdb.maxSizeBytes) — the storage layer
// raised nothing, the write was never attempted.
type QuotaExceededError struct{ msg string }

func (e *QuotaExceededError) Error() string { return e.msg }

// sizeGuard is what a data-growing write (a row insert, a source import, a
// table derivation) checks before writing and reports to afterwards.
// Declared here, the implementer, since RowService/ImportService/
// DeriveService each need only this narrow slice of SizeGuardService.
type sizeGuard interface {
	Check(ctx context.Context) error
	Grew()
}

// SizeGuardService enforces duckdb.maxSizeBytes: a hard cap on the
// database's on-disk size, config-only and never exposed through
// PATCH /settings — an operator decision about the disk the process runs
// on, not a per-request tuning knob.
//
// The size is read live only occasionally: Grew, called after every
// data-growing write completes, invalidates the cached value so the next
// Check re-reads it; ttl is a backstop for the rare case a stale cache
// outlives its last invalidation. Between those, Check answers off the
// cache — the cap is enforced with at most one write's worth of drift,
// never with a pragma_database_size query on every single write.
type SizeGuardService struct {
	reader     storage.DiagnosticsReader
	limitBytes int64
	ttl        time.Duration

	mu        sync.Mutex
	size      int64
	checkedAt time.Time
}

// NewSizeGuardService builds a SizeGuardService reading the database's size
// through reader. limitBytes <= 0 disables the guard entirely (Check always
// succeeds) — duckdb.maxSizeBytes' zero value.
func NewSizeGuardService(reader storage.DiagnosticsReader, limitBytes int64, ttl time.Duration) *SizeGuardService {
	return &SizeGuardService{reader: reader, limitBytes: limitBytes, ttl: ttl}
}

// Check refuses a data-growing write once the database's last-known size is
// at or over limitBytes, refreshing that value first if it is stale.
func (s *SizeGuardService) Check(ctx context.Context) error {
	if s.limitBytes <= 0 {
		return nil
	}

	size, err := s.currentSize(ctx)
	if err != nil {
		return NewDisplayError("unable to check the database size limit", err)
	}
	if size >= s.limitBytes {
		return &QuotaExceededError{msg: fmt.Sprintf(
			"database size (%d bytes) is at or over the configured limit of %d bytes", size, s.limitBytes,
		)}
	}
	return nil
}

// Grew invalidates the cached size, so the next Check re-reads it instead
// of waiting out ttl.
func (s *SizeGuardService) Grew() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.checkedAt = time.Time{}
}

func (s *SizeGuardService) currentSize(ctx context.Context) (int64, error) {
	s.mu.Lock()
	stale := time.Since(s.checkedAt) > s.ttl
	size := s.size
	s.mu.Unlock()
	if !stale {
		return size, nil
	}

	d, err := s.reader.Diagnostics(ctx)
	if err != nil {
		return 0, err
	}

	s.mu.Lock()
	s.size = d.DatabaseSizeBytes
	s.checkedAt = time.Now()
	s.mu.Unlock()
	return d.DatabaseSizeBytes, nil
}
