// Package version holds the build identity: Version and Commit are set at
// build time via -ldflags -X, never edited by hand.
package version

// Version is the git tag/describe output the binary was built from
// ("dev" when built without ldflags, e.g. `go run`).
var Version = "dev"

// Commit is the short git commit SHA the binary was built from
// ("unknown" when built without ldflags).
var Commit = "unknown"

// BuildDate is the UTC build timestamp, RFC 3339
// ("unknown" when built without ldflags).
var BuildDate = "unknown"
