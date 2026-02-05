package logging_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/0xalexb/hjarta-di/logging"

	"github.com/stretchr/testify/require"
)

func TestNewLogger_JSONOutput(t *testing.T) {
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

func TestNewLogger_Levels(t *testing.T) {
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
			name:          "info level logs info",
			configLevel:   "INFO",
			logLevel:      slog.LevelInfo,
			shouldLog:     true,
			expectedLevel: "INFO",
		},
		{
			name:          "warn level logs warn",
			configLevel:   "WARN",
			logLevel:      slog.LevelWarn,
			shouldLog:     true,
			expectedLevel: "WARN",
		},
		{
			name:          "warning level logs warn",
			configLevel:   "WARNING",
			logLevel:      slog.LevelWarn,
			shouldLog:     true,
			expectedLevel: "WARN",
		},
		{
			name:          "error level logs error",
			configLevel:   "ERROR",
			logLevel:      slog.LevelError,
			shouldLog:     true,
			expectedLevel: "ERROR",
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
			name:          "lowercase level is accepted",
			configLevel:   "debug",
			logLevel:      slog.LevelDebug,
			shouldLog:     true,
			expectedLevel: "DEBUG",
		},
		{
			name:          "empty level defaults to info",
			configLevel:   "",
			logLevel:      slog.LevelInfo,
			shouldLog:     true,
			expectedLevel: "INFO",
		},
		{
			name:          "invalid level defaults to info",
			configLevel:   "INVALID",
			logLevel:      slog.LevelInfo,
			shouldLog:     true,
			expectedLevel: "INFO",
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

func TestLoggerConfig_ZeroValue(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	config := logging.LoggerConfig{}
	logger := logging.NewLogger(config, &buf)

	logger.Info("test message")

	var logEntry map[string]any

	err := json.Unmarshal(buf.Bytes(), &logEntry)
	require.NoError(t, err, "output should be valid JSON")
	require.Equal(t, "INFO", logEntry["level"], "default level should be INFO")
}
