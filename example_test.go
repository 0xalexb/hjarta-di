package di_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"

	di "github.com/0xalexb/hjarta-di"
	"github.com/0xalexb/hjarta-di/config"
	filefetcher "github.com/0xalexb/hjarta-di/config/fetcher/file"
	"github.com/0xalexb/hjarta-di/listener"
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

// Example_versionInformation demonstrates how to access build-time version variables.
// These values default to "dev"/"unknown" and can be overridden via ldflags at build time.
func Example_versionInformation() {
	fmt.Printf("Version: %s\n", di.Version)
	fmt.Printf("DIVersion: %s\n", di.DIVersion)
	fmt.Printf("CompiledAt: %s\n", di.CompiledAt)
	// Output:
	// Version: dev
	// DIVersion: dev
	// CompiledAt: unknown
}

// Example_appRun demonstrates how to use app.Run() for blocking execution with graceful shutdown.
// Run() blocks until an OS signal or fx.Shutdowner triggers shutdown.
func Example_appRun() {
	module := fx.Module("app",
		fx.Invoke(func(shutdowner fx.Shutdowner) {
			go func() {
				_ = shutdowner.Shutdown()
			}()
		}),
	)

	app := di.NewApp(
		di.WithLogLevel("error"),
		di.WithModules(module),
	)

	fmt.Println("Starting app...")
	app.Run()
	fmt.Println("App stopped gracefully.")
	// Output:
	// Starting app...
	// App stopped gracefully.
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

// Example_appWithHTTPListener demonstrates how to use WithHTTPListener to create
// an app with a named HTTP listener, make a request, and shut down gracefully.
func Example_appWithHTTPListener() {
	// Find a free port.
	freePortListener, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		fmt.Printf("Error finding free port: %v\n", err)

		return
	}

	addr := freePortListener.Addr().String()
	_ = freePortListener.Close()

	// Provide a named http.Handler for the "api" listener.
	handlerModule := fx.Module("handler",
		fx.Provide(
			fx.Annotate(
				func() http.Handler {
					return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						_, _ = fmt.Fprint(w, "hello from listener")
					})
				},
				fx.ResultTags(`name:"api"`),
			),
		),
	)

	app := di.NewApp(
		di.WithLogLevel("error"),
		di.WithModules(handlerModule),
		di.WithHTTPListener("api", listener.WithAddress(addr)),
	)

	err = app.Start()
	if err != nil {
		fmt.Printf("Error starting app: %v\n", err)

		return
	}

	defer func() { _ = app.Stop() }()

	resp, err := http.Get("http://" + addr + "/") //nolint:noctx // example test, context not needed
	if err != nil {
		fmt.Printf("Error making request: %v\n", err)

		return
	}
	defer resp.Body.Close() //nolint:errcheck // example test

	body, _ := io.ReadAll(resp.Body)
	fmt.Println(string(body))
	// Output:
	// hello from listener
}
