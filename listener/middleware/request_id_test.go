package middleware

import (
	"encoding/binary"
	"encoding/hex"
	"hash/fnv"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func decodeSnowflakeID(t *testing.T, id string) (int64, uint64, uint64) {
	t.Helper()

	require.Len(t, id, 16, "ID must be 16 hex characters")

	raw, err := hex.DecodeString(id)
	require.NoError(t, err, "ID must be valid hex")

	val := binary.BigEndian.Uint64(raw)
	seq := val & snowflakeMaxSequence
	machine := (val >> snowflakeMachineShift) & snowflakeMachineMask
	ts := int64(val >> snowflakeTimestampShift)

	return ts, machine, seq
}

func expectedMachineID(t *testing.T) uint64 {
	t.Helper()

	hostname, err := os.Hostname()
	require.NoError(t, err)

	h := fnv.New64a()
	_, _ = h.Write([]byte(hostname))

	return h.Sum64() & snowflakeMachineMask
}

func TestGenerateRequestID_Format(t *testing.T) {
	t.Parallel()

	gen := newSnowflakeGenerator()
	id := gen.generate()

	assert.Len(t, id, 16, "request ID should be 16 hex characters")

	raw, err := hex.DecodeString(id)
	require.NoError(t, err, "request ID should be valid hex")

	val := binary.BigEndian.Uint64(raw)
	timestamp := val >> snowflakeTimestampShift
	assert.NotZero(t, timestamp, "timestamp bits should be non-zero")
}

func TestGenerateRequestID_Uniqueness(t *testing.T) {
	t.Parallel()

	gen := newSnowflakeGenerator()
	seen := make(map[string]struct{}, 10000)

	for range 10000 {
		id := gen.generate()

		_, exists := seen[id]
		assert.False(t, exists, "duplicate request ID generated: %s", id)

		seen[id] = struct{}{}
	}
}

func TestSnowflakeGenerator_Structure(t *testing.T) {
	t.Parallel()

	gen := newSnowflakeGenerator()

	before := time.Now()
	id := gen.generate()
	after := time.Now()

	timestamp, machineID, sequence := decodeSnowflakeID(t, id)

	beforeMs := before.UnixMilli() - snowflakeEpochMs
	afterMs := after.UnixMilli() - snowflakeEpochMs

	assert.GreaterOrEqual(t, timestamp, beforeMs, "timestamp should be >= test start time")
	assert.LessOrEqual(t, timestamp, afterMs, "timestamp should be <= test end time")

	assert.Equal(t, expectedMachineID(t), machineID, "machineID should match hostname FNV-1a hash")

	assert.Equal(t, uint64(0), sequence, "sequence should be 0 for first ID")
}

func TestSnowflakeGenerator_SequenceIncrement(t *testing.T) {
	t.Parallel()

	gen := newSnowflakeGenerator()
	fixedTime := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	gen.timeNow = func() time.Time {
		return fixedTime
	}

	const count = 10

	ids := make([]string, count)
	for i := range count {
		ids[i] = gen.generate()
	}

	expectedTs := fixedTime.UnixMilli() - snowflakeEpochMs

	for i, id := range ids {
		ts, _, seq := decodeSnowflakeID(t, id)
		assert.Equal(t, expectedTs, ts, "all IDs should share the same timestamp")
		assert.Equal(t, uint64(i), seq, "sequence should increment: ID %d", i)
	}
}

func TestSnowflakeGenerator_SpinWait(t *testing.T) {
	t.Parallel()

	gen := newSnowflakeGenerator()
	fixedTime := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	fixedMs := fixedTime.UnixMilli() - snowflakeEpochMs

	gen.lastTimestamp = fixedMs
	gen.sequence = snowflakeMaxSequence

	callCount := 0
	gen.timeNow = func() time.Time {
		callCount++
		if callCount == 1 {
			return fixedTime
		}

		return fixedTime.Add(time.Millisecond)
	}

	id := gen.generate()
	ts, _, seq := decodeSnowflakeID(t, id)

	nextMs := fixedMs + 1
	assert.Equal(t, nextMs, ts, "timestamp should advance to next ms after spin-wait")
	assert.Equal(t, uint64(0), seq, "sequence should reset to 0 after spin-wait")
}

func TestSnowflakeGenerator_ClockBackward(t *testing.T) {
	t.Parallel()

	gen := newSnowflakeGenerator()
	baseTime := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

	callCount := 0
	gen.timeNow = func() time.Time {
		callCount++

		switch {
		case callCount <= 3:
			return baseTime
		case callCount == 4:
			return baseTime.Add(-10 * time.Millisecond)
		default:
			return baseTime
		}
	}

	ids := make([]string, 4)
	for i := range 4 {
		ids[i] = gen.generate()
	}

	seen := make(map[string]struct{}, 4)
	for _, id := range ids {
		_, exists := seen[id]
		assert.False(t, exists, "duplicate ID: %s", id)

		seen[id] = struct{}{}
	}

	var prevTs int64

	for i, id := range ids {
		ts, _, _ := decodeSnowflakeID(t, id)
		assert.GreaterOrEqual(t, ts, prevTs, "timestamp should be non-decreasing at ID %d", i)
		prevTs = ts
	}

	ts3, _, _ := decodeSnowflakeID(t, ids[2])
	ts4, _, _ := decodeSnowflakeID(t, ids[3])
	assert.GreaterOrEqual(t, ts4, ts3, "post-backward-jump timestamp should be >= pre-jump timestamp")
}

func TestSnowflakeGenerator_LargeClockBackward(t *testing.T) {
	t.Parallel()

	gen := newSnowflakeGenerator()
	baseTime := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

	callCount := 0
	gen.timeNow = func() time.Time {
		callCount++

		switch {
		case callCount <= 2:
			return baseTime
		case callCount == 3:
			// Jump backward by 1 second — exceeds maxClockDriftMs (500ms).
			return baseTime.Add(-1 * time.Second)
		default:
			return baseTime.Add(-1 * time.Second)
		}
	}

	ids := make([]string, 3)
	for i := range 3 {
		ids[i] = gen.generate()
	}

	// All IDs must be unique.
	seen := make(map[string]struct{}, 3)
	for _, id := range ids {
		_, exists := seen[id]
		assert.False(t, exists, "duplicate ID: %s", id)

		seen[id] = struct{}{}
	}

	// Third ID should advance past last known timestamp (monotonicity preserved, no spin-wait).
	ts1, _, _ := decodeSnowflakeID(t, ids[0])
	ts3, _, seq3 := decodeSnowflakeID(t, ids[2])
	expectedTs := ts1 + 1 // lastTimestamp was baseTimeMs; large drift advances to lastTimestamp + 1
	assert.Equal(t, expectedTs, ts3, "large drift should advance past last known timestamp")
	assert.Equal(t, uint64(0), seq3, "sequence should reset to 0 after large drift reset")
}

func TestSnowflakeGenerator_ConcurrentUniqueness(t *testing.T) {
	t.Parallel()

	gen := newSnowflakeGenerator()

	var collected sync.Map

	var waitGroup sync.WaitGroup

	const goroutines = 100

	const idsPerGoroutine = 100

	waitGroup.Add(goroutines)

	for range goroutines {
		go func() {
			defer waitGroup.Done()

			for range idsPerGoroutine {
				id := gen.generate()

				_, loaded := collected.LoadOrStore(id, struct{}{})
				assert.False(t, loaded, "duplicate concurrent ID: %s", id)
			}
		}()
	}

	waitGroup.Wait()
}

func BenchmarkSnowflakeGenerator(b *testing.B) {
	gen := newSnowflakeGenerator()

	for b.Loop() {
		gen.generate()
	}
}

func BenchmarkSnowflakeGenerator_Parallel(b *testing.B) {
	gen := newSnowflakeGenerator()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			gen.generate()
		}
	})
}

func FuzzSnowflakeGenerator_Uniqueness(f *testing.F) {
	f.Add(1)
	f.Add(10)
	f.Add(100)
	f.Add(1000)

	f.Fuzz(func(t *testing.T, batchSize int) {
		if batchSize <= 0 || batchSize > 10000 {
			t.Skip("batch size out of range")
		}

		gen := newSnowflakeGenerator()
		seen := make(map[string]struct{}, batchSize)

		for range batchSize {
			id := gen.generate()

			assert.Len(t, id, 16, "ID should be 16 hex chars")

			raw, err := hex.DecodeString(id)
			require.NoError(t, err, "ID should be valid hex")

			val := binary.BigEndian.Uint64(raw)
			timestamp := val >> snowflakeTimestampShift
			assert.NotZero(t, timestamp, "timestamp bits should be non-zero")

			_, exists := seen[id]
			assert.False(t, exists, "duplicate ID: %s", id)

			seen[id] = struct{}{}
		}
	})
}

func FuzzSnowflakeGenerator_Structure(f *testing.F) {
	f.Add(1)
	f.Add(10)
	f.Add(100)

	f.Fuzz(func(t *testing.T, batchSize int) {
		if batchSize <= 0 || batchSize > 10000 {
			t.Skip("batch size out of range")
		}

		gen := newSnowflakeGenerator()

		hostname, _ := os.Hostname()
		h := fnv.New64a()
		_, _ = h.Write([]byte(hostname))
		wantMachineID := h.Sum64() & snowflakeMachineMask

		maxTimestampMs := int64(100 * 365 * 24 * 60 * 60 * 1000)

		for range batchSize {
			id := gen.generate()

			raw, err := hex.DecodeString(id)
			require.NoError(t, err)

			val := binary.BigEndian.Uint64(raw)
			seq := val & snowflakeMaxSequence
			machineID := (val >> snowflakeMachineShift) & snowflakeMachineMask
			timestamp := int64(val >> snowflakeTimestampShift)

			assert.Positive(t, timestamp, "timestamp should be after epoch")
			assert.Less(t, timestamp, maxTimestampMs, "timestamp should be before reasonable future")
			assert.Equal(t, wantMachineID, machineID, "machine ID should be consistent")
			assert.LessOrEqual(t, seq, snowflakeMaxSequence, "sequence should be within 0-127")
		}
	})
}

func TestRequestID_GeneratesNewID(t *testing.T) {
	t.Parallel()

	handler := RequestID()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := GetRequestID(r.Context())
		assert.NotEmpty(t, id)
		assert.Len(t, id, 16)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	responseID := rec.Header().Get(RequestIDHeader)
	assert.NotEmpty(t, responseID)
	assert.Len(t, responseID, 16)
}

func TestRequestID_ReusesExistingID(t *testing.T) {
	t.Parallel()

	existingID := "abcdef1234567890"

	var contextID string

	handler := RequestID()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contextID = GetRequestID(r.Context())

		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set(RequestIDHeader, existingID)

	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, existingID, rec.Header().Get(RequestIDHeader))
	assert.Equal(t, existingID, contextID)
}

func TestRequestID_RejectsOverlyLongID(t *testing.T) {
	t.Parallel()

	longID := strings.Repeat("a", maxRequestIDLength+1)

	handler := RequestID()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := GetRequestID(r.Context())
		assert.NotEqual(t, longID, id, "overly long ID should be replaced")
		assert.Len(t, id, 16, "should generate a new 16-char ID")
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set(RequestIDHeader, longID)

	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	responseID := rec.Header().Get(RequestIDHeader)
	assert.Len(t, responseID, 16)
}

func TestRequestID_HeaderPropagation(t *testing.T) {
	t.Parallel()

	handler := RequestID()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	responseID := rec.Header().Get(RequestIDHeader)
	assert.NotEmpty(t, responseID, "X-Request-ID should be set in response")
}

func TestRequestID_ContextStorage(t *testing.T) {
	t.Parallel()

	var capturedID string

	handler := RequestID()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedID = GetRequestID(r.Context())

		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	responseID := rec.Header().Get(RequestIDHeader)
	assert.Equal(t, responseID, capturedID, "context ID should match response header")
}

func TestGetRequestID_EmptyContext(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	id := GetRequestID(req.Context())

	assert.Empty(t, id, "should return empty string for context without request ID")
}

func TestRequestID_RejectsControlCharacters(t *testing.T) {
	t.Parallel()

	handler := RequestID()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := GetRequestID(r.Context())
		assert.Len(t, id, 16, "should generate a new 16-char ID for input with control chars")
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set(RequestIDHeader, "evil\x00injection")

	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	responseID := rec.Header().Get(RequestIDHeader)
	assert.Len(t, responseID, 16)
}

func TestIsPrintableASCII(t *testing.T) { //nolint:paralleltest // table-driven subtests
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"printable", "abc-123_XYZ", true},
		{"empty", "", true},
		{"null byte", "abc\x00def", false},
		{"tab", "abc\tdef", false},
		{"high byte", "abc\x80def", false},
		{"space", "hello world", true},
		{"tilde", "~", true},
	}

	for _, tt := range tests { //nolint:paralleltest // subtests share table-driven data
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isPrintableASCII(tt.input))
		})
	}
}
