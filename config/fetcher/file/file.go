package file

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ErrPathIsDirectory is returned when the path provided to the Fetcher points to a directory instead of a file.
var ErrPathIsDirectory = errors.New("path is a directory, not a file")

// Fetcher implements config.DataFetcher interface for file-based configuration.
// It reads configuration data from a file at construction time and caches the contents.
type Fetcher struct {
	filepath string
	data     []byte
}

// NewFetcher returns a constructor function that creates a new file-based Fetcher
// with the specified filepath. The file is read at construction time and cached.
// This pattern is Fx-friendly, allowing the DI container to control when instantiation happens.
// Returns an error if the file cannot be read or if the path points to a directory.
func NewFetcher(fpath string) func() (*Fetcher, error) {
	return func() (*Fetcher, error) {
		cleanPath := filepath.Clean(fpath)

		stat, err := os.Stat(cleanPath)
		if err != nil {
			return nil, fmt.Errorf("stat file %q: %w", cleanPath, err)
		}

		if stat.IsDir() {
			return nil, fmt.Errorf("path %q: %w", cleanPath, ErrPathIsDirectory)
		}

		data, err := os.ReadFile(cleanPath) // #nosec G304 -- path is cleaned and validated
		if err != nil {
			return nil, fmt.Errorf("reading file %q: %w", cleanPath, err)
		}

		return &Fetcher{
			filepath: cleanPath,
			data:     data,
		}, nil
	}
}

// Fetch returns a copy of the cached configuration data that was read at construction time.
// A copy is returned to prevent callers from mutating the cached data.
func (f *Fetcher) Fetch() ([]byte, error) {
	result := make([]byte, len(f.data))
	copy(result, f.data)

	return result, nil
}
