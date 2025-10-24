package menu

import "fmt"

type keyManager struct {
	keymaps []*Keymap
}

func (km *keyManager) register(k ...*Keymap) {
	km.keymaps = append(km.keymaps, k...)
}

func (km *keyManager) len() int {
	return len(km.keymaps)
}

func (km *keyManager) list() []*Keymap {
	return km.keymaps
}

// Keymap holds the keymap configuration.
type Keymap struct {
	Bind    string   `json:"bind"           yaml:"bind"`           // keybind combination
	Desc    string   `json:"description"    yaml:"description"`    // keybind description
	Action  string   `json:"-"              yaml:"-"`              // action to execute
	Enabled bool     `json:"enabled"        yaml:"enabled"`        // keybind enabled
	Hidden  bool     `json:"hidden"         yaml:"hidden"`         // keybind hidden
	Args    []string `json:"args,omitempty" yaml:"args,omitempty"` // keybind arguments
}

// WithAction returns a new Keymap with the given action command.
func (k *Keymap) WithAction(cmd string) *Keymap {
	k.Action = fmt.Sprintf("execute(%s)", cmd)
	return k
}

// Keymaps holds the keymaps for FZF.
type Keymaps struct {
	Edit      *Keymap `json:"edit"       yaml:"edit"`
	EditNotes *Keymap `json:"notes"      yaml:"notes"`
	Open      *Keymap `json:"open"       yaml:"open"`
	Preview   *Keymap `json:"preview"    yaml:"preview"`
	QR        *Keymap `json:"qr"         yaml:"qr"`
	OpenQR    *Keymap `json:"open_qr"    yaml:"open_qr"`
	ToggleAll *Keymap `json:"toggle_all" yaml:"toggle_all"`
	Yank      *Keymap `json:"yank"       yaml:"yank"`
}
