package duckdb

import (
	"sync"

	"darow/internal/storage"
)

// writeLocks hands out one mutex per storage.WriteScope, so writes to
// different tables run in parallel while writes to the same one stay
// serialized. A scope's entry is dropped once nobody awaits it.
type writeLocks struct {
	mu      sync.Mutex
	entries map[storage.WriteScope]*writeLockEntry
}

type writeLockEntry struct {
	mu       sync.Mutex
	refCount int
}

// acquire blocks until scope is free and returns the function releasing it.
func (l *writeLocks) acquire(scope storage.WriteScope) func() {
	l.mu.Lock()
	if l.entries == nil {
		l.entries = make(map[storage.WriteScope]*writeLockEntry)
	}
	entry, ok := l.entries[scope]
	if !ok {
		entry = &writeLockEntry{}
		l.entries[scope] = entry
	}
	entry.refCount++
	l.mu.Unlock()

	entry.mu.Lock()

	return func() {
		entry.mu.Unlock()

		l.mu.Lock()
		defer l.mu.Unlock()
		entry.refCount--
		if entry.refCount == 0 {
			delete(l.entries, scope)
		}
	}
}
