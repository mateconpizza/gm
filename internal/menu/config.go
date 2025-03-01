package menu

import (
	"errors"
	"fmt"
)

var (
	ErrInvalidConfigKeymap   = errors.New("invalid keymap")
	ErrInvalidConfigSettings = errors.New("invalid settings")
)

// colorEnabled is a flag to enable color support.
var colorEnabled bool = false

// menuConfig holds the menu configuration.
var menuConfig *FzfConfig

// FzfSettings holds the FZF settings.
type FzfSettings []string

// FzfConfig holds the menu configuration.
type FzfConfig struct {
	Prompt   string      `yaml:"prompt"`   // Fzf prompt
	Preview  bool        `yaml:"preview"`  // Fzf enable preview
	Header   FzfHeader   `yaml:"header"`   // Fzf header
	Keymaps  Keymaps     `yaml:"keymaps"`  // Fzf keymaps
	Settings FzfSettings `yaml:"settings"` // Fzf settings
}

// FzfHeader holds the header configuration for FZF.
type FzfHeader struct {
	Enabled bool   `yaml:"enabled"`
	Sep     string `yaml:"separator"`
}

// Keymap holds the keymap configuration.
type Keymap struct {
	Bind    string `yaml:"bind"`        // keybind combination.
	Desc    string `yaml:"description"` // keybind description
	Action  string `yaml:"-"`           // action to execute.
	Enabled bool   `yaml:"enabled"`     // keybind enabled.
	Hidden  bool   `yaml:"hidden"`      // keybind hidden.
}

// Keymaps holds the keymaps for FZF.
type Keymaps struct {
	Edit      Keymap `yaml:"edit"`
	Open      Keymap `yaml:"open"`
	Preview   Keymap `yaml:"preview"`
	QR        Keymap `yaml:"qr"`
	OpenQR    Keymap `yaml:"open_qr"`
	ToggleAll Keymap `yaml:"toggle_all"`
	Yank      Keymap `yaml:"yank"`
}

// SetConfig sets menu configuration.
func SetConfig(cfg *FzfConfig) {
	menuConfig = cfg
}

// EnableColor enables color support.
func EnableColor(b bool) {
	colorEnabled = b
}

// ValidateConfig validates the menu configuration.
func ValidateConfig(cfg *FzfConfig) error {
	keymaps := []Keymap{
		cfg.Keymaps.Edit,
		cfg.Keymaps.Open,
		cfg.Keymaps.QR,
		cfg.Keymaps.OpenQR,
		cfg.Keymaps.Yank,
		cfg.Keymaps.Preview,
		cfg.Keymaps.ToggleAll,
	}

	for _, k := range keymaps {
		if k.Bind == "" {
			return fmt.Errorf("%w: empty keybind", ErrInvalidConfigKeymap)
		}
	}
	if len(cfg.Settings) == 0 {
		return fmt.Errorf("%w: empty settings", ErrInvalidConfigSettings)
	}

	return nil
}
