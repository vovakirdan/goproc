package app

// Options configures the top-level controller.
type Options struct {
	// ConfigPath points to the optional daemon config file.
	ConfigPath string
}

// App exposes high-level operations that the CLI/TUI can reuse.
type App struct {
	cfgPath string
}

// New constructs the shared controller facade.
func New(opts Options) *App {
	return &App{
		cfgPath: opts.ConfigPath,
	}
}

// ConfigPath returns the configured config file path (if any).
func (a *App) ConfigPath() string {
	return a.cfgPath
}
