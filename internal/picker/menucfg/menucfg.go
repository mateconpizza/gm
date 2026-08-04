package menucfg

import (
	"errors"
	"log/slog"

	menu "github.com/mateconpizza/go-fzf"
)

var ErrInvalidConfigKeymap = errors.New("invalid keymap")

const (
	defaultFormatter = "oneline"
	defaultPrompt    = "\u25B6 "  // ▶
	defaultHeaderSep = " \u00b7 " // ·
)

// Config holds the menu configuration.
type Config struct {
	// Use $FZF_DEFAULT_OPTS_FILE n $FZF_DEFAULT_OPTS
	Defaults bool `json:"defaults" yaml:"defaults"`

	// Fzf items format
	Format string `json:"format" yaml:"format"`

	// Fzf prompt
	Prompt string `json:"prompt" yaml:"prompt"`

	// Fzf enable preview
	Preview bool `json:"preview" yaml:"preview"`

	// Fzf header
	Header Header `json:"header" yaml:"header"`

	// Fzf keymaps
	DefaultKeymaps *Keymaps `json:"keymaps" yaml:"keymaps"`

	// Fzf arguments
	Arguments menu.Args `json:"arguments,omitempty" yaml:"arguments,omitempty"`
}

// Header holds the header configuration for FZF.
type Header struct {
	Enabled bool   `yaml:"enabled"`
	Sep     string `yaml:"separator"`
}

func NewDefault() *Config {
	return &Config{
		Defaults: true,
		Prompt:   defaultPrompt,
		Preview:  true,
		Format:   defaultFormatter,
		Header: Header{
			Enabled: true,
			Sep:     defaultHeaderSep,
		},
		DefaultKeymaps: &Keymaps{
			Edit:      menu.NewKeymap().WithBind(menu.KeyCtrlE).WithDesc("edit"),
			EditNotes: menu.NewKeymap().WithBind(menu.KeyCtrlW).WithDesc("edit-notes"),
			Open:      menu.NewKeymap().WithBind(menu.KeyEnter).WithDesc("open"),
			OpenQR:    menu.NewKeymap().WithBind(menu.KeyCtrlL).WithDesc("open-qr"),
			QR:        menu.NewKeymap().WithBind(menu.KeyCtrlK).WithDesc("qr-code"),
			Yank:      menu.NewKeymap().WithBind(menu.KeyCtrlY).WithDesc("yank"),
			ToggleAll: menu.NewKeymap().WithBind(menu.KeyCtrlA).WithDesc("toggle-all"),
			Preview:   menu.NewKeymap().WithBind(menu.KeyCtrlSlash).WithDesc("toggle-preview"),
		},
	}
}

func (c *Config) Keymaps() *Keymaps {
	return c.DefaultKeymaps
}

func (c *Config) KeymapsList() []*menu.Keymap {
	k := c.Keymaps()
	return []*menu.Keymap{
		k.Edit,
		k.EditNotes,
		k.Open,
		k.QR,
		k.OpenQR,
		k.Yank,
		k.Preview,
		k.ToggleAll,
	}
}

// Validate validates the menu configuration.
func (c *Config) Validate() error {
	if err := c.Keymaps().Validate(); err != nil {
		return err
	}

	// set default prompt
	if c.Prompt == "" {
		slog.Debug("empty prompt, loading default prompt")
		c.Prompt = defaultPrompt
	}

	// set default header separator
	if c.Header.Sep == "" {
		slog.Debug("empty header separator, loading default header separator")
		c.Header.Sep = defaultHeaderSep
	}

	// set default settings
	if len(c.Arguments) == 0 {
		slog.Warn("empty settings, loading default settings")
	}

	return nil
}

func (c *Config) LoadKeymaps(kb *KeymapBuilder) []*menu.Keymap {
	k := c.Keymaps()
	k.Edit = kb.From(k.Edit).WithExecute("edit")
	k.EditNotes = kb.From(k.EditNotes).WithExecute("notes edit")
	k.Open = kb.From(k.Open).WithExecute("open")
	k.QR = kb.From(k.QR).WithExecute("qr")
	k.OpenQR = kb.From(k.OpenQR).WithExecute("qr open")
	k.Yank = kb.From(k.Yank).WithExecute("yank")
	k.ToggleAll = kb.Builtin(k.ToggleAll, menu.KeybindActionToggleAll)
	k.Preview = kb.Builtin(k.Preview, menu.KeybindActionTogglePreview)

	return c.KeymapsList()
}
