package middleware

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"hash/fnv"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"
)

const (
	// RequestIDHeader is the HTTP header used for request IDs.
	RequestIDHeader = "X-Request-ID"

	// maxRequestIDLength is the maximum allowed length for an externally-provided request ID.
	maxRequestIDLength = 256

	// snowflakeEpochMs is 2026-01-01 00:00:00 UTC in milliseconds since Unix epoch.
	snowflakeEpochMs int64 = 1767225600000

	// Bit widths for snowflake ID components.
	snowflakeMachineBits  = 16
	snowflakeSequenceBits = 7

	// Masks for snowflake ID components.
	snowflakeMaxSequence uint64 = (1 << snowflakeSequenceBits) - 1 // 0x7F = 127
	snowflakeMachineMask uint64 = (1 << snowflakeMachineBits) - 1 // 0xFFFF = 65535

	// maxClockDriftMs is the maximum backward clock drift (in ms) the generator
	// will spin-wait for. Beyond this threshold it resets rather than blocking.
	maxClockDriftMs int64 = 500

	// Bit shifts for composing the snowflake ID.
	snowflakeMachineShift   = snowflakeSequenceBits                          // 7
	snowflakeTimestampShift = snowflakeSequenceBits + snowflakeMachineBits   // 23
)

type requestIDKeyType struct{}

var requestIDKey = requestIDKeyType{} //nolint:gochecknoglobals

// snowflakeGenerator produces snowflake-like unique IDs composed of
// 41 bits timestamp (ms since 2026-01-01 UTC), 16 bits machine hash,
// and 7 bits sequence counter.
type snowflakeGenerator struct {
	mu            sync.Mutex
	machineID     uint64
	sequence      uint64
	lastTimestamp int64
	timeNow       func() time.Time
}

// newSnowflakeGenerator creates a snowflake generator with machine ID
// derived from FNV-1a hash of the hostname.
func newSnowflakeGenerator() *snowflakeGenerator {
	hostname, err := os.Hostname()
	if err != nil {
		slog.Warn("middleware: failed to get hostname for snowflake generator, using empty string",
			"error", err)

		hostname = ""
	}

	h := fnv.New64a()
	_, _ = h.Write([]byte(hostname))

	return &snowflakeGenerator{
		machineID: h.Sum64() & snowflakeMachineMask,
		timeNow:   time.Now,
	}
}

// generate produces a unique 16-character hex string snowflake ID.
func (g *snowflakeGenerator) generate() string {
	g.mu.Lock()
	defer g.mu.Unlock()

	now := g.currentTimestampMs()

	// Clock went backward: spin-wait for small drifts, reset for large ones.
	if now < g.lastTimestamp {
		drift := g.lastTimestamp - now

		if drift > maxClockDriftMs {
			slog.Error("middleware: clock moved backward beyond tolerance, advancing from last known timestamp",
				"drift", time.Duration(drift)*time.Millisecond,
				"maxDrift", time.Duration(maxClockDriftMs)*time.Millisecond)

			now = g.lastTimestamp + 1
			g.sequence = 0
		} else {
			slog.Warn("middleware: clock moved backward, spin-waiting",
				"drift", time.Duration(drift)*time.Millisecond)

			for now < g.lastTimestamp {
				now = g.currentTimestampMs()
			}
		}
	}

	if now == g.lastTimestamp {
		g.sequence++

		// Sequence overflow: spin-wait until next millisecond.
		if g.sequence > snowflakeMaxSequence {
			for now <= g.lastTimestamp {
				now = g.currentTimestampMs()
			}

			g.sequence = 0
		}
	} else {
		g.sequence = 0
	}

	g.lastTimestamp = now

	ts := max(now, 0)

	id := (uint64(ts) << snowflakeTimestampShift) |
		(g.machineID << snowflakeMachineShift) |
		g.sequence

	var buf [8]byte

	binary.BigEndian.PutUint64(buf[:], id)

	return hex.EncodeToString(buf[:])
}

// currentTimestampMs returns the current time in milliseconds since the snowflake epoch.
func (g *snowflakeGenerator) currentTimestampMs() int64 {
	return g.timeNow().UnixMilli() - snowflakeEpochMs
}

// GetRequestID retrieves the request ID from the context.
func GetRequestID(ctx context.Context) string {
	val, ok := ctx.Value(requestIDKey).(string)
	if !ok {
		return ""
	}

	return val
}

// isPrintableASCII reports whether s contains only printable ASCII characters (0x20-0x7E).
func isPrintableASCII(s string) bool {
	for i := range len(s) {
		if s[i] < 0x20 || s[i] > 0x7E {
			return false
		}
	}

	return true
}

// RequestID is a middleware that assigns a unique snowflake-based request ID to each request.
// The ID is a 16-character hex string encoding a 64-bit snowflake composed of:
// 41 bits timestamp (ms since 2026-01-01 UTC), 16 bits machine hash (FNV-1a of hostname),
// and 7 bits sequence counter.
// If the X-Request-ID header is already present in the request, it reuses that value.
// Otherwise, it generates a new snowflake ID. The ID is stored in the request context
// and set as the X-Request-ID response header.
func RequestID() func(http.Handler) http.Handler {
	gen := newSnowflakeGenerator()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := r.Header.Get(RequestIDHeader)
			if id == "" || len(id) > maxRequestIDLength || !isPrintableASCII(id) {
				id = gen.generate()
			}

			r.Header.Set(RequestIDHeader, id)
			w.Header().Set(RequestIDHeader, id)

			ctx := context.WithValue(r.Context(), requestIDKey, id)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
