package config

import (
	"errors"
	"testing"
)

type mockParser struct {
	parseFunc func(data []byte, target any, path string) error
}

func (m *mockParser) Parse(data []byte, target any, path string) error {
	return m.parseFunc(data, target, path)
}

type mockDataFetcher struct {
	fetchFunc func() ([]byte, error)
}

func (m *mockDataFetcher) Fetch() ([]byte, error) {
	return m.fetchFunc()
}

type simpleConfig struct {
	Name string
}

type configWithDefaults struct {
	Name    string
	changed bool
}

func (c *configWithDefaults) SetDefaults() bool {
	return c.changed
}

type configWithValidator struct {
	Name string
	err  error
}

func (c *configWithValidator) Validate() error {
	return c.err
}

type configWithBoth struct {
	Name    string
	changed bool
	err     error
}

func (c *configWithBoth) SetDefaults() bool {
	return c.changed
}

func (c *configWithBoth) Validate() error {
	return c.err
}

func TestProvider_Success(t *testing.T) {
	t.Parallel()

	target := &simpleConfig{}
	parser := &mockParser{
		parseFunc: func(_ []byte, target any, _ string) error {
			cfg, ok := target.(*simpleConfig)
			if !ok {
				return errors.New("invalid target type")
			}

			cfg.Name = "test"

			return nil
		},
	}

	fetcher := &mockDataFetcher{
		fetchFunc: func() ([]byte, error) {
			return []byte("data"), nil
		},
	}

	provider := Provider(target, "test/path")

	result, err := provider(parser, fetcher)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != target {
		t.Error("expected result to be the same as target")
	}

	if result.Name != "test" {
		t.Errorf("expected Name to be 'test', got %q", result.Name)
	}
}

func TestProvider_WithValidation_Success(t *testing.T) {
	t.Parallel()

	target := &configWithValidator{err: nil}
	parser := &mockParser{
		parseFunc: func(_ []byte, _ any, _ string) error {
			return nil
		},
	}
	fetcher := &mockDataFetcher{
		fetchFunc: func() ([]byte, error) {
			return []byte("data"), nil
		},
	}

	provider := Provider(target, "test/path")

	result, err := provider(parser, fetcher)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != target {
		t.Error("expected result to be the same as target")
	}
}

func TestProvider_WithDefaultsAndValidation_Success(t *testing.T) {
	t.Parallel()

	target := &configWithBoth{changed: true, err: nil}
	parser := &mockParser{
		parseFunc: func(_ []byte, _ any, _ string) error {
			return nil
		},
	}
	fetcher := &mockDataFetcher{
		fetchFunc: func() ([]byte, error) {
			return []byte("data"), nil
		},
	}

	provider := Provider(target, "test/path")

	result, err := provider(parser, fetcher)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != target {
		t.Error("expected result to be the same as target")
	}
}

func TestProvider_Errors(t *testing.T) {
	t.Parallel()

	fetchErr := errors.New("fetch failed")
	parseErr := errors.New("parse failed")
	validationErr := errors.New("validation failed")

	tests := []struct {
		name      string
		fetchFunc func() ([]byte, error)
		parseFunc func(data []byte, target any, path string) error
		targetErr error
		wantErr   error
	}{
		{
			name: "fetch error",
			fetchFunc: func() ([]byte, error) {
				return nil, fetchErr
			},
			parseFunc: func(_ []byte, _ any, _ string) error {
				return nil
			},
			targetErr: nil,
			wantErr:   fetchErr,
		},
		{
			name: "parse error",
			fetchFunc: func() ([]byte, error) {
				return []byte("data"), nil
			},
			parseFunc: func(_ []byte, _ any, _ string) error {
				return parseErr
			},
			targetErr: nil,
			wantErr:   parseErr,
		},
		{
			name: "validation error",
			fetchFunc: func() ([]byte, error) {
				return []byte("data"), nil
			},
			parseFunc: func(_ []byte, _ any, _ string) error {
				return nil
			},
			targetErr: validationErr,
			wantErr:   validationErr,
		},
	}

	for _, testInfo := range tests {
		t.Run(testInfo.name, func(t *testing.T) {
			t.Parallel()

			target := &configWithBoth{err: testInfo.targetErr}
			parser := &mockParser{parseFunc: testInfo.parseFunc}
			fetcher := &mockDataFetcher{fetchFunc: testInfo.fetchFunc}

			provider := Provider(target, "test/path")

			result, err := provider(parser, fetcher)

			if result != nil {
				t.Error("expected result to be nil")
			}

			if err == nil {
				t.Fatal("expected error, got nil")
			}

			if !errors.Is(err, testInfo.wantErr) {
				t.Errorf("expected error to wrap %v, got %v", testInfo.wantErr, err)
			}
		})
	}
}

func TestProvider_Defaults(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		changed bool
	}{
		{
			name:    "defaults changed",
			changed: true,
		},
		{
			name:    "defaults not changed",
			changed: false,
		},
	}

	for _, testInfo := range tests {
		t.Run(testInfo.name, func(t *testing.T) {
			t.Parallel()

			target := &configWithDefaults{changed: testInfo.changed}
			parser := &mockParser{
				parseFunc: func(_ []byte, _ any, _ string) error {
					return nil
				},
			}
			fetcher := &mockDataFetcher{
				fetchFunc: func() ([]byte, error) {
					return []byte("data"), nil
				},
			}

			provider := Provider(target, "test/path")

			result, err := provider(parser, fetcher)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result != target {
				t.Error("expected result to be the same as target")
			}
		})
	}
}
