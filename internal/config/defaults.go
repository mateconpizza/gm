package config

import (
	"fmt"

	"github.com/mateconpizza/gm/internal/ui/menu"
)

// ConfigFile represents the configuration file.
type ConfigFile struct {
	Menu *menu.Config `json:"menu" yaml:"menu"`          // Menu configuration
	Git  *Git         `json:"git"  yaml:"git,omitempty"` // Git status
}

// Defaults holds the default configuration.
var Defaults = &ConfigFile{
	Menu: Fzf,
}

// Validate validates the configuration file.
func Validate(cfg *ConfigFile) error {
	if err := cfg.Menu.Validate(); err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}

func NewDefaultConfig(version string) *Config {
	return &Config{
		Name:   AppName,
		Cmd:    AppCommand,
		DBName: MainDBName,
		Flags:  &Flags{},
		Info: &Information{
			URL:     "https://github.com/mateconpizza/gm#readme",
			Title:   "Gomarks: A bookmark manager",
			Tags:    "golang,awesome,bookmarks,cli",
			Desc:    "Simple yet powerful bookmark manager for your terminal",
			Version: version,
		},
		Path: &Path{},
		Git: &Git{
			Enabled: false,
			GPG:     false,
			Log:     true, // FIX: not implemented yet
		},
		Env: &Env{
			Home:   EnvHome,
			Editor: EnvEditor,
		},
	}
}
