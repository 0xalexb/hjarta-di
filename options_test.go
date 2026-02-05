package di_test

import (
	"testing"

	di "github.com/0xalexb/hjarta-di"

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
