package filters

import "github.com/MyAppLibs/go-pkg/document"

// FilterID names a filter.
type FilterID string

// Filter is a named SQL condition on the table it was saved on, or on any table (Table empty).
type Filter struct {
	document.Meta[FilterID] `tstype:",extends"`
	Table                   string `db:"table_name" json:"table"`
	Name                    string `db:"name" json:"name"`
	Where                   string `db:"where_sql" json:"where"`
}
