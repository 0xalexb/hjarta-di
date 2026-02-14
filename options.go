package di

import (
	"github.com/0xalexb/hjarta-di/listener"

	"go.uber.org/fx"
)

// Options holds configuration settings for the application.
type Options struct {
	Modules  []fx.Option
	LogLevel string
}

// Option defines a function type for applying configuration options.
type Option func(*Options)

// WithModules adds Fx modules to the application.
func WithModules(modules ...fx.Option) Option {
	return func(opts *Options) {
		opts.Modules = append(opts.Modules, modules...)
	}
}

// WithHTTPListener adds a named HTTP listener module to the application.
// The name is used as both the Fx module name and the DI named tag for http.Handler and Config.
// When options are provided (e.g., WithAddress), Config is supplied to DI automatically.
// Call multiple times with different names to create multiple listeners.
func WithHTTPListener(name string, opts ...listener.Option) Option {
	return func(o *Options) {
		o.Modules = append(o.Modules, listener.NewModule(name, opts...))
	}
}

// WithLogLevel sets the log level for the application.
// Valid levels are: "debug", "info", "warn", "error".
// If not set or invalid, defaults to "info".
func WithLogLevel(level string) Option {
	return func(opts *Options) {
		opts.LogLevel = level
	}
}
