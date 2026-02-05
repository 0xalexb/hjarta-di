// Package file provides a file-based DataFetcher implementation for the config package.
//
// This package reads configuration data from files on the filesystem.
// It implements the config.DataFetcher interface, returning raw bytes
// for subsequent parsing.
//
// The file is read at construction time and cached, meaning subsequent calls
// to Fetch() return the same data without re-reading the filesystem. This
// provides consistent configuration data throughout the application lifecycle.
//
// Usage:
//
//	fetcher, err := file.NewFetcher("/path/to/config.yaml")()
//	if err != nil {
//	    // Handle error: file not found, permission denied, path is directory, etc.
//	}
//	data, err := fetcher.Fetch()
//
// Error Handling:
//   - Construction returns error if file cannot be read or path is a directory
//   - Errors include the filepath for easier debugging
//   - Use errors.Is(err, file.ErrPathIsDirectory) to check for directory errors
package file
