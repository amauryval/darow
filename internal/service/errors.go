package service

import "fmt"

// ValidationError marks a service failure caused by the request itself,
// rather than by storage, so the router maps it to 400 instead of 500.
type ValidationError struct{ msg string }

func (e *ValidationError) Error() string { return e.msg }

// NotFoundError marks a service failure caused by a missing entity, so the
// router maps it to 404 instead of 500.
type NotFoundError struct{ msg string }

func (e *NotFoundError) Error() string { return e.msg }

// ConflictError marks a service failure caused by state that already exists,
// so the router maps it to 409 instead of 500.
type ConflictError struct{ msg string }

func (e *ConflictError) Error() string { return e.msg }

// DisplayError wraps an internal failure with a message meant to be shown
// as-is instead of the cause's raw text. It carries no status of its own:
// the wrapped cause still decides it, through Unwrap.
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
