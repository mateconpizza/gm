package menu

import (
	"errors"
	"testing"
)

func testValidConfig(t *testing.T) *Config {
	t.Helper()
	return &Config{
		Prompt:  defaultPrompt,
		Preview: false,
		Header: Header{
			Enabled: false,
			Sep:     " ",
		},
		DefaultKeymaps: &BuiltinKeymaps{
			Edit:      &Keymap{Bind: KeyCtrlE, Desc: "edit", Enabled: true, Hidden: false},
			Open:      &Keymap{Bind: KeyCtrlO, Desc: "open", Enabled: true, Hidden: false},
			QR:        &Keymap{Bind: KeyCtrlK, Desc: "QRcode", Enabled: true, Hidden: false},
			OpenQR:    &Keymap{Bind: KeyCtrlL, Desc: "openQR", Enabled: true, Hidden: false},
			Yank:      &Keymap{Bind: KeyCtrlY, Desc: "yank", Enabled: true, Hidden: false},
			Preview:   &Keymap{Bind: KeyCtrlSlash, Desc: "toggle-preview", Enabled: true, Hidden: false},
			ToggleAll: &Keymap{Bind: KeyCtrlA, Desc: "toggle-all", Enabled: true, Hidden: false},
		},
		Arguments: newArgsBuilder().withAnsi().
			withLayout("default").
			withTac().
			withHeight("95%").build(),
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
		cfg.DefaultKeymaps.Edit.Bind = ""
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
