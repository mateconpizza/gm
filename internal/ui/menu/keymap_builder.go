package menu

import (
	"fmt"
	"strings"
)

// BuiltinAction represents a native FZF action.
type BuiltinAction int

const (
	ToggleAll BuiltinAction = iota
	TogglePreview
)

func (b BuiltinAction) String() string {
	switch b {
	case ToggleAll:
		return "toggle-all"
	case TogglePreview:
		return "toggle-preview"
	default:
		panic("menu: invalid builtin action")
	}
}

// Builder constructs CLI-backed keybinds for a specific command and database.
type Builder struct {
	cmd         string
	dbName      string
	placeholder string
}

// NewBindBuilder creates a new keybind builder for the given command and
// database.
func NewBindBuilder(cmd, dbName string) *Builder {
	return &Builder{cmd: cmd, dbName: dbName}
}

// WithPlaceholder sets the default FZF placeholder (e.g. "{+1}", "{+2}").
func (b *Builder) WithPlaceholder(p string) *Builder {
	b.placeholder = p
	return b
}

// cmd builds the full CLI command string.
func (b *Builder) baseCmd(action string) string {
	return fmt.Sprintf("%s --db=%s %s", b.cmd, b.dbName, action)
}

// From clones a Keymap from user config and prepares it for modification.
func (b *Builder) From(k *Keymap) *KeymapConfig {
	clone := *k
	return &KeymapConfig{base: &clone, builder: b}
}

// New creates a new Keymap with the given bind and description.
func (b *Builder) New(keybind bind, desc string) *KeymapConfig {
	return &KeymapConfig{
		base:    &Keymap{Bind: keybind, Desc: desc, Enabled: true},
		builder: b,
	}
}

// Builtin creates a Keymap using a native FZF action (no CLI command).
func (b *Builder) Builtin(k *Keymap, a BuiltinAction) *Keymap {
	clone := *k
	clone.Action = action(a.String())
	return &clone
}

// KeymapConfig builds a single Keymap with a resolved action and placeholder.
type KeymapConfig struct {
	base        *Keymap
	builder     *Builder
	placeholder string
}

// WithPlaceholder overrides the builder-level placeholder for this keymap
// only.
func (kc *KeymapConfig) WithPlaceholder(p string) *KeymapConfig {
	kc.placeholder = p
	return kc
}

// Desc overrides the keymap description.
func (kc *KeymapConfig) Desc(d string) *KeymapConfig {
	kc.base.Desc = d
	return kc
}

// Execute sets an execute action with the given CLI subcommand.
func (kc *KeymapConfig) Execute(action string) *Keymap {
	kc.base.WithAction(kc.builder.baseCmd(kc.applyPlaceholder(action)))
	return kc.base
}

// ExecuteSilent sets an execute-silent action with the given CLI subcommand.
func (kc *KeymapConfig) ExecuteSilent(action string) *Keymap {
	kc.base.WithSilentAction(kc.builder.baseCmd(kc.applyPlaceholder(action)))
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
