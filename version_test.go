package di_test

import (
	"testing"

	di "github.com/0xalexb/hjarta-di"

	"github.com/stretchr/testify/require"
)

func TestVersion_DefaultValues(t *testing.T) {
	t.Parallel()

	require.Equal(t, "dev", di.Version)
	require.Equal(t, "dev", di.DIVersion)
	require.Equal(t, "unknown", di.CompiledAt)
}
