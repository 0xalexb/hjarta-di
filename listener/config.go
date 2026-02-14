// Package listener provides an HTTP listener module for the Fx DI container.
package listener

import "errors"

// DefaultAddress is the default address for the HTTP listener.
const DefaultAddress = ":8080"

// ErrEmptyAddress is returned when the address is empty.
var ErrEmptyAddress = errors.New("address must not be empty")

// ErrListenFailed is returned when the server fails to listen on the configured address.
var ErrListenFailed = errors.New("failed to listen")

// ErrShutdownFailed is returned when the server fails to shut down gracefully.
var ErrShutdownFailed = errors.New("shutdown failed")

// ErrEmptyName is returned when the listener name is empty.
var ErrEmptyName = errors.New("listener name must not be empty")

// ErrNilHandler is returned when a nil http.Handler is provided.
var ErrNilHandler = errors.New("handler must not be nil")

// Config holds the configuration for an HTTP listener.
type Config struct {
	Address string
}

// SetDefaults sets default values for the Config.
func (c *Config) SetDefaults() {
	if c.Address == "" {
		c.Address = DefaultAddress
	}
}

// Validate validates the Config.
func (c *Config) Validate() error {
	if c.Address == "" {
		return ErrEmptyAddress
	}

	return nil
}
