package config

var app *Config

// Set assigns the given config as the global application config.
func Set(a *Config) {
	app = a
}

// New returns the global application config.
// It panics if the database path or name has not been set.
func New() *Config {
	if app.DBName == "" {
		panic("repo name not set")
	}

	if app.DBPath == "" {
		panic("repo path not set")
	}

	return app
}
