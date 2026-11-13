---
name: go-implementation
description:
    'Handles Golang implementation tasks including writing new code, imports organization, and
    public API documentation.'
---

## When to Use This Skill

-   Writing or modifying Go source files (non-test), in `cmd/`, `go-pkg/` or `internal/`
-   Organizing imports
-   Documenting public methods and types
-   Reviewing Go code for style/readability issues

# Go Implementation Skill

Provides guidance for writing idiomatic, readable Go code in this repo.

## Core Principles

-   **Simple, SOLID, DRY** - The essential rules of the `essentials` skill come before everything
    below; read it first
-   **Keep it readable** - Use clear variable names
-   Never add comments on the code itself, if needed it means that the code is not self-explanatory,
    and consequently ugly
-   **No text for the user** - The server never writes a sentence the interface shows
    (`go-pkg/spec/back.md` §0 rule 6). A failure the user sees is an `apperr.Code`
    (`<namespace>.<name>`: `platform`, `common` or the module's name) with its params: a
    `FieldError` in a `ValidationError` (422), a problem through `httpx.WriteProblem` (4xx/5xx), a
    `runs.Failure` on a run. A label the application chose (a category, a family of data, a status)
    is a code or a key, labelled by the front's catalog; a label the server serves is data. Error
    strings, `detail` and logs are English, for the operator. A new code is declared in the
    `codes.go` of the package raising it, and its message is added to the front catalog of its
    namespace in the same change

## Documentation travels with the code

A change that adds, removes or alters a route, a package, a table, a document kind or a pointer
updates the application's documentation in the same change: the route's `swag` block and
`mise run generate-api-doc`, and `docs/architecture.md`. The `app-docs` skill gives how; `test-go`
fails on a stale or incomplete API description.

## Platform or module

The fundamental rule of `CLAUDE.md` holds here: `go-pkg/` is the platform, a reusable application
layout any application uses as it is; `internal/<module>` is a business package, complete on its
own, that never imports another module. Before writing, run the evaluation of the `platform` skill
(**Every implementation: evaluate, then split**): sort the change into its platform and business
parts, build a missing generic piece in `go-pkg/` first and on its own, then the module over it. In
short:

-   `go-pkg/` never imports `internal/` or `cmd/`, and never names a module's resource, entity,
    table or label — not even in a comment or a test fixture.
-   A module reaches the platform only through `starter.Module` / `starter.Host`, its
    `database.Schema`, `repository.Repository[T, R]` (a document embedding `document.Meta`, stored
    by `repository.NewPlainTable`, served by `resource.Register`), `events.Notifier` and
    `config.Env*`.
-   A generic helper the module is about to write (a middleware, a transaction wrapper, a JSON
    response helper) goes to `go-pkg/`, written without the module's terms — or is recorded as a
    platform improvement when the change does not need it.
-   A module never imports another (`internal/<module>/arch_test.go`). A mechanism two modules both
    need is made generic in `go-pkg/`; business they both hold goes to `internal/common`, which
    imports `go-pkg/` and never a module; never imported from one by the other nor copied.

## Import Organization

Organize imports in three groups (separated by blank lines):

1. Standard library imports
2. External package imports
3. Internal package imports

**Example:**

```go
import (
	"context"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"

	"github.com/amauryval/darow/internal/darow/engine"
	"github.com/MyAppLibs/go-pkg/database"
)
```

**Rules:**

-   Keep imports sorted alphabetically within each group
-   Remove unused imports

## Public Method Documentation

Document all public methods and types using godoc comments. Keep comments **concise and clear** (max
2 lines).

**Rules:**

-   Start comment with the name: `// MethodName ...`
-   Explain what it does, not how it works
-   Maximum 3 lines per method/type documentation
-   Use simple, clear language
-   No parameter/return documentation in the comment itself (obvious from code)
-   Don't explain the struct/function arguments

**Good example:**

```go
// Backuper writes, prunes and restores the database archives.
// It serialises every operation on the backup directory.
type Backuper struct { ... }

// NewBackuper creates a Backuper over the given database and directory.
// It creates the directory when it does not exist.
func NewBackuper(db *database.DB, dir string, clock clock.Clock) (*Backuper, error) { ... }

// RegisterRoutes registers the backup endpoints on the router.
// Sets up listing, creation, download, pin, rename and restore.
func (h *BackupHandler) RegisterRoutes(router chi.Router) { ... }
```

**Bad example (too long):**

```go
// NewBackuper creates a new Backuper with the provided database and directory.
// It checks the directory, creates it with 0700 permissions when missing,
// creates the pinned sub-directory, prepares the temporary directory and the mutex.
// Parameters: db (*database.DB), dir (string), clock (clock.Clock)
// Returns: (*Backuper, error)
func NewBackuper(db *database.DB, dir string, clock clock.Clock) (*Backuper, error) { ... }
```

When generating implementation code, always prioritize clarity and maintainability over brevity.
Don't explain me what you're doing! Finish by listing your changes, platform and module apart.
