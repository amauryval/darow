---
name: build-version
description:
    'Wires or audits a build-time version/commit/date identity for a Go service and its embedded or
    companion frontend: ldflags injection, a public version endpoint, and a permanent display in the
    UI. Use whenever asked to add, fix, or check "the app version", "build info", "commit SHA
    display", or why a shown version is stale.'
---

## When to Use This Skill

-   Adding a version/build-identity mechanism to a service for the first time
-   Auditing an existing one: is it still wired end to end, or did a build path drift out of it
-   A shown version is wrong, frozen, or missing after a build/deploy change (new task, new
    Dockerfile stage, a CI pipeline)
-   Porting the same mechanism to another Go (or comparable compiled-binary) project

## The shape, independent of any one project

A build identity mechanism has exactly four links, and every one of them must be live for the value
shown to be trustworthy — a mechanism that stops at three (the endpoint exists but nothing calls it
at build time, say) silently freezes at whatever the vars default to:

1. **A version package** — a handful of string variables (`Version`, `Commit`, `BuildDate` at
   minimum), each defaulted to something self-evidently synthetic (`"dev"`, `"unknown"`) so an
   unlinked build is obviously not a real one. Never a file that gets hand-edited or committed per
   release — the whole point is removing that step. `BuildDate` is the _commit's_ date
   (`git log -1 --format=%cI`, normalized to UTC), never wall-clock build time: rebuilding the same
   commit a year later must still stamp the original date, not the rebuild's — a value that moves on
   every rebuild of unchanged code is worse than no value, since it looks like freshness that isn't
   there.
2. **Injection at every build path that produces a real artifact** —
   `-ldflags "-X pkg/path.Var=value"` for Go, computed from the actual tree being built, wired into
   _every_ task that builds a real binary (`make build`, `mise run build`, a plain `go build` your
   CI calls, whatever this repo's task runner is) and into the container build. A dev-loop task
   (`go run`, a watch mode) can carry the same flags too for consistency, but is not the one that
   matters — what ships is.
3. **A read endpoint** — a route outside authentication (this is operational metadata, not a secret)
   that serves the three fields as JSON, next to whatever health/readiness probes already exist.
   Reuse that handler family rather than inventing a new one.
4. **A permanent display** — surfaced in the UI shell (not a page you have to navigate to) so it is
   seen without being asked for, fetched once and shared (not refetched by every component that
   wants it) since it cannot change while the process is running.

## The two build-context problems, generically

-   **`go run` / dev loop**: usually fine to inject the same ldflags computed from git inline — no
    separate mechanism needed, since git is right there.
-   **Container build**: the `.git` directory is normally excluded from the build context (check
    `.dockerignore`/`.containerignore`) for image-size and cache-correctness reasons, so
    `git describe`/`git rev-parse` cannot run _inside_ the Dockerfile. The value must travel in from
    the _caller_ as build args (`ARG VERSION=dev`, `ARG COMMIT=unknown`, `ARG BUILD_DATE=unknown`,
    passed with `--build-arg` computed from git on the host or CI runner, right before
    `docker build`). Do not try to reconstruct git state inside the image — that is fighting the
    dockerignore rather than working with it. If the CI job only ever runs on a tagged commit,
    `VERSION` can just be that tag (already in hand as a workflow output) instead of re-deriving it
    with `git describe`.

Watch the quoting when the three `$(...)` substitutions are assembled into one `-ldflags "..."`
string for a task runner (`mise.toml`, a `Makefile`): a substitution that itself needs double quotes
(`date -u -d "$(git log -1 --format=%cI)" ...`) closes the outer `-ldflags "..."` early if nested
directly inside it. Compute each value into its own shell variable first, then interpolate the
variables into the ldflags string — never nest a second double-quoted command substitution inside
the one already wrapping the whole flag.

## Auditing an existing mechanism

Trace the four links in order and stop at the first missing one — that is the actual gap, not
whatever the report or the user's hunch names:

1. Does the version package exist, with defaults that are obviously placeholder values?
2. Grep every task/script that produces a real binary or image for the ldflags/build-arg wiring — a
   task added after the mechanism was first wired (a new deploy target, a second Dockerfile stage, a
   packaging script) is the most common way this silently regresses.
3. Does the read endpoint return the _live_ variables (not a hardcoded string, not a value read from
   a config file that itself needs manual bumping)?
4. Is the display fetched from that endpoint, shown unconditionally in the shell, and not gated
   behind a route nobody visits?

## Implementing it fresh

Follow this project's own implementation skills for each half (`go-implementation`/`go-testing` for
the version package, endpoint and its test; `vue3-implementation` or the equivalent frontend skill
for the client call, the composable/store that loads it once, and the shell display) — this skill
only supplies the version-specific shape above, not the surrounding code-quality rules. Close the
loop in whatever spec/doc files this project keeps in the same change: a new endpoint documented
where the others are, the build-task table updated, the injection mechanism itself recorded once in
the project-level (not per-domain) doc, since it belongs to the toolchain rather than to the back or
front architecture on its own.
