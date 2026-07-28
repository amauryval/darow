package service

// ValidationError marks a service failure caused by the request itself
// (malformed input, an unsupported option) rather than a storage or
// internal failure, so the router can map it to 400 instead of the 500
// default (NEW-SEC-1) — the same role ExportValidationError already plays
// for Export.
type ValidationError struct{ msg string }

func (e *ValidationError) Error() string { return e.msg }

// NotFoundError marks a service failure caused by a missing entity (an
// unknown source, layer, or table) so the router can map it to 404 instead
// of the 500 default (NEW-SEC-1).
type NotFoundError struct{ msg string }

func (e *NotFoundError) Error() string { return e.msg }

// ConflictError marks a service failure caused by state that already
// exists (e.g. a source already loaded into a table) so the router can map
// it to 409 instead of the 500 default (NEW-SEC-1).
type ConflictError struct{ msg string }

func (e *ConflictError) Error() string { return e.msg }
