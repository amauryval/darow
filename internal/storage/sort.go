package storage

import "darow/internal/storage/sort"

// SortField is one column of a multi-column ORDER BY. Re-exported from the
// dependency-free sort package so both storage and catalog can use the
// same type without a storage<->catalog import cycle.
type SortField = sort.SortField

// ParseSort parses a comma-separated `sort` query value (e.g.
// "tableName,-updatedAt") into an ordered list of SortFields — a leading
// "-" marks a column descending. Empty segments are skipped; "" yields nil,
// letting the caller fall back to its own default order.
func ParseSort(raw string) []SortField {
	return sort.ParseSort(raw)
}
