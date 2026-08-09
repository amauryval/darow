package storage

import "context"

// Diagnostics is a snapshot of the storage engine's own operating state —
// on-disk size, buffer-manager memory usage, bytes currently spilled to
// disk, and the extensions loaded in the session. Every field is read live;
// none of it is cached.
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
