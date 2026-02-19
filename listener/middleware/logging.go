package middleware

import (
	"bufio"
	"log/slog"
	"net"
	"net/http"
	"time"
)

// statusWriter wraps http.ResponseWriter to capture the status code.
type statusWriter struct {
	http.ResponseWriter

	status   int
	written  bool
	hijacked bool
}

func (w *statusWriter) WriteHeader(code int) {
	if !w.written {
		w.status = code
		w.written = true

		w.ResponseWriter.WriteHeader(code)
	}
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if !w.written {
		w.status = http.StatusOK
		w.written = true
	}

	return w.ResponseWriter.Write(b) //nolint:wrapcheck
}

// Hijack implements http.Hijacker by delegating to the underlying ResponseWriter
// via http.ResponseController. This allows WebSocket upgrades and other connection
// hijacking to work through the logging middleware, including code that performs
// direct w.(http.Hijacker) type assertions.
func (w *statusWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	rc := http.NewResponseController(w.ResponseWriter)

	conn, buf, err := rc.Hijack()
	if err == nil {
		w.hijacked = true
	}

	return conn, buf, err //nolint:wrapcheck
}

// Flush delegates to the underlying ResponseWriter via http.ResponseController,
// allowing streaming responses to work through the logging middleware.
func (w *statusWriter) Flush() {
	rc := http.NewResponseController(w.ResponseWriter)
	err := rc.Flush()

	if err == nil && !w.written {
		w.status = http.StatusOK
		w.written = true
	}
}

// Unwrap returns the underlying ResponseWriter, allowing http.ResponseController
// to access interfaces like http.Flusher and http.Hijacker through the wrapper chain.
func (w *statusWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

// Logging returns a middleware that logs request/response details via global slog.
// It logs method, path, status code, duration, and request ID (if available).
// Log level is Info for 2xx/3xx, Warn for 4xx, Error for 5xx.
func Logging() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			sw := &statusWriter{ResponseWriter: w}

			next.ServeHTTP(sw, r)

			if sw.status == 0 {
				if sw.hijacked {
					sw.status = http.StatusSwitchingProtocols
				} else {
					sw.status = http.StatusOK
				}
			}

			duration := time.Since(start)

			attrs := []any{
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", sw.status),
				slog.Duration("duration", duration),
			}

			if reqID := GetRequestID(r.Context()); reqID != "" {
				attrs = append(attrs, slog.String("request_id", reqID))
			}

			msg := "http request"

			switch {
			case sw.status >= http.StatusInternalServerError:
				slog.Error(msg, attrs...) //nolint:gosec // G706: msg is a hardcoded constant, not user input.
			case sw.status >= http.StatusBadRequest:
				slog.Warn(msg, attrs...) //nolint:gosec // G706: msg is a hardcoded constant, not user input.
			default:
				slog.Info(msg, attrs...) //nolint:gosec // G706: msg is a hardcoded constant, not user input.
			}
		})
	}
}
