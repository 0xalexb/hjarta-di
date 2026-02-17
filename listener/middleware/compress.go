// Package middleware provides HTTP middleware components for the listener package.
package middleware

import (
	"bufio"
	"compress/gzip"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

// minCompressSize is the minimum response size in bytes before compression is applied.
const minCompressSize = 256

// compressedContentTypes contains content types that are already compressed
// and should not be compressed again.
var compressedContentTypes = map[string]bool{ //nolint:gochecknoglobals
	"application/gzip":              true,
	"application/zip":               true,
	"application/x-gzip":            true,
	"application/x-compressed":      true,
	"application/x-bzip2":           true,
	"application/x-xz":              true,
	"application/zstd":              true,
	"image/png":                     true,
	"image/jpeg":                    true,
	"image/gif":                     true,
	"image/webp":                    true,
	"audio/mpeg":                    true,
	"audio/ogg":                     true,
	"video/mp4":                     true,
	"video/webm":                    true,
	"application/octet-stream":      true,
	"application/x-tar":             true,
	"application/x-rar-compressed":  true,
	"application/x-7z-compressed":   true,
	"application/vnd.rar":           true,
	"application/java-archive":      true,
	"application/wasm":              true,
	"font/woff":                     true,
	"font/woff2":                    true,
	"application/font-woff":         true,
	"application/x-font-woff":       true,
	"application/pdf":               true,
	"application/x-shockwave-flash": true,
}

var gzipWriterPool = sync.Pool{ //nolint:gochecknoglobals
	New: func() any {
		return gzip.NewWriter(io.Discard)
	},
}

// gzipResponseWriter wraps http.ResponseWriter to apply gzip compression.
// It buffers data until it can decide whether to compress, then commits
// headers and flushes the buffer.
type gzipResponseWriter struct {
	http.ResponseWriter

	gw         *gzip.Writer
	buf        []byte
	statusCode int
	decided    bool
	skipGzip   bool
	hijacked   bool
	commitErr  error
}

func (w *gzipResponseWriter) WriteHeader(code int) {
	if w.statusCode == 0 {
		w.statusCode = code
	}
}

func (w *gzipResponseWriter) Write(b []byte) (int, error) { //nolint:varnamelen
	if w.commitErr != nil {
		return 0, w.commitErr
	}

	if w.statusCode == 0 {
		w.statusCode = http.StatusOK
	}

	if w.decided {
		if w.skipGzip {
			return w.ResponseWriter.Write(b) //nolint:wrapcheck
		}

		return w.gw.Write(b) //nolint:wrapcheck
	}

	w.buf = append(w.buf, b...)

	if len(w.buf) >= minCompressSize {
		w.commit()

		if w.commitErr != nil {
			return 0, w.commitErr
		}
	}

	return len(b), nil
}

// Flush commits any buffered data, flushes the gzip internal state to the underlying
// writer, and then flushes the underlying writer. This ensures streaming responses
// (e.g. SSE) produce valid gzip output when explicitly flushed.
func (w *gzipResponseWriter) Flush() {
	w.commit()

	if !w.skipGzip {
		_ = w.gw.Flush()
	}

	rc := http.NewResponseController(w.ResponseWriter)
	_ = rc.Flush()
}

// Hijack implements http.Hijacker by delegating to the underlying ResponseWriter
// via http.ResponseController. It marks the connection as hijacked so that
// close() does not attempt to write headers or body on a hijacked connection.
func (w *gzipResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	rc := http.NewResponseController(w.ResponseWriter)

	conn, buf, err := rc.Hijack()
	if err == nil {
		w.hijacked = true
	}

	return conn, buf, err //nolint:wrapcheck
}

// Unwrap returns the underlying ResponseWriter, allowing http.ResponseController
// to access interfaces like http.Flusher and http.Hijacker through the wrapper chain.
func (w *gzipResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func (w *gzipResponseWriter) shouldSkipGzip() bool {
	ct := w.ResponseWriter.Header().Get("Content-Type")
	if ct == "" {
		ct = http.DetectContentType(w.buf)
	}

	baseType, _, _ := strings.Cut(ct, ";")
	baseType = strings.TrimSpace(baseType)

	switch {
	case compressedContentTypes[baseType]:
		return true
	case len(w.buf) < minCompressSize:
		return true
	case w.ResponseWriter.Header().Get("Content-Encoding") != "":
		return true
	case w.statusCode == http.StatusNoContent || w.statusCode == http.StatusNotModified:
		return true
	case w.statusCode == http.StatusPartialContent:
		return true
	case w.statusCode < http.StatusOK:
		return true
	default:
		return false
	}
}

func (w *gzipResponseWriter) commit() {
	if w.decided {
		return
	}

	w.decided = true
	w.skipGzip = w.shouldSkipGzip()

	if w.statusCode == 0 {
		w.statusCode = http.StatusOK
	}

	if !w.skipGzip {
		if w.ResponseWriter.Header().Get("Content-Type") == "" {
			w.ResponseWriter.Header().Set("Content-Type", http.DetectContentType(w.buf))
		}

		w.ResponseWriter.Header().Set("Content-Encoding", "gzip")
		w.ResponseWriter.Header().Del("Content-Length")
	}

	w.ResponseWriter.WriteHeader(w.statusCode)

	if len(w.buf) > 0 {
		if w.skipGzip {
			_, w.commitErr = w.ResponseWriter.Write(w.buf)
		} else {
			_, w.commitErr = w.gw.Write(w.buf)
		}

		w.buf = nil
	}
}

func (w *gzipResponseWriter) close() {
	if w.hijacked {
		return
	}

	w.commit()

	if !w.skipGzip {
		_ = w.gw.Close()
	}
}

// acceptsGzip checks whether the Accept-Encoding header includes gzip
// with a non-zero quality value. It rejects quality values that parse to zero
// (e.g. "gzip;q=0", "gzip;q=0.0", "gzip;q=0.000") which explicitly disable
// gzip per RFC 7231. Encoding tokens are matched case-insensitively per RFC 7231.
func acceptsGzip(header string) bool {
	for part := range strings.SplitSeq(header, ",") {
		part = strings.TrimSpace(part)

		encoding, params, _ := strings.Cut(part, ";")
		encoding = strings.TrimSpace(encoding)

		if !strings.EqualFold(encoding, "gzip") {
			continue
		}

		if params == "" {
			return true
		}

		for param := range strings.SplitSeq(params, ";") {
			param = strings.TrimSpace(param)

			key, val, _ := strings.Cut(param, "=")
			if strings.EqualFold(strings.TrimSpace(key), "q") {
				qval, err := strconv.ParseFloat(strings.TrimSpace(val), 64)
				if err == nil && qval == 0 {
					return false
				}
			}
		}

		return true
	}

	return false
}

// Compress returns a middleware that compresses response bodies using gzip
// when the client supports it (via Accept-Encoding header). It skips compression
// for small responses (under 256 bytes) and already-compressed content types.
func Compress() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { //nolint:varnamelen
			w.Header().Add("Vary", "Accept-Encoding")

			if !acceptsGzip(r.Header.Get("Accept-Encoding")) {
				next.ServeHTTP(w, r)

				return
			}

			gz, ok := gzipWriterPool.Get().(*gzip.Writer) //nolint:varnamelen
			if !ok {
				next.ServeHTTP(w, r)

				return
			}

			gz.Reset(w)

			grw := &gzipResponseWriter{ //nolint:exhaustruct
				ResponseWriter: w,
				gw:             gz,
			}

			panicked := true

			defer func() {
				if panicked {
					// On panic after gzip has committed, close the writer to produce
					// a valid gzip stream end. This also ensures the pooled writer
					// is not returned in a dirty state.
					if grw.decided && !grw.skipGzip && !grw.hijacked {
						_ = gz.Close()
					}
				} else {
					grw.close()
				}

				gz.Reset(io.Discard)
				gzipWriterPool.Put(gz)
			}()

			next.ServeHTTP(grw, r)

			panicked = false
		})
	}
}
