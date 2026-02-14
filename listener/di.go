package listener

import (
	"fmt"
	"log/slog"
	"net/http"

	"go.uber.org/fx"
)

// NewModule creates an Fx module for a named HTTP listener.
// The name is used as both the module name and the DI named tag for http.Handler and Config.
// If any options are passed, the module supplies Config to DI from those options.
// Otherwise, Config must be provided externally (e.g., via config.Provider).
//
//nolint:ireturn // fx.Option is the standard return type for Fx modules
func NewModule(name string, opts ...Option) fx.Option {
	if name == "" {
		return fx.Error(ErrEmptyName)
	}

	var cfg Config

	for _, apply := range opts {
		apply(&cfg)
	}

	hasConfigFromOptions := len(opts) > 0

	var moduleOpts []fx.Option

	if hasConfigFromOptions {
		moduleOpts = append(moduleOpts, fx.Supply(
			fx.Annotate(cfg, fx.ResultTags(fmt.Sprintf(`name:"%s"`, name))),
		))
	}

	moduleOpts = append(moduleOpts, fx.Invoke(
		fx.Annotate(
			func(lifecycle fx.Lifecycle, shutdowner fx.Shutdowner, handler http.Handler, listenerCfg Config) error {
				srv, err := NewServer(name, handler, listenerCfg, func() {
					shutdownErr := shutdowner.Shutdown()
					if shutdownErr != nil {
						slog.Error("failed to trigger shutdown", "name", name, "error", shutdownErr)
					}
				})
				if err != nil {
					return err
				}

				lifecycle.Append(fx.Hook{
					OnStart: srv.Start,
					OnStop:  srv.Stop,
				})

				return nil
			},
			fx.ParamTags("", "", fmt.Sprintf(`name:"%s"`, name), fmt.Sprintf(`name:"%s"`, name)),
		),
	))

	return fx.Module(name, moduleOpts...)
}
