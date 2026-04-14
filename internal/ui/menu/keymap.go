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

// KeybindBuilder builds command keybindings for menu keymaps.
type KeybindBuilder struct {
	cmd    string // executable name
	dbName string
}

// NewKeybindBuilder creates a new keybind builder.
func NewKeybindBuilder(cmd, dbName string) *KeybindBuilder {
	return &KeybindBuilder{
		cmd:    cmd,
		dbName: dbName,
	}
}

func (kb *KeybindBuilder) NewKeymap(action string) *Keymap {
	k := NewKeymap()
	k.Enabled = true
	k.WithAction(kb.BaseCmd(action + " {+1}"))
	return k
}

func (kb *KeybindBuilder) BaseCmd(s string) string {
	return fmt.Sprintf("%s --db=%s %s", kb.cmd, kb.dbName, s)
}

// Edit returns a keybind to edit the selected record.
func (kb *KeybindBuilder) Edit(km *Keymap, args ...string) *Keymap {
	cmd := "edit "
	if len(args) > 0 {
		cmd += strings.Join(args, " ") + " "
	}
	cmd += "{+1}"

	return km.WithAction(kb.BaseCmd(cmd))
}

// EditNotes returns a keybind to edit notes of the selected record.
func (kb *KeybindBuilder) EditNotes(km *Keymap) *Keymap {
	return km.WithAction(kb.BaseCmd("notes edit {+1}"))
}

func (kb *KeybindBuilder) EditJSON(km *Keymap) *Keymap {
	return km.WithAction(kb.BaseCmd("edit --format json {+1}"))
}

// Open returns a keybind to open the selected record in the default browser.
func (kb *KeybindBuilder) Open(km *Keymap) *Keymap {
	return km.WithSilentAction(kb.BaseCmd("open {+1}"))
}

// QR returns a keybind to show the QR code of the selected record.
func (kb *KeybindBuilder) QR(km *Keymap) *Keymap { return km.WithAction(kb.BaseCmd("qr {+1}")) }

// QROpen returns a keybind to open the QR code in the default image viewer.
func (kb *KeybindBuilder) QROpen(km *Keymap) *Keymap {
	return km.WithSilentAction(kb.BaseCmd("qr --open {+1}"))
}

// Yank returns a keybind to copy the selected record to the clipboard.
func (kb *KeybindBuilder) Yank(km *Keymap) *Keymap {
	return km.WithSilentAction(kb.BaseCmd("copy {+1}"))
}

func (kb *KeybindBuilder) ToggleAll(km *Keymap) *Keymap {
	km.Action = "toggle-all"
	return km
}

func (kb *KeybindBuilder) Preview(km *Keymap) *Keymap {
	km.Action = "toggle-preview"
	return km
}

func NewKeymap() *Keymap {
	return &Keymap{}
}
