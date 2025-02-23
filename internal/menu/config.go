package menu

import (
	"fmt"
	"log"

	"github.com/haaag/gm/internal/config"
	"github.com/haaag/gm/internal/format"
	"github.com/haaag/gm/internal/sys/files"
)

// menuConfig holds the menu configuration.
var menuConfig *Config = defaultMenuConfig

// fzfDefaults are the default options for FZF.
var fzfDefaults = []string{
	"--ansi",                // Enable processing of ANSI color codes
	"--cycle",               // Enable cyclic scroll
	"--reverse",             // A synonym for --layout=reverse
	"--sync",                // Synchronous search for multi-staged filtering
	"--info=inline-right",   // Determines the display style of the finder info.
	"--tac",                 // Reverse the order of the input
	"--layout=default",      // Choose the layout (default: default)
	"--color=header:italic", // Header style
}

// Config holds the menu configuration.
type Config struct {
	Prompt  string     `yaml:"prompt"`  // Fzf prompt
	Preview bool       `yaml:"preview"` // Fzf enable preview
	Header  fzfHeader  `yaml:"header"`  // Fzf header
	Keymaps fzfKeymaps `yaml:"keymaps"` // Fzf keymaps
}

// fzfHeader holds the header configuration for FZF.
type fzfHeader struct {
	Enabled   bool   `yaml:"enabled"`
	Separator string `yaml:"separator"`
}

// Keymap holds the keymap configuration.
type Keymap struct {
	Bind    string `yaml:"bind"`
	Desc    string `yaml:"description"`
	Enabled bool   `yaml:"enabled"`
	Hidden  bool   `yaml:"hidden"`
}

// fzfKeymaps holds the keymaps for FZF.
type fzfKeymaps struct {
	Edit      Keymap `yaml:"edit"`
	Open      Keymap `yaml:"open"`
	Preview   Keymap `yaml:"preview"`
	QR        Keymap `yaml:"qr"`
	OpenQR    Keymap `yaml:"open_qr"`
	ToggleAll Keymap `yaml:"toggle_all"`
	Yank      Keymap `yaml:"yank"`
}

// NewKeymap creates a new keymap.
func NewKeymap(bind, description string) *Keymap {
	return &Keymap{
		Bind:    bind,
		Desc:    description,
		Enabled: true,
		Hidden:  false,
	}
}

// defaultMenuConfig holds the default menu configuration.
var defaultMenuConfig = &Config{
	Prompt:  config.App.Name + "> ",
	Preview: true,
	Header: fzfHeader{
		Enabled:   true,
		Separator: " " + format.UnicodeMidBulletPoint + " ",
	},
	Keymaps: fzfKeymaps{
		Edit:      Keymap{Bind: "ctrl-e", Desc: "edit", Enabled: true, Hidden: false},
		Open:      Keymap{Bind: "ctrl-o", Desc: "open", Enabled: true, Hidden: false},
		Preview:   Keymap{Bind: "ctrl-/", Desc: "toggle-preview", Enabled: true, Hidden: false},
		QR:        Keymap{Bind: "ctrl-k", Desc: "QRcode", Enabled: true, Hidden: false},
		OpenQR:    Keymap{Bind: "ctrl-l", Desc: "openQR", Enabled: true, Hidden: false},
		ToggleAll: Keymap{Bind: "ctrl-a", Desc: "toggle-all", Enabled: true, Hidden: false},
		Yank:      Keymap{Bind: "ctrl-y", Desc: "yank", Enabled: true, Hidden: false},
	},
}

// DumpConfig dumps the menu configuration to a YAML file.
func DumpConfig() error {
	if err := files.WriteYamlFile(config.App.Path.ConfigFile, &defaultMenuConfig); err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}

// LoadConfig loads the defaults if the config YAML file does not exist.
func LoadConfig() error {
	p := config.App.Path.ConfigFile
	if !files.Exists(p) {
		log.Println("menu configfile not found. loading defaults")
		menuConfig = defaultMenuConfig
		return nil
	}
	if err := files.ReadYamlFile(p, &menuConfig); err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}
