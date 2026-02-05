package di_test

import (
	"errors"
	"fmt"

	di "github.com/0xalexb/hjarta-di"
	"github.com/0xalexb/hjarta-di/config"
	filefetcher "github.com/0xalexb/hjarta-di/config/fetcher/file"
	yamlparser "github.com/0xalexb/hjarta-di/config/parser/yaml"

	"go.uber.org/fx"
)

// ServerConfig represents application server configuration.
// It implements both Defaulter and Validator interfaces from the config package.
type ServerConfig struct {
	Host    string `yaml:"host"`
	Port    int    `yaml:"port"`
	Timeout int    `yaml:"timeout"`
}

// SetDefaults sets default values for the configuration.
func (c *ServerConfig) SetDefaults() bool {
	changed := false

	if c.Host == "" {
		c.Host = "localhost"
		changed = true
	}

	if c.Port == 0 {
		c.Port = 8080
		changed = true
	}

	if c.Timeout == 0 {
		c.Timeout = 30
		changed = true
	}

	return changed
}

// Validate validates the configuration.
func (c *ServerConfig) Validate() error {
	if c.Port < 1 || c.Port > 65535 {
		return errors.New("port must be between 1 and 65535")
	}

	if c.Timeout < 1 {
		return errors.New("timeout must be positive")
	}

	return nil
}

// ServerService is a service that depends on config.
type ServerService struct {
	Config *ServerConfig
}

// Address returns the server address from config.
func (s *ServerService) Address() string {
	return fmt.Sprintf("%s:%d", s.Config.Host, s.Config.Port)
}

// Example_appWithConfigIntegration demonstrates how to use App, Options, and Config together.
// It shows the complete workflow from defining configuration to dependency injection.
func Example_appWithConfigIntegration() {
	// Step 1: Create an Fx module that provides config dependencies using Fx-friendly constructors.
	// yamlparser.NewParser and filefetcher.NewFetcher return constructor functions.
	// fx.Annotate with fx.As casts concrete types to their interfaces for config.Provider.
	configModule := fx.Module("config",
		fx.Provide(
			fx.Annotate(
				yamlparser.NewParser,
				fx.As(new(config.Parser)),
			),
		),
		fx.Provide(
			fx.Annotate(
				filefetcher.NewFetcher("testdata/config.yaml"),
				fx.As(new(config.DataFetcher)),
			),
		),
		fx.Provide(config.Provider(new(ServerConfig), "")),
	)

	serviceModule := fx.Module("service",
		fx.Provide(func(cfg *ServerConfig) *ServerService {
			return &ServerService{
				Config: cfg,
			}
		}),
	)

	// Step 2: Create and start the App with logging and modules.
	var service *ServerService

	invokeModule := fx.Module("invoke",
		fx.Invoke(func(s *ServerService) {
			service = s
		}),
	)

	app := di.NewApp(
		di.WithLogLevel("error"),
		di.WithModules(configModule, serviceModule, invokeModule),
	)

	err := app.Start()
	if err != nil {
		fmt.Printf("Error starting app: %v\n", err)

		return
	}

	defer func() { _ = app.Stop() }()

	// Step 3: Verify the service has config injected.
	fmt.Printf("Server address: %s\n", service.Address())
	fmt.Printf("Timeout: %d\n", service.Config.Timeout)
	// Output:
	// Server address: api.example.com:9000
	// Timeout: 30
}
