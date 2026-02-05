package yaml

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	"github.com/goccy/go-yaml"
)

// ErrEmptyData is returned when the input data is empty.
var ErrEmptyData = errors.New("empty data")

// ErrPathNotFound is returned when the specified path is not found in the YAML document.
var ErrPathNotFound = errors.New("path not found")

// Parser implements config.Parser interface for YAML data.
// It uses goccy/go-yaml PathString for efficient path navigation.
type Parser struct{}

// NewParser creates a new YAML parser instance.
func NewParser() *Parser {
	return &Parser{}
}

// Parse parses YAML data and unmarshals it into the target.
// The path parameter specifies a navigation path using colon (:) as separator.
// Empty path parses the entire document.
func (p *Parser) Parse(data []byte, target any, path string) error {
	if len(data) == 0 {
		return ErrEmptyData
	}

	if path == "" {
		err := yaml.Unmarshal(data, target)
		if err != nil {
			return fmt.Errorf("unmarshal error: %w", err)
		}

		return nil
	}

	yamlPath := convertToYAMLPath(path)

	pathObj, err := yaml.PathString(yamlPath)
	if err != nil {
		return fmt.Errorf("invalid path %q: %w", path, err)
	}

	reader := bytes.NewReader(data)

	err = pathObj.Read(reader, target)
	if err != nil {
		if isKeyNotFoundError(err) {
			return fmt.Errorf("%w: %s", ErrPathNotFound, path)
		}

		return fmt.Errorf("reading path %q: %w", path, err)
	}

	return nil
}

// convertToYAMLPath converts a colon-separated path to goccy/go-yaml PathString format.
// Examples:
//   - "key" -> "$.key"
//   - "api:permissions" -> "$.api.permissions"
func convertToYAMLPath(path string) string {
	parts := strings.Split(path, ":")

	return "$." + strings.Join(parts, ".")
}

// isKeyNotFoundError checks if the error indicates a key was not found.
func isKeyNotFoundError(err error) bool {
	return yaml.IsNotFoundNodeError(err)
}
