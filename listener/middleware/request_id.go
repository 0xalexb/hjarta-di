package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
)

const (
	// RequestIDHeader is the HTTP header used for request IDs.
	RequestIDHeader = "X-Request-ID"

	// maxRequestIDLength is the maximum allowed length for an externally-provided request ID.
	maxRequestIDLength = 256

	// requestIDBytes is the number of random bytes used to generate a request ID (8 bytes = 16 hex chars).
	requestIDBytes = 8
)

type requestIDKeyType struct{}

var requestIDKey = requestIDKeyType{} //nolint:gochecknoglobals

// GetRequestID retrieves the request ID from the context.
func GetRequestID(ctx context.Context) string {
	val, ok := ctx.Value(requestIDKey).(string)
	if !ok {
		return ""
	}

	return val
}

// generateRequestID creates a new 16-character hex request ID using crypto/rand.
func generateRequestID() string {
	var buf [requestIDBytes]byte

	_, err := rand.Read(buf[:])
	if err != nil {
		panic("middleware: failed to generate request ID: " + err.Error())
	}

	return hex.EncodeToString(buf[:])
}

// isPrintableASCII reports whether s contains only printable ASCII characters (0x20-0x7E).
func isPrintableASCII(s string) bool {
	for i := range len(s) {
		if s[i] < 0x20 || s[i] > 0x7E {
			return false
		}
	}

	return true
}

// RequestID is a middleware that assigns a unique request ID to each request.
// If the X-Request-ID header is already present in the request, it reuses that value.
// Otherwise, it generates a new ID. The ID is stored in the request context
// and set as the X-Request-ID response header.
func RequestID() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { //nolint:varnamelen
			id := r.Header.Get(RequestIDHeader) //nolint:varnamelen
			if id == "" || len(id) > maxRequestIDLength || !isPrintableASCII(id) {
				id = generateRequestID()
			}

			w.Header().Set(RequestIDHeader, id)

			ctx := context.WithValue(r.Context(), requestIDKey, id)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
