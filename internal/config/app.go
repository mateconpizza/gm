package config

var app *Config

// SetDefault assigns the given config as the global application config.
func SetDefault(a *Config) {
	app = a
}

// New returns the global application config.
// It panics if the database path has not been set.
func New() *Config {
	if app.DBName == "" {
		panic("repo name not set")
	}

	if app.DBPath == "" {
		panic("repo path not set")
	}

	return app
}
