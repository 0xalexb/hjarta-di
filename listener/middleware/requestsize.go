package middleware

import (
	"log/slog"
	"net/http"
)

const defaultMaxRequestSizeBytes int64 = 1048576 // 1MB

// MaxRequestSize returns a middleware that limits the size of incoming request
// bodies using http.MaxBytesReader. Handlers that read the body will receive an
// error when the limit is exceeded and should respond with 413 Request Entity
// Too Large.
//
// If bytes is zero or negative, it defaults to 1MB (1048576 bytes) and logs a
// warning via slog.
func MaxRequestSize(bytes int64) func(http.Handler) http.Handler {
	if bytes <= 0 {
		slog.Warn("middleware: bytes must be positive, using default",
			"provided", bytes, "default", defaultMaxRequestSizeBytes)

		bytes = defaultMaxRequestSizeBytes
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, bytes)
			next.ServeHTTP(w, r)
		})
	}
}
