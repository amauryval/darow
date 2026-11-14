package engine

// RowQuery is a page of a table's rows: the rows passing Where (a SQL condition), ordered by Sort (the row
// number when empty), descending when Desc.
type RowQuery struct {
	Where  string
	Sort   string
	Desc   bool
	Offset int
	Limit  int
}
