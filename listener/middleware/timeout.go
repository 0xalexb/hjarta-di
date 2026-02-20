package middleware

import (
	"log/slog"
	"net/http"
	"time"
)

const defaultTimeoutDuration = 30 * time.Second

// Timeout returns a middleware that enforces a request processing deadline.
// If the handler does not complete within the given duration, a 503 Service
// Unavailable response is sent to the client.
// If duration is not positive, it defaults to 30s with a warning log.
func Timeout(duration time.Duration) func(http.Handler) http.Handler {
	if duration <= 0 {
		slog.Warn("middleware: duration must be positive, using default",
			"provided", duration, "default", defaultTimeoutDuration)

		duration = defaultTimeoutDuration
	}

	return func(next http.Handler) http.Handler {
		return http.TimeoutHandler(next, duration, "Service Unavailable")
	}
}
