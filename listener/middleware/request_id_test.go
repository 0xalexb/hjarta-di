package middleware

import (
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateRequestID_Format(t *testing.T) {
	t.Parallel()

	id := generateRequestID()

	assert.Len(t, id, 16, "request ID should be 16 hex characters")

	_, err := hex.DecodeString(id)
	require.NoError(t, err, "request ID should be valid hex")
}

func TestGenerateRequestID_Uniqueness(t *testing.T) {
	t.Parallel()

	seen := make(map[string]struct{})

	for range 100 {
		id := generateRequestID()

		_, exists := seen[id]
		assert.False(t, exists, "duplicate request ID generated: %s", id)

		seen[id] = struct{}{}
	}
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
