package middleware

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompress_GzipResponseWhenAccepted(t *testing.T) {
	t.Parallel()

	body := strings.Repeat("Hello, World! This is a compressible response body. ", 20)

	handler := Compress()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "gzip", rr.Header().Get("Content-Encoding"))
	assert.Empty(t, rr.Header().Get("Content-Length"))

	gr, err := gzip.NewReader(rr.Body)
	require.NoError(t, err)

	defer func() { _ = gr.Close() }()

	decompressed, err := io.ReadAll(gr)
	require.NoError(t, err)
	assert.Equal(t, body, string(decompressed))
}

func TestCompress_NoCompressionWhenNotAccepted(t *testing.T) {
	t.Parallel()

	body := strings.Repeat("Hello, World! ", 50)

	handler := Compress()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Empty(t, rr.Header().Get("Content-Encoding"))
	assert.Equal(t, body, rr.Body.String())
}

func TestCompress_ProperHeaders(t *testing.T) {
	t.Parallel()

	body := strings.Repeat("data", 100)

	handler := Compress()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/data", nil)
	req.Header.Set("Accept-Encoding", "gzip, deflate")

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "gzip", rr.Header().Get("Content-Encoding"))
	assert.Empty(t, rr.Header().Get("Content-Length"))
}

func TestCompress_SkipSmallResponses(t *testing.T) {
	t.Parallel()

	body := "tiny"

	handler := Compress()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Empty(t, rr.Header().Get("Content-Encoding"))
	assert.Equal(t, body, rr.Body.String())
}

func TestCompress_SkipAlreadyCompressedContentTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		contentType string
	}{
		{"gzip", "application/gzip"},
		{"png", "image/png"},
		{"jpeg", "image/jpeg"},
		{"zip", "application/zip"},
		{"woff2", "font/woff2"},
		{"pdf", "application/pdf"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			body := strings.Repeat("x", 1024)

			handler := Compress()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", tt.contentType)
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(body))
			}))

			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("Accept-Encoding", "gzip")

			handler.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code)
			assert.Empty(t, rr.Header().Get("Content-Encoding"), "should not compress %s", tt.contentType)
			assert.Equal(t, body, rr.Body.String())
		})
	}
}

func TestCompress_ContentTypeWithCharset(t *testing.T) {
	t.Parallel()

	body := strings.Repeat("Hello, World! ", 50)

	handler := Compress()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "gzip", rr.Header().Get("Content-Encoding"))

	gr, err := gzip.NewReader(rr.Body)
	require.NoError(t, err)

	defer func() { _ = gr.Close() }()

	decompressed, err := io.ReadAll(gr)
	require.NoError(t, err)
	assert.Equal(t, body, string(decompressed))
}

func TestCompress_ImplicitWriteHeader(t *testing.T) {
	t.Parallel()

	body := strings.Repeat("implicit header test ", 20)

	handler := Compress()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte(body))
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "gzip", rr.Header().Get("Content-Encoding"))

	gr, err := gzip.NewReader(rr.Body)
	require.NoError(t, err)

	defer func() { _ = gr.Close() }()

	decompressed, err := io.ReadAll(gr)
	require.NoError(t, err)
	assert.Equal(t, body, string(decompressed))
}

func TestAcceptsGzip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		header string
		want   bool
	}{
		{"plain gzip", "gzip", true},
		{"gzip with deflate", "gzip, deflate", true},
		{"gzip with quality", "gzip;q=1.0", true},
		{"gzip q=0 rejected", "gzip;q=0", false},
		{"gzip q=0 with spaces", "gzip; q=0", false},
		{"only deflate", "deflate", false},
		{"empty header", "", false},
		{"br and gzip", "br, gzip", true},
		{"gzip q=0.5", "gzip;q=0.5", true},
		{"gzip q=0.0 rejected", "gzip;q=0.0", false},
		{"gzip q=0.000 rejected", "gzip;q=0.000", false},
		{"uppercase GZIP", "GZIP", true},
		{"mixed case Gzip", "Gzip", true},
		{"uppercase GZIP q=0 rejected", "GZIP;q=0", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, acceptsGzip(tt.header))
		})
	}
}

func TestCompress_GzipQZeroNotCompressed(t *testing.T) {
	t.Parallel()

	body := strings.Repeat("Hello, World! ", 50)

	handler := Compress()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip;q=0, deflate")

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Empty(t, rr.Header().Get("Content-Encoding"))
	assert.Equal(t, body, rr.Body.String())
}

func TestCompress_MultipleWriteCalls(t *testing.T) {
	t.Parallel()

	chunk := strings.Repeat("chunk data ", 10)

	handler := Compress()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)

		for range 5 {
			_, _ = w.Write([]byte(chunk))
		}
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "gzip", rr.Header().Get("Content-Encoding"))

	gr, err := gzip.NewReader(rr.Body)
	require.NoError(t, err)

	defer func() { _ = gr.Close() }()

	decompressed, err := io.ReadAll(gr)
	require.NoError(t, err)
	assert.Equal(t, strings.Repeat(chunk, 5), string(decompressed))
}

func TestCompress_SkipExistingContentEncoding(t *testing.T) {
	t.Parallel()

	body := strings.Repeat("already encoded content ", 50)

	handler := Compress()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("Content-Encoding", "br")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "br", rr.Header().Get("Content-Encoding"))
	assert.Equal(t, body, rr.Body.String())
}

func TestCompress_SkipNoContentStatus(t *testing.T) {
	t.Parallel()

	handler := Compress()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNoContent, rr.Code)
	assert.Empty(t, rr.Header().Get("Content-Encoding"))
}

func TestCompress_SkipNotModifiedStatus(t *testing.T) {
	t.Parallel()

	handler := Compress()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotModified)
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNotModified, rr.Code)
	assert.Empty(t, rr.Header().Get("Content-Encoding"))
}

func TestCompress_SkipPartialContentStatus(t *testing.T) {
	t.Parallel()

	body := strings.Repeat("partial content range data ", 50)

	handler := Compress()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("Content-Range", "bytes 0-99/200")
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write([]byte(body))
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusPartialContent, rr.Code)
	assert.Empty(t, rr.Header().Get("Content-Encoding"), "should not compress 206 Partial Content")
	assert.Equal(t, body, rr.Body.String())
}

func TestCompress_VaryHeaderPresent(t *testing.T) {
	t.Parallel()

	body := "small"

	handler := Compress()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	handler.ServeHTTP(rr, req)

	assert.Contains(t, rr.Header().Get("Vary"), "Accept-Encoding")
}
