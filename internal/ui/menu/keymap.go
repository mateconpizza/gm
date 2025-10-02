package menu

import "fmt"

// Keymap holds the keymap configuration.
type Keymap struct {
	Bind    string `yaml:"bind"`        // keybind combination.
	Desc    string `yaml:"description"` // keybind description
	Action  string `yaml:"-"`           // action to execute.
	Enabled bool   `yaml:"enabled"`     // keybind enabled.
	Hidden  bool   `yaml:"hidden"`      // keybind hidden.
}

// WithAction returns a new Keymap with the given action command.
func (k Keymap) WithAction(cmd string) Keymap {
	k.Action = fmt.Sprintf("execute(%s)", cmd)
	return k
}

// Keymaps holds the keymaps for FZF.
type Keymaps struct {
	Edit      Keymap `yaml:"edit"`
	EditNotes Keymap `yaml:"notes"`
	Open      Keymap `yaml:"open"`
	Preview   Keymap `yaml:"preview"`
	QR        Keymap `yaml:"qr"`
	OpenQR    Keymap `yaml:"open_qr"`
	ToggleAll Keymap `yaml:"toggle_all"`
	Yank      Keymap `yaml:"yank"`
}
