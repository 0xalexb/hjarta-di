package di_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	di "github.com/0xalexb/hjarta-di"
	"github.com/0xalexb/hjarta-di/logging"

	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
)

func TestNewApp_CreatesAppWithDefaultLogLevel(t *testing.T) {
	t.Parallel()

	app := di.NewApp()
	require.NotNil(t, app)
}

func TestNewApp_WithLogLevel(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name  string
		level string
	}{
		{"debug level", "debug"},
		{"info level", "info"},
		{"warn level", "warn"},
		{"error level", "error"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			app := di.NewApp(di.WithLogLevel(tc.level))
			require.NotNil(t, app)
		})
	}
}

func TestNewApp_WithModules(t *testing.T) {
	t.Parallel()

	var invoked bool

	module := fx.Module("test",
		fx.Invoke(func() {
			invoked = true
		}),
	)

	app := di.NewApp(di.WithModules(module))
	require.NotNil(t, app)

	err := app.Start()
	require.NoError(t, err)
	t.Cleanup(func() { _ = app.Stop() })
	require.True(t, invoked)
}

func TestNewApp_LoggerIsAvailableInFxContainer(t *testing.T) {
	t.Parallel()

	var capturedLogger *slog.Logger

	module := fx.Module("test",
		fx.Invoke(func(logger *slog.Logger) {
			capturedLogger = logger
		}),
	)

	app := di.NewApp(
		di.WithLogLevel("debug"),
		di.WithModules(module),
	)
	require.NotNil(t, app)

	err := app.Start()
	require.NoError(t, err)
	t.Cleanup(func() { _ = app.Stop() })
	require.NotNil(t, capturedLogger)
}

func TestNewApp_LoggerOutputsJSON(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	config := logging.LoggerConfig{Level: "INFO"}
	logger := logging.NewLogger(config, &buf)

	logger.Info("test message", slog.String("key", "value"))

	var logEntry map[string]any

	err := json.Unmarshal(buf.Bytes(), &logEntry)
	require.NoError(t, err, "output should be valid JSON")
	require.Equal(t, "test message", logEntry["msg"])
	require.Equal(t, "value", logEntry["key"])
	require.Equal(t, "INFO", logEntry["level"])
}

func TestNewApp_LoggerRespectsLogLevel(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	config := logging.LoggerConfig{Level: "ERROR"}
	logger := logging.NewLogger(config, &buf)

	logger.Info("should not appear")
	require.Empty(t, buf.String(), "info log should not be written when level is error")

	logger.Error("should appear")
	require.NotEmpty(t, buf.String(), "error log should be written when level is error")
}

func TestNewApp_LoggerConfigIsSupplied(t *testing.T) {
	t.Parallel()

	var capturedConfig logging.LoggerConfig

	module := fx.Module("test",
		fx.Invoke(func(config logging.LoggerConfig) {
			capturedConfig = config
		}),
	)

	app := di.NewApp(
		di.WithLogLevel("warn"),
		di.WithModules(module),
	)
	require.NotNil(t, app)

	err := app.Start()
	require.NoError(t, err)
	t.Cleanup(func() { _ = app.Stop() })
	require.Equal(t, "warn", capturedConfig.Level)
}

func TestNewApp_InjectedLoggerHasCorrectLevel(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name          string
		configLevel   string
		logLevel      slog.Level
		shouldLog     bool
		expectedLevel string
	}{
		{
			name:          "debug level logs debug",
			configLevel:   "DEBUG",
			logLevel:      slog.LevelDebug,
			shouldLog:     true,
			expectedLevel: "DEBUG",
		},
		{
			name:          "info level does not log debug",
			configLevel:   "INFO",
			logLevel:      slog.LevelDebug,
			shouldLog:     false,
			expectedLevel: "",
		},
		{
			name:          "error level does not log info",
			configLevel:   "ERROR",
			logLevel:      slog.LevelInfo,
			shouldLog:     false,
			expectedLevel: "",
		},
		{
			name:          "error level logs error",
			configLevel:   "ERROR",
			logLevel:      slog.LevelError,
			shouldLog:     true,
			expectedLevel: "ERROR",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer

			config := logging.LoggerConfig{Level: testCase.configLevel}
			logger := logging.NewLogger(config, &buf)

			logger.Log(context.Background(), testCase.logLevel, "test message")

			if testCase.shouldLog {
				require.NotEmpty(t, buf.String(), "log should be written")

				var logEntry map[string]any

				err := json.Unmarshal(buf.Bytes(), &logEntry)
				require.NoError(t, err, "output should be valid JSON")
				require.Equal(t, testCase.expectedLevel, logEntry["level"])
			} else {
				require.Empty(t, buf.String(), "log should not be written")
			}
		})
	}
}

func TestApp_Stop(t *testing.T) {
	t.Parallel()

	var stopCalled bool

	module := fx.Module("test",
		fx.Invoke(func(lc fx.Lifecycle) {
			lc.Append(fx.Hook{
				OnStop: func(_ context.Context) error {
					stopCalled = true

					return nil
				},
			})
		}),
	)

	app := di.NewApp(di.WithModules(module))
	require.NotNil(t, app)

	err := app.Start()
	require.NoError(t, err)

	err = app.Stop()
	require.NoError(t, err)
	require.True(t, stopCalled, "OnStop hook should be called")
}

func TestApp_StopOnNilApp(t *testing.T) {
	t.Parallel()

	var app *di.App

	err := app.Stop()
	require.Error(t, err)
}

func TestApp_StartOnNilApp(t *testing.T) {
	t.Parallel()

	var app *di.App

	err := app.Start()
	require.Error(t, err)
}

func TestApp_RunOnNilApp(t *testing.T) {
	t.Parallel()

	var app *di.App

	require.NotPanics(t, func() {
		app.Run()
	})
}

func TestApp_Run(t *testing.T) {
	t.Parallel()

	module := fx.Module("test",
		fx.Invoke(func(shutdowner fx.Shutdowner) {
			go func() {
				_ = shutdowner.Shutdown()
			}()
		}),
	)

	app := di.NewApp(di.WithModules(module))
	require.NotNil(t, app)

	require.NotPanics(t, func() {
		app.Run()
	})
}
