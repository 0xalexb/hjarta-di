package config_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/0xalexb/hjarta-di/config"
	filefetcher "github.com/0xalexb/hjarta-di/config/fetcher/file"
	yamlparser "github.com/0xalexb/hjarta-di/config/parser/yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// AppConfig represents application configuration.
type AppConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

// SetDefaults sets default values for the configuration.
func (c *AppConfig) SetDefaults() bool {
	changed := false

	if c.Host == "" {
		c.Host = "localhost"
		changed = true
	}

	if c.Port == 0 {
		c.Port = 8080
		changed = true
	}

	return changed
}

// Validate validates the configuration.
func (c *AppConfig) Validate() error {
	if c.Port < 1 || c.Port > 65535 {
		return errors.New("port must be between 1 and 65535")
	}

	return nil
}

// StaticDataFetcher implements config.DataFetcher with static data.
// Useful for unit tests that don't need file I/O.
type StaticDataFetcher struct {
	Data []byte
}

// Fetch returns the static data.
func (f *StaticDataFetcher) Fetch() ([]byte, error) {
	return f.Data, nil
}

func ExampleProvider() {
	// Create a target configuration struct.
	cfg := &AppConfig{}

	// Create a provider function using an empty path (parses entire document).
	// The path parameter is used for both logging and navigation.
	// An empty path means the entire document will be parsed.
	provider := config.Provider(cfg, "")

	// Create production YAML parser and static data fetcher.
	// For file-based configuration, use filefetcher.NewFetcher(filepath)() instead.
	parser := yamlparser.NewParser()
	fetcher := &StaticDataFetcher{
		Data: []byte("host: example.com\n"),
	}

	// Execute the provider to read, parse, set defaults, and validate.
	result, err := provider(parser, fetcher)
	if err != nil {
		fmt.Printf("Error: %v\n", err)

		return
	}

	fmt.Printf("Host: %s, Port: %d\n", result.Host, result.Port)
	// Output: Host: example.com, Port: 8080
}

// ServerConfig represents a nested server configuration for testing.
type ServerConfig struct {
	Host    string `yaml:"host"`
	Port    int    `yaml:"port"`
	Timeout int    `yaml:"timeout"`
}

// DatabaseConfig represents a database configuration for testing.
type DatabaseConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Name     string `yaml:"name"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
}

// TestYAMLParser_PathNavigation tests the production YAML parser with various path navigation scenarios.
func TestYAMLParser_PathNavigation(t *testing.T) {
	t.Parallel()

	yamlData := []byte(`
server:
  host: api.example.com
  port: 8080
  timeout: 30
database:
  connection:
    host: db.example.com
    port: 5432
    name: myapp
  credentials:
    user: admin
    password: secret
`)

	parser := yamlparser.NewParser()

	t.Run("navigate to nested section", func(t *testing.T) {
		t.Parallel()

		cfg := &ServerConfig{}
		err := parser.Parse(yamlData, cfg, "server")
		require.NoError(t, err)

		assert.Equal(t, "api.example.com", cfg.Host)
		assert.Equal(t, 8080, cfg.Port)
		assert.Equal(t, 30, cfg.Timeout)
	})

	t.Run("navigate to deeply nested section", func(t *testing.T) {
		t.Parallel()

		cfg := &DatabaseConfig{}
		err := parser.Parse(yamlData, cfg, "database:connection")
		require.NoError(t, err)

		assert.Equal(t, "db.example.com", cfg.Host)
		assert.Equal(t, 5432, cfg.Port)
		assert.Equal(t, "myapp", cfg.Name)
	})

	t.Run("empty path parses entire document", func(t *testing.T) {
		t.Parallel()

		cfg := make(map[string]any)
		err := parser.Parse(yamlData, &cfg, "")
		require.NoError(t, err)

		// Verify the top-level structure exists
		assert.Contains(t, cfg, "server")
		assert.Contains(t, cfg, "database")
	})

	t.Run("invalid path returns error", func(t *testing.T) {
		t.Parallel()

		cfg := &ServerConfig{}
		err := parser.Parse(yamlData, cfg, "nonexistent:path")
		require.Error(t, err)
		assert.ErrorIs(t, err, yamlparser.ErrPathNotFound)
	})

	t.Run("path to non-map value returns error", func(t *testing.T) {
		t.Parallel()

		cfg := &ServerConfig{}
		err := parser.Parse(yamlData, cfg, "server:host:invalid")
		require.Error(t, err)
	})
}

func ExampleProvider_pathNavigation() {
	// Example YAML configuration with nested sections
	yamlData := []byte(`
api:
  host: api.example.com
  port: 3000
  timeout: 60
admin:
  host: admin.example.com
  port: 8080
  timeout: 120
`)

	// Create target configuration struct
	cfg := &ServerConfig{}

	// Create provider with path navigation - targeting the "api" section
	provider := config.Provider(cfg, "api")

	// Create production YAML parser and static data fetcher
	parser := yamlparser.NewParser()
	fetcher := &StaticDataFetcher{Data: yamlData}

	// Execute the provider - it will navigate to the "api" section
	result, err := provider(parser, fetcher)
	if err != nil {
		fmt.Printf("Error: %v\n", err)

		return
	}

	fmt.Printf("API Config - Host: %s, Port: %d, Timeout: %d\n", result.Host, result.Port, result.Timeout)
	// Output: API Config - Host: api.example.com, Port: 3000, Timeout: 60
}

func ExampleProvider_fileDataFetcher() {
	// This example shows how to use the file-based DataFetcher.
	// In production, you would use a real file path.
	// Create target configuration struct
	cfg := &AppConfig{}

	// Create provider - path "server" would navigate to the server section
	provider := config.Provider(cfg, "")

	// Create production YAML parser and file fetcher
	parser := yamlparser.NewParser()

	// Use file fetcher for production configuration
	// fetcher := filefetcher.New("/path/to/config.yml")
	_ = filefetcher.NewFetcher("/path/to/config.yml") // For documentation purposes

	// For this example, use static data
	fetcher := &StaticDataFetcher{
		Data: []byte("host: production.example.com\nport: 443\n"),
	}

	// Execute the provider
	result, err := provider(parser, fetcher)
	if err != nil {
		fmt.Printf("Error: %v\n", err)

		return
	}

	fmt.Printf("Host: %s, Port: %d\n", result.Host, result.Port)
	// Output: Host: production.example.com, Port: 443
}
