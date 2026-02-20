package middleware

import (
	"bufio"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"runtime/debug"
)

// recoveryWriter wraps http.ResponseWriter to track whether headers have been sent.
type recoveryWriter struct {
	http.ResponseWriter

	written bool
}

func (w *recoveryWriter) WriteHeader(code int) {
	if code == http.StatusSwitchingProtocols || code >= http.StatusOK {
		w.written = true
	}

	w.ResponseWriter.WriteHeader(code)
}

func (w *recoveryWriter) Write(b []byte) (int, error) {
	w.written = true

	return w.ResponseWriter.Write(b) //nolint:wrapcheck
}

// Flush implements http.Flusher by using http.ResponseController to traverse
// the full wrapper chain. This ensures flushing works even when intermediate
// wrappers (e.g. statusWriter, gzipResponseWriter) only expose Unwrap.
func (w *recoveryWriter) Flush() {
	rc := http.NewResponseController(w.ResponseWriter)

	err := rc.Flush()
	if err == nil {
		w.written = true
	}
}

// Hijack implements http.Hijacker by using http.ResponseController to traverse
// the full wrapper chain. This ensures hijacking works even when intermediate
// wrappers only expose Unwrap.
func (w *recoveryWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	rc := http.NewResponseController(w.ResponseWriter)

	conn, buf, err := rc.Hijack()
	if err == nil {
		w.written = true
	}

	return conn, buf, err //nolint:wrapcheck
}

// Unwrap returns the underlying ResponseWriter, allowing http.ResponseController
// to access interfaces like http.Flusher and http.Hijacker through the wrapper chain.
func (w *recoveryWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

// Recovery returns a middleware that recovers from panics in downstream handlers.
// It logs the panic value and stack trace via global slog.Error and responds
// with 500 Internal Server Error. If a request ID is available in the context,
// it is included in the log entry. If the response has already been partially
// written, it logs an error instead of attempting to write a 500 status.
func Recovery() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			recWriter := &recoveryWriter{ResponseWriter: w}

			defer func() { //nolint:contextcheck
				if rec := recover(); rec != nil {
					err, ok := rec.(error)
					if ok && err == http.ErrAbortHandler { //nolint:errorlint,err113
						panic(rec)
					}

					stack := debug.Stack()

					attrs := []any{
						slog.String("panic", fmt.Sprintf("%v", rec)),
						slog.String("stack", string(stack)),
						slog.String("method", r.Method),
						slog.String("path", r.URL.Path),
					}

					if reqID := GetRequestID(r.Context()); reqID != "" {
						attrs = append(attrs, slog.String("request_id", reqID))
					}

					if recWriter.written {
						attrs = append(attrs, slog.Bool("response_already_written", true))
						slog.Error("panic recovered after response was already written", attrs...) //nolint:gosec

						return
					}

					slog.Error("panic recovered", attrs...) //nolint:gosec // G706: message is a hardcoded constant.

					http.Error(recWriter, "Internal Server Error", http.StatusInternalServerError)
				}
			}()

			next.ServeHTTP(recWriter, r)
		})
	}
}
