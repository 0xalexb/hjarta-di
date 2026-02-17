package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type logRecord struct {
	Level   slog.Level
	Message string
	Attrs   map[string]any
}

type captureHandler struct {
	records []logRecord
}

func (h *captureHandler) Enabled(_ context.Context, _ slog.Level) bool { return true }

//nolint:varnamelen // r is conventional for slog.Record.
func (h *captureHandler) Handle(_ context.Context, r slog.Record) error {
	rec := logRecord{
		Level:   r.Level,
		Message: r.Message,
		Attrs:   make(map[string]any),
	}

	r.Attrs(func(a slog.Attr) bool {
		rec.Attrs[a.Key] = a.Value.Any()

		return true
	})

	h.records = append(h.records, rec)

	return nil
}

func (h *captureHandler) WithAttrs(_ []slog.Attr) slog.Handler { return h }
func (h *captureHandler) WithGroup(_ string) slog.Handler      { return h }

func setupTestLogger(t *testing.T) *captureHandler {
	t.Helper()

	oldDefault := slog.Default()

	h := &captureHandler{}
	slog.SetDefault(slog.New(h))

	t.Cleanup(func() { slog.SetDefault(oldDefault) })

	return h
}

func TestLogging_LogFields(t *testing.T) { //nolint:paralleltest // modifies global slog default
	h := setupTestLogger(t) //nolint:varnamelen // h is conventional for handler

	handler := Logging()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test/path", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	require.Len(t, h.records, 1)

	record := h.records[0]
	assert.Equal(t, "http request", record.Message)
	assert.Equal(t, "GET", record.Attrs["method"])
	assert.Equal(t, "/test/path", record.Attrs["path"])
	assert.Equal(t, int64(http.StatusOK), record.Attrs["status"])

	dur, ok := record.Attrs["duration"].(time.Duration)
	require.True(t, ok)
	assert.GreaterOrEqual(t, dur, time.Duration(0))
}

func TestLogging_InfoLevelForSuccess(t *testing.T) { //nolint:paralleltest // modifies global slog default
	h := setupTestLogger(t) //nolint:varnamelen // h is conventional for handler

	handler := Logging()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	require.Len(t, h.records, 1)
	assert.Equal(t, slog.LevelInfo, h.records[0].Level)
}

func TestLogging_WarnLevelFor4xx(t *testing.T) { //nolint:paralleltest // modifies global slog default
	h := setupTestLogger(t) //nolint:varnamelen // h is conventional for handler

	handler := Logging()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))

	req := httptest.NewRequest(http.MethodGet, "/missing", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	require.Len(t, h.records, 1)
	assert.Equal(t, slog.LevelWarn, h.records[0].Level)
	assert.Equal(t, int64(http.StatusNotFound), h.records[0].Attrs["status"])
}

func TestLogging_ErrorLevelFor5xx(t *testing.T) { //nolint:paralleltest // modifies global slog default
	h := setupTestLogger(t) //nolint:varnamelen // h is conventional for handler

	handler := Logging()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))

	req := httptest.NewRequest(http.MethodGet, "/error", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	require.Len(t, h.records, 1)
	assert.Equal(t, slog.LevelError, h.records[0].Level)
	assert.Equal(t, int64(http.StatusInternalServerError), h.records[0].Attrs["status"])
}

func TestLogging_DurationTracking(t *testing.T) { //nolint:paralleltest // modifies global slog default
	h := setupTestLogger(t) //nolint:varnamelen // h is conventional for handler

	handler := Logging()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(10 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/slow", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	require.Len(t, h.records, 1)

	dur, ok := h.records[0].Attrs["duration"].(time.Duration)
	require.True(t, ok)
	assert.GreaterOrEqual(t, dur, 10*time.Millisecond)
}

func TestLogging_IncludesRequestID(t *testing.T) { //nolint:paralleltest // modifies global slog default
	h := setupTestLogger(t) //nolint:varnamelen // h is conventional for handler

	handler := Logging()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/with-id", nil)
	ctx := context.WithValue(req.Context(), requestIDKey, "test-request-id")
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	require.Len(t, h.records, 1)
	assert.Equal(t, "test-request-id", h.records[0].Attrs["request_id"])
}

func TestLogging_NoRequestID(t *testing.T) { //nolint:paralleltest // modifies global slog default
	h := setupTestLogger(t) //nolint:varnamelen // h is conventional for handler

	handler := Logging()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/no-id", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	require.Len(t, h.records, 1)
	_, hasRequestID := h.records[0].Attrs["request_id"]
	assert.False(t, hasRequestID)
}

func TestLogging_ImplicitOKStatus(t *testing.T) { //nolint:paralleltest // modifies global slog default
	h := setupTestLogger(t) //nolint:varnamelen // h is conventional for handler

	handler := Logging()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("hello"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/implicit", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	require.Len(t, h.records, 1)
	assert.Equal(t, int64(http.StatusOK), h.records[0].Attrs["status"])
	assert.Equal(t, slog.LevelInfo, h.records[0].Level)
}
