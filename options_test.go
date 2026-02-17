package di_test

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"

	di "github.com/0xalexb/hjarta-di"
	"github.com/0xalexb/hjarta-di/listener"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
)

func TestWithLogLevel(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		level    string
		expected string
	}{
		{
			name:     "debug level",
			level:    "debug",
			expected: "debug",
		},
		{
			name:     "info level",
			level:    "info",
			expected: "info",
		},
		{
			name:     "warn level",
			level:    "warn",
			expected: "warn",
		},
		{
			name:     "error level",
			level:    "error",
			expected: "error",
		},
		{
			name:     "empty level",
			level:    "",
			expected: "",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var opts di.Options

			di.WithLogLevel(testCase.level)(&opts)

			require.Equal(t, testCase.expected, opts.LogLevel)
		})
	}
}

func TestWithLogLevelDefault(t *testing.T) {
	t.Parallel()

	var opts di.Options
	// Without calling WithLogLevel, LogLevel should be empty string (zero value)
	require.Empty(t, opts.LogLevel)
}

func TestWithModules(t *testing.T) {
	t.Parallel()

	module1 := fx.Module("test1")
	module2 := fx.Module("test2")

	var opts di.Options

	di.WithModules(module1)(&opts)
	require.Len(t, opts.Modules, 1)

	di.WithModules(module2)(&opts)
	require.Len(t, opts.Modules, 2)
}

func TestWithModulesMultiple(t *testing.T) {
	t.Parallel()

	module1 := fx.Module("test1")
	module2 := fx.Module("test2")

	var opts di.Options

	di.WithModules(module1, module2)(&opts)
	require.Len(t, opts.Modules, 2)
}

func freePort(t *testing.T) string {
	t.Helper()

	listenCfg := net.ListenConfig{}

	ln, err := listenCfg.Listen(context.Background(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)

	defer func() { _ = ln.Close() }()

	return ln.Addr().String()
}

func TestWithHTTPListener(t *testing.T) {
	t.Parallel()

	var opts di.Options

	di.WithHTTPListener("api")(&opts)
	require.Len(t, opts.Modules, 1)
}

func TestWithHTTPListenerMultiple(t *testing.T) {
	t.Parallel()

	var opts di.Options

	di.WithHTTPListener("api")(&opts)
	di.WithHTTPListener("metrics")(&opts)
	require.Len(t, opts.Modules, 2)
}

func TestWithHTTPListener_WithAddress(t *testing.T) {
	t.Parallel()

	addr := freePort(t)
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, "hello")
	})

	app := di.NewApp(
		di.WithHTTPListener("api", listener.WithAddress(addr)),
		di.WithModules(
			fx.Supply(fx.Annotate(handler, fx.As(new(http.Handler)), fx.ResultTags(`name:"api"`))),
		),
	)

	require.NoError(t, app.Start())

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://"+addr, nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req) //nolint:gosec // G704: test code, URL from test server
	require.NoError(t, err)

	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	assert.Equal(t, "hello", string(body))

	require.NoError(t, app.Stop())
}

func TestWithHTTPListener_ExternalConfig(t *testing.T) {
	t.Parallel()

	addr := freePort(t)
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	cfg := listener.Config{Address: addr}

	app := di.NewApp(
		di.WithHTTPListener("ext"),
		di.WithModules(
			fx.Supply(
				fx.Annotate(handler, fx.As(new(http.Handler)), fx.ResultTags(`name:"ext"`)),
				fx.Annotate(cfg, fx.ResultTags(`name:"ext"`)),
			),
		),
	)

	require.NoError(t, app.Start())

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://"+addr, nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req) //nolint:gosec // G704: test code, URL from test server
	require.NoError(t, err)

	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	require.NoError(t, app.Stop())
}

func TestWithHTTPListener_MultipleListeners(t *testing.T) {
	t.Parallel()

	addr1 := freePort(t)
	addr2 := freePort(t)

	handler1 := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, "api")
	})
	handler2 := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, "metrics")
	})

	app := di.NewApp(
		di.WithHTTPListener("api", listener.WithAddress(addr1)),
		di.WithHTTPListener("metrics", listener.WithAddress(addr2)),
		di.WithModules(
			fx.Supply(
				fx.Annotate(handler1, fx.As(new(http.Handler)), fx.ResultTags(`name:"api"`)),
				fx.Annotate(handler2, fx.As(new(http.Handler)), fx.ResultTags(`name:"metrics"`)),
			),
		),
	)

	require.NoError(t, app.Start())

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

	require.NoError(t, app.Stop())
}
