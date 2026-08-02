---
name: go-implementation
description:
    'Handles Golang implementation tasks including writing new code, imports organization, and
    public API documentation.'
---

## When to Use This Skill

-   Writing or modifying Go source files (non-test)
-   Organizing imports
-   Documenting public methods and types
-   Reviewing Go code for style/readability issues

# Go Implementation Skill

Provides guidance for writing idiomatic, readable Go code in this repo.

## Core Principles

-   **Keep it simple** - Avoid complex logic and data structures
-   **Keep it DRY** - Avoid code duplication
-   **Keep it readable** - Use clear variable names
-   Never add comments on the code itself, if needed it means that the code is not self-explanatory,
    and consequently ugly

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

	"github.com/stretchr/testify/assert"
	"github.com/go-chi/chi/v5"

	"server/auth/service"
	"server/auth/types"
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
// Auth provides a complete OAuth2 authentication system.
// It handles session management, authorization, and route registration.
type Auth struct { ... }

// New creates and initializes a new Auth instance.
// It validates encryption key and sets up OAuth2 provider.
func New(devMode bool, config *AuthConfig, authorizedIDs []int64) (*Auth, error) { ... }

// RegisterRoutes registers all authentication endpoints on the router.
// Sets up /auth/login, /auth/callback, /auth/logout, and /auth/session.
func (a *Auth) RegisterRoutes(router chi.Router) { ... }
```

**Bad example (too long):**

```go
// New creates a new Auth instance with the provided configuration.
// It validates encryption key, sets up OAuth2 provider (GitHub or demo mode),
// creates encrypted session store, initializes route handlers, and manages sessions.
// Parameters: devMode (bool), config (*AuthConfig), authorizedIDs ([]int64)
// Returns: (*Auth, error)
func New(devMode bool, config *AuthConfig, authorizedIDs []int64) (*Auth, error) { ... }
```

## Closing the loop on `spec/back/`

Every implementation ends with the documentation in the same state as the code. Five files:
`spec.md`, `blockers.md`, `limitations.md`, `todo.md`, `improvements.md` — see the `audit` skill for
how to classify an entry.

**Before writing code**, check whether what you are about to do is already an entry (`BLK-n`,
`TODO-n`, `IMP-n`). If it is, that entry states the problem — start from it rather than restating
it.

**After the change**:

-   `spec.md` — update it when a contract, an invariant, a layer boundary or the project structure
    changed. It describes the current code only: no "was", no "used to".
-   `blockers.md` / `todo.md` — delete the rows the change resolves, in the same change. A todo file
    that still lists fixed work is worse than an empty one.
-   `limitations.md` — add an entry when you deliberately left something out of scope, with the
    reason and what it costs the caller.
-   `improvements.md` — add an entry for a lead you spotted and are not taking.
-   Nothing changed on any of the five? Then nothing to write — an untouched file is a valid outcome,
    and each file exists even when empty.

### Analysis before implementation

No entry goes straight to code. But the analysis is not the same one from one file to the next, and
it is not the same person who concludes it:

| Entry | What the analysis produces | Who concludes |
|---|---|---|
| `BLK-n` | a **diagnosis** — the root cause, not the symptom the entry describes. Skipping it patches the symptom and the blocker comes back under another form | you; whether to fix it is never in question |
| `TODO-n` | a **design** — the cut, the files touched, the order, what the change breaks. A todo states a problem, never its solution | you, with a check-in when the change is structural or spans layers |
| `IMP-n` | a **decision of opportunity** — is the need real, what scope, what cost, which approach | the user, always. Nothing has been decided about a lead |
| `LIM-n` | nothing while it holds; lifting one is a scope change | the user |

The consequence differs too. A blocker or a todo can be picked up as soon as its analysis is done —
they are observed facts, and the code is expected to change. An `IMP-n` cannot: implementing it
directly skips the only decision the entry was waiting for. Its analysis ends either as a `TODO-n`
(retained, now specified) or as a `LIM-n` / a deletion (dropped).

Ids are stable and never renumbered; a retired id is never reused.

When generating implementation code, always prioritize clarity and maintainability over brevity.
Don't explain me what you're doing! Finish by listing your changes.
