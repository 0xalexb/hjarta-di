package di

//nolint:gochecknoglobals // set via ldflags at build time.
var (
	// Version is the application version, set via ldflags.
	Version = "dev"
	// DIVersion is the DI framework version, set via ldflags.
	DIVersion = "dev"
	// CompiledAt is the build timestamp, set via ldflags.
	CompiledAt = "unknown"
)
