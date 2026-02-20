# HTTP Middleware Collection for listener Package

## Overview

Add a collection of HTTP middlewares to a new `listener/middleware` subpackage, providing common cross-cutting concerns for HTTP services. All middlewares use stdlib only (no external deps) and use the global slog logger (slog.Info, slog.Error, etc.) for logging. Each middleware follows the standard `func(http.Handler) http.Handler` signature, compatible with go-pkgz/routegroup which users will use for composition.

## Context

- Files involved: new `listener/middleware/` subpackage
- Related patterns: slog logging (global via slog.SetDefault in di package), stdlib-only dependencies (depguard enforced)
- Dependencies: stdlib only (compress/gzip, encoding/binary, encoding/hex, hash/crc32, net/http, os, sync, sync/atomic, time, etc.)

## Development Approach

- **Testing approach**: Regular (code first, then tests)
- Complete each task fully before moving to the next
- Each middleware lives in its own file within `listener/middleware/` subpackage
- All middlewares follow `func(http.Handler) http.Handler` signature
- Logging middlewares use global slog (slog.Info, slog.Error, etc.) - no logger parameter
- No chain/composition utility - users compose via go-pkgz/routegroup
- **CRITICAL: every task MUST include new/updated tests**
- **CRITICAL: all tests must pass before starting next task**

## Implementation Steps

### Task 1: RequestID middleware

**Files:**
- Create: `listener/middleware/request_id.go`
- Create: `listener/middleware/request_id_test.go`

- [x] Create `listener/middleware/` subpackage with package declaration
- [x] Implement request ID generation using 8 random bytes from crypto/rand, encoded as 16 hex characters
- [x] Implement `RequestID()` middleware that generates an ID per request using the above algorithm
- [x] Reject externally-provided IDs exceeding 256 characters or containing non-printable ASCII
- [x] Store request ID in request context (export getter function `GetRequestID(ctx) string`)
- [x] Set X-Request-ID response header
- [x] If X-Request-ID is present in request, reuse it instead of generating
- [x] Write tests: ID format (16 hex chars), uniqueness across calls, existing ID reuse, header propagation, context storage, control character rejection
- [x] Run project test suite - must pass before task 2

### Task 2: Recovery middleware

**Files:**
- Create: `listener/middleware/recovery.go`
- Create: `listener/middleware/recovery_test.go`

- [x] Implement `Recovery()` middleware (no parameters) that catches panics from downstream handlers
- [x] Log panic value and stack trace via global slog.Error
- [x] Respond with 500 Internal Server Error
- [x] Include request ID from context in log entry if available
- [x] Write tests: panic recovery, 500 response, log output verification (set slog.SetDefault with test handler), non-panic pass-through
- [x] Run project test suite - must pass before task 3

### Task 3: Logging middleware

**Files:**
- Create: `listener/middleware/logging.go`
- Create: `listener/middleware/logging_test.go`

- [x] Implement `Logging()` middleware (no parameters) that logs request/response details via global slog
- [x] Log: method, path, status code, duration, request ID (from context)
- [x] Use slog structured fields (slog.String, slog.Int, slog.Duration)
- [x] Log at Info level for successful requests, Warn for 4xx, Error for 5xx
- [x] Use a response writer wrapper to capture status code
- [x] Write tests: log fields verification, status level mapping, duration tracking (set slog.SetDefault with test handler to capture log output)
- [x] Run project test suite - must pass before task 4

### Task 4: CORS middleware

**Files:**
- Create: `listener/middleware/cors.go`
- Create: `listener/middleware/cors_test.go`

- [x] Define `CORSConfig` struct with AllowedOrigins, AllowedMethods, AllowedHeaders, AllowCredentials, MaxAge fields
- [x] Implement `CORS(cfg CORSConfig)` middleware
- [x] Handle preflight OPTIONS requests and return early with appropriate headers
- [x] Set Access-Control-Allow-Origin, Access-Control-Allow-Methods, Access-Control-Allow-Headers, Access-Control-Max-Age headers
- [x] Support wildcard "*" origin
- [x] Write tests: preflight handling, origin matching, wildcard, credentials flag, normal request pass-through
- [x] Run project test suite - must pass before task 5

### Task 5: Timeout middleware

**Files:**
- Create: `listener/middleware/timeout.go`
- Create: `listener/middleware/timeout_test.go`

- [x] Implement `Timeout(duration time.Duration)` middleware using http.TimeoutHandler
- [x] Return 503 Service Unavailable on timeout
- [x] Write tests: request within timeout succeeds, request exceeding timeout returns 503
- [x] Run project test suite - must pass before task 6

### Task 6: RateLimit middleware

**Files:**
- Create: `listener/middleware/ratelimit.go`
- Create: `listener/middleware/ratelimit_test.go`

- [x] Implement a simple token bucket rate limiter (stdlib only)
- [x] `RateLimit(requestsPerSecond float64, burst int)` middleware signature
- [x] Use sync.Mutex and time-based token replenishment
- [x] Return 429 Too Many Requests with Retry-After header when limit exceeded
- [x] Rate limit is global (not per-client) for simplicity
- [x] Write tests: requests within limit pass, exceeding limit returns 429, token replenishment over time
- [x] Run project test suite - must pass before task 7

### Task 7: RequestSize limiter middleware

**Files:**
- Create: `listener/middleware/requestsize.go`
- Create: `listener/middleware/requestsize_test.go`

- [x] Implement `MaxRequestSize(bytes int64)` middleware
- [x] Use http.MaxBytesReader to wrap request body
- [x] Return 413 Request Entity Too Large when body exceeds limit
- [x] Write tests: small body passes, oversized body returns 413
- [x] Run project test suite - must pass before task 8

### Task 8: Compression (gzip) middleware

**Files:**
- Create: `listener/middleware/compress.go`
- Create: `listener/middleware/compress_test.go`

- [x] Implement `Compress()` middleware using compress/gzip from stdlib
- [x] Check Accept-Encoding header for gzip support
- [x] Wrap response writer with gzip writer when client supports it
- [x] Set Content-Encoding: gzip header
- [x] Remove Content-Length header (since compressed size differs)
- [x] Properly close gzip writer via defer
- [x] Skip compression for small responses or already-compressed content types
- [x] Write tests: gzip response when accepted, no compression when not accepted, proper headers
- [x] Run project test suite - must pass before task 9

### Task 9: Verify acceptance criteria

- [x] All middlewares follow `func(http.Handler) http.Handler` signature
- [x] All middlewares are stdlib-only (no external dependencies)
- [x] Logging uses global slog (no logger parameters)
- [x] Run full test suite: `go test ./...`
- [x] Run linter: `golangci-lint run`
- [x] Verify test coverage meets 80%+

### Task 10: Update documentation

- [x] Update CLAUDE.md with middleware subpackage documentation
- [x] Move this plan to `docs/plans/completed/`
