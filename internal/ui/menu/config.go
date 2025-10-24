package menu

import (
	"errors"
	"fmt"
	"log/slog"
)

var (
	ErrInvalidConfigKeymap   = errors.New("invalid keymap")
	ErrInvalidConfigSettings = errors.New("invalid settings")
)

// Args holds the FZF arguments.
type Args []string

// Config holds the menu configuration.
type Config struct {
	Defaults       bool            `json:"defaults"  yaml:"defaults"`  // Use $FZF_DEFAULT_OPTS_FILE n $FZF_DEFAULT_OPTS
	Prompt         string          `json:"prompt"    yaml:"prompt"`    // Fzf prompt
	Preview        bool            `json:"preview"   yaml:"preview"`   // Fzf enable preview
	Header         Header          `json:"header"    yaml:"header"`    // Fzf header
	BuiltinKeymaps *BuiltinKeymaps `json:"keymaps"   yaml:"keymaps"`   // Fzf keymaps
	Arguments      Args            `json:"arguments" yaml:"arguments"` // Fzf arguments
}

// Header holds the header configuration for FZF.
type Header struct {
	Enabled bool   `yaml:"enabled"`
	Sep     string `yaml:"separator"`
}

// Validate validates the menu configuration.
func (c *Config) Validate() error {
	keymaps := []*Keymap{
		c.BuiltinKeymaps.Edit,
		c.BuiltinKeymaps.Open,
		c.BuiltinKeymaps.QR,
		c.BuiltinKeymaps.OpenQR,
		c.BuiltinKeymaps.Yank,
		c.BuiltinKeymaps.Preview,
		c.BuiltinKeymaps.ToggleAll,
	}

	for _, k := range keymaps {
		if !k.Enabled {
			continue
		}

		if k.Bind == "" {
			return fmt.Errorf("%w: empty keybind", ErrInvalidConfigKeymap)
		}
	}

	// set default prompt
	if c.Prompt == "" {
		slog.Warn("empty prompt, loading default prompt")

		c.Prompt = DefaultPrompt
	}

	// set default header separator
	if c.Header.Sep == "" {
		slog.Warn("empty header separator, loading default header separator")

		c.Header.Sep = DefaultHeaderSep
	}

	// set default settings
	if len(c.Arguments) == 0 {
		slog.Warn("empty settings, loading default settings")
	}

	return nil
}
