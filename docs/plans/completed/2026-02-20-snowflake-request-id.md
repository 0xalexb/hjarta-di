# Snowflake Request ID Generator

## Overview

Replace the random request ID generator with a snowflake-like ID scheme: 41 bits timestamp (ms since 2026-01-01 UTC), 16 bits machine hash (FNV-1a of hostname), 7 bits sequence counter. Output remains a 16-char hex string. When the sequence counter overflows within a single millisecond (>127 IDs in 1ms), the generator spin-waits until the next millisecond to guarantee uniqueness without returning errors. When a clock-backward event is detected (current time < last recorded time), the generator spin-waits until the clock catches up to preserve monotonicity.

## Context

- Files involved: `listener/middleware/request_id.go`, `listener/middleware/request_id_test.go`
- Related patterns: stateful middleware pattern from `ratelimit.go` (struct with mutex, created in factory function)
- Dependencies: stdlib only (`hash/fnv`, `os`, `sync`, `time`, `encoding/binary`)

## Development Approach

- **Testing approach**: Regular (code first, then tests)
- Complete each task fully before moving to the next
- **CRITICAL: every task MUST include new/updated tests**
- **CRITICAL: all tests must pass before starting next task**

## Implementation Steps

### Task 1: Implement snowflake generator and update RequestID middleware

**Files:**
- Modify: `listener/middleware/request_id.go`

- [x] Add snowflake constants: epoch (2026-01-01 UTC as ms), bit widths (41 timestamp, 16 machine, 7 sequence), max sequence mask (0x7F = 127), machine mask (0xFFFF = 65535), shifts (machineShift=7, timestampShift=23)
- [x] Add `snowflakeGenerator` struct with fields: `mu sync.Mutex`, `machineID uint64`, `sequence uint64`, `lastTimestamp int64`, `timeNow func() time.Time` (injectable clock for testing, defaults to `time.Now`)
- [x] Add `newSnowflakeGenerator()` constructor that computes machine ID via `hash/fnv` FNV-1a of `os.Hostname()` masked to 16 bits; if hostname fails, hash empty string and log `slog.Warn`; set `timeNow` to `time.Now`
- [x] Add `generate() string` method with spin-wait and clock-backward logic:
  - Lock mutex
  - Get current ms since epoch using `timeNow()`
  - If current timestamp < lastTimestamp (clock went backward): spin-wait in a loop calling `timeNow()` until current timestamp >= lastTimestamp, log `slog.Warn` with drift duration once before spinning
  - If same ms as lastTimestamp: increment sequence; if sequence exceeds max (127), spin-wait in a loop calling `timeNow()` until timestamp advances to next ms, then reset sequence to 0
  - If later ms: reset sequence to 0, update lastTimestamp
  - Compose uint64 from `(timestamp << 23) | (machineID << 7) | sequence`
  - Unlock mutex, return as 16-char zero-padded hex string
- [x] Update `RequestID()` factory to create a `snowflakeGenerator` via `newSnowflakeGenerator()` and call `gen.generate()` instead of `generateRequestID()`
- [x] Remove old `generateRequestID()` function and `requestIDBytes` constant

### Task 2: Update tests, add benchmark and fuzz tests

**Files:**
- Modify: `listener/middleware/request_id_test.go`

- [x] Update `TestGenerateRequestID_Format` -> test that generated ID is 16 hex chars and encodes a valid uint64 with non-zero timestamp bits
- [x] Update `TestGenerateRequestID_Uniqueness` -> generate 10000 IDs, verify all unique
- [x] Add `TestSnowflakeGenerator_Structure` -> generate an ID, decode uint64, extract timestamp/machineID/sequence parts, verify timestamp is recent (within last second), machineID matches expected hostname hash masked to 16 bits, sequence is 0 for first ID
- [x] Add `TestSnowflakeGenerator_SequenceIncrement` -> generate multiple IDs in tight loop, verify they share same or adjacent timestamps and have incrementing sequences
- [x] Add `TestSnowflakeGenerator_SpinWait` -> set sequence to max (127) via injectable clock trick, generate one more ID, verify it has a later timestamp and sequence reset to 0 (confirms spin-wait advances to next ms)
- [x] Add `TestSnowflakeGenerator_ClockBackward` -> inject a `timeNow` function that returns a time in the past after several normal calls (simulating NTP clock adjustment backward); verify the generator still produces unique IDs with monotonically non-decreasing timestamps, and that the ID generated after the backward jump has a timestamp >= the last timestamp before the jump
- [x] Add `TestSnowflakeGenerator_ConcurrentUniqueness` -> launch 100 goroutines each generating 100 IDs concurrently from one generator, collect all in a sync.Map, verify zero duplicates
- [x] Add `BenchmarkSnowflakeGenerator` -> benchmark single-goroutine generation to verify throughput exceeds 10000 ops/sec
- [x] Add `BenchmarkSnowflakeGenerator_Parallel` -> benchmark with b.RunParallel to verify concurrent throughput
- [x] Add `FuzzSnowflakeGenerator_Uniqueness` -> fuzz with varying batch sizes (seed corpus: 1, 10, 100, 1000); for each fuzzed batch size, generate that many IDs from a single generator and verify all are unique and all decode to valid 16-char hex with non-zero timestamp bits
- [x] Add `FuzzSnowflakeGenerator_Structure` -> fuzz by generating IDs and decoding: verify timestamp bits represent a time after epoch (2026-01-01) and before a reasonable future bound, machine bits are consistent across all IDs from one generator, sequence bits are within 0-127 range
- [x] Existing middleware-level tests (`TestRequestID_GeneratesNewID`, `TestRequestID_ReusesExistingID`, etc.) keep their structure but update length/format assertions if needed (16-char hex remains the same, so most stay as-is)

### Task 3: Verify acceptance criteria

- [x] Run full test suite: `go test ./...`
- [x] Run linter: `golangci-lint run`
- [x] Run fuzz tests for a short duration: `go test -fuzz=Fuzz -fuzztime=10s ./listener/middleware/`
- [x] Verify benchmark shows >10000 ops/sec
- [x] Verify test coverage meets 80%+

### Task 4: Update documentation

- [x] Update CLAUDE.md `RequestID()` description to reflect snowflake ID scheme (41-bit timestamp, 16-bit machine hash, 7-bit sequence, spin-wait on overflow, clock-backward handling)
- [x] Move this plan to `docs/plans/completed/`
