package menu

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func testValidConfig(t *testing.T) *FzfConfig {
	t.Helper()

	return &FzfConfig{
		Prompt:  "> ",
		Preview: false,
		Header: FzfHeader{
			Enabled: false,
			Sep:     " ",
		},
		Keymaps: Keymaps{
			Edit:      Keymap{Bind: "ctrl-e", Desc: "edit", Enabled: true, Hidden: false},
			Open:      Keymap{Bind: "ctrl-o", Desc: "open", Enabled: true, Hidden: false},
			QR:        Keymap{Bind: "ctrl-k", Desc: "QRcode", Enabled: true, Hidden: false},
			OpenQR:    Keymap{Bind: "ctrl-l", Desc: "openQR", Enabled: true, Hidden: false},
			Yank:      Keymap{Bind: "ctrl-y", Desc: "yank", Enabled: true, Hidden: false},
			Preview:   Keymap{Bind: "ctrl-/", Desc: "toggle-preview", Enabled: true, Hidden: false},
			ToggleAll: Keymap{Bind: "ctrl-a", Desc: "toggle-all", Enabled: true, Hidden: false},
		},
		Settings: []string{"--ansi", "--reverse", "--tac", "--height=95%"},
	}
}

func TestValidateConfig(t *testing.T) {
	cfg := testValidConfig(t)
	// invalid keybind
	cfg.Keymaps.Edit.Bind = ""
	assert.Error(t, ValidateConfig(cfg))
	assert.ErrorIs(t, ValidateConfig(cfg), ErrInvalidConfigKeymap)
	// invalid empty settings
	cfg.Keymaps.Edit.Bind = "ctrl-e"
	cfg.Settings = []string{}
	assert.Error(t, ValidateConfig(cfg))
	assert.ErrorIs(t, ValidateConfig(cfg), ErrInvalidConfigSettings)
}
