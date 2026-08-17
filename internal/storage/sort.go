package storage

import "strings"

// SortField is one column of a multi-column ORDER BY.
type SortField struct {
	Column string
	Desc   bool
}

// ParseSort parses a comma-separated `sort` query value ("tableName,-updatedAt")
// into an ordered list of SortFields, a leading "-" marking a column
// descending. An empty value yields nil, for the caller's own default order.
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
