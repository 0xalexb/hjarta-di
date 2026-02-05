package file

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFetcher_Fetch_Success(t *testing.T) {
	t.Parallel()

	content := []byte(`
name: test-app
version: "1.0"
`)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	err := os.WriteFile(configPath, content, 0o600)
	require.NoError(t, err)

	fetcher, err := NewFetcher(configPath)()
	require.NoError(t, err)

	data, err := fetcher.Fetch()

	require.NoError(t, err)
	assert.Equal(t, content, data)
}

func TestFetcher_Fetch_FileNotFound(t *testing.T) {
	t.Parallel()

	fetcher, err := NewFetcher("/nonexistent/path/config.yaml")()

	require.Error(t, err)
	assert.Nil(t, fetcher)
	assert.Contains(t, err.Error(), "stat file")
	assert.Contains(t, err.Error(), "nonexistent")
}

func TestFetcher_Fetch_EmptyFile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "empty.yaml")

	err := os.WriteFile(configPath, []byte{}, 0o600)
	require.NoError(t, err)

	fetcher, err := NewFetcher(configPath)()
	require.NoError(t, err)

	data, err := fetcher.Fetch()

	require.NoError(t, err)
	assert.Empty(t, data)
}

func TestFetcher_Fetch_LargeFile(t *testing.T) {
	t.Parallel()

	// Create a large content
	content := make([]byte, 1024*1024) // 1MB
	for i := range content {
		content[i] = byte('a' + (i % 26))
	}

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "large.yaml")

	err := os.WriteFile(configPath, content, 0o600)
	require.NoError(t, err)

	fetcher, err := NewFetcher(configPath)()
	require.NoError(t, err)

	data, err := fetcher.Fetch()

	require.NoError(t, err)
	assert.Equal(t, content, data)
}

func TestNewFetcher_ReturnsValidConstructor(t *testing.T) {
	t.Parallel()

	content := []byte("test: value")
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	err := os.WriteFile(configPath, content, 0o600)
	require.NoError(t, err)

	constructor := NewFetcher(configPath)

	assert.NotNil(t, constructor)

	fetcher, err := constructor()
	require.NoError(t, err)
	assert.NotNil(t, fetcher)
	assert.Equal(t, configPath, fetcher.filepath)
}

func TestFetcher_Fetch_DirectoryPath(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	fetcher, err := NewFetcher(tmpDir)()

	require.Error(t, err)
	assert.Nil(t, fetcher)
	require.ErrorIs(t, err, ErrPathIsDirectory)
	assert.Contains(t, err.Error(), "is a directory")
}

func TestFetcher_Fetch_MultipleCalls_ReturnsSameData(t *testing.T) {
	t.Parallel()

	content := []byte(`
database:
  host: localhost
  port: 5432
`)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	err := os.WriteFile(configPath, content, 0o600)
	require.NoError(t, err)

	fetcher, err := NewFetcher(configPath)()
	require.NoError(t, err)

	// Call Fetch multiple times
	data1, err := fetcher.Fetch()
	require.NoError(t, err)

	data2, err := fetcher.Fetch()
	require.NoError(t, err)

	data3, err := fetcher.Fetch()
	require.NoError(t, err)

	// All calls should return the same data
	assert.Equal(t, content, data1)
	assert.Equal(t, content, data2)
	assert.Equal(t, content, data3)
	assert.Equal(t, data1, data2)
	assert.Equal(t, data2, data3)
}

func TestFetcher_Fetch_FileModifiedAfterConstruction_ReturnsCachedData(t *testing.T) {
	t.Parallel()

	originalContent := []byte(`version: "1.0"`)
	modifiedContent := []byte(`version: "2.0"`)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Write original content
	err := os.WriteFile(configPath, originalContent, 0o600)
	require.NoError(t, err)

	// Create fetcher (reads file at construction time)
	fetcher, err := NewFetcher(configPath)()
	require.NoError(t, err)

	// Modify the file after construction
	err = os.WriteFile(configPath, modifiedContent, 0o600)
	require.NoError(t, err)

	// Fetch should return the original cached content, not the modified content
	data, err := fetcher.Fetch()
	require.NoError(t, err)

	assert.Equal(t, originalContent, data, "Fetch should return cached data, not current file content")
	assert.NotEqual(t, modifiedContent, data, "Fetch should not return modified file content")
}

func TestFetcher_Fetch_ReturnsCopy_MutationSafe(t *testing.T) {
	t.Parallel()

	content := []byte(`original: value`)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	err := os.WriteFile(configPath, content, 0o600)
	require.NoError(t, err)

	fetcher, err := NewFetcher(configPath)()
	require.NoError(t, err)

	// Get first copy and mutate it
	data1, err := fetcher.Fetch()
	require.NoError(t, err)

	data1[0] = 'X' // Mutate the returned slice

	// Get second copy - should be unaffected by mutation
	data2, err := fetcher.Fetch()
	require.NoError(t, err)

	assert.Equal(t, content, data2, "Fetch should return unmodified cached data")
}

