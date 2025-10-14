package config

var cfg *Config

// Set assigns the given config as the global application config.
func Set(a *Config) {
	cfg = a
}

// New returns the global application config.
// It panics if the database path or name has not been set.
func New() *Config {
	if cfg.DBName == "" {
		panic("repo name not set")
	}

	if cfg.DBPath == "" {
		panic("repo path not set")
	}

	return cfg
}
