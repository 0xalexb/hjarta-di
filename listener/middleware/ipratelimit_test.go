package middleware

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testAddr1 = "10.0.0.1:1234"
	testAddr2 = "10.0.0.2:1234"
	testAddr3 = "192.168.1.1:1234"
)

func newTestLimiter(requests int, window time.Duration, burst int) *ipRateLimiter {
	return &ipRateLimiter{
		cfg: perIPRateLimitConfig{
			requests:        requests,
			window:          window,
			burst:           burst,
			cleanupInterval: 5 * time.Minute,
			staleDuration:   10 * time.Minute,
		},
		now: time.Now,
	}
}

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func TestPerIPRateLimit_IndependentIPLimits(t *testing.T) { //nolint:paralleltest // shared state
	handler := PerIPRateLimit(
		WithRateLimit(2, time.Second),
	)(okHandler())

	// IP1 makes 2 requests - both succeed.
	for i := range 2 {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = testAddr1
		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code, "IP1 request %d should succeed", i)
	}

	// IP2 makes 2 requests - both succeed (independent limit).
	for i := range 2 {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = testAddr2
		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code, "IP2 request %d should succeed", i)
	}

	// IP1 next request should be limited.
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = testAddr1
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusTooManyRequests, rr.Code, "IP1 should be rate limited")

	// IP2 next request should also be limited.
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = testAddr2
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusTooManyRequests, rr.Code, "IP2 should be rate limited")
}

func TestPerIPRateLimit_SingleIPExceedsLimitOtherUnaffected(t *testing.T) { //nolint:paralleltest // shared state
	handler := PerIPRateLimit(
		WithRateLimit(3, time.Second),
	)(okHandler())

	// IP1 exhausts its limit.
	for range 3 {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = testAddr1
		handler.ServeHTTP(rr, req)
		require.Equal(t, http.StatusOK, rr.Code)
	}

	// IP1 is limited.
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = testAddr1
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusTooManyRequests, rr.Code)

	// IP2 is unaffected.
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = testAddr2
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestPerIPRateLimit_SlidingWindowInterpolation(t *testing.T) { //nolint:paralleltest // shared state
	// Use the limiter directly for precise time control.
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	timeMu := &sync.Mutex{}

	limiter := newTestLimiter(10, time.Second, 0)
	limiter.now = func() time.Time {
		timeMu.Lock()
		defer timeMu.Unlock()

		return now
	}

	advanceTime := func(d time.Duration) {
		timeMu.Lock()
		defer timeMu.Unlock()

		now = now.Add(d)
	}

	// Fill up the first window with 10 requests.
	for range 10 {
		allowed, _ := limiter.allow("testip")
		require.True(t, allowed)
	}

	// 11th should be rejected.
	allowed, _ := limiter.allow("testip")
	assert.False(t, allowed, "11th request in same window should be rejected")

	// Move to halfway through the next window (0.5s into 1s window).
	// prevCount=10, currCount=0, fraction=0.5
	// estimate = 10 * (1 - 0.5) + 0 = 5.0
	// So we should be able to make 5 more requests (estimates: 5,6,7,8,9 all < 10).
	advanceTime(time.Second + 500*time.Millisecond)

	successCount := 0

	for range 10 {
		allowed, _ := limiter.allow("testip")
		if allowed {
			successCount++
		} else {
			break
		}
	}

	assert.Equal(t, 5, successCount, "should allow 5 requests at 50%% into next window with prevCount=10")
}

func TestPerIPRateLimit_SlidingWindowSkipsTwoWindows(t *testing.T) { //nolint:paralleltest // shared state
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	timeMu := &sync.Mutex{}

	limiter := newTestLimiter(5, time.Second, 0)
	limiter.now = func() time.Time {
		timeMu.Lock()
		defer timeMu.Unlock()

		return now
	}

	advanceTime := func(d time.Duration) {
		timeMu.Lock()
		defer timeMu.Unlock()

		now = now.Add(d)
	}

	// Fill up the window.
	for range 5 {
		allowed, _ := limiter.allow("testip")
		require.True(t, allowed)
	}

	// Skip more than two full windows - state should be fully reset.
	advanceTime(3 * time.Second)

	// Should have full capacity again.
	for i := range 5 {
		allowed, _ := limiter.allow("testip")
		assert.True(t, allowed, "request %d after reset should succeed", i)
	}

	allowed, _ := limiter.allow("testip")
	assert.False(t, allowed, "6th request should be rejected after full window")
}

func TestPerIPRateLimit_BurstAllowance(t *testing.T) { //nolint:paralleltest // shared state
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	timeMu := &sync.Mutex{}

	limiter := newTestLimiter(5, time.Second, 3)
	limiter.now = func() time.Time {
		timeMu.Lock()
		defer timeMu.Unlock()

		return now
	}

	// With limit=5 and burst=3, effective limit is 8.
	for i := range 8 {
		allowed, _ := limiter.allow("testip")
		assert.True(t, allowed, "request %d should succeed within burst allowance", i)
	}

	// 9th request should be rejected.
	allowed, _ := limiter.allow("testip")
	assert.False(t, allowed, "request exceeding burst should be rejected")
}

func TestPerIPRateLimit_IPExtractionXForwardedFor(t *testing.T) { //nolint:paralleltest // shared state
	handler := PerIPRateLimit(
		WithRateLimit(1, time.Second),
	)(okHandler())

	// Request with X-Forwarded-For (multiple IPs, first one is used).
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "1.2.3.4, 5.6.7.8")
	req.RemoteAddr = testAddr1
	handler.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)

	// Same XFF first IP should be rate limited.
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "1.2.3.4, 9.9.9.9")
	req.RemoteAddr = "10.0.0.2:5678"
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusTooManyRequests, rr.Code, "same XFF first IP should share the limit")

	// Different XFF first IP should be independent.
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "5.6.7.8, 1.2.3.4")
	req.RemoteAddr = testAddr1
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code, "different XFF first IP should be independent")
}

func TestPerIPRateLimit_IPExtractionXRealIP(t *testing.T) { //nolint:paralleltest // shared state
	handler := PerIPRateLimit(
		WithRateLimit(1, time.Second),
	)(okHandler())

	// Request with X-Real-IP (no X-Forwarded-For).
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Real-IP", "2.3.4.5")
	req.RemoteAddr = testAddr1
	handler.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)

	// Same X-Real-IP should be rate limited.
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Real-IP", "2.3.4.5")
	req.RemoteAddr = "10.0.0.2:5678"
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusTooManyRequests, rr.Code, "same X-Real-IP should share the limit")
}

func TestPerIPRateLimit_IPExtractionRemoteAddr(t *testing.T) { //nolint:paralleltest // shared state
	handler := PerIPRateLimit(
		WithRateLimit(1, time.Second),
	)(okHandler())

	// Request with only RemoteAddr (no proxy headers).
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.168.1.1:9999"
	handler.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)

	// Same IP (different port) should share the limit.
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.168.1.1:8888"
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusTooManyRequests, rr.Code, "same IP different port should share the limit")
}

func TestPerIPRateLimit_CustomKeyFunc(t *testing.T) { //nolint:paralleltest // shared state
	handler := PerIPRateLimit(
		WithRateLimit(1, time.Second),
		WithKeyFunc(func(r *http.Request) string {
			return r.Header.Get("X-Api-Key")
		}),
	)(okHandler())

	// API key "key-a" makes 1 request.
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Api-Key", "key-a")
	handler.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)

	// Same API key is limited.
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Api-Key", "key-a")
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusTooManyRequests, rr.Code, "same API key should be rate limited")

	// Different API key is independent.
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Api-Key", "key-b")
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code, "different API key should be independent")
}

func TestPerIPRateLimit_StaleCleanup(t *testing.T) { //nolint:paralleltest // shared state
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	timeMu := &sync.Mutex{}

	limiter := &ipRateLimiter{
		cfg: perIPRateLimitConfig{
			requests:        5,
			window:          time.Second,
			burst:           0,
			cleanupInterval: time.Minute,
			staleDuration:   30 * time.Second,
		},
		now: func() time.Time {
			timeMu.Lock()
			defer timeMu.Unlock()

			return now
		},
	}

	advanceTime := func(d time.Duration) {
		timeMu.Lock()
		defer timeMu.Unlock()

		now = now.Add(d)
	}

	// Create entries for two IPs.
	limiter.allow("ip-1")
	limiter.allow("ip-2")

	// Verify both entries exist.
	count := 0

	limiter.entries.Range(func(_, _ any) bool {
		count++

		return true
	})

	require.Equal(t, 2, count, "should have 2 entries")

	// Advance past stale duration.
	advanceTime(31 * time.Second)

	// Run cleanup.
	empty := limiter.runCleanup()
	assert.True(t, empty, "cleanup should report empty after all entries are stale")

	// Verify entries are removed.
	count = 0

	limiter.entries.Range(func(_, _ any) bool {
		count++

		return true
	})

	assert.Equal(t, 0, count, "all stale entries should be removed")
}

func TestPerIPRateLimit_StaleCleanupKeepsFreshEntries(t *testing.T) { //nolint:paralleltest // shared state
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	timeMu := &sync.Mutex{}

	limiter := &ipRateLimiter{
		cfg: perIPRateLimitConfig{
			requests:        5,
			window:          time.Second,
			burst:           0,
			cleanupInterval: time.Minute,
			staleDuration:   30 * time.Second,
		},
		now: func() time.Time {
			timeMu.Lock()
			defer timeMu.Unlock()

			return now
		},
	}

	advanceTime := func(d time.Duration) {
		timeMu.Lock()
		defer timeMu.Unlock()

		now = now.Add(d)
	}

	// Create entries for two IPs at different times.
	limiter.allow("ip-old")

	advanceTime(25 * time.Second)

	limiter.allow("ip-fresh")

	// Advance so ip-old is stale (>30s) but ip-fresh is not (<30s).
	advanceTime(10 * time.Second)

	empty := limiter.runCleanup()
	assert.False(t, empty, "cleanup should report not empty when fresh entries exist")

	// Verify only old entry is removed.
	_, oldExists := limiter.entries.Load("ip-old")
	_, freshExists := limiter.entries.Load("ip-fresh")

	assert.False(t, oldExists, "stale entry should be removed")
	assert.True(t, freshExists, "fresh entry should be kept")
}

func TestPerIPRateLimit_DefaultParameters(t *testing.T) { //nolint:paralleltest // shared state
	// Creating with defaults should work without panicking.
	handler := PerIPRateLimit()(okHandler())

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestPerIPRateLimit_InvalidRequestsLogsWarning(t *testing.T) { //nolint:paralleltest // shared slog
	var buf bytes.Buffer

	oldDefault := slog.Default()
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))
	slog.SetDefault(logger)

	t.Cleanup(func() { slog.SetDefault(oldDefault) })

	handler := PerIPRateLimit(
		WithRateLimit(0, time.Second),
	)(okHandler())

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, buf.String(), "requests must be positive")
}

func TestPerIPRateLimit_InvalidWindowLogsWarning(t *testing.T) { //nolint:paralleltest // shared slog
	var buf bytes.Buffer

	oldDefault := slog.Default()
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))
	slog.SetDefault(logger)

	t.Cleanup(func() { slog.SetDefault(oldDefault) })

	handler := PerIPRateLimit(
		WithRateLimit(10, -time.Second),
	)(okHandler())

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, buf.String(), "window must be positive")
}

func TestPerIPRateLimit_NegativeBurstLogsWarning(t *testing.T) { //nolint:paralleltest // shared slog
	var buf bytes.Buffer

	oldDefault := slog.Default()
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))
	slog.SetDefault(logger)

	t.Cleanup(func() { slog.SetDefault(oldDefault) })

	handler := PerIPRateLimit(
		WithRateLimit(10, time.Second),
		WithBurst(-1),
	)(okHandler())

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, buf.String(), "burst must be non-negative")
}

func TestPerIPRateLimit_InvalidCleanupIntervalLogsWarning(t *testing.T) { //nolint:paralleltest // shared slog
	var buf bytes.Buffer

	oldDefault := slog.Default()
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))
	slog.SetDefault(logger)

	t.Cleanup(func() { slog.SetDefault(oldDefault) })

	PerIPRateLimit(
		WithRateLimit(10, time.Second),
		WithCleanupInterval(-time.Minute),
	)(okHandler())

	assert.Contains(t, buf.String(), "cleanupInterval must be positive")
}

func TestPerIPRateLimit_InvalidStaleDurationLogsWarning(t *testing.T) { //nolint:paralleltest // shared slog
	var buf bytes.Buffer

	oldDefault := slog.Default()
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))
	slog.SetDefault(logger)

	t.Cleanup(func() { slog.SetDefault(oldDefault) })

	PerIPRateLimit(
		WithRateLimit(10, time.Second),
		WithStaleDuration(0),
	)(okHandler())

	assert.Contains(t, buf.String(), "staleDuration must be positive")
}

func TestPerIPRateLimit_RetryAfterHeader(t *testing.T) { //nolint:paralleltest // shared state
	handler := PerIPRateLimit(
		WithRateLimit(1, 5*time.Second),
	)(okHandler())

	// Use up the limit.
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = testAddr1
	handler.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)

	// Next request should get 429 with Retry-After.
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = testAddr1
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusTooManyRequests, rr.Code)
	assert.Contains(t, rr.Body.String(), "Too Many Requests")

	retryAfter := rr.Header().Get("Retry-After")
	assert.NotEmpty(t, retryAfter, "Retry-After header should be set")

	seconds, err := strconv.Atoi(retryAfter)
	require.NoError(t, err, "Retry-After should be a valid integer")
	assert.GreaterOrEqual(t, seconds, 1, "Retry-After should be at least 1 second")
	assert.LessOrEqual(t, seconds, 5, "Retry-After should not exceed window size")
}

func TestPerIPRateLimit_RetryAfterValuePrecise(t *testing.T) { //nolint:paralleltest // shared state
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	timeMu := &sync.Mutex{}

	limiter := newTestLimiter(1, 10*time.Second, 0)
	limiter.now = func() time.Time {
		timeMu.Lock()
		defer timeMu.Unlock()

		return now
	}

	advanceTime := func(d time.Duration) {
		timeMu.Lock()
		defer timeMu.Unlock()

		now = now.Add(d)
	}

	// Use the single request.
	allowed, _ := limiter.allow("testip")
	require.True(t, allowed)

	// Advance 3 seconds into the 10-second window.
	advanceTime(3 * time.Second)

	// Next request should be rejected with ~7 seconds remaining.
	allowed, retryAfter := limiter.allow("testip")
	assert.False(t, allowed)
	assert.Equal(t, 7*time.Second, retryAfter, "Retry-After should be 7 seconds (10s window - 3s elapsed)")
}

func TestPerIPRateLimit_PassesThroughToHandler(t *testing.T) { //nolint:paralleltest // shared state
	called := false
	handler := PerIPRateLimit(
		WithRateLimit(10, time.Second),
	)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true

		w.Header().Set("X-Custom", "value")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("created"))
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/resource", nil)
	handler.ServeHTTP(rr, req)

	assert.True(t, called)
	assert.Equal(t, http.StatusCreated, rr.Code)
	assert.Equal(t, "value", rr.Header().Get("X-Custom"))
	assert.Equal(t, "created", rr.Body.String())
}

func TestExtractClientIP_XForwardedFor(t *testing.T) { //nolint:paralleltest // pure function test
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "1.2.3.4, 5.6.7.8, 9.10.11.12")
	req.RemoteAddr = testAddr3

	assert.Equal(t, "1.2.3.4", extractClientIP(req))
}

func TestExtractClientIP_XForwardedForSingle(t *testing.T) { //nolint:paralleltest // pure function test
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	req.RemoteAddr = testAddr3

	assert.Equal(t, "1.2.3.4", extractClientIP(req))
}

func TestExtractClientIP_XRealIP(t *testing.T) { //nolint:paralleltest // pure function test
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Real-IP", "2.3.4.5")
	req.RemoteAddr = testAddr3

	assert.Equal(t, "2.3.4.5", extractClientIP(req))
}

func TestExtractClientIP_RemoteAddrWithPort(t *testing.T) { //nolint:paralleltest // pure function test
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.168.1.1:9999"

	assert.Equal(t, "192.168.1.1", extractClientIP(req))
}

func TestExtractClientIP_RemoteAddrWithoutPort(t *testing.T) { //nolint:paralleltest // pure function test
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.168.1.1"

	assert.Equal(t, "192.168.1.1", extractClientIP(req))
}

func TestExtractClientIP_XForwardedForPriority(t *testing.T) { //nolint:paralleltest // pure function test
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	req.Header.Set("X-Real-IP", "5.6.7.8")
	req.RemoteAddr = testAddr3

	assert.Equal(t, "1.2.3.4", extractClientIP(req), "X-Forwarded-For should take priority over X-Real-IP")
}

func TestPerIPRateLimit_NilOption(t *testing.T) { //nolint:paralleltest // shared state
	// Should not panic with nil options.
	handler := PerIPRateLimit(nil, WithRateLimit(10, time.Second), nil)(okHandler())

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}
