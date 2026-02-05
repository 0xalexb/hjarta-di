// Package yaml provides a YAML parser implementation for the config package.
//
// This package uses github.com/goccy/go-yaml for YAML parsing with native
// PathString support for efficient path navigation. The parser converts
// colon-separated paths (e.g., "api:permissions") to YAML path format
// (e.g., "$.api.permissions") internally.
//
// Usage:
//
//	parser := yaml.NewParser()
//	var cfg Config
//	err := parser.Parse(data, &cfg, "api:permissions")
//
// Path Conversion:
//   - Empty path "" -> unmarshal entire document
//   - Single key "key" -> "$.key"
//   - Nested path "api:permissions" -> "$.api.permissions"
package yaml
