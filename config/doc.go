// Package config provides configuration management functionalities and interfaces.
//
// The package uses an interface-based design with four extension points:
//   - Parser: deserializes raw data into config struct, with path navigation support
//   - DataFetcher: retrieves raw config data (file, env, etc.)
//   - Validator: validates config after parsing
//   - Defaulter: applies default values before validation
//
// # Path Navigation
//
// The Provider function accepts a path parameter that allows targeting a specific
// section within configuration files. Paths use colon (:) as the separator:
//
//	"api:permissions"           -> config["api"]["permissions"]
//	"database:connection"       -> config["database"]["connection"]
//	""                          -> entire document (backwards compatible)
//
// Parser implementations handle path navigation internally. For example, the
// YAML parser in config/parser/yaml uses goccy/go-yaml PathString to efficiently
// navigate to the target section before unmarshaling.
//
// # Example
//
// A typical usage pattern:
//
//	type APIConfig struct {
//	    Timeout int    `yaml:"timeout"`
//	    BaseURL string `yaml:"base_url"`
//	}
//
//	provider := config.Provider(&APIConfig{}, "services:api")
//	cfg, err := provider(yamlparser.NewParser(), filefetcher.New("config.yaml"))
package config
