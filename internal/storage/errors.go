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
