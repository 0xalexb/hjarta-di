package listener

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_SetDefaults(t *testing.T) {
	t.Parallel()

	t.Run("sets default address when empty", func(t *testing.T) {
		t.Parallel()

		cfg := &Config{}
		cfg.SetDefaults()

		assert.Equal(t, DefaultAddress, cfg.Address)
	})

	t.Run("does not override existing address", func(t *testing.T) {
		t.Parallel()

		cfg := &Config{Address: ":9090"}
		cfg.SetDefaults()

		assert.Equal(t, ":9090", cfg.Address)
	})
}

func TestConfig_Validate(t *testing.T) {
	t.Parallel()

	t.Run("valid config", func(t *testing.T) {
		t.Parallel()

		cfg := &Config{Address: ":8080"}
		err := cfg.Validate()

		require.NoError(t, err)
	})

	t.Run("empty address", func(t *testing.T) {
		t.Parallel()

		cfg := &Config{}
		err := cfg.Validate()

		require.Error(t, err)
		assert.ErrorIs(t, err, ErrEmptyAddress)
	})
}
