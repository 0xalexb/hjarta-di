package di

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/0xalexb/hjarta-di/logging"

	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
)

var errAppNotInitialized = errors.New("app not initialized")

// App is a configured starting point for application using Fx.
type App struct {
	app *fx.App
}

// NewApp creates a new instance of App with Fx configured.
func NewApp(opts ...Option) *App {
	var options Options

	for _, apply := range opts {
		apply(&options)
	}

	return &App{
		app: configure(&options),
	}
}

func configure(options *Options) *fx.App {
	logger := createLogger(options.LogLevel, os.Stderr)
	slog.SetDefault(logger)

	return fx.New(
		fx.WithLogger(func() fxevent.Logger {
			return &fxevent.SlogLogger{Logger: logger}
		}),
		fx.Supply(logging.LoggerConfig{Level: options.LogLevel}),
		fx.Supply(logger),
		fx.Options(options.Modules...),
	)
}

func createLogger(level string, w io.Writer) *slog.Logger {
	config := logging.LoggerConfig{Level: level}

	return logging.NewLogger(config, w)
}

// Start starts the Fx application.
func (app *App) Start() error {
	if app != nil && app.app != nil {
		err := app.app.Start(context.Background())
		if err != nil {
			return fmt.Errorf("failed to start app: %w", err)
		}

		return nil
	}

	return errAppNotInitialized
}

// Run starts the application and blocks until an OS signal is received, then shuts down gracefully.
func (app *App) Run() {
	if app == nil || app.app == nil {
		slog.Error("attempted to run an uninitialized app")

		return
	}

	app.app.Run()
}

// Stop stops the Fx application gracefully.
func (app *App) Stop() error {
	if app != nil && app.app != nil {
		err := app.app.Stop(context.Background())
		if err != nil {
			return fmt.Errorf("failed to stop app: %w", err)
		}

		return nil
	}

	return errAppNotInitialized
}
