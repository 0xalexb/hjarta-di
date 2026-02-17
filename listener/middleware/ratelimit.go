package middleware

import (
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"
)

type tokenBucket struct {
	mu              sync.Mutex
	tokens          float64
	maxTokens       float64
	refillRate      float64
	lastRefillTime  time.Time
}

func newTokenBucket(requestsPerSecond float64, burst int) *tokenBucket {
	return &tokenBucket{ //nolint:exhaustruct
		tokens:         float64(burst),
		maxTokens:      float64(burst),
		refillRate:     requestsPerSecond,
		lastRefillTime: time.Now(),
	}
}

func (tb *tokenBucket) tryAcquire() (bool, time.Duration) {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := time.Now()
	elapsed := max(0.0, now.Sub(tb.lastRefillTime).Seconds())
	tb.tokens = math.Min(tb.maxTokens, tb.tokens+elapsed*tb.refillRate)
	tb.lastRefillTime = now

	if tb.tokens >= 1 {
		tb.tokens--

		return true, 0
	}

	deficit := 1.0 - tb.tokens
	retryAfter := time.Duration(deficit / tb.refillRate * float64(time.Second))

	return false, retryAfter
}

// RateLimit returns a middleware that enforces a global rate limit using a
// token bucket algorithm. When the limit is exceeded, it responds with
// 429 Too Many Requests and includes a Retry-After header.
// If requestsPerSecond is not positive, it defaults to 1.0 with a warning log.
// If burst is not positive, it defaults to 1 with a warning log.
func RateLimit(requestsPerSecond float64, burst int) func(http.Handler) http.Handler {
	if requestsPerSecond <= 0 {
		slog.Warn("middleware: requestsPerSecond must be positive, using default",
			"provided", requestsPerSecond, "default", 1.0)

		requestsPerSecond = 1.0
	}

	if burst <= 0 {
		slog.Warn("middleware: burst must be positive, using default", "provided", burst, "default", 1)
		burst = 1
	}

	bucket := newTokenBucket(requestsPerSecond, burst)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { //nolint:varnamelen
			allowed, retryAfter := bucket.tryAcquire()
			if !allowed {
				seconds := max(int(math.Ceil(retryAfter.Seconds())), 1)

				w.Header().Set("Retry-After", strconv.Itoa(seconds))
				http.Error(w, "Too Many Requests", http.StatusTooManyRequests)

				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
