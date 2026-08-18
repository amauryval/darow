package storage

// InvalidSortError marks a client-supplied sort field matching no sortable
// column, so the router maps it to 400 instead of 500.
type InvalidSortError struct{ msg string }

func (e *InvalidSortError) Error() string { return e.msg }

// NewInvalidSortError builds an InvalidSortError with the given message.
func NewInvalidSortError(msg string) *InvalidSortError {
	return &InvalidSortError{msg: msg}
}

// ValidationError marks a storage write rejected because of the data the
// caller supplied, so the router maps it to 400 instead of 500.
type ValidationError struct{ msg string }

func (e *ValidationError) Error() string { return e.msg }

// NewValidationError builds a ValidationError with the given message.
func NewValidationError(msg string) *ValidationError {
	return &ValidationError{msg: msg}
}

// NotFoundError marks a storage failure caused by a missing entity, so the
// router maps it to 404 instead of 500. Any other failure reading the same
// row must never be reported as "not found".
type NotFoundError struct{ msg string }

func (e *NotFoundError) Error() string { return e.msg }

// NewNotFoundError builds a NotFoundError with the given message.
func NewNotFoundError(msg string) *NotFoundError {
	return &NotFoundError{msg: msg}
}
