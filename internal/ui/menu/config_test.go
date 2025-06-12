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
	t.Run("valid config", func(t *testing.T) {
		t.Parallel()
		cfg := testValidConfig(t)
		assert.NoError(t, cfg.Validate())
	})
	t.Run("invalid config", func(t *testing.T) {
		t.Parallel()
		cfg := testValidConfig(t)
		cfg.Keymaps.Edit.Bind = ""
		assert.Error(t, cfg.Validate())
		assert.ErrorIs(t, cfg.Validate(), ErrInvalidConfigKeymap)
	})
	t.Run("default prompt and header separator", func(t *testing.T) {
		t.Parallel()
		cfg := testValidConfig(t)
		cfg.Prompt = ""
		cfg.Header.Sep = ""
		assert.Empty(t, cfg.Prompt)
		assert.Empty(t, cfg.Header.Sep)
		assert.NoError(t, cfg.Validate())
		assert.NotEmpty(t, cfg.Prompt)
		assert.NotEmpty(t, cfg.Header.Sep)
	})
}
