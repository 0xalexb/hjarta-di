# Remove Unnecessary nolint Annotations

## Overview

Audit and clean up nolint annotations across the codebase. Remove annotations that are unnecessary because the linter is globally disabled or can be handled via config, keeping only legitimately needed ones.

## Context

- Files involved: `.golangci.yml`, `listener/server.go`, `listener/middleware/*.go`, `listener/middleware/*_test.go`, `options_test.go`, `listener/di_test.go`, `listener/server_test.go`, `config/example_test.go`, `example_test.go`
- Related patterns: exhaustruct is globally disabled in linter config; varnamelen has ignore-decls for w/r but not for other conventional short names
- Dependencies: none

## Development Approach

- **Testing approach**: Regular (verify via linter and test suite)
- Complete each task fully before moving to the next
- **CRITICAL: all tests and linter must pass before starting next task**

## Implementation Steps

### Task 1: Remove unnecessary nolint:exhaustruct annotations

The `exhaustruct` linter is globally disabled in `.golangci.yml` (line 7), making all `//nolint:exhaustruct` annotations dead code.

**Files:**

- Modify: `listener/server.go` (lines 47, 59)
- Modify: `listener/middleware/ratelimit.go` (line 21)
- Modify: `listener/middleware/logging.go` (line 73 - remove `exhaustruct` from `//nolint:exhaustruct,varnamelen`, keep `varnamelen`)
- Modify: `listener/middleware/recovery.go` (line 73)
- Modify: `listener/middleware/compress.go` (line 275)

- [x] Remove `//nolint:exhaustruct` from `listener/server.go:47` and `listener/server.go:59`
- [x] Remove `//nolint:exhaustruct` from `listener/middleware/ratelimit.go:21`
- [x] Change `//nolint:exhaustruct,varnamelen` to `//nolint:varnamelen` in `listener/middleware/logging.go:73`
- [x] Remove `//nolint:exhaustruct` from `listener/middleware/recovery.go:73`
- [x] Remove `//nolint:exhaustruct` from `listener/middleware/compress.go:275`

### Task 2: Expand varnamelen config and remove nolint:varnamelen annotations

Add `ignore-names` and additional `ignore-decls` to `.golangci.yml` varnamelen settings for conventional Go short variable names used throughout the codebase. Then remove the corresponding `//nolint:varnamelen` annotations.

**Files:**

- Modify: `.golangci.yml`
- Modify: `listener/middleware/cors.go` (line 55)
- Modify: `listener/middleware/request_id.go` (lines 64, 65)
- Modify: `listener/middleware/ratelimit.go` (line 71)
- Modify: `listener/middleware/logging.go` (lines 28, 70, 73)
- Modify: `listener/middleware/recovery.go` (line 72)
- Modify: `listener/middleware/compress.go` (lines 78, 257, 266)
- Modify: `listener/middleware/compress_test.go` (lines 26, 57, 78, 100, 126, 138, 162, 191, 254, 279, 310, 328, 345, 367)
- Modify: `listener/middleware/ratelimit_test.go` (lines 20, 44, 59, 87, 111)
- Modify: `listener/middleware/requestsize_test.go` (lines 25, 37, 49, 62, 75, 92)
- Modify: `listener/middleware/timeout_test.go` (line 40)
- Modify: `listener/middleware/logging_test.go` (lines 27, 63, 88, 104, 121, 138, 158, 177, 194)

- [x] Add varnamelen `ignore-names` to `.golangci.yml` for: `rr`, `tt`, `gz`, `sw`, `id`, `grw`
- [x] Add varnamelen `ignore-decls` to `.golangci.yml` for: `b []byte`, `h *testLogHandler`
- [x] Remove all `//nolint:varnamelen` annotations from production code files listed above
- [x] Remove all `//nolint:varnamelen` annotations from test files listed above

### Task 3: Verify acceptance criteria

- [x] Run linter: `golangci-lint run` - must pass with no new warnings
- [x] Run full test suite: `go test ./...` - all tests must pass
- [x] Verify no remaining nolint:exhaustruct annotations exist
- [x] Verify no remaining nolint:varnamelen annotations exist (for names covered by config)

### Task 4: Update documentation

- [x] Update CLAUDE.md if internal patterns changed
- [x] Move this plan to `docs/plans/completed/`
