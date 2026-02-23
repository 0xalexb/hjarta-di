# Refactor CORS Middleware to Functional Options with Logical Defaults

## Overview

Replace `CORSConfig` struct-based configuration with a functional options (vararg) pattern for the CORS middleware, and apply logical defaults so callers only need to specify what they want to override. This follows the same `Option func(*Config)` pattern already used in the `listener` package.

## Context

- Files involved: `listener/middleware/cors.go`, `listener/middleware/cors_test.go`, `CLAUDE.md`
- Related patterns: `listener.Option` / `listener.Config` for functional options convention
- Dependencies: none (stdlib + existing project code only)

## Defaults

When `CORS()` is called with no options:
- AllowedOrigins: `["*"]` (wildcard, matching rs/cors and other popular Go CORS libraries)
- AllowedMethods: `["GET", "HEAD", "POST"]`
- AllowedHeaders: `["Origin", "Accept", "Content-Type", "X-Requested-With"]`
- ExposedHeaders: none
- MaxAge: `3600` (1 hour)
- AllowCredentials: `false`
- ValidateOrigins: none (validators are opt-in; default wildcard origin would conflict with hostname validators)

## Development Approach

- **Testing approach**: Regular (code first, then tests)
- Complete each task fully before moving to the next
- **CRITICAL: every task MUST include new/updated tests**
- **CRITICAL: all tests must pass before starting next task**

## Implementation Steps

### Task 1: Add CORSOption type and refactor CORSConfig

**Files:**
- Modify: `listener/middleware/cors.go`

- [x] Rename `CORSConfig` to `corsConfig` (unexported) since the functional options become the public API
- [x] Add exported `CORSOption func(*corsConfig)` type
- [x] Add default constants: `defaultCORSMaxAge = 3600`, `defaultCORSMethods = []string{"GET", "HEAD", "POST"}`, `defaultCORSHeaders = []string{"Origin", "Accept", "Content-Type", "X-Requested-With"}`, `defaultCORSOrigins = []string{"*"}`
- [x] Add functional option constructors:
  - `WithAllowedOrigins(origins ...string) CORSOption` - replaces default origins
  - `WithAllowedMethods(methods ...string) CORSOption` - replaces default methods
  - `WithAllowedHeaders(headers ...string) CORSOption` - replaces default headers
  - `WithExposedHeaders(headers ...string) CORSOption` - sets exposed headers
  - `WithMaxAge(seconds int) CORSOption` - sets max age
  - `WithAllowCredentials() CORSOption` - enables credentials (no bool arg; absence means false)
  - `WithOriginValidators(validators ...OriginValidator) CORSOption` - sets origin validators
- [x] Update `CORS` function signature from `CORS(cfg CORSConfig)` to `CORS(opts ...CORSOption)`
- [x] Inside `CORS`, build `corsConfig` from defaults, then apply each option
- [x] All existing internal logic (hostname map building, wildcard handling, credentials safety, header joining) stays the same

### Task 2: Update all CORS tests to use functional options API

**Files:**
- Modify: `listener/middleware/cors_test.go`

- [x] Update every test that constructs `CORSConfig{...}` to use `CORS(WithAllowedOrigins(...), WithAllowedMethods(...), ...)` instead
- [x] Add test `TestCORS_Defaults` verifying zero-option `CORS()` produces wildcard origin matching with default methods/headers/maxAge
- [x] Add test `TestCORS_DefaultMethodsInPreflight` verifying default methods appear in preflight response
- [x] Add test `TestCORS_OverrideDefaults` verifying that options replace defaults (e.g., setting methods replaces default methods, not appends)
- [x] Run tests for this task

### Task 3: Update CLAUDE.md

**Files:**
- Modify: `CLAUDE.md`

- [x] Update the CORS middleware description to document the new functional options API and defaults
- [x] Remove reference to `CORSConfig` struct, replace with option functions

### Task 4: Verify acceptance criteria

- [x] Run full test suite (`go test ./...`)
- [x] Run linter (`golangci-lint run`)
- [x] Verify test coverage meets 80%+

### Task 5: Update documentation

- [x] Update README.md if user-facing changes
- [x] Move this plan to `docs/plans/completed/`
