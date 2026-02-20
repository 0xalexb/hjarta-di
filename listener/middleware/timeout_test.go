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

func TestTimeout_RequestWithinTimeout(t *testing.T) { //nolint:paralleltest // timing-sensitive test
	handler := Timeout(500 * time.Millisecond)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/fast", nil)

	handler.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "ok", recorder.Body.String())
}

func TestTimeout_RequestExceedsTimeout(t *testing.T) { //nolint:paralleltest // timing-sensitive test
	handler := Timeout(50 * time.Millisecond)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(5 * time.Second):
			w.WriteHeader(http.StatusOK)
		case <-r.Context().Done():
			return
		}
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/slow", nil)

	handler.ServeHTTP(rr, req)

	require.Equal(t, http.StatusServiceUnavailable, rr.Code)
	assert.Contains(t, rr.Body.String(), "Service Unavailable")
}

func TestTimeout_ContextCancelledOnTimeout(t *testing.T) { //nolint:paralleltest // timing-sensitive test
	ctxDone := make(chan struct{})

	handler := Timeout(50 * time.Millisecond)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
		close(ctxDone)
	}))

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ctx", nil)

	handler.ServeHTTP(recorder, req)

	select {
	case <-ctxDone:
		// context was cancelled as expected
	case <-time.After(time.Second):
		t.Fatal("expected context to be cancelled on timeout")
	}

	assert.Equal(t, http.StatusServiceUnavailable, recorder.Code)
}

func TestTimeout_DefaultsOnZeroDuration(t *testing.T) { //nolint:paralleltest // timing-sensitive test
	var buf bytes.Buffer

	oldDefault := slog.Default()

	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))

	t.Cleanup(func() { slog.SetDefault(oldDefault) })

	handler := Timeout(0)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, buf.String(), "middleware: duration must be positive, using default")
}

func TestTimeout_DefaultsOnNegativeDuration(t *testing.T) { //nolint:paralleltest // timing-sensitive test
	var buf bytes.Buffer

	oldDefault := slog.Default()

	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))

	t.Cleanup(func() { slog.SetDefault(oldDefault) })

	handler := Timeout(-time.Second)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, buf.String(), "middleware: duration must be positive, using default")
}
