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

	handler := CORS(
		WithAllowedOrigins("example.com"),
		WithAllowedMethods("GET", "POST"),
		WithAllowedHeaders("Content-Type", "Authorization"),
		WithMaxAge(3600),
	)(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
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

	nextCalled := false
	handler := CORS(
		WithAllowedOrigins("example.com", "other.com"),
		WithAllowedMethods("GET"),
	)(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
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

	nextCalled := false
	handler := CORS(
		WithAllowedOrigins("example.com"),
	)(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		nextCalled = true
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://evil.com")

	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	require.True(t, nextCalled)
	assert.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "Origin", rec.Header().Get("Vary"))
}

func TestCORS_WildcardOrigin(t *testing.T) {
	t.Parallel()

	handler := CORS(
		WithAllowedOrigins("*"),
		WithAllowedMethods("GET", "POST"),
	)(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))

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

	handler := CORS(
		WithAllowedOrigins("*"),
		WithAllowCredentials(),
	)(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))

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

	handler := CORS(
		WithAllowedOrigins("*", "trusted.com"),
		WithAllowCredentials(),
	)(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://trusted.com")

	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Explicitly listed origin should still work with credentials.
	assert.Equal(t, "https://trusted.com", rec.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "true", rec.Header().Get("Access-Control-Allow-Credentials"))

	// Untrusted origin should be rejected since wildcard is disabled with credentials.
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.Header.Set("Origin", "https://untrusted.com")

	rec2 := httptest.NewRecorder()

	handler.ServeHTTP(rec2, req2)

	assert.Empty(t, rec2.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORS_CredentialsFlag(t *testing.T) {
	t.Parallel()

	handler := CORS(
		WithAllowedOrigins("example.com"),
		WithAllowCredentials(),
	)(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://example.com")

	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, "true", rec.Header().Get("Access-Control-Allow-Credentials"))
	assert.Equal(t, "https://example.com", rec.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORS_NormalRequestPassThrough(t *testing.T) {
	t.Parallel()

	nextCalled := false
	handler := CORS(
		WithAllowedOrigins("example.com"),
	)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
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

	nextCalled := false
	handler := CORS(
		WithAllowedOrigins("example.com"),
	)(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		nextCalled = true
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	require.True(t, nextCalled)
	assert.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "Origin", rec.Header().Get("Vary"))
}

func TestCORS_PreflightNoMaxAge(t *testing.T) {
	t.Parallel()

	handler := CORS(
		WithAllowedOrigins("example.com"),
		WithAllowedMethods("GET"),
		WithMaxAge(0),
	)(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
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

	nextCalled := false

	handler := CORS(
		WithAllowedOrigins("example.com"),
		WithAllowedMethods("GET", "POST"),
	)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
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

func TestCORS_ExposedHeaders(t *testing.T) {
	t.Parallel()

	handler := CORS(
		WithAllowedOrigins("example.com"),
		WithExposedHeaders("X-Request-ID", "X-Total-Count"),
	)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://example.com")

	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, "X-Request-ID, X-Total-Count", rec.Header().Get("Access-Control-Expose-Headers"))
}

func TestCORS_ExposedHeadersOnPreflight(t *testing.T) {
	t.Parallel()

	handler := CORS(
		WithAllowedOrigins("example.com"),
		WithAllowedMethods("GET"),
		WithExposedHeaders("X-Request-ID"),
	)(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Error("handler should not be called for preflight")
	}))

	req := httptest.NewRequest(http.MethodOptions, "/", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", "GET")

	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, "X-Request-ID", rec.Header().Get("Access-Control-Expose-Headers"))
}

func TestCORS_NoExposedHeaders(t *testing.T) {
	t.Parallel()

	handler := CORS(
		WithAllowedOrigins("example.com"),
	)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://example.com")

	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Empty(t, rec.Header().Get("Access-Control-Expose-Headers"))
}

func TestCORS_HostnameMatchesDifferentSchemes(t *testing.T) {
	t.Parallel()

	handler := CORS(
		WithAllowedOrigins("example.com"),
	)(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))

	for _, origin := range []string{"http://example.com", "https://example.com"} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Origin", origin)

		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		assert.Equal(t, origin, rec.Header().Get("Access-Control-Allow-Origin"),
			"origin %s should match hostname example.com", origin)
	}
}

func TestCORS_HostnameMatchesDifferentPorts(t *testing.T) {
	t.Parallel()

	handler := CORS(
		WithAllowedOrigins("localhost"),
	)(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))

	for _, origin := range []string{"http://localhost:3000", "http://localhost:8080"} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Origin", origin)

		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		assert.Equal(t, origin, rec.Header().Get("Access-Control-Allow-Origin"),
			"origin %s should match hostname localhost", origin)
	}
}

func TestCORS_HostnameIPv6(t *testing.T) {
	t.Parallel()

	handler := CORS(
		WithAllowedOrigins("::1"),
	)(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))

	origin := "http://[::1]:9090"
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", origin)

	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, origin, rec.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORS_HostnameNoMatchDifferentHost(t *testing.T) {
	t.Parallel()

	nextCalled := false
	handler := CORS(
		WithAllowedOrigins("example.com"),
	)(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		nextCalled = true
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://evil.example.com")

	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	require.True(t, nextCalled)
	assert.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORS_MalformedOriginRejected(t *testing.T) {
	t.Parallel()

	nextCalled := false
	handler := CORS(
		WithAllowedOrigins("example.com"),
	)(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		nextCalled = true
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "://not-a-url")

	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	require.True(t, nextCalled)
	assert.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORS_ValidateOriginsRejectsScheme(t *testing.T) { //nolint:paralleltest // modifies global slog default
	var buf bytes.Buffer

	oldDefault := slog.Default()

	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))

	t.Cleanup(func() { slog.SetDefault(oldDefault) })

	handler := CORS(
		WithAllowedOrigins("https://example.com", "good.com"),
		WithOriginValidators(ValidateHostname()...),
	)(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))

	// The entry with scheme should be skipped, so https://example.com won't match.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://example.com")

	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Contains(t, buf.String(), "origin contains scheme")
	assert.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))

	// The valid entry should still work.
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.Header.Set("Origin", "https://good.com")

	rec2 := httptest.NewRecorder()

	handler.ServeHTTP(rec2, req2)

	assert.Equal(t, "https://good.com", rec2.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORS_ValidateOriginsRejectsPort(t *testing.T) { //nolint:paralleltest // modifies global slog default
	var buf bytes.Buffer

	oldDefault := slog.Default()

	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))

	t.Cleanup(func() { slog.SetDefault(oldDefault) })

	handler := CORS(
		WithAllowedOrigins("localhost:8080", "good.com"),
		WithOriginValidators(ValidateHostname()...),
	)(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "http://localhost:8080")

	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Contains(t, buf.String(), "origin contains port")
	assert.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORS_ValidateOriginsRejectsIPv6Port(t *testing.T) { //nolint:paralleltest // modifies global slog default
	testValidateOriginsRejects(t, "[::1]:8080", "origin contains port")
}

func TestCORS_ValidateOriginsRejectsWildcard(t *testing.T) { //nolint:paralleltest // modifies global slog default
	var buf bytes.Buffer

	oldDefault := slog.Default()

	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))

	t.Cleanup(func() { slog.SetDefault(oldDefault) })

	handler := CORS(
		WithAllowedOrigins("*", "good.com"),
		WithOriginValidators(ValidateNoWildcard()),
	)(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))

	// ValidateNoWildcard should reject "*" and log an error.
	assert.Contains(t, buf.String(), "origin is wildcard")

	// Wildcard should NOT work since "*" was rejected by the validator.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://any.com")

	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))

	// But good.com should still work.
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.Header.Set("Origin", "https://good.com")

	rec2 := httptest.NewRecorder()

	handler.ServeHTTP(rec2, req2)

	assert.Equal(t, "https://good.com", rec2.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORS_ValidateOriginsPassesValid(t *testing.T) { //nolint:paralleltest // modifies global slog default
	var buf bytes.Buffer

	oldDefault := slog.Default()

	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))

	t.Cleanup(func() { slog.SetDefault(oldDefault) })

	handler := CORS(
		WithAllowedOrigins("localhost", "127.0.0.1", "::1"),
		WithOriginValidators(ValidateHostname()...),
	)(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))

	// No errors should be logged for valid hostnames.
	assert.Empty(t, buf.String())

	for _, origin := range []string{"http://localhost", "http://127.0.0.1", "http://[::1]:9090"} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Origin", origin)

		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		assert.Equal(t, origin, rec.Header().Get("Access-Control-Allow-Origin"),
			"origin %s should match a valid hostname", origin)
	}
}

func TestCORS_NoValidation(t *testing.T) { //nolint:paralleltest // modifies global slog default
	var buf bytes.Buffer

	oldDefault := slog.Default()

	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))

	t.Cleanup(func() { slog.SetDefault(oldDefault) })

	handler := CORS(
		WithAllowedOrigins("https://example.com", "localhost:8080"),
		// No WithOriginValidators - no validation.
	)(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))

	// Without validators, entries with schemes/ports are added as-is (no error logs).
	assert.Empty(t, buf.String())

	// These entries won't match via hostname extraction because the map keys
	// are full strings, not bare hostnames extracted from Origin headers.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://example.com")

	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"),
		"https://example.com should not match map key 'https://example.com' via hostname extraction")

	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.Header.Set("Origin", "http://localhost:8080")

	rec2 := httptest.NewRecorder()

	handler.ServeHTTP(rec2, req2)

	assert.Empty(t, rec2.Header().Get("Access-Control-Allow-Origin"),
		"http://localhost:8080 should not match map key 'localhost:8080' via hostname extraction")
}

func testValidateOriginsRejects(t *testing.T, invalidOrigin, expectedLog string) {
	t.Helper()

	var buf bytes.Buffer

	oldDefault := slog.Default()

	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))

	t.Cleanup(func() { slog.SetDefault(oldDefault) })

	handler := CORS(
		WithAllowedOrigins(invalidOrigin, "good.com"),
		WithOriginValidators(ValidateHostname()...),
	)(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))

	assert.Contains(t, buf.String(), expectedLog)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://good.com")

	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, "https://good.com", rec.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORS_ValidateOriginsRejectsPath(t *testing.T) { //nolint:paralleltest // modifies global slog default
	testValidateOriginsRejects(t, "example.com/path", "origin contains path")
}

func TestCORS_ValidateOriginsRejectsEmpty(t *testing.T) { //nolint:paralleltest // modifies global slog default
	testValidateOriginsRejects(t, "", "origin is empty")
}

func TestCORS_EmptyStringOriginIgnored(t *testing.T) {
	t.Parallel()

	// Empty string in AllowedOrigins (e.g., from trailing comma in config parsing)
	// should be silently skipped, not stored in the map where it could match
	// Origin: null or other malformed origins that extract to empty hostname.
	handler := CORS(
		WithAllowedOrigins("", "example.com"),
	)(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))

	// Origin: null should NOT match, even though extractHostname("null") returns "".
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "null")

	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"),
		"Origin: null should not match empty string in AllowedOrigins")

	// Valid origin should still work.
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.Header.Set("Origin", "https://example.com")

	rec2 := httptest.NewRecorder()

	handler.ServeHTTP(rec2, req2)

	assert.Equal(t, "https://example.com", rec2.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORS_NilValidatorSkipped(t *testing.T) {
	t.Parallel()

	// A nil validator in the slice should be skipped without panicking.
	handler := CORS(
		WithAllowedOrigins("example.com"),
		WithOriginValidators(nil, ValidateNotEmpty()),
	)(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://example.com")

	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, "https://example.com", rec.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORS_CaseInsensitiveHostname(t *testing.T) {
	t.Parallel()

	handler := CORS(
		WithAllowedOrigins("Example.COM"),
	)(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://example.com")

	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, "https://example.com", rec.Header().Get("Access-Control-Allow-Origin"),
		"hostname matching should be case-insensitive")
}

func TestCORS_Defaults(t *testing.T) {
	t.Parallel()

	handler := CORS()(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://any-origin.com")

	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Zero-option CORS() should use wildcard matching.
	assert.Equal(t, "*", rec.Header().Get("Access-Control-Allow-Origin"))

	// No credentials by default.
	assert.Empty(t, rec.Header().Get("Access-Control-Allow-Credentials"))

	// No exposed headers by default.
	assert.Empty(t, rec.Header().Get("Access-Control-Expose-Headers"))
}

func TestCORS_DefaultMethodsInPreflight(t *testing.T) {
	t.Parallel()

	handler := CORS()(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Error("handler should not be called for preflight")
	}))

	req := httptest.NewRequest(http.MethodOptions, "/", nil)
	req.Header.Set("Origin", "https://any-origin.com")
	req.Header.Set("Access-Control-Request-Method", "POST")

	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, "*", rec.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "GET, HEAD, POST", rec.Header().Get("Access-Control-Allow-Methods"))
	assert.Equal(t, "Origin, Accept, Content-Type, X-Requested-With", rec.Header().Get("Access-Control-Allow-Headers"))
	assert.Equal(t, "3600", rec.Header().Get("Access-Control-Max-Age"))
}

func TestCORS_OverrideDefaults(t *testing.T) {
	t.Parallel()

	handler := CORS(
		WithAllowedMethods("PUT", "DELETE"),
		WithAllowedHeaders("Authorization"),
		WithMaxAge(600),
	)(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Error("handler should not be called for preflight")
	}))

	req := httptest.NewRequest(http.MethodOptions, "/", nil)
	req.Header.Set("Origin", "https://any-origin.com")
	req.Header.Set("Access-Control-Request-Method", "PUT")

	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)

	// Options should replace defaults, not append.
	assert.Equal(t, "PUT, DELETE", rec.Header().Get("Access-Control-Allow-Methods"))
	assert.Equal(t, "Authorization", rec.Header().Get("Access-Control-Allow-Headers"))
	assert.Equal(t, "600", rec.Header().Get("Access-Control-Max-Age"))

	// Default wildcard origin should still be active since we didn't override origins.
	assert.Equal(t, "*", rec.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORS_EmptyAllowedOrigins(t *testing.T) {
	t.Parallel()

	// Explicitly setting empty origins should block all cross-origin requests.
	nextCalled := false
	handler := CORS(
		WithAllowedOrigins(),
	)(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		nextCalled = true
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://example.com")

	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	require.True(t, nextCalled)
	assert.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))
}
