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
	Defaults       bool      `json:"defaults"  yaml:"defaults"`  // Use $FZF_DEFAULT_OPTS_FILE n $FZF_DEFAULT_OPTS
	Format         string    `json:"format"    yaml:"format"`    // Fzf items format
	Prompt         string    `json:"prompt"    yaml:"prompt"`    // Fzf prompt
	Preview        bool      `json:"preview"   yaml:"preview"`   // Fzf enable preview
	Header         Header    `json:"header"    yaml:"header"`    // Fzf header
	DefaultKeymaps *Keymaps  `json:"keymaps"   yaml:"keymaps"`   // Fzf keymaps
	Arguments      menu.Args `json:"arguments" yaml:"arguments"` // Fzf arguments
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

// Header holds the header configuration for FZF.
type Header struct {
	Enabled bool   `yaml:"enabled"`
	Sep     string `yaml:"separator"`
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
