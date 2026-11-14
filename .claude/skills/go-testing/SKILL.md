---
name: go-testing
description:
    'Handles all Golang testing tasks including running tests, writing new tests, and fixing test
    failures.'
---

## When to Use This Skill

-   Running unit tests with `go test`
-   Writing new test files and test cases
-   Debugging and fixing failing tests
-   Implementing test fixtures and mocks
-   Improving test coverage

# Go Testing Skill

Provides guidance and automation for Golang testing tasks.

## Testing Philosophy

-   uses **table-driven tests**
-   **Consistency** across all tests in the package
-   **Maintainability** through centralized setup functions
-   **Clarity** via descriptive case names and table-driven structure
-   **Efficiency** by avoiding duplicated code
-   **Reliability** through comprehensive test coverage

When generating tests, always prioritize clarity and maintainability over brevity. Don't explain me
what you're doing! Finish by listing your changes.

-   Every public method must have its own test function containing multiple test cases.
-   Use `require` (`github.com/stretchr/testify/require`) for assertions that should stop test
    execution on failure
-   Use `assert` (`github.com/stretchr/testify/assert`) for non-critical assertions where test
    should continue
-   Mock method argument if needed
-   Test internal functions by placing test files in the same package (no `_test` suffix)
-   Avoid creating externally facing functions solely for testing purposes
-   Never add comments

### Table-Driven Tests Structure

#### Test template

```go
func Test_<Struct>_<Method>(t *testing.T) {
    cases := []struct {
        name          string
        // Input fields
        hasCookie     bool
        failGetUser   bool
        // Expected output fields
        wantErr       bool
        wantStatus    int
        wantErrorCode string
    }{
      {
          name:       "Case 1",
          hasCookie:  true,
          wantStatus: http.StatusOK,
      },
      {
          name:       "Case 2",
          hasCookie:  false,
          wantErr:    true,
      },
        // ...
    }

    for _, c := range cases {
        t.Run(c.name, func(t *testing.T) {
            // Test implementation
            // Require calls if relevant
            // Assert calls
        })
    }
}
```

##### Input Fields

-   Use boolean flags for optional behaviors: `fail<Action>`, `has<Resource>`, `enable<Feature>`
-   Use string fields for state/mode selection: `mode`, `action`, `state`
-   Use integers for counts or indices: `retries`, `timeout`, `count`
-   Examples: `failFetch`, `hasCursor`, `pinned`, `mode: "fast"`, `retries: 3`

##### Output Fields

-   `want<Type>` for expected values: `wantStatus`, `wantErr`, `wantUser`, `wantResult`
-   `want<Condition>` for boolean expectations: `wantPinned`, `wantTruncated`, `wantEmpty`
-   Include only non-zero fields in the struct (Go convention)

#### Setup function to avoid duplication

Create helper functions with the `setup*` prefix to construct test data. This prevents duplication
and makes tests more maintainable.

-   `setupMockDependency()` - Creates a mock dependency
-   `setupConfig()` - Creates configuration objects
-   `setupRequest()` - Creates HTTP requests with optional setup
-   `setupDatabase()` - Sets up test database
-   `setupContext()` - Creates contexts with values

**Example setup functions:**

```go
func setupMockUserService(shouldFail bool) *mockUserService {
    return &mockUserService{
        ShouldFail: shouldFail,
        Users:      testdata.SampleUsers(),
    }
}

func setupServiceConfig(userService UserService) *ServiceConfig {
    return &ServiceConfig{
        UserService:  userService,
        Timeout:      30 * time.Second,
        MaxRetries:   3,
        LogLevel:     "info",
    }
}

func setupTestRequest(t *testing.T, headers map[string]string) *http.Request {
    req := httptest.NewRequest("GET", "/", nil)
    for key, value := range headers {
        req.Header.Set(key, value)
    }
    return req
}

func setupTestContext(ctx context.Context, values map[string]interface{}) context.Context {
    for key, value := range values {
        ctx = context.WithValue(ctx, key, value)
    }
    return ctx
}
```

## Platform tests

A test under `go-pkg/` proves the platform works for any module (rule 7 of the `platform` skill):

-   It runs with no module: a module it needs is a fake declared in the test file, implementing
    `starter.Module` with neutral names (`"mod"`, `"a"`, `"b"`), its tables and resources neutral
    too (`items`, `notes`).
-   It never imports `internal/` and never borrows a module's vocabulary — no `darow`, no table, no
    product file name; a database file is `app.db`.
-   A platform improvement brought by a module's change gets its platform tests first, passing on
    their own, before the module's tests use it.
-   `go-pkg`'s architecture rules (`starter/arch_test.go`, `mise run test` there) pass after any
    change to it or to its imports; a module's `internal/<module>/arch_test.go` lists the platform
    packages it imports as external ones (`platform + "<package>"`).

A module's tests cover the business; they run over the platform's real `database.DB`, opened on a
temporary file with the module's `Schema`, rather than a mock of it.

## Running Tests

The whole Go suite is a `mise` task:

```bash
mise run test-go
```

Never `go test ./...` from the repository root: it reaches the Go package inside `web/node_modules`.
Target `${GO_PKGS}` (`./cmd/... ./internal/... ./pkg/... ./web`) or one package.

Run one package, with race detection:

```bash
go test -race ./internal/darow/store/...
```

Run a specific test:

```bash
go test -run Test_<MethodName> -v ./internal/darow/store/...
```

Run tests with coverage:

```bash
go test -cover ./internal/darow/...
go test -coverprofile=coverage.out ./internal/darow/...
go tool cover -html=coverage.out
```

Tests tagged `live` (they call a real site) are never run by you; they are run by hand.
