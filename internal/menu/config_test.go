package menu

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func testValidConfig(t *testing.T) *Config {
	t.Helper()

	return &Config{
		Prompt:  "> ",
		Preview: false,
		Header: FzfHeader{
			Enabled: false,
			Sep:     " ",
		},
		Keymaps: Keymaps{
			Edit:   Keymap{Bind: "ctrl-e", Desc: "edit", Enabled: true, Hidden: false},
			Open:   Keymap{Bind: "ctrl-o", Desc: "open", Enabled: true, Hidden: false},
			QR:     Keymap{Bind: "ctrl-k", Desc: "QRcode", Enabled: true, Hidden: false},
			OpenQR: Keymap{Bind: "ctrl-l", Desc: "openQR", Enabled: true, Hidden: false},
			Yank:   Keymap{Bind: "ctrl-y", Desc: "yank", Enabled: true, Hidden: false},
			Preview: Keymap{
				Bind:    "ctrl-/",
				Desc:    "toggle-preview",
				Enabled: true,
				Hidden:  false,
			},
			ToggleAll: Keymap{
				Bind:    "ctrl-a",
				Desc:    "toggle-all",
				Enabled: true,
				Hidden:  false,
			},
		},
		Settings: []string{"--ansi", "--reverse", "--tac", "--height=95%"},
	}
}

func TestValidateConfig(t *testing.T) {
	t.Parallel()
	cfg := testValidConfig(t)
	cfgKeys := *cfg
	// invalid keybind
	cfgKeys.Keymaps.Edit.Bind = ""
	assert.Error(t, cfgKeys.Validate())
	assert.ErrorIs(t, cfgKeys.Validate(), ErrInvalidConfigKeymap)
	// default prompt and header separator
	cfgStr := *cfg
	cfgStr.Prompt = ""
	cfgStr.Header.Sep = ""
	assert.Empty(t, cfgStr.Prompt)
	assert.Empty(t, cfgStr.Header.Sep)
	assert.NoError(t, cfgStr.Validate())
	assert.NotEmpty(t, cfgStr.Prompt)
	assert.NotEmpty(t, cfgStr.Header.Sep)
}
