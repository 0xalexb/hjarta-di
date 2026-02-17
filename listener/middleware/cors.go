package middleware

import (
	"log/slog"
	"net/http"
	"strconv"
	"strings"
)

// CORSConfig holds configuration for the CORS middleware.
type CORSConfig struct {
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	AllowCredentials bool
	MaxAge           int
}

// CORS returns a middleware that handles Cross-Origin Resource Sharing.
// It processes preflight OPTIONS requests and sets appropriate CORS headers.
// If AllowCredentials is true with only wildcard origins and no explicit origins,
// credentials are automatically disabled and a warning is logged.
func CORS(cfg CORSConfig) func(http.Handler) http.Handler { //nolint:gocognit,cyclop,funlen
	allowedOrigins := make(map[string]struct{}, len(cfg.AllowedOrigins))
	wildcard := false

	for _, origin := range cfg.AllowedOrigins {
		if origin == "*" {
			wildcard = true

			continue
		}

		allowedOrigins[origin] = struct{}{}
	}

	// When credentials are enabled, wildcard origin matching is disabled
	// to prevent reflecting arbitrary origins with Access-Control-Allow-Credentials: true.
	// Only explicitly listed (non-wildcard) origins are matched in this case.
	if cfg.AllowCredentials {
		if wildcard && len(allowedOrigins) == 0 {
			slog.Warn("middleware: CORS AllowCredentials with only wildcard origin is invalid, disabling credentials")

			cfg.AllowCredentials = false
		} else {
			wildcard = false
		}
	}

	methods := strings.Join(cfg.AllowedMethods, ", ")
	headers := strings.Join(cfg.AllowedHeaders, ", ")
	maxAge := strconv.Itoa(cfg.MaxAge)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { //nolint:varnamelen
			w.Header().Add("Vary", "Origin")

			origin := r.Header.Get("Origin")

			if origin == "" {
				next.ServeHTTP(w, r)

				return
			}

			_, matched := allowedOrigins[origin]
			if !matched && !wildcard {
				next.ServeHTTP(w, r)

				return
			}

			if wildcard {
				w.Header().Set("Access-Control-Allow-Origin", "*")
			} else {
				w.Header().Set("Access-Control-Allow-Origin", origin)
			}

			if cfg.AllowCredentials {
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}

			if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
				w.Header().Add("Vary", "Access-Control-Request-Method")
				w.Header().Add("Vary", "Access-Control-Request-Headers")

				if methods != "" {
					w.Header().Set("Access-Control-Allow-Methods", methods)
				}

				if headers != "" {
					w.Header().Set("Access-Control-Allow-Headers", headers)
				}

				if cfg.MaxAge > 0 {
					w.Header().Set("Access-Control-Max-Age", maxAge)
				}

				w.WriteHeader(http.StatusNoContent)

				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
