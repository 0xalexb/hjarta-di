package listener

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"
)

// ReadHeaderTimeout is the default timeout for reading request headers.
const ReadHeaderTimeout = 10 * time.Second

// Server manages an HTTP server lifecycle.
type Server struct {
	name       string
	config     Config
	server     *http.Server
	listener   net.Listener
	onServeErr func()
}

// NewServer creates a new Server with the given name, handler, and config.
// It sets config defaults, validates the config, and creates the underlying http.Server.
// The onServeErr callback, if non-nil, is called when the background Serve goroutine encounters a fatal error.
func NewServer(name string, handler http.Handler, cfg Config, onServeErr func()) (*Server, error) {
	if name == "" {
		return nil, ErrEmptyName
	}

	if handler == nil {
		return nil, ErrNilHandler
	}

	cfg.SetDefaults()

	err := cfg.Validate()
	if err != nil {
		return nil, err
	}

	return &Server{
		name:   name,
		config: cfg,
		server: &http.Server{ //nolint:exhaustruct // only relevant fields needed
			Addr:              cfg.Address,
			Handler:           handler,
			ReadHeaderTimeout: ReadHeaderTimeout,
		},
		listener:   nil,
		onServeErr: onServeErr,
	}, nil
}

// Start begins listening on TCP and serves HTTP requests in a background goroutine.
func (s *Server) Start(ctx context.Context) error {
	listenCfg := net.ListenConfig{} //nolint:exhaustruct // zero-value defaults are fine

	listener, err := listenCfg.Listen(ctx, "tcp", s.server.Addr)
	if err != nil {
		slog.Error("failed to listen", "name", s.name, "address", s.server.Addr, "error", err)

		return fmt.Errorf("%w: %w", ErrListenFailed, err)
	}

	s.listener = listener

	slog.Info("starting HTTP listener", "name", s.name, "address", s.server.Addr)

	go func() {
		serveErr := s.server.Serve(listener)
		if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			slog.Error("HTTP listener error", "name", s.name, "error", serveErr)

			if s.onServeErr != nil {
				s.onServeErr()
			}
		}
	}()

	return nil
}

// Stop gracefully shuts down the HTTP server.
func (s *Server) Stop(ctx context.Context) error {
	slog.Info("stopping HTTP listener", "name", s.name)

	err := s.server.Shutdown(ctx)
	if err != nil {
		slog.Error("shutdown failed", "name", s.name, "error", err)

		return fmt.Errorf("%w: %w", ErrShutdownFailed, err)
	}

	return nil
}
