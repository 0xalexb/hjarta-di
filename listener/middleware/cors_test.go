package middleware

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCORS_PreflightRequest(t *testing.T) {
	t.Parallel()

	cfg := CORSConfig{
		AllowedOrigins: []string{"https://example.com"},
		AllowedMethods: []string{"GET", "POST"},
		AllowedHeaders: []string{"Content-Type", "Authorization"},
		MaxAge:         3600,
	}

	handler := CORS(cfg)(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Error("handler should not be called for preflight")
	}))

	req := httptest.NewRequest(http.MethodOptions, "/api/data", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")

	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, "https://example.com", rec.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "GET, POST", rec.Header().Get("Access-Control-Allow-Methods"))
	assert.Equal(t, "Content-Type, Authorization", rec.Header().Get("Access-Control-Allow-Headers"))
	assert.Equal(t, "3600", rec.Header().Get("Access-Control-Max-Age"))
}

func TestCORS_OriginMatching(t *testing.T) {
	t.Parallel()

	cfg := CORSConfig{
		AllowedOrigins: []string{"https://example.com", "https://other.com"},
		AllowedMethods: []string{"GET"},
	}

	nextCalled := false
	handler := CORS(cfg)(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		nextCalled = true
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://example.com")

	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	require.True(t, nextCalled)
	assert.Equal(t, "https://example.com", rec.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORS_OriginNotAllowed(t *testing.T) {
	t.Parallel()

	cfg := CORSConfig{
		AllowedOrigins: []string{"https://example.com"},
	}

	nextCalled := false
	handler := CORS(cfg)(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		nextCalled = true
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://evil.com")

	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	require.True(t, nextCalled)
	assert.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORS_WildcardOrigin(t *testing.T) {
	t.Parallel()

	cfg := CORSConfig{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST"},
	}

	handler := CORS(cfg)(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://any-origin.com")

	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, "*", rec.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORS_WildcardWithCredentials(t *testing.T) { //nolint:paralleltest // modifies global slog default
	var buf bytes.Buffer

	oldDefault := slog.Default()

	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))

	t.Cleanup(func() { slog.SetDefault(oldDefault) })

	cfg := CORSConfig{
		AllowedOrigins:   []string{"*"},
		AllowCredentials: true,
	}

	handler := CORS(cfg)(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://any-origin.com")

	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Contains(t, buf.String(), "CORS AllowCredentials with only wildcard origin is invalid")
	assert.Empty(t, rec.Header().Get("Access-Control-Allow-Credentials"))
	assert.Equal(t, "*", rec.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORS_WildcardWithCredentialsExplicitOrigin(t *testing.T) {
	t.Parallel()

	cfg := CORSConfig{
		AllowedOrigins:   []string{"*", "https://trusted.com"},
		AllowCredentials: true,
	}

	handler := CORS(cfg)(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://trusted.com")

	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Explicitly listed origin should still work with credentials.
	assert.Equal(t, "https://trusted.com", rec.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "true", rec.Header().Get("Access-Control-Allow-Credentials"))
}

func TestCORS_CredentialsFlag(t *testing.T) {
	t.Parallel()

	cfg := CORSConfig{
		AllowedOrigins:   []string{"https://example.com"},
		AllowCredentials: true,
	}

	handler := CORS(cfg)(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://example.com")

	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, "true", rec.Header().Get("Access-Control-Allow-Credentials"))
	assert.Equal(t, "https://example.com", rec.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORS_NormalRequestPassThrough(t *testing.T) {
	t.Parallel()

	cfg := CORSConfig{
		AllowedOrigins: []string{"https://example.com"},
	}

	nextCalled := false
	handler := CORS(cfg)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		nextCalled = true

		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/data", nil)
	req.Header.Set("Origin", "https://example.com")

	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	require.True(t, nextCalled)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "https://example.com", rec.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORS_NoOriginHeader(t *testing.T) {
	t.Parallel()

	cfg := CORSConfig{
		AllowedOrigins: []string{"https://example.com"},
	}

	nextCalled := false
	handler := CORS(cfg)(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		nextCalled = true
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	require.True(t, nextCalled)
	assert.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORS_PreflightNoMaxAge(t *testing.T) {
	t.Parallel()

	cfg := CORSConfig{
		AllowedOrigins: []string{"https://example.com"},
		AllowedMethods: []string{"GET"},
	}

	handler := CORS(cfg)(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Error("handler should not be called for preflight")
	}))

	req := httptest.NewRequest(http.MethodOptions, "/", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", "GET")

	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Empty(t, rec.Header().Get("Access-Control-Max-Age"))
}

func TestCORS_PlainOptionsPassesThrough(t *testing.T) {
	t.Parallel()

	cfg := CORSConfig{
		AllowedOrigins: []string{"https://example.com"},
		AllowedMethods: []string{"GET", "POST"},
	}

	nextCalled := false

	handler := CORS(cfg)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		nextCalled = true

		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodOptions, "/", nil)
	req.Header.Set("Origin", "https://example.com")
	// No Access-Control-Request-Method header - this is a plain OPTIONS, not a preflight.

	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	require.True(t, nextCalled, "handler should be called for plain OPTIONS request")
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "https://example.com", rec.Header().Get("Access-Control-Allow-Origin"))
	assert.Empty(t, rec.Header().Get("Access-Control-Allow-Methods"))
}
