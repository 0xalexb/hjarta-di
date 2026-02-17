package middleware

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMaxRequestSize_SmallBodyPasses(t *testing.T) {
	t.Parallel()

	handler := MaxRequestSize(1024)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		assert.NoError(t, err)

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}))

	rr := httptest.NewRecorder() //nolint:varnamelen // rr is conventional for recorder
	req := httptest.NewRequest(http.MethodPost, "/upload", strings.NewReader("small body"))

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "small body", rr.Body.String())
}

func TestMaxRequestSize_OversizedBodyReturns413(t *testing.T) {
	t.Parallel()

	//nolint:varnamelen // w, r are conventional for http handler params.
	handler := MaxRequestSize(10)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Request Entity Too Large", http.StatusRequestEntityTooLarge)

			return
		}

		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder() //nolint:varnamelen // rr is conventional
	body := strings.Repeat("x", 100)
	req := httptest.NewRequest(http.MethodPost, "/upload", strings.NewReader(body))

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, rr.Code)
}

func TestMaxRequestSize_ExactLimitPasses(t *testing.T) {
	t.Parallel()

	limit := int64(10)
	//nolint:varnamelen // w is conventional for http.ResponseWriter.
	handler := MaxRequestSize(limit)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Request Entity Too Large", http.StatusRequestEntityTooLarge)

			return
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}))

	rr := httptest.NewRecorder() //nolint:varnamelen // rr is conventional for recorder
	req := httptest.NewRequest(http.MethodPost, "/upload", strings.NewReader("0123456789"))

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "0123456789", rr.Body.String())
}

func TestMaxRequestSize_NoBodyPasses(t *testing.T) {
	t.Parallel()

	handler := MaxRequestSize(1024)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))

	rr := httptest.NewRecorder() //nolint:varnamelen // rr is conventional for recorder
	req := httptest.NewRequest(http.MethodGet, "/no-body", nil)

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "ok", rr.Body.String())
}

func TestMaxRequestSize_ZeroBytesUsesDefault(t *testing.T) { //nolint:paralleltest // uses global slog
	var buf strings.Builder

	oldDefault := slog.Default()

	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))

	t.Cleanup(func() { slog.SetDefault(oldDefault) })

	handler := MaxRequestSize(0)(http.HandlerFunc(func(writer http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(writer, "Request Entity Too Large", http.StatusRequestEntityTooLarge)

			return
		}

		writer.WriteHeader(http.StatusOK)
	}))

	// Send body larger than 1MB default to verify it's enforced.
	recorder := httptest.NewRecorder()
	body := strings.Repeat("x", 1048576+1)
	req := httptest.NewRequest(http.MethodPost, "/upload", strings.NewReader(body))

	handler.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, recorder.Code)
	assert.Contains(t, buf.String(), "middleware: bytes must be positive, using default")
}

func TestMaxRequestSize_NegativeBytesUsesDefault(t *testing.T) { //nolint:paralleltest // uses global slog
	var buf strings.Builder

	oldDefault := slog.Default()

	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))

	t.Cleanup(func() { slog.SetDefault(oldDefault) })

	handler := MaxRequestSize(-1)(http.HandlerFunc(func(writer http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(writer, "Request Entity Too Large", http.StatusRequestEntityTooLarge)

			return
		}

		writer.WriteHeader(http.StatusOK)
	}))

	recorder := httptest.NewRecorder()
	// Send body larger than 1MB default to verify it's enforced
	body := strings.Repeat("x", 1048576+1)
	req := httptest.NewRequest(http.MethodPost, "/upload", strings.NewReader(body))

	handler.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, recorder.Code)
	assert.Contains(t, buf.String(), "middleware: bytes must be positive, using default")
}
