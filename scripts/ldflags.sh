#!/usr/bin/env bash
# Prints the -ldflags value injecting the build identity into the platform's version package.
set -euo pipefail

PKG=github.com/MyAppLibs/go-pkg/version

# BUILD_VERSION, when set (the version a build workflow is given), wins over git describe.
version=${BUILD_VERSION:-$(git describe --tags --always --dirty)}

# git formats the commit date itself: `date -d` is GNU-only, and this script also runs on macOS.
build_date=$(TZ=UTC git log -1 --date=format-local:%Y-%m-%dT%H:%M:%SZ --format=%cd)

printf -- '-X %s.Version=%s -X %s.Commit=%s -X %s.BuildDate=%s' \
    "$PKG" "$version" \
    "$PKG" "$(git rev-parse --short HEAD)" \
    "$PKG" "$build_date"
