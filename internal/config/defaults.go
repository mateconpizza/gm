package config

import (
	"fmt"
	"log/slog"

	"github.com/haaag/gm/internal/menu"
)

// ConfigFile represents the configuration file.
type ConfigFile struct {
	Colorscheme string       `json:"colorscheme" yaml:"colorscheme"` // App colorscheme
	Menu        *menu.Config `json:"menu"        yaml:"menu"`        // Menu configuration
}

// fzfSettings are the options for FZF.
var fzfSettings = menu.FzfSettings{
	// TODO: maybe, put it in `menu.go`
	"--ansi",                            // Enable processing of ANSI color codes
	"--reverse",                         // A synonym for --layout=reverse
	"--sync",                            // Synchronous search for multi-staged filtering
	"--info=inline-right",               // Determines the display style of the finder info.
	"--tac",                             // Reverse the order of the input
	"--layout=default",                  // Choose the layout (default: default)
	"--color=prompt:bold",               // Prompt style
	"--color=header:italic:bright-blue", // Header style
	"--height=100%",                     // Set the height of the menu
	"--marker=\u00b7",                   // Multi-selection marker
	"--no-scrollbar",                    // Remove scrollbar
	"--border-label= GoMarks ",          // Label to print on the horizontal border line
	"--border",                          // Border around the window
}

// Fzf holds the default menu configuration.
var Fzf = &menu.Config{
	// TODO: maybe, put it in `menu.go`
	Defaults: true,
	Prompt:   menu.DefaultPrompt,
	Preview:  true,
	Header: menu.FzfHeader{
		Enabled: true,
		Sep:     menu.DefaultHeaderSep,
	},
	Keymaps: menu.Keymaps{
		Edit:      menu.Keymap{Bind: "ctrl-e", Desc: "edit", Enabled: true, Hidden: false},
		Open:      menu.Keymap{Bind: "ctrl-o", Desc: "open", Enabled: true, Hidden: false},
		QR:        menu.Keymap{Bind: "ctrl-k", Desc: "QRcode", Enabled: true, Hidden: false},
		OpenQR:    menu.Keymap{Bind: "ctrl-l", Desc: "openQR", Enabled: true, Hidden: false},
		Yank:      menu.Keymap{Bind: "ctrl-y", Desc: "yank", Enabled: true, Hidden: false},
		Preview:   menu.Keymap{Bind: "ctrl-/", Desc: "toggle-preview", Enabled: true, Hidden: false},
		ToggleAll: menu.Keymap{Bind: "ctrl-a", Desc: "toggle-all", Enabled: true, Hidden: false},
	},
	Settings: fzfSettings,
}

func fmtKeybindCmd(s string) string {
	return fmt.Sprintf("execute(%s --name=%s records %s", App.Cmd, App.DBName, s)
}

// FzfKeybindEdit keybind to edit the selected record.
func FzfKeybindEdit() menu.Keymap {
	return menu.Keymap{
		Bind:    Fzf.Keymaps.Edit.Bind,
		Desc:    Fzf.Keymaps.Edit.Desc,
		Action:  fmtKeybindCmd("--edit {+1})"),
		Enabled: Fzf.Keymaps.Edit.Enabled,
		Hidden:  Fzf.Keymaps.Edit.Hidden,
	}
}

// FzfKeybindOpen keybind to open the selected record in the default browser.
func FzfKeybindOpen() menu.Keymap {
	return menu.Keymap{
		Bind:    Fzf.Keymaps.Open.Bind,
		Desc:    Fzf.Keymaps.Open.Desc,
		Action:  fmtKeybindCmd("--open {+1})"),
		Enabled: Fzf.Keymaps.Open.Enabled,
		Hidden:  Fzf.Keymaps.Open.Hidden,
	}
}

// FzfKeybindQR keybind to show the QR code of the selected record.
func FzfKeybindQR() menu.Keymap {
	return menu.Keymap{
		Bind:    Fzf.Keymaps.QR.Bind,
		Desc:    Fzf.Keymaps.QR.Desc,
		Action:  fmtKeybindCmd("--qr {+1})"),
		Enabled: Fzf.Keymaps.QR.Enabled,
		Hidden:  Fzf.Keymaps.QR.Hidden,
	}
}

// FzfKeybindOpenQR keybind to open the QR code of the selected record in the
// default image viewer.
func FzfKeybindOpenQR() menu.Keymap {
	return menu.Keymap{
		Bind:    Fzf.Keymaps.OpenQR.Bind,
		Desc:    Fzf.Keymaps.OpenQR.Desc,
		Action:  fmtKeybindCmd("--qr --open {+1})"),
		Enabled: Fzf.Keymaps.OpenQR.Enabled,
		Hidden:  Fzf.Keymaps.OpenQR.Hidden,
	}
}

// FzfKeybindYank keybind to copy the selected record to the system clipboard.
func FzfKeybindYank() menu.Keymap {
	return menu.Keymap{
		Bind:    Fzf.Keymaps.Yank.Bind,
		Desc:    Fzf.Keymaps.Yank.Desc,
		Action:  fmtKeybindCmd("--copy {+1})"),
		Enabled: Fzf.Keymaps.Yank.Enabled,
		Hidden:  Fzf.Keymaps.Yank.Hidden,
	}
}

// App is the default application configuration.
var App = &AppConfig{
	Name:        appName,
	Cmd:         command,
	Version:     version,
	DBName:      DefaultDBName,
	Colorscheme: "default",
	Color:       false,
	Force:       false,
	Info: information{
		URL:   "https://github.com/haaag/gomarks#readme",
		Title: "Gomarks: A bookmark manager",
		Tags:  "golang,awesome,bookmarks,cli",
		Desc:  "Simple yet powerful bookmark manager for your terminal",
	},
	Env: environment{
		Home:   "GOMARKS_HOME",
		Editor: "GOMARKS_EDITOR",
	},
}

// Defaults holds the default configuration.
var Defaults = &ConfigFile{
	Colorscheme: "default",
	Menu:        Fzf,
}

// Validate validates the configuration file.
func Validate(cfg *ConfigFile) error {
	if cfg.Colorscheme == "" {
		slog.Warn("empty colorscheme, loading default colorscheme")
		cfg.Colorscheme = "default"
	}

	if err := cfg.Menu.Validate(); err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}
