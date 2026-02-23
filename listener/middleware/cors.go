package middleware

import (
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// OriginValidator is a function that validates an AllowedOrigins entry.
// It returns an error if the origin entry is invalid.
type OriginValidator func(origin string) error

var (
	errOriginHasScheme = errors.New("origin contains scheme (://)")
	errOriginHasPath   = errors.New("origin contains path (/)")
	errOriginHasPort   = errors.New("origin contains port")
	errOriginIsWildcard = errors.New("origin is wildcard (*)")
	errOriginIsEmpty   = errors.New("origin is empty")
)

const defaultCORSMaxAge = 3600

// corsConfig holds internal configuration for the CORS middleware.
type corsConfig struct {
	allowedOrigins   []string
	allowedMethods   []string
	allowedHeaders   []string
	exposedHeaders   []string
	validateOrigins  []OriginValidator
	allowCredentials bool
	maxAge           int
}

// CORSOption configures the CORS middleware.
type CORSOption func(*corsConfig)

// WithAllowedOrigins sets the allowed origins, replacing defaults.
// Origins are bare hostnames (e.g., "example.com") or "*" for wildcard.
func WithAllowedOrigins(origins ...string) CORSOption {
	return func(c *corsConfig) {
		c.allowedOrigins = origins
	}
}

// WithAllowedMethods sets the allowed HTTP methods, replacing defaults.
func WithAllowedMethods(methods ...string) CORSOption {
	return func(c *corsConfig) {
		c.allowedMethods = methods
	}
}

// WithAllowedHeaders sets the allowed request headers, replacing defaults.
func WithAllowedHeaders(headers ...string) CORSOption {
	return func(c *corsConfig) {
		c.allowedHeaders = headers
	}
}

// WithExposedHeaders sets the headers exposed to the browser.
func WithExposedHeaders(headers ...string) CORSOption {
	return func(c *corsConfig) {
		c.exposedHeaders = headers
	}
}

// WithMaxAge sets the preflight cache duration in seconds.
func WithMaxAge(seconds int) CORSOption {
	return func(c *corsConfig) {
		c.maxAge = seconds
	}
}

// WithAllowCredentials enables Access-Control-Allow-Credentials.
func WithAllowCredentials() CORSOption {
	return func(c *corsConfig) {
		c.allowCredentials = true
	}
}

// WithOriginValidators sets validators that reject invalid AllowedOrigins entries at construction time.
func WithOriginValidators(validators ...OriginValidator) CORSOption {
	return func(c *corsConfig) {
		c.validateOrigins = validators
	}
}

// extractHostname parses an origin URL and returns just the hostname.
// For malformed origins, it returns an empty string.
func extractHostname(origin string) string {
	u, err := url.Parse(origin)
	if err != nil {
		return ""
	}

	return u.Hostname()
}

// ValidateNoScheme returns a validator that rejects origins containing "://".
func ValidateNoScheme() OriginValidator {
	return func(origin string) error {
		if strings.Contains(origin, "://") {
			return errOriginHasScheme
		}

		return nil
	}
}

// ValidateNoPath returns a validator that rejects origins containing "/".
func ValidateNoPath() OriginValidator {
	return func(origin string) error {
		if strings.Contains(origin, "/") {
			return errOriginHasPath
		}

		return nil
	}
}

// ValidateNoPort returns a validator that rejects origins containing a port separator.
// IPv6 addresses with multiple colons (e.g., "::1") are allowed.
func ValidateNoPort() OriginValidator {
	return func(origin string) error {
		// Allow IPv6 multi-colon addresses (more than one colon means IPv6, not port).
		if strings.Count(origin, ":") == 1 {
			return errOriginHasPort
		}

		// Check for port after bracket-enclosed IPv6 (e.g., "[::1]:8080").
		if strings.Contains(origin, "]:") {
			return errOriginHasPort
		}

		return nil
	}
}

// ValidateNoWildcard returns a validator that rejects the wildcard origin "*".
func ValidateNoWildcard() OriginValidator {
	return func(origin string) error {
		if origin == "*" {
			return errOriginIsWildcard
		}

		return nil
	}
}

// ValidateNotEmpty returns a validator that rejects empty origin strings.
func ValidateNotEmpty() OriginValidator {
	return func(origin string) error {
		if origin == "" {
			return errOriginIsEmpty
		}

		return nil
	}
}

// ValidateHostname returns all hostname validators combined:
// ValidateNoScheme, ValidateNoPath, ValidateNoPort, ValidateNoWildcard, ValidateNotEmpty.
func ValidateHostname() []OriginValidator {
	return []OriginValidator{
		ValidateNoScheme(),
		ValidateNoPath(),
		ValidateNoPort(),
		ValidateNoWildcard(),
		ValidateNotEmpty(),
	}
}

func validateOrigin(origin string, validators []OriginValidator) bool {
	for _, v := range validators {
		if v == nil {
			continue
		}

		err := v(origin)
		if err != nil {
			slog.Error("middleware: CORS invalid origin, skipping", "origin", origin, "error", err)

			return false
		}
	}

	return true
}

// CORS returns a middleware that handles Cross-Origin Resource Sharing.
// It processes preflight OPTIONS requests and sets appropriate CORS headers.
// AllowedOrigins entries are bare hostnames (e.g., "example.com"), and incoming
// Origin headers are matched by extracting their hostname component.
// If AllowCredentials is true with only wildcard origins and no explicit origins,
// credentials are automatically disabled and a warning is logged.
//
// When called with no options, sensible defaults are applied:
// origins ["*"], methods ["GET","HEAD","POST"], common headers, maxAge 3600.
func CORS(opts ...CORSOption) func(http.Handler) http.Handler { //nolint:gocognit,cyclop,funlen
	cfg := &corsConfig{
		allowedOrigins: []string{"*"},
		allowedMethods: []string{"GET", "HEAD", "POST"},
		allowedHeaders: []string{"Origin", "Accept", "Content-Type", "X-Requested-With"},
		maxAge:         defaultCORSMaxAge,
	}

	for _, opt := range opts {
		if opt == nil {
			continue
		}

		opt(cfg)
	}

	allowedHostnames := make(map[string]struct{}, len(cfg.allowedOrigins))
	wildcard := false

	for _, hostname := range cfg.allowedOrigins {
		if valid := validateOrigin(hostname, cfg.validateOrigins); !valid {
			continue
		}

		if hostname == "" {
			continue
		}

		if hostname == "*" {
			wildcard = true

			continue
		}

		allowedHostnames[strings.ToLower(hostname)] = struct{}{}
	}

	// When credentials are enabled, wildcard origin matching is disabled
	// to prevent reflecting arbitrary origins with Access-Control-Allow-Credentials: true.
	// Only explicitly listed (non-wildcard) origins are matched in this case.
	if cfg.allowCredentials {
		if wildcard && len(allowedHostnames) == 0 {
			slog.Warn("middleware: CORS AllowCredentials with only wildcard origin is invalid, disabling credentials")

			cfg.allowCredentials = false
		} else {
			wildcard = false
		}
	}

	methods := strings.Join(cfg.allowedMethods, ", ")
	headers := strings.Join(cfg.allowedHeaders, ", ")
	exposedHeaders := strings.Join(cfg.exposedHeaders, ", ")
	maxAge := strconv.Itoa(cfg.maxAge)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Add("Vary", "Origin")

			origin := r.Header.Get("Origin")

			if origin == "" {
				next.ServeHTTP(w, r)

				return
			}

			hostname := strings.ToLower(extractHostname(origin))

			_, matched := allowedHostnames[hostname]
			if !matched && !wildcard {
				next.ServeHTTP(w, r)

				return
			}

			if wildcard {
				w.Header().Set("Access-Control-Allow-Origin", "*")
			} else {
				w.Header().Set("Access-Control-Allow-Origin", origin)
			}

			if cfg.allowCredentials {
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}

			if exposedHeaders != "" {
				w.Header().Set("Access-Control-Expose-Headers", exposedHeaders)
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

				if cfg.maxAge > 0 {
					w.Header().Set("Access-Control-Max-Age", maxAge)
				}

				w.WriteHeader(http.StatusNoContent)

				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
