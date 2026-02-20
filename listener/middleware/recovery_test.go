package middleware

import (
	"bufio"
	"bytes"
	"context"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRecovery_PanicReturns500(t *testing.T) { //nolint:paralleltest // modifies global slog default
	handler := Recovery()(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("something went wrong")
	}))

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestRecovery_LogsPanicAndStack(t *testing.T) { //nolint:paralleltest // modifies global slog default
	var buf bytes.Buffer

	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelError}))
	original := slog.Default()

	slog.SetDefault(logger)

	defer slog.SetDefault(original)

	handler := Recovery()(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("test panic value")
	}))

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	logOutput := buf.String()
	assert.Contains(t, logOutput, "panic recovered")
	assert.Contains(t, logOutput, "test panic value")
	assert.Contains(t, logOutput, "goroutine")
	assert.Contains(t, logOutput, "/panic")
	assert.Contains(t, logOutput, http.MethodGet)
}

func TestRecovery_IncludesRequestIDInLog(t *testing.T) { //nolint:paralleltest // modifies global slog default
	var buf bytes.Buffer

	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelError}))
	original := slog.Default()

	slog.SetDefault(logger)

	defer slog.SetDefault(original)

	handler := Recovery()(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("with request id")
	}))

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	ctx := context.WithValue(req.Context(), requestIDKey, "test-request-id-123")
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	logOutput := buf.String()
	assert.Contains(t, logOutput, "request_id")
	assert.Contains(t, logOutput, "test-request-id-123")
}

func TestRecovery_NoRequestIDOmitsField(t *testing.T) { //nolint:paralleltest // modifies global slog default
	var buf bytes.Buffer

	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelError}))
	original := slog.Default()

	slog.SetDefault(logger)

	defer slog.SetDefault(original)

	handler := Recovery()(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("no request id")
	}))

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	logOutput := buf.String()
	assert.Contains(t, logOutput, "panic recovered")
	assert.NotContains(t, logOutput, "request_id")
}

func TestRecovery_ErrAbortHandlerRePanics(t *testing.T) { //nolint:paralleltest // modifies global slog default
	handler := Recovery()(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic(http.ErrAbortHandler)
	}))

	req := httptest.NewRequest(http.MethodGet, "/abort", nil)
	rec := httptest.NewRecorder()

	assert.PanicsWithValue(t, http.ErrAbortHandler, func() {
		handler.ServeHTTP(rec, req)
	})
}

func TestRecovery_PanicAfterPartialWrite(t *testing.T) { //nolint:paralleltest // modifies global slog default
	var buf bytes.Buffer

	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelError}))
	original := slog.Default()

	slog.SetDefault(logger)

	defer slog.SetDefault(original)

	handler := Recovery()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("partial response"))

		panic("panic after write")
	}))

	req := httptest.NewRequest(http.MethodGet, "/partial", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	logOutput := buf.String()
	assert.Contains(t, logOutput, "response_already_written")
	assert.Contains(t, logOutput, "panic after write")
	assert.Equal(t, http.StatusOK, rec.Code, "status should remain 200, not overwritten to 500")
}

type flusherRecorder struct {
	*httptest.ResponseRecorder

	flushed bool
}

func (f *flusherRecorder) Flush() {
	f.flushed = true
	f.ResponseRecorder.Flush()
}

type hijackerRecorder struct {
	*httptest.ResponseRecorder

	hijacked bool
}

func (h *hijackerRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h.hijacked = true

	return nil, nil, nil
}

func TestRecovery_FlusherPassthrough(t *testing.T) { //nolint:paralleltest // modifies global slog default
	rec := &flusherRecorder{ResponseRecorder: httptest.NewRecorder()}

	handler := Recovery()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		f, ok := w.(http.Flusher)
		assert.True(t, ok, "recoveryWriter should implement http.Flusher")

		f.Flush()
	}))

	req := httptest.NewRequest(http.MethodGet, "/flush", nil)
	handler.ServeHTTP(rec, req)

	assert.True(t, rec.flushed, "Flush should delegate to underlying writer")
}

func TestRecovery_HijackerPassthrough(t *testing.T) { //nolint:paralleltest // modifies global slog default
	rec := &hijackerRecorder{ResponseRecorder: httptest.NewRecorder()}

	handler := Recovery()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		h, ok := w.(http.Hijacker)
		assert.True(t, ok, "recoveryWriter should implement http.Hijacker")

		_, _, _ = h.Hijack()
	}))

	req := httptest.NewRequest(http.MethodGet, "/hijack", nil)
	handler.ServeHTTP(rec, req)

	assert.True(t, rec.hijacked, "Hijack should delegate to underlying writer")
}

func TestRecovery_HijackReturnsErrWhenNotSupported(t *testing.T) { //nolint:paralleltest // modifies global slog default
	handler := Recovery()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		h, ok := w.(http.Hijacker)
		assert.True(t, ok, "recoveryWriter should implement http.Hijacker")

		_, _, err := h.Hijack()
		assert.ErrorIs(t, err, http.ErrNotSupported)
	}))

	req := httptest.NewRequest(http.MethodGet, "/hijack-no-support", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
}

func TestRecovery_PanicAfterFlush(t *testing.T) { //nolint:paralleltest // modifies global slog default
	var buf bytes.Buffer

	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelError}))
	original := slog.Default()

	slog.SetDefault(logger)

	defer slog.SetDefault(original)

	rec := &flusherRecorder{ResponseRecorder: httptest.NewRecorder()}

	handler := Recovery()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		f, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("recoveryWriter should implement http.Flusher")
		}

		f.Flush()

		panic("panic after flush")
	}))

	req := httptest.NewRequest(http.MethodGet, "/flush-panic", nil)
	handler.ServeHTTP(rec, req)

	logOutput := buf.String()
	assert.Contains(t, logOutput, "response_already_written")
	assert.Contains(t, logOutput, "panic after flush")
}

func TestRecovery_PanicAfterHijack(t *testing.T) { //nolint:paralleltest // modifies global slog default
	var buf bytes.Buffer

	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelError}))
	original := slog.Default()

	slog.SetDefault(logger)

	defer slog.SetDefault(original)

	rec := &hijackerRecorder{ResponseRecorder: httptest.NewRecorder()}

	handler := Recovery()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		h, ok := w.(http.Hijacker)
		if !ok {
			t.Fatal("recoveryWriter should implement http.Hijacker")
		}

		_, _, _ = h.Hijack()

		panic("panic after hijack")
	}))

	req := httptest.NewRequest(http.MethodGet, "/hijack-panic", nil)
	handler.ServeHTTP(rec, req)

	logOutput := buf.String()
	assert.Contains(t, logOutput, "response_already_written")
	assert.Contains(t, logOutput, "panic after hijack")
}

func TestRecovery_NoPanicPassesThrough(t *testing.T) { //nolint:paralleltest // modifies global slog default
	called := false

	handler := Recovery()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.True(t, called, "downstream handler should be called")
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "ok", rec.Body.String())
}
