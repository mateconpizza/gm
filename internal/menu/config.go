package menu

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/haaag/gm/internal/config"
	"github.com/haaag/gm/internal/sys/files"
)

const (
	defaultPrompt     string = " Gomarks> " // Default input prompt
	keymapsConfigFile string = "menu.yml"    // Default keymaps config file
)

var ErrConfigFileExists = errors.New("config file already exists")

var menuConfig Config

type Keymap struct {
	Bind        string `yaml:"bind"`
	Description string `yaml:"description"`
	Enabled     bool   `yaml:"enabled"`
	Hidden      bool   `yaml:"hidden"`
}

type FZFKeymaps struct {
	Edit      Keymap `yaml:"edit"`
	Open      Keymap `yaml:"open"`
	Preview   Keymap `yaml:"preview"`
	QR        Keymap `yaml:"qr"`
	ToggleAll Keymap `yaml:"toggle_all"`
	Yank      Keymap `yaml:"yank"`
}

type Config struct {
	Prompt  string     `yaml:"prompt"`
	Keymaps FZFKeymaps `yaml:"keymaps"`
	Header  bool       `yaml:"header"`
	Preview bool       `yaml:"preview"`
}

var defaultKeymaps = FZFKeymaps{
	// TODO)): Maybe move this to setup.go?
	Edit:      Keymap{Bind: "ctrl-e", Description: "edit", Enabled: true, Hidden: false},
	Open:      Keymap{Bind: "ctrl-o", Description: "open", Enabled: true, Hidden: false},
	Preview:   Keymap{Bind: "ctrl-/", Description: "toggle-preview", Enabled: true, Hidden: false},
	QR:        Keymap{Bind: "ctrl-k", Description: "QRcode", Enabled: true, Hidden: false},
	ToggleAll: Keymap{Bind: "ctrl-a", Description: "toggle-all", Enabled: true, Hidden: false},
	Yank:      Keymap{Bind: "ctrl-y", Description: "yank", Enabled: true, Hidden: false},
}

var defaultMenuConfig = Config{
	Prompt:  defaultPrompt,
	Keymaps: defaultKeymaps,
}

func DumpConfig() error {
	p := filepath.Join(config.App.Path.Data, keymapsConfigFile)

	if files.Exists(p) {
		return fmt.Errorf("%s %w", p, ErrConfigFileExists)
	}

	f, err := files.Touch(p, false)
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

	log.Printf("loading configfile: %s", f)
	menuConfig = fileMenuConfig

	return nil
}
