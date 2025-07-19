package menu

import (
	"errors"
	"testing"
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
		if err := cfg.Validate(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("invalid config", func(t *testing.T) {
		t.Parallel()
		cfg := testValidConfig(t)
		cfg.Keymaps.Edit.Bind = ""
		err := cfg.Validate()
		if err == nil {
			t.Error("expected error, got nil")
		} else if !errors.Is(err, ErrInvalidConfigKeymap) {
			t.Errorf("expected ErrInvalidConfigKeymap, got %v", err)
		}
	})

	t.Run("default prompt and header separator", func(t *testing.T) {
		t.Parallel()
		cfg := testValidConfig(t)
		cfg.Prompt = ""
		cfg.Header.Sep = ""

		if cfg.Prompt != "" {
			t.Errorf("expected empty prompt before validate, got %q", cfg.Prompt)
		}
		if cfg.Header.Sep != "" {
			t.Errorf("expected empty separator before validate, got %q", cfg.Header.Sep)
		}

		err := cfg.Validate()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if cfg.Prompt == "" {
			t.Error("expected non-empty prompt after validate")
		}
		if cfg.Header.Sep == "" {
			t.Error("expected non-empty header separator after validate")
		}
	})
}
