package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"darow/internal/storage"
)

// QuotaExceededError marks a write refused because the database is at or over
// duckdb.maxSizeBytes: the write was never attempted.
type QuotaExceededError struct{ msg string }

func (e *QuotaExceededError) Error() string { return e.msg }

// sizeGuard is what a data-growing write checks before writing and reports to
// afterwards.
type sizeGuard interface {
	Check(ctx context.Context) error
	Grew()
}

// SizeGuardService enforces duckdb.maxSizeBytes, a config-only hard cap on
// the database's on-disk size. Check answers off a cached size, which Grew
// invalidates after every write and ttl expires as a backstop.
type SizeGuardService struct {
	reader     storage.DiagnosticsReader
	limitBytes int64
	ttl        time.Duration

	mu        sync.Mutex
	size      int64
	checkedAt time.Time
}

// NewSizeGuardService builds a SizeGuardService reading the database's size
// through reader. A limitBytes <= 0 disables the guard entirely.
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
