package storage

import "context"

// Diagnostics is a snapshot of the storage engine's own operating state: its
// on-disk size, memory usage, spilled bytes and loaded extensions. Every field
// is read live, none is cached.
type Diagnostics struct {
	DatabaseSizeBytes   int64
	MemoryUsageBytes    int64
	TemporaryFilesBytes int64
	Extensions          []string
}

// DiagnosticsReader answers Diagnostics. Implemented by duckdb.Adapter.
type DiagnosticsReader interface {
	Diagnostics(ctx context.Context) (Diagnostics, error)
}
