package menu

import (
	"fmt"
	"log/slog"
	"sort"
	"strings"
)

type (
	action string // (e.g., "toggle-preview", "toggle-all")
	bind   string // (e.g., "ctrl-a", "ctrl-e", etc...)
)

// Keymap holds the keymap configuration.
type Keymap struct {
	Bind    bind   `json:"bind"           yaml:"bind"`           // keybind combination
	Action  action `json:"-"              yaml:"-"`              // action to execute
	Desc    string `json:"description"    yaml:"description"`    // keybind description
	Enabled bool   `json:"enabled"        yaml:"enabled"`        // keybind enabled
	Hidden  bool   `json:"hidden"         yaml:"hidden"`         // keybind hidden
	Args    Args   `json:"args,omitempty" yaml:"args,omitempty"` // keybind arguments
}

// WithAction returns a new Keymap with the given action command.
func (k *Keymap) WithAction(cmd string) *Keymap {
	k.Action = action(fmt.Sprintf("execute(%s)", cmd))
	return k
}

// WithSilentAction returns a new Keymap with the given action command.
func (k *Keymap) WithSilentAction(cmd string) *Keymap {
	k.Action = action(fmt.Sprintf("execute-silent(%s)", cmd))
	return k
}

// BuiltinKeymaps holds the keymaps for FZF.
type BuiltinKeymaps struct {
	Edit      *Keymap `json:"edit"       yaml:"edit"`
	EditNotes *Keymap `json:"notes"      yaml:"notes"`
	Open      *Keymap `json:"open"       yaml:"open"`
	Preview   *Keymap `json:"preview"    yaml:"preview"`
	QR        *Keymap `json:"qr"         yaml:"qr"`
	OpenQR    *Keymap `json:"open_qr"    yaml:"open_qr"`
	ToggleAll *Keymap `json:"toggle_all" yaml:"toggle_all"`
	Yank      *Keymap `json:"yank"       yaml:"yank"`
}

type keyManager struct {
	keymaps map[action]*Keymap // keymaps action:keymap
}

func (km *keyManager) register(keys ...*Keymap) {
	for i := range keys {
		k := keys[i]
		if key, exists := km.keymaps[k.Action]; exists &&
			strings.EqualFold(string(k.Bind), string(key.Bind)) {
			slog.Warn("keybind conflict, replacing bind-1",
				"bind-1", k.Bind, "action-1", k.Action,
				"bind-2", key.Bind, "action-2", key.Action)
			km.remove(k)
		}

		km.keymaps[k.Action] = k
		slog.Debug("keybind register", "bind", k.Bind, "action", k.Action)
	}
}

// remove removes bind from keymaps map.
func (km *keyManager) remove(k *Keymap) {
	slog.Debug("keybind remove", "bind", k.Bind, "action", k.Action)
	delete(km.keymaps, k.Action)
}

func (km *keyManager) len() int { return len(km.keymaps) }

// list returns a sorted keymap slice.
func (km *keyManager) list() []*Keymap {
	// extract keys and sort them
	keys := make([]string, 0, len(km.keymaps))
	for k := range km.keymaps {
		keys = append(keys, string(k))
	}
	sort.Strings(keys)

	// build sorted result
	result := make([]*Keymap, 0, len(keys))
	for _, k := range keys {
		result = append(result, km.keymaps[action(k)])
	}

	return result
}

// find returns the first Keymap that matches the given action or bind.
// It returns nil if no keymap is found.
func (km *keyManager) find(bind *Keymap) *Keymap {
	for _, k := range km.keymaps {
		if strings.EqualFold(string(k.Action), string(bind.Action)) ||
			strings.EqualFold(string(k.Bind), string(bind.Bind)) {
			return k
		}
	}
	return nil
}

func newKeyManager() *keyManager {
	return &keyManager{keymaps: make(map[action]*Keymap)}
}

func NewKeymap() *Keymap {
	return &Keymap{Enabled: true}
}
