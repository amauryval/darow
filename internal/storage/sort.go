package storage

import "strings"

// SortField is one column of a multi-column ORDER BY.
type SortField struct {
	Column string
	Desc   bool
}

// ParseSort parses a comma-separated `sort` query value (e.g.
// "tableName,-updatedAt") into an ordered list of SortFields — a leading
// "-" marks a column descending. Empty segments are skipped; "" yields nil,
// letting the caller fall back to its own default order.
func ParseSort(raw string) []SortField {
	if raw == "" {
		return nil
	}

	var fields []SortField
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if desc := strings.HasPrefix(part, "-"); desc {
			fields = append(fields, SortField{Column: part[1:], Desc: true})
		} else {
			fields = append(fields, SortField{Column: part, Desc: false})
		}
	}
	return fields
}
