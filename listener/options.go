package listener

// Option defines a function type for configuring an HTTP listener.
type Option func(*Config)

// WithAddress sets the address for the HTTP listener.
func WithAddress(addr string) Option {
	return func(cfg *Config) {
		cfg.Address = addr
	}
}
