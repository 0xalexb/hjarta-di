package listener

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func freePort(t *testing.T) string {
	t.Helper()

	listenCfg := net.ListenConfig{}

	ln, err := listenCfg.Listen(context.Background(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)

	defer func() { _ = ln.Close() }()

	return ln.Addr().String()
}

func TestNewServer_SetsDefaults(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {})
	srv, err := NewServer("test", handler, Config{}, nil)
	require.NoError(t, err)

	assert.Equal(t, DefaultAddress, srv.config.Address)
	assert.Equal(t, "test", srv.name)
}

func TestServer_StartStop(t *testing.T) {
	t.Parallel()

	addr := freePort(t)
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)

		_, _ = fmt.Fprint(w, "hello")
	})

	srv, err := NewServer("api", handler, Config{Address: addr}, nil)
	require.NoError(t, err)

	err = srv.Start(context.Background())
	require.NoError(t, err)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://"+addr, nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req) //nolint:gosec // G704: test code, URL from test server
	require.NoError(t, err)

	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "hello", string(body))

	err = srv.Stop(context.Background())
	require.NoError(t, err)

	// Verify server is stopped
	dialer := net.Dialer{Timeout: 100 * time.Millisecond}

	conn, dialErr := dialer.DialContext(context.Background(), "tcp", addr)
	if dialErr == nil {
		_ = conn.Close()
	}

	assert.Error(t, dialErr, "should not be able to connect after stop")
}

func TestServer_StartFailure(t *testing.T) {
	t.Parallel()

	listenCfg := net.ListenConfig{}

	ln, err := listenCfg.Listen(context.Background(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)

	defer func() { _ = ln.Close() }()

	addr := ln.Addr().String()
	handler := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {})

	srv, srvErr := NewServer("fail", handler, Config{Address: addr}, nil)
	require.NoError(t, srvErr)

	err = srv.Start(context.Background())
	require.Error(t, err, "should fail when port is already in use")
	assert.ErrorIs(t, err, ErrListenFailed, "error should wrap ErrListenFailed")
}

func TestServer_ServeErrorCallsOnServeErr(t *testing.T) {
	t.Parallel()

	addr := freePort(t)

	var called atomic.Bool

	handler := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {})
	srv, srvErr := NewServer("test", handler, Config{Address: addr}, func() {
		called.Store(true)
	})
	require.NoError(t, srvErr)

	err := srv.Start(context.Background())
	require.NoError(t, err)

	// Close the underlying listener directly (not via http.Server) to force a non-ErrServerClosed error
	_ = srv.listener.Close()

	// Give the goroutine time to detect the error and call the callback
	assert.Eventually(
		t, called.Load, time.Second, 10*time.Millisecond,
		"onServeErr callback should be called on serve error",
	)
}

func TestNewServer_NilHandler(t *testing.T) {
	t.Parallel()

	srv, err := NewServer("test", nil, Config{}, nil)
	require.ErrorIs(t, err, ErrNilHandler)
	assert.Nil(t, srv)
}

func TestNewServer_SetsDefaultsBeforeValidation(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {})

	srv, err := NewServer("test", handler, Config{}, nil)
	require.NoError(t, err)
	assert.Equal(t, DefaultAddress, srv.config.Address, "SetDefaults should have been called before Validate")
}

func TestNewServer_EmptyName(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {})

	srv, err := NewServer("", handler, Config{}, nil)
	require.ErrorIs(t, err, ErrEmptyName)
	assert.Nil(t, srv)
}

func TestServer_ServeError_NilCallback(t *testing.T) {
	t.Parallel()

	addr := freePort(t)
	handler := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {})

	srv, err := NewServer("test", handler, Config{Address: addr}, nil)
	require.NoError(t, err)

	err = srv.Start(context.Background())
	require.NoError(t, err)

	// Close the underlying listener to trigger a serve error with nil onServeErr.
	_ = srv.listener.Close()

	// Should not panic. Give the goroutine time to process.
	time.Sleep(100 * time.Millisecond)
}

func TestWithAddress_Empty(t *testing.T) {
	t.Parallel()

	var cfg Config

	WithAddress("")(&cfg)

	assert.Empty(t, cfg.Address, "WithAddress should set address even when empty")
}

func TestServer_StopWithCancelledContext(t *testing.T) {
	t.Parallel()

	addr := freePort(t)
	received := make(chan struct{})

	// Use a handler that signals when entered, then blocks until context done.
	handler := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		close(received)
		<-r.Context().Done()
	})

	srv, err := NewServer("test", handler, Config{Address: addr}, nil)
	require.NoError(t, err)

	err = srv.Start(context.Background())
	require.NoError(t, err)

	// Make a request that will block, keeping a connection active.
	reqCtx, reqCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer reqCancel()

	go func() {
		req, reqErr := http.NewRequestWithContext(reqCtx, http.MethodGet, "http://"+addr, nil)
		if reqErr != nil {
			return
		}

		resp, doErr := http.DefaultClient.Do(req) //nolint:gosec // G704: test code, URL from test server
		if doErr == nil {
			_ = resp.Body.Close()
		}
	}()

	// Wait for the handler to confirm the request is in-flight.
	<-received

	// Cancel context before calling Stop - Shutdown cannot complete gracefully.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = srv.Stop(ctx)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrShutdownFailed, "error should wrap ErrShutdownFailed")
}
