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

When generating implementation code, always prioritize clarity and maintainability over brevity.
Don't explain me what you're doing! Finish by listing your changes.
