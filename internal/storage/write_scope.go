package storage

// WriteScope names the writes that exclude one another: two writes sharing
// a scope run one after the other, two writes on different scopes run in
// parallel.
type WriteScope string

// GlobalWriteScope covers the writes that belong to no single table — the
// settings singleton row.
const GlobalWriteScope WriteScope = "__global"

// TxWriteScope is named by the writes that already run inside a
// caller-owned transaction, which holds the real scope for them.
const TxWriteScope WriteScope = "__tx"

// TableWriteScope covers every write concerning one table: its rows, its
// structure, its catalogue row and the rows attached to it.
func TableWriteScope(tableId string) WriteScope {
	return WriteScope(tableId)
}
