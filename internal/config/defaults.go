package config

import (
	"fmt"
	"log/slog"

	"github.com/mateconpizza/gm/internal/ui/menu"
)

// ConfigFile represents the configuration file.
type ConfigFile struct {
	Colorscheme string       `json:"colorscheme" yaml:"colorscheme"` // App colorscheme
	Menu        *menu.Config `json:"menu"        yaml:"menu"`        // Menu configuration
}

// Defaults holds the default configuration.
var Defaults = &ConfigFile{
	Colorscheme: "default",
	Menu:        Fzf,
}

// Validate validates the configuration file.
func Validate(cfg *ConfigFile) error {
	if cfg.Colorscheme == "" {
		slog.Warn("empty colorscheme, loading default colorscheme")

		cfg.Colorscheme = "default"
	}

	if err := cfg.Menu.Validate(); err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}
