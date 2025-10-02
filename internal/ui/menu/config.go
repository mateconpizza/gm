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

const (
	unicodePathBigSegment = "\u25B6" // ▶
	unicodeMiddleDot      = "\u00b7" // ·
	DefaultPrompt         = unicodePathBigSegment + " "
	DefaultHeaderSep      = " " + unicodeMiddleDot + " "
)

// colorEnabled is a flag to enable color support.
var colorEnabled bool = false

// menuConfig holds the menu configuration.
var menuConfig *Config = &Config{}

// FzfSettings holds the FZF settings.
type FzfSettings []string

// Config holds the menu configuration.
type Config struct {
	// TODO: complete `Defaults` option. This will be used to load fzf's users
	// configuration
	Defaults bool        `json:"defaults" yaml:"defaults"` // Use defaults ($FZF_DEFAULT_OPTS_FILE and $FZF_DEFAULT_OPTS)
	Prompt   string      `json:"prompt"   yaml:"prompt"`   // Fzf prompt
	Preview  bool        `json:"preview"  yaml:"preview"`  // Fzf enable preview
	Header   FzfHeader   `json:"header"   yaml:"header"`   // Fzf header
	Keymaps  Keymaps     `json:"keymaps"  yaml:"keymaps"`  // Fzf keymaps
	Settings FzfSettings `json:"settings" yaml:"settings"` // Fzf settings
}

// FzfHeader holds the header configuration for FZF.
type FzfHeader struct {
	Enabled bool   `yaml:"enabled"`
	Sep     string `yaml:"separator"`
}

// Validate validates the menu configuration.
func (c *Config) Validate() error {
	keymaps := []Keymap{
		c.Keymaps.Edit,
		c.Keymaps.Open,
		c.Keymaps.QR,
		c.Keymaps.OpenQR,
		c.Keymaps.Yank,
		c.Keymaps.Preview,
		c.Keymaps.ToggleAll,
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
	if len(c.Settings) == 0 {
		slog.Warn("empty settings, loading default settings")
	}

	return nil
}

// SetConfig sets menu configuration.
func SetConfig(cfg *Config) {
	menuConfig = cfg
}

// ColorEnable enables color support.
func ColorEnable(b bool) {
	colorEnabled = b
}
