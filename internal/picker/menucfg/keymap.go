package menucfg

import (
	"fmt"
	"strings"

	menu "github.com/mateconpizza/go-fzf"
)

// Keymaps holds the keymaps for FZF.
type Keymaps struct {
	Edit      *menu.Keymap `json:"edit"       yaml:"edit"`
	EditNotes *menu.Keymap `json:"notes"      yaml:"notes"`
	Open      *menu.Keymap `json:"open"       yaml:"open"`
	Preview   *menu.Keymap `json:"preview"    yaml:"preview"`
	QR        *menu.Keymap `json:"qr"         yaml:"qr"`
	OpenQR    *menu.Keymap `json:"open_qr"    yaml:"open_qr"`
	ToggleAll *menu.Keymap `json:"toggle_all" yaml:"toggle_all"`
	Yank      *menu.Keymap `json:"yank"       yaml:"yank"`
}

func (k *Keymaps) Validate() error {
	check := func(name string, km *menu.Keymap) error {
		if km == nil || !km.IsEnabled() {
			return nil
		}
		if km.Bind == "" {
			return fmt.Errorf("%w: keymap %q: missing bind", ErrInvalidConfigKeymap, name)
		}
		return nil
	}

	for _, entry := range []struct {
		name string
		km   *menu.Keymap
	}{
		{"edit", k.Edit},
		{"notes", k.EditNotes},
		{"open", k.Open},
		{"preview", k.Preview},
		{"qr", k.QR},
		{"open_qr", k.OpenQR},
		{"toggle_all", k.ToggleAll},
		{"toggle-preview", k.ToggleAll},
		{"yank", k.Yank},
	} {
		if err := check(entry.name, entry.km); err != nil {
			return err
		}
	}

	return nil
}

// KeymapBuilder constructs CLI-backed keybinds for a specific command and database.
type KeymapBuilder struct {
	cmd         string
	dbName      string
	placeholder string
}

// NewBindBuilder creates a new keybind builder for the given command and
// database.
func NewBindBuilder() *KeymapBuilder {
	return &KeymapBuilder{}
}

// WithPlaceholder sets the default FZF placeholder (e.g. "{+1}", "{+2}").
func (bb *KeymapBuilder) WithPlaceholder(p string) *KeymapBuilder {
	bb.placeholder = p
	return bb
}

// WithCommand sets the CLI command to be executed.
func (bb *KeymapBuilder) WithCommand(cmd string) *KeymapBuilder {
	bb.cmd = cmd
	return bb
}

// WithDBName sets the database name for the command.
func (bb *KeymapBuilder) WithDBName(dbName string) *KeymapBuilder {
	bb.dbName = dbName
	return bb
}

// From clones a Keymap from user config and prepares it for modification.
func (bb *KeymapBuilder) From(k *menu.Keymap) *KeymapConfig {
	clone := *k
	return &KeymapConfig{base: &clone, builder: bb}
}

// New creates a new Keymap with the given bind and description.
func (bb *KeymapBuilder) New(keybind menu.Keybind, desc string) *KeymapConfig {
	return &KeymapConfig{
		base:    &menu.Keymap{Bind: keybind, Desc: desc, Enabled: true},
		builder: bb,
	}
}

// NewNew creates a new Keymap with the given bind and description.
func (bb *KeymapBuilder) NewNew() *KeymapConfig {
	return &KeymapConfig{
		base:    menu.NewKeymap(),
		builder: bb,
	}
}

func (bb *KeymapBuilder) NewKeymap() *menu.Keymap {
	return menu.NewKeymap()
}

// Builtin creates a Keymap using a native FZF action (no CLI command).
func (bb *KeymapBuilder) Builtin(k *menu.Keymap, a menu.KeybindAction) *menu.Keymap {
	clone := *k
	clone.Action = a
	return &clone
}

// cmd builds the full CLI command string.
func (bb *KeymapBuilder) baseCmd(action string) string {
	return fmt.Sprintf("%s --db=%s %s", bb.cmd, bb.dbName, action)
}

// KeymapConfig builds a single Keymap.
type KeymapConfig struct {
	base        *menu.Keymap
	builder     *KeymapBuilder
	placeholder string
}

// WithPlaceholder overrides the builder-level placeholder for this keymap
// only.
func (kc *KeymapConfig) WithPlaceholder(p string) *KeymapConfig {
	kc.placeholder = p
	return kc
}

// WithDesc overrides the keymap description.
func (kc *KeymapConfig) WithDesc(d string) *KeymapConfig {
	kc.base.Desc = d
	return kc
}

func (kc *KeymapConfig) WithBind(bind menu.Keybind) *KeymapConfig {
	kc.base.Bind = bind
	return kc
}

// WithExecute sets an execute action with the given CLI subcommand.
func (kc *KeymapConfig) WithExecute(action string) *menu.Keymap {
	kc.base.WithExecute(kc.builder.baseCmd(kc.applyPlaceholder(action)))
	return kc.base
}

// WithExecuteSilent sets an execute-silent action with the given CLI subcommand.
func (kc *KeymapConfig) WithExecuteSilent(action string) *menu.Keymap {
	kc.base.WithSilentExecute(kc.builder.baseCmd(kc.applyPlaceholder(action)))
	return kc.base
}

func (kc *KeymapConfig) WithBecome(cmd string) *menu.Keymap {
	kc.base.WithBecome(kc.builder.baseCmd(cmd))
	return kc.base
}

func (kc *KeymapConfig) resolvePlaceholder() string {
	if kc.placeholder != "" {
		return kc.placeholder
	}
	return kc.builder.placeholder
}

func (kc *KeymapConfig) applyPlaceholder(action string) string {
	p := kc.resolvePlaceholder()
	if p == "" {
		return action
	}
	if strings.Contains(action, "{+1}") {
		return strings.ReplaceAll(action, "{+1}", p)
	}
	return action + " " + p
}
