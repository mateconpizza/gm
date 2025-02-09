package menu

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"

	yaml "gopkg.in/yaml.v3"

	"github.com/haaag/gm/internal/config"
	"github.com/haaag/gm/internal/format/color"
	"github.com/haaag/gm/internal/sys/files"
)

const (
	defaultPrompt     string = " " + config.AppName + "> " // Default input prompt
	keymapsConfigFile string = "menu.yml"                   // Default keymaps config file
)

var ErrConfigFileExists = errors.New("config file already exists")

// menuConfig holds the menu configuration.
var menuConfig Config = defaultMenuConfig

// Keymap holds the keymap configuration.
type Keymap struct {
	Bind        string `yaml:"bind"`
	Description string `yaml:"description"`
	Enabled     bool   `yaml:"enabled"`
	Hidden      bool   `yaml:"hidden"`
}

// NewKeymap creates a new keymap.
func NewKeymap(bind, description string, enabled, hidden bool) *Keymap {
	return &Keymap{
		Bind:        bind,
		Description: description,
		Enabled:     enabled,
		Hidden:      hidden,
	}
}

// FZFKeymaps holds the keymaps for FZF.
type FZFKeymaps struct {
	Edit      Keymap `yaml:"edit"`
	Open      Keymap `yaml:"open"`
	Preview   Keymap `yaml:"preview"`
	QR        Keymap `yaml:"qr"`
	OpenQR    Keymap `yaml:"open_qr"`
	ToggleAll Keymap `yaml:"toggle_all"`
	Yank      Keymap `yaml:"yank"`
}

// Config holds the menu configuration.
type Config struct {
	Prompt  string     `yaml:"prompt"`
	Header  bool       `yaml:"header"`
	Preview bool       `yaml:"preview"`
	Keymaps FZFKeymaps `yaml:"keymaps"`
}

// DefaultKeymaps holds the default keymaps.
var defaultKeymaps = FZFKeymaps{
	Edit:      Keymap{Bind: "ctrl-e", Description: "edit", Enabled: true, Hidden: false},
	Open:      Keymap{Bind: "ctrl-o", Description: "open", Enabled: true, Hidden: false},
	Preview:   Keymap{Bind: "ctrl-/", Description: "toggle-preview", Enabled: true, Hidden: false},
	QR:        Keymap{Bind: "ctrl-k", Description: "QRcode", Enabled: true, Hidden: false},
	OpenQR:    Keymap{Bind: "ctrl-l", Description: "openQR", Enabled: true, Hidden: false},
	ToggleAll: Keymap{Bind: "ctrl-a", Description: "toggle-all", Enabled: true, Hidden: false},
	Yank:      Keymap{Bind: "ctrl-y", Description: "yank", Enabled: true, Hidden: false},
}

// defaultMenuConfig holds the default menu configuration.
var defaultMenuConfig = Config{
	Prompt:  defaultPrompt,
	Header:  true,
	Preview: true,
	Keymaps: defaultKeymaps,
}

// DumpConfig dumps the menu configuration to a YAML file.
func DumpConfig(force bool) error {
	p := filepath.Join(config.App.Path.Data, keymapsConfigFile)

	if files.Exists(p) && !force {
		f := color.BrightYellow("--force").Italic().String()
		return fmt.Errorf("%s %w. use '%s' to overwrite", p, ErrConfigFileExists, f)
	}

	f, err := files.Touch(p, force)
	if err != nil {
		return fmt.Errorf("error creating file: %w", err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			log.Printf("error closing %s file: %v", p, err)
		}
	}()

	// Marshal the menu config
	data, err := yaml.Marshal(&defaultMenuConfig)
	if err != nil {
		return fmt.Errorf("error marshalling YAML: %w", err)
	}

	// Write YAML data
	_, err = f.Write(data)
	if err != nil {
		return fmt.Errorf("error writing to file: %w", err)
	}

	fmt.Println(("menu configfile path: '" + p + "'"))

	return nil
}

func LoadConfig() error {
	f := filepath.Join(config.App.Path.Data, keymapsConfigFile)
	if !files.Exists(f) {
		log.Println("menu configfile not found. loading defaults")
		menuConfig = defaultMenuConfig
		return nil
	}

	content, err := os.ReadFile(f)
	if err != nil {
		return fmt.Errorf("error reading config file: %w", err)
	}

	var fileMenuConfig Config
	err = yaml.Unmarshal(content, &fileMenuConfig)
	if err != nil {
		return fmt.Errorf("error unmarshalling YAML: %w", err)
	}

	log.Printf("loading menu configfile: %s", f)
	menuConfig = fileMenuConfig

	return nil
}
