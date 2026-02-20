# Replace Middleware Panics with Sensible Defaults

## Overview

Replace panic calls in middleware factory functions with sensible default values and slog warnings. Panics in request_id.go (crypto/rand failure) and recovery.go (http.ErrAbortHandler re-panic) are intentionally kept.

## Context

- Files involved: listener/middleware/ratelimit.go, listener/middleware/timeout.go, listener/middleware/requestsize.go, listener/middleware/cors.go, and their corresponding test files
- Related patterns: middlewares use global slog (set via slog.SetDefault in di package)
- Dependencies: log/slog (stdlib)

## Development Approach

- **Testing approach**: Regular (code first, then tests)
- Complete each task fully before moving to the next
- **CRITICAL: every task MUST include new/updated tests**
- **CRITICAL: all tests must pass before starting next task**

## Implementation Steps

### Task 1: RateLimit - replace panics with defaults

**Files:**
- Modify: `listener/middleware/ratelimit.go`
- Modify: `listener/middleware/ratelimit_test.go`

- [x] Replace panic for requestsPerSecond <= 0 with default of 1.0 and slog.Warn
- [x] Replace panic for burst <= 0 with default of 1 and slog.Warn
- [x] Update doc comment to document default behavior
- [x] Update tests: remove panic-expectation tests, add tests verifying defaults are applied and warnings are logged

### Task 2: Timeout - replace panic with default

**Files:**
- Modify: `listener/middleware/timeout.go`
- Modify: `listener/middleware/timeout_test.go`

- [x] Replace panic for duration <= 0 with default of 30s and slog.Warn
- [x] Update doc comment
- [x] Update tests: remove panic-expectation test, add test verifying 30s default and warning logged

### Task 3: MaxRequestSize - replace panic with default

**Files:**
- Modify: `listener/middleware/requestsize.go`
- Modify: `listener/middleware/requestsize_test.go`

- [x] Replace panic for bytes <= 0 with default of 1048576 (1MB) and slog.Warn
- [x] Update doc comment
- [x] Update tests: remove panic-expectation test, add test verifying 1MB default and warning logged

### Task 4: CORS - replace panic with default behavior

**Files:**
- Modify: `listener/middleware/cors.go`
- Modify: `listener/middleware/cors_test.go`

- [x] Replace panic for AllowCredentials with only wildcard by disabling credentials and logging slog.Warn
- [x] Update doc comment
- [x] Update tests: remove panic-expectation test, add test verifying credentials disabled and warning logged

### Task 5: Update CLAUDE.md middleware documentation

**Files:**
- Modify: `CLAUDE.md`

- [x] Update middleware descriptions to reflect default-on-invalid-input behavior instead of panic behavior

### Task 6: Verify acceptance criteria

- [x] Run full test suite: `go test ./...`
- [x] Run linter: `golangci-lint run`
- [x] Verify no remaining panics in middleware (except request_id crypto/rand and recovery http.ErrAbortHandler)
