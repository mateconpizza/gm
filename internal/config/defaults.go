package config

import (
	"github.com/mateconpizza/gm/internal/ui/menu"
)

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
		Menu: menu.NewDefaultConfig(),
	}
}
