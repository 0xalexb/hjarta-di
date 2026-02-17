package middleware

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRateLimit_RequestsWithinLimit(t *testing.T) { //nolint:paralleltest // uses shared rate limiter state
	handler := RateLimit(10, 5)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for i := range 5 { //nolint:varnamelen // i is conventional for loop index
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code, "request %d should succeed", i)
	}
}

func TestRateLimit_ExceedingLimitReturns429(t *testing.T) { //nolint:paralleltest // uses shared rate limiter state
	handler := RateLimit(1, 2)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Exhaust the burst.
	for range 2 {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		handler.ServeHTTP(rr, req)
		require.Equal(t, http.StatusOK, rr.Code)
	}

	// Next request should be rate limited.
	rr := httptest.NewRecorder() //nolint:varnamelen // rr is conventional for recorder
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusTooManyRequests, rr.Code)
	assert.NotEmpty(t, rr.Header().Get("Retry-After"))
	assert.Contains(t, rr.Body.String(), "Too Many Requests")
}

func TestRateLimit_TokenReplenishmentOverTime(t *testing.T) { //nolint:paralleltest // uses shared rate limiter state
	handler := RateLimit(100, 1)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Use the single burst token.
	rr := httptest.NewRecorder() //nolint:varnamelen // rr is conventional for httptest.ResponseRecorder
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)

	// Should be limited now.
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(rr, req)
	require.Equal(t, http.StatusTooManyRequests, rr.Code)

	// Wait for tokens to replenish (100 rps = 10ms per token, wait a bit more).
	time.Sleep(50 * time.Millisecond)

	// Should succeed again after replenishment.
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestRateLimit_RetryAfterHeader(t *testing.T) { //nolint:paralleltest // uses shared rate limiter state
	handler := RateLimit(1, 1)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Exhaust tokens.
	rr := httptest.NewRecorder() //nolint:varnamelen // rr is conventional for recorder
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)

	// Should get Retry-After header.
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusTooManyRequests, rr.Code)
	assert.Equal(t, "1", rr.Header().Get("Retry-After"))
}

func TestRateLimit_PassesThroughToHandler(t *testing.T) { //nolint:paralleltest // uses shared rate limiter state
	called := false
	handler := RateLimit(10, 10)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true

		w.Header().Set("X-Custom", "value")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("created"))
	}))

	rr := httptest.NewRecorder() //nolint:varnamelen // rr is conventional for recorder
	req := httptest.NewRequest(http.MethodPost, "/resource", nil)
	handler.ServeHTTP(rr, req)

	assert.True(t, called)
	assert.Equal(t, http.StatusCreated, rr.Code)
	assert.Equal(t, "value", rr.Header().Get("X-Custom"))
	assert.Equal(t, "created", rr.Body.String())
}

func TestRateLimit_DefaultsOnZeroRate(t *testing.T) { //nolint:paralleltest // uses shared rate limiter state
	var buf bytes.Buffer

	oldDefault := slog.Default()
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))
	slog.SetDefault(logger)

	t.Cleanup(func() { slog.SetDefault(oldDefault) })

	handler := RateLimit(0, 1)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, buf.String(), "requestsPerSecond must be positive")
}

func TestRateLimit_DefaultsOnNegativeRate(t *testing.T) { //nolint:paralleltest // uses shared rate limiter state
	var buf bytes.Buffer

	oldDefault := slog.Default()
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))
	slog.SetDefault(logger)

	t.Cleanup(func() { slog.SetDefault(oldDefault) })

	handler := RateLimit(-1, 1)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, buf.String(), "requestsPerSecond must be positive")
}

func TestRateLimit_DefaultsOnZeroBurst(t *testing.T) { //nolint:paralleltest // uses shared rate limiter state
	var buf bytes.Buffer

	oldDefault := slog.Default()
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))
	slog.SetDefault(logger)

	t.Cleanup(func() { slog.SetDefault(oldDefault) })

	handler := RateLimit(10, 0)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, buf.String(), "burst must be positive")
}

func TestRateLimit_DefaultsOnNegativeBurst(t *testing.T) { //nolint:paralleltest // uses shared rate limiter state
	var buf bytes.Buffer

	oldDefault := slog.Default()
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))
	slog.SetDefault(logger)

	t.Cleanup(func() { slog.SetDefault(oldDefault) })

	handler := RateLimit(10, -1)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, buf.String(), "burst must be positive")
}
