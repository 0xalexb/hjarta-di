# CLAUDE.md - hjarta-di

## Project Overview

Simple dependency injection container wrapping Uber's Fx framework with integrated structured logging via `log/slog`.

- **Go version**: 1.25
- **Module**: `github.com/0xalexb/hjarta-di`

## Development Commands

```bash
# Run all tests
go test ./...

# Run linter (all linters enabled, strict mode)
golangci-lint run
```

No Makefile - use standard Go commands.

## Architecture

Three packages with distinct responsibilities:

### `di` (root package)
- `App` wrapper around `fx.App` with Start/Stop lifecycle management
- Uses Option pattern for configuration (`WithModules`, `WithLogLevel`)
- Automatically supplies `*slog.Logger` and `logging.LoggerConfig` to DI container
- Sets created logger as default via `slog.SetDefault()`
- Entry point: `NewApp(options...)` returns `*App`

### `logging`
- Creates `*slog.Logger` instances with JSON handler
- Configurable via `LoggerConfig` struct (level only)
- Constructor: `NewLogger(config LoggerConfig, w io.Writer)` returns `*slog.Logger`

### `config`
- Generic config `Provider[T]` for loading typed configuration
- Interface-based design with four extension points:
  - `Parser` - deserializes raw data into config struct (handles path navigation internally)
  - `DataFetcher` - retrieves raw config data (file, env, etc.)
  - `Validator` - validates config after parsing
  - `Defaulter` - applies default values before validation

#### `config/parser/yaml`
- Production YAML parser using `github.com/goccy/go-yaml`
- Uses goccy/go-yaml PathString for efficient path navigation
- Converts colon-separated paths (e.g., "api:permissions") to YAML path format internally
- Constructor: `NewParser()` returns `*Parser`

#### `config/fetcher/file`
- File-based DataFetcher for reading configuration from filesystem
- Reads file at construction time and caches contents (subsequent Fetch() calls return cached data)
- Validates that path points to a file (not a directory) before reading
- Exports `ErrPathIsDirectory` sentinel error for `errors.Is()` checking
- Constructor: `NewFetcher(filepath string)` returns `func() (*Fetcher, error)`

## Key Patterns

### Option Pattern
The `di` package uses functional options:
```go
app := di.NewApp(
    di.WithModules(myModule),
    di.WithLogLevel("info"),
)
```

### Interface-Based Extension
Config package uses interfaces for each processing step, allowing custom implementations while providing sensible defaults.

## Dependency Constraints

Strict dependency guard enforced via `.golangci.yml`:

**Allowed in main code:**
- Go stdlib
- `github.com/0xalexb/*` (personal repos)
- `go.uber.org/fx`
- `github.com/goccy/go-yaml`

**Additional allowed in tests:**
- `github.com/stretchr/testify/*`

Do not add external dependencies without updating `.golangci.yml` depguard rules.
