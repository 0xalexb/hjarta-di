package config

import (
	"fmt"
	"log/slog"
)

// Parser defines an interface for parsing configuration data into a target structure.
//
// The path parameter specifies a navigation path within the configuration data
// using colon (:) as the separator for nested keys. For example:
//   - "api:permissions" navigates to config["api"]["permissions"]
//   - "database:connection:timeout" navigates three levels deep
//   - "" (empty path) means parse the entire document
//
// Parser implementations are responsible for path navigation internally.
// See config/parser/yaml for an example using goccy/go-yaml PathString.
type Parser interface {
	Parse(data []byte, target any, path string) error
}

// DataFetcher defines an interface for reading configuration data.
type DataFetcher interface {
	Fetch() ([]byte, error)
}

// Validator defines an interface for validating configuration structures.
type Validator interface {
	Validate() error
}

// Defaulter defines an interface for setting default values in configuration structures.
type Defaulter interface {
	SetDefaults() (changed bool)
}

// Provider returns a function that reads, parses, sets defaults, and validates configuration data.
func Provider[T any](target *T, path string) func(Parser, DataFetcher) (*T, error) {
	return func(parser Parser, dataSourcer DataFetcher) (*T, error) {
		data, err := dataSourcer.Fetch()
		if err != nil {
			return nil, fmt.Errorf("reading data error: %w", err)
		}

		err = parser.Parse(data, target, path)
		if err != nil {
			return nil, fmt.Errorf("parsing error: %w", err)
		}

		targetDefaulter, isDefaulter := any(target).(Defaulter)
		if isDefaulter {
			changed := targetDefaulter.SetDefaults()
			if changed {
				slog.Info("defaults applied", slog.String("path", path))
			}
		}

		targetValidatable, isValidatable := any(target).(Validator)
		if isValidatable {
			err := targetValidatable.Validate()
			if err != nil {
				return nil, fmt.Errorf("validating error: %w", err)
			}
		}

		return target, nil
	}
}
