package storage

// InvalidSortError marks a client-supplied sort field that doesn't match
// any sortable column, so the router can map it to 400 instead of the 500
// default.
type InvalidSortError struct{ msg string }

func (e *InvalidSortError) Error() string { return e.msg }

// NewInvalidSortError builds an InvalidSortError with the given message.
func NewInvalidSortError(msg string) *InvalidSortError {
	return &InvalidSortError{msg: msg}
}

// NotFoundError marks a storage failure caused by a missing entity (e.g. no
// metadata row for a tableId), so the router can map it to 404 instead of
// the 500 default — distinct from any other failure reading the same row
// (a dropped connection, a cancelled context), which must not be reported
// as "not found".
type NotFoundError struct{ msg string }

func (e *NotFoundError) Error() string { return e.msg }

// NewNotFoundError builds a NotFoundError with the given message.
func NewNotFoundError(msg string) *NotFoundError {
	return &NotFoundError{msg: msg}
}
