package service

import "fmt"

// ValidationError marks a service failure caused by the request itself
// (malformed input, an unsupported option) rather than a storage or
// internal failure, so the router can map it to 400 instead of the 500
// default — the same role ExportValidationError already plays
// for Export.
type ValidationError struct{ msg string }

func (e *ValidationError) Error() string { return e.msg }

// NotFoundError marks a service failure caused by a missing entity (an
// unknown source, layer, or table) so the router can map it to 404 instead
// of the 500 default.
type NotFoundError struct{ msg string }

func (e *NotFoundError) Error() string { return e.msg }

// ConflictError marks a service failure caused by state that already
// exists (e.g. a source already loaded into a table) so the router can map
// it to 409 instead of the 500 default.
type ConflictError struct{ msg string }

func (e *ConflictError) Error() string { return e.msg }

// DisplayError wraps an internal failure with a message meant to be shown
// as-is, for the case where a service builds a clean, deliberate message
// about an otherwise messy underlying cause (a multi-step rollback, a
// DuckDB error whose own text is too internal to show) and wants that
// message displayed instead of the cause's raw text. It carries no status
// of its own: response.WriteError still classifies the wrapped cause
// through MapServiceError's normal dispatch (Unwrap keeps errors.As/
// errors.Is working through it), and only substitutes DisplayMessage for
// what reaches the client.
type DisplayError struct {
	message string
	cause   error
}

// NewDisplayError builds a DisplayError, message reaching the client and
// cause staying in the server-side log for diagnosis.
func NewDisplayError(message string, cause error) *DisplayError {
	return &DisplayError{message: message, cause: cause}
}

func (e *DisplayError) Error() string { return fmt.Sprintf("%s: %v", e.message, e.cause) }
func (e *DisplayError) Unwrap() error { return e.cause }

// DisplayMessage returns the message meant to be shown, without the
// wrapped cause's own detail.
func (e *DisplayError) DisplayMessage() string { return e.message }
