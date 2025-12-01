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
	defaultPrompt         = unicodePathBigSegment + " "
	defaultHeaderSep      = " " + unicodeMiddleDot + " "
)

// Config holds the menu configuration.
type Config struct {
	Defaults       bool            `json:"defaults"  yaml:"defaults"`  // Use $FZF_DEFAULT_OPTS_FILE n $FZF_DEFAULT_OPTS
	Prompt         string          `json:"prompt"    yaml:"prompt"`    // Fzf prompt
	Preview        bool            `json:"preview"   yaml:"preview"`   // Fzf enable preview
	Header         Header          `json:"header"    yaml:"header"`    // Fzf header
	DefaultKeymaps *BuiltinKeymaps `json:"keymaps"   yaml:"keymaps"`   // Fzf keymaps
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
		c.DefaultKeymaps.Edit,
		c.DefaultKeymaps.Open,
		c.DefaultKeymaps.QR,
		c.DefaultKeymaps.OpenQR,
		c.DefaultKeymaps.Yank,
		c.DefaultKeymaps.Preview,
		c.DefaultKeymaps.ToggleAll,
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
		c.Prompt = defaultPrompt
	}

	// set default header separator
	if c.Header.Sep == "" {
		slog.Warn("empty header separator, loading default header separator")
		c.Header.Sep = defaultHeaderSep
	}

	// set default settings
	if len(c.Arguments) == 0 {
		slog.Warn("empty settings, loading default settings")
	}

	return nil
}

func NewDefaultConfig() *Config {
	return &Config{
		Defaults: true,
		Prompt:   defaultPrompt,
		Preview:  true,
		Header: Header{
			Enabled: true,
			Sep:     defaultHeaderSep,
		},
		DefaultKeymaps: &BuiltinKeymaps{
			Edit:      &Keymap{Bind: "ctrl-e", Desc: "edit", Enabled: true, Hidden: false},
			EditNotes: &Keymap{Bind: "ctrl-w", Desc: "edit notes", Enabled: true, Hidden: false},
			Open:      &Keymap{Bind: "enter", Desc: "open", Enabled: true, Hidden: false},
			OpenQR:    &Keymap{Bind: "ctrl-l", Desc: "openQR", Enabled: true, Hidden: false},
			Preview:   &Keymap{Bind: "ctrl-/", Desc: "toggle-preview", Enabled: true, Hidden: false},
			QR:        &Keymap{Bind: "ctrl-k", Desc: "QRcode", Enabled: true, Hidden: false},
			ToggleAll: &Keymap{Bind: "ctrl-a", Desc: "toggle-all", Enabled: true, Hidden: false},
			Yank:      &Keymap{Bind: "ctrl-y", Desc: "yank", Enabled: true, Hidden: false},
		},
		Arguments: newArgsBuilder().withAnsi().
			withLayout("default").
			withSync().
			withInfo("inline-right").
			withTac().
			withHeight("100%").
			withNoScrollbar().
			withColor("prompt", "bold").
			withColor("header", "italic", "bright-blue").build(),
	}
}

func builtinKeymaps(a *ArgsBuilder, action string) *Keymap {
	binds := map[string]*Keymap{
		"toggle-all": {
			Bind:    "ctrl-a",
			Desc:    "toggle-all",
			Action:  "toggle-all",
			Enabled: true,
			Hidden:  false,
			Args:    Args{a.highlightLine, a.multi},
		},

		"toggle-preview": {
			Bind:    "ctrl-/",
			Desc:    "toggle-preview",
			Action:  "toggle-preview",
			Enabled: true,
			Hidden:  false,
		},
	}

	return binds[action]
}
