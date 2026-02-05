package di

import "go.uber.org/fx"

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

// WithLogLevel sets the log level for the application.
// Valid levels are: "debug", "info", "warn", "error".
// If not set or invalid, defaults to "info".
func WithLogLevel(level string) Option {
	return func(opts *Options) {
		opts.LogLevel = level
	}
}
