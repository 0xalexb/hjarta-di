package listener

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
)

func TestNewModule_WithOptions(t *testing.T) {
	t.Parallel()

	addr := freePort(t)
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)

		_, _ = fmt.Fprint(w, "ok")
	})

	app := fxtest.New(t,
		fx.Supply(fx.Annotate(handler, fx.As(new(http.Handler)), fx.ResultTags(`name:"api"`))),
		NewModule("api", WithAddress(addr)),
	)

	app.RequireStart()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://"+addr, nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req) //nolint:gosec // G704: test code, URL from test server
	require.NoError(t, err)

	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "ok", string(body))

	app.RequireStop()
}

func TestNewModule_WithExternalConfig(t *testing.T) {
	t.Parallel()

	addr := freePort(t)
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	cfg := Config{Address: addr}

	app := fxtest.New(t,
		fx.Supply(
			fx.Annotate(handler, fx.As(new(http.Handler)), fx.ResultTags(`name:"metrics"`)),
			fx.Annotate(cfg, fx.ResultTags(`name:"metrics"`)),
		),
		NewModule("metrics"),
	)

	app.RequireStart()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://"+addr, nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req) //nolint:gosec // G704: test code, URL from test server
	require.NoError(t, err)

	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	app.RequireStop()
}

func TestNewModule_TwoListeners(t *testing.T) {
	t.Parallel()

	addr1 := freePort(t)
	addr2 := freePort(t)

	handler1 := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, "api")
	})
	handler2 := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, "metrics")
	})

	app := fxtest.New(t,
		fx.Supply(
			fx.Annotate(handler1, fx.As(new(http.Handler)), fx.ResultTags(`name:"api"`)),
			fx.Annotate(handler2, fx.As(new(http.Handler)), fx.ResultTags(`name:"metrics"`)),
		),
		NewModule("api", WithAddress(addr1)),
		NewModule("metrics", WithAddress(addr2)),
	)

	app.RequireStart()

	req1, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://"+addr1, nil)
	require.NoError(t, err)

	resp1, err := http.DefaultClient.Do(req1) //nolint:gosec // G704: test code, URL from test server
	require.NoError(t, err)

	defer func() { _ = resp1.Body.Close() }()

	body1, _ := io.ReadAll(resp1.Body)
	assert.Equal(t, "api", string(body1))

	req2, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://"+addr2, nil)
	require.NoError(t, err)

	resp2, err := http.DefaultClient.Do(req2) //nolint:gosec // G704: test code, URL from test server
	require.NoError(t, err)

	defer func() { _ = resp2.Body.Close() }()

	body2, _ := io.ReadAll(resp2.Body)
	assert.Equal(t, "metrics", string(body2))

	app.RequireStop()
}

func TestNewModule_ShutdownStopsServer(t *testing.T) {
	t.Parallel()

	addr := freePort(t)
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	app := fxtest.New(t,
		fx.Supply(fx.Annotate(handler, fx.As(new(http.Handler)), fx.ResultTags(`name:"api"`))),
		NewModule("api", WithAddress(addr)),
	)

	app.RequireStart()
	app.RequireStop()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://"+addr, nil)
	require.NoError(t, err)

	resp, doErr := http.DefaultClient.Do(req) //nolint:gosec // G704: test code, URL from test server
	if doErr == nil {
		_ = resp.Body.Close()
	}

	require.Error(t, doErr, "server should be stopped after shutdown")

	dialer := net.Dialer{Timeout: 100 * time.Millisecond}

	conn, dialErr := dialer.DialContext(context.Background(), "tcp", addr)
	if dialErr == nil {
		_ = conn.Close()
	}

	assert.Error(t, dialErr, "should not be able to connect after shutdown")
}

func TestNewModule_ListenFailure(t *testing.T) {
	t.Parallel()

	listenCfg := net.ListenConfig{}

	ln, err := listenCfg.Listen(context.Background(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)

	defer func() { _ = ln.Close() }()

	addr := ln.Addr().String()
	handler := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {})

	app := fx.New(
		fx.Supply(fx.Annotate(handler, fx.As(new(http.Handler)), fx.ResultTags(`name:"fail"`))),
		NewModule("fail", WithAddress(addr)),
		fx.NopLogger,
	)

	err = app.Start(context.Background())
	assert.Error(t, err, "should fail when port is already in use")
}

func TestNewModule_EmptyName(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {})

	app := fx.New(
		fx.Supply(fx.Annotate(handler, fx.As(new(http.Handler)), fx.ResultTags(`name:""`))),
		NewModule(""),
		fx.NopLogger,
	)

	err := app.Err()
	require.Error(t, err, "should fail with empty name")
	assert.ErrorIs(t, err, ErrEmptyName)
}
