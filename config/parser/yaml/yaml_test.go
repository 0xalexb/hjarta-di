package yaml

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParser_Parse_EmptyPath(t *testing.T) {
	t.Parallel()

	parser := NewParser()

	data := []byte(`
name: test-app
version: "1.0"
`)

	var result struct {
		Name    string `yaml:"name"`
		Version string `yaml:"version"`
	}

	err := parser.Parse(data, &result, "")

	require.NoError(t, err)
	assert.Equal(t, "test-app", result.Name)
	assert.Equal(t, "1.0", result.Version)
}

func TestParser_Parse_SingleLevelPath(t *testing.T) {
	t.Parallel()

	parser := NewParser()

	data := []byte(`
api:
  host: localhost
  port: 8080
database:
  host: db.example.com
`)

	var result struct {
		Host string `yaml:"host"`
		Port int    `yaml:"port"`
	}

	err := parser.Parse(data, &result, "api")

	require.NoError(t, err)
	assert.Equal(t, "localhost", result.Host)
	assert.Equal(t, 8080, result.Port)
}

func TestParser_Parse_MultiLevelPath(t *testing.T) {
	t.Parallel()

	parser := NewParser()

	data := []byte(`
api:
  permissions:
    admin:
      read: true
      write: true
    user:
      read: true
      write: false
`)

	var result struct {
		Read  bool `yaml:"read"`
		Write bool `yaml:"write"`
	}

	err := parser.Parse(data, &result, "api:permissions:admin")

	require.NoError(t, err)
	assert.True(t, result.Read)
	assert.True(t, result.Write)
}

func TestParser_Parse_NonExistentKey(t *testing.T) {
	t.Parallel()

	parser := NewParser()

	data := []byte(`
api:
  host: localhost
`)

	var result struct {
		Host string `yaml:"host"`
	}

	err := parser.Parse(data, &result, "nonexistent")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestParser_Parse_NonMappingIntermediate(t *testing.T) {
	t.Parallel()

	parser := NewParser()

	data := []byte(`
api: "just a string"
`)

	var result struct {
		Host string `yaml:"host"`
	}

	err := parser.Parse(data, &result, "api:nested")

	require.Error(t, err)
}

func TestParser_Parse_StringValue(t *testing.T) {
	t.Parallel()

	parser := NewParser()

	data := []byte(`
config:
  name: test-value
`)

	var result string

	err := parser.Parse(data, &result, "config:name")

	require.NoError(t, err)
	assert.Equal(t, "test-value", result)
}

func TestParser_Parse_IntValue(t *testing.T) {
	t.Parallel()

	parser := NewParser()

	data := []byte(`
config:
  port: 8080
`)

	var result int

	err := parser.Parse(data, &result, "config:port")

	require.NoError(t, err)
	assert.Equal(t, 8080, result)
}

func TestParser_Parse_ArrayValue(t *testing.T) {
	t.Parallel()

	parser := NewParser()

	data := []byte(`
config:
  hosts:
    - host1.example.com
    - host2.example.com
    - host3.example.com
`)

	var result []string

	err := parser.Parse(data, &result, "config:hosts")

	require.NoError(t, err)
	assert.Equal(t, []string{"host1.example.com", "host2.example.com", "host3.example.com"}, result)
}

func TestParser_Parse_NestedMapValue(t *testing.T) {
	t.Parallel()

	parser := NewParser()

	data := []byte(`
config:
  database:
    primary:
      host: primary.db.com
      port: 5432
    replica:
      host: replica.db.com
      port: 5432
`)

	var result map[string]struct {
		Host string `yaml:"host"`
		Port int    `yaml:"port"`
	}

	err := parser.Parse(data, &result, "config:database")

	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, "primary.db.com", result["primary"].Host)
	assert.Equal(t, "replica.db.com", result["replica"].Host)
}

func TestParser_Parse_EmptyData(t *testing.T) {
	t.Parallel()

	parser := NewParser()

	var result struct{}

	err := parser.Parse([]byte{}, &result, "")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty data")
}

func TestParser_Parse_InvalidYAML(t *testing.T) {
	t.Parallel()

	parser := NewParser()

	data := []byte(`
invalid: yaml: content: [
`)

	var result struct{}

	err := parser.Parse(data, &result, "")

	require.Error(t, err)
}

func TestConvertToYAMLPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "single key",
			input:    "key",
			expected: "$.key",
		},
		{
			name:     "two level path",
			input:    "api:permissions",
			expected: "$.api.permissions",
		},
		{
			name:     "three level path",
			input:    "database:connection:timeout",
			expected: "$.database.connection.timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := convertToYAMLPath(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParser_Parse_BoolValue(t *testing.T) {
	t.Parallel()

	parser := NewParser()

	data := []byte(`
config:
  enabled: true
  disabled: false
`)

	var enabled bool

	err := parser.Parse(data, &enabled, "config:enabled")
	require.NoError(t, err)
	assert.True(t, enabled)

	var disabled bool

	err = parser.Parse(data, &disabled, "config:disabled")
	require.NoError(t, err)
	assert.False(t, disabled)
}

func TestParser_Parse_FloatValue(t *testing.T) {
	t.Parallel()

	parser := NewParser()

	data := []byte(`
config:
  ratio: 3.14159
`)

	var result float64

	err := parser.Parse(data, &result, "config:ratio")

	require.NoError(t, err)
	assert.InDelta(t, 3.14159, result, 0.00001)
}
