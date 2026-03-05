package middleware

import (
	"log/slog"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	defaultIPRateRequests        = 10
	defaultIPRateWindow          = time.Second
	defaultIPRateCleanupInterval = 5 * time.Minute
	defaultIPRateStaleDuration   = 10 * time.Minute
	xffSplitLimit                = 2
)

// perIPRateLimitConfig holds configuration for the per-IP rate limiter.
type perIPRateLimitConfig struct {
	requests        int
	window          time.Duration
	burst           int
	cleanupInterval time.Duration
	staleDuration   time.Duration
	keyFunc         func(*http.Request) string
}

// PerIPRateLimitOption configures the PerIPRateLimit middleware.
type PerIPRateLimitOption func(*perIPRateLimitConfig)

// WithRateLimit sets the maximum number of requests allowed per window.
// If requests is not positive, it defaults to 10 with a warning log.
// If window is not positive, it defaults to 1s with a warning log.
func WithRateLimit(requests int, window time.Duration) PerIPRateLimitOption {
	return func(c *perIPRateLimitConfig) {
		c.requests = requests
		c.window = window
	}
}

// WithBurst sets the burst allowance above the sliding window limit.
// Burst allows short traffic spikes to pass through above the base rate.
// Default is 0 (strict sliding window, no burst).
func WithBurst(n int) PerIPRateLimitOption {
	return func(c *perIPRateLimitConfig) {
		c.burst = n
	}
}

// WithCleanupInterval sets how often stale IP entries are evicted.
// Default is 5 minutes. If not positive, defaults to 5 minutes with a warning log.
func WithCleanupInterval(d time.Duration) PerIPRateLimitOption {
	return func(c *perIPRateLimitConfig) {
		c.cleanupInterval = d
	}
}

// WithStaleDuration sets how long an idle IP entry lives before eviction.
// Default is 10 minutes. If not positive, defaults to 10 minutes with a warning log.
func WithStaleDuration(d time.Duration) PerIPRateLimitOption {
	return func(c *perIPRateLimitConfig) {
		c.staleDuration = d
	}
}

// WithKeyFunc sets a custom key extraction function for rate limiting.
// This overrides the default IP-based extraction, allowing rate limiting by
// API key, user ID, or any other request attribute.
func WithKeyFunc(fn func(*http.Request) string) PerIPRateLimitOption {
	return func(c *perIPRateLimitConfig) {
		c.keyFunc = fn
	}
}

// ipEntry tracks the sliding window state for a single key (IP or custom).
type ipEntry struct {
	mu          sync.Mutex
	prevCount   int
	currCount   int
	windowStart time.Time
	lastSeen    time.Time
}

// ipRateLimiter holds the shared state for the per-IP rate limiter.
type ipRateLimiter struct {
	entries sync.Map
	cfg     perIPRateLimitConfig
	running atomic.Bool
	now     func() time.Time
}

// extractClientIP extracts the client IP from the request.
// Priority: X-Forwarded-For (first entry) > X-Real-IP > RemoteAddr (host part only).
func extractClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.SplitN(xff, ",", xffSplitLimit)
		if ip := strings.TrimSpace(parts[0]); ip != "" {
			return ip
		}
	}

	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}

	return host
}

func (l *ipRateLimiter) startCleanup() {
	if l.running.CompareAndSwap(false, true) {
		go l.cleanupLoop()
	}
}

func (l *ipRateLimiter) cleanupLoop() {
	ticker := time.NewTicker(l.cfg.cleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		if l.runCleanup() {
			l.running.Store(false)

			return
		}
	}
}

// runCleanup evicts stale entries and returns true if no entries remain.
func (l *ipRateLimiter) runCleanup() bool {
	now := l.now()
	empty := true

	l.entries.Range(func(key, value any) bool {
		entry, ok := value.(*ipEntry)
		if !ok {
			return true
		}

		entry.mu.Lock()
		stale := now.Sub(entry.lastSeen) > l.cfg.staleDuration
		entry.mu.Unlock()

		if stale {
			l.entries.Delete(key)
		} else {
			empty = false
		}

		return true
	})

	return empty
}

func (l *ipRateLimiter) allow(key string) (bool, time.Duration) {
	now := l.now()
	val, _ := l.entries.LoadOrStore(key, &ipEntry{
		windowStart: now,
		lastSeen:    now,
	})

	entry, ok := val.(*ipEntry)
	if !ok {
		return true, 0
	}

	entry.mu.Lock()
	defer entry.mu.Unlock()

	entry.lastSeen = now

	elapsed := now.Sub(entry.windowStart)

	switch {
	case elapsed >= 2*l.cfg.window:
		// Skipped more than one full window; reset everything.
		entry.prevCount = 0
		entry.currCount = 0
		entry.windowStart = now
	case elapsed >= l.cfg.window:
		// Moved to a new window; rotate counts.
		entry.prevCount = entry.currCount
		entry.currCount = 0
		entry.windowStart = entry.windowStart.Add(l.cfg.window)
	}

	windowElapsed := now.Sub(entry.windowStart)
	fraction := float64(windowElapsed) / float64(l.cfg.window)
	estimate := float64(entry.prevCount)*(1.0-fraction) + float64(entry.currCount)

	limit := float64(l.cfg.requests + l.cfg.burst)

	if estimate >= limit {
		remaining := l.cfg.window - windowElapsed
		retrySeconds := max(int(math.Ceil(remaining.Seconds())), 1)

		return false, time.Duration(retrySeconds) * time.Second
	}

	entry.currCount++

	return true, 0
}

func applyIPRateLimitDefaults(cfg *perIPRateLimitConfig) {
	if cfg.requests <= 0 {
		slog.Warn("middleware: PerIPRateLimit requests must be positive, using default",
			"provided", cfg.requests, "default", defaultIPRateRequests)

		cfg.requests = defaultIPRateRequests
	}

	if cfg.window <= 0 {
		slog.Warn("middleware: PerIPRateLimit window must be positive, using default",
			"provided", cfg.window, "default", defaultIPRateWindow)

		cfg.window = defaultIPRateWindow
	}

	if cfg.burst < 0 {
		slog.Warn("middleware: PerIPRateLimit burst must be non-negative, using default",
			"provided", cfg.burst, "default", 0)

		cfg.burst = 0
	}

	if cfg.cleanupInterval <= 0 {
		slog.Warn("middleware: PerIPRateLimit cleanupInterval must be positive, using default",
			"provided", cfg.cleanupInterval, "default", defaultIPRateCleanupInterval)

		cfg.cleanupInterval = defaultIPRateCleanupInterval
	}

	if cfg.staleDuration <= 0 {
		slog.Warn("middleware: PerIPRateLimit staleDuration must be positive, using default",
			"provided", cfg.staleDuration, "default", defaultIPRateStaleDuration)

		cfg.staleDuration = defaultIPRateStaleDuration
	}
}

// PerIPRateLimit returns a middleware that enforces per-IP rate limiting using a
// sliding window counter algorithm. Each unique client IP (or custom key via
// WithKeyFunc) gets independent rate tracking. When a key exceeds its limit,
// the middleware responds with 429 Too Many Requests and a Retry-After header.
//
// The sliding window algorithm interpolates between the previous and current
// window counts for smoother rate limiting than fixed windows.
//
// Options:
//   - WithRateLimit(requests, window) - max requests per window (default: 10 req/1s)
//   - WithBurst(n) - extra burst capacity above the base rate (default: 0)
//   - WithCleanupInterval(d) - stale entry eviction interval (default: 5 min)
//   - WithStaleDuration(d) - idle entry lifetime before eviction (default: 10 min)
//   - WithKeyFunc(fn) - custom key extraction replacing IP-based lookup
func PerIPRateLimit(opts ...PerIPRateLimitOption) func(http.Handler) http.Handler {
	cfg := perIPRateLimitConfig{
		requests:        defaultIPRateRequests,
		window:          defaultIPRateWindow,
		burst:           0,
		cleanupInterval: defaultIPRateCleanupInterval,
		staleDuration:   defaultIPRateStaleDuration,
	}

	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}

	applyIPRateLimitDefaults(&cfg)

	limiter := &ipRateLimiter{
		cfg: cfg,
		now: time.Now,
	}

	keyFunc := cfg.keyFunc
	if keyFunc == nil {
		keyFunc = extractClientIP
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			limiter.startCleanup()

			key := keyFunc(r)
			allowed, retryAfter := limiter.allow(key)

			if !allowed {
				w.Header().Set("Retry-After", strconv.Itoa(max(int(retryAfter.Seconds()), 1)))
				http.Error(w, "Too Many Requests", http.StatusTooManyRequests)

				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
