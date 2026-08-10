// Package auth answers two questions for an HTTP transport layer, and keeps
// them apart: who is calling (authentication), and may that caller do this
// (authorization). It knows the identity headers an authenticating reverse
// proxy sets and the upstream those headers may come from; it knows nothing
// about any domain, any storage or any error format, and imports the
// standard library alone — see README.md.
package auth
