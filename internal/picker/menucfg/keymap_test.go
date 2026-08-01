package menucfg

import (
	"errors"
	"testing"

	menu "github.com/mateconpizza/go-fzf"
)

func TestBuiltinKeymaps_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		k       *Keymaps
		wantErr error
	}{
		{
			name: "all_valid_and_enabled",
			k: &Keymaps{
				Edit:      &menu.Keymap{Enabled: true, Bind: "e"},
				EditNotes: &menu.Keymap{Enabled: true, Bind: "n"},
				Open:      &menu.Keymap{Enabled: true, Bind: "o"},
				Preview:   &menu.Keymap{Enabled: true, Bind: "p"},
				QR:        &menu.Keymap{Enabled: true, Bind: "q"},
				OpenQR:    &menu.Keymap{Enabled: true, Bind: "O"},
				ToggleAll: &menu.Keymap{Enabled: true, Bind: "t"},
				Yank:      &menu.Keymap{Enabled: true, Bind: "y"},
			},
			wantErr: nil,
		},
		{
			name:    "all_nil_keymaps",
			k:       &Keymaps{},
			wantErr: nil,
		},
		{
			name: "all_disabled_with_empty_binds",
			k: &Keymaps{
				Edit:      &menu.Keymap{Enabled: false, Bind: ""},
				EditNotes: &menu.Keymap{Enabled: false, Bind: ""},
				Open:      &menu.Keymap{Enabled: false, Bind: ""},
				Preview:   &menu.Keymap{Enabled: false, Bind: ""},
				QR:        &menu.Keymap{Enabled: false, Bind: ""},
				OpenQR:    &menu.Keymap{Enabled: false, Bind: ""},
				ToggleAll: &menu.Keymap{Enabled: false, Bind: ""},
				Yank:      &menu.Keymap{Enabled: false, Bind: ""},
			},
			wantErr: nil,
		},
		{
			name: "mixed_nil_disabled_and_enabled",
			k: &Keymaps{
				Edit:      nil,
				EditNotes: &menu.Keymap{Enabled: false, Bind: ""},
				Open:      &menu.Keymap{Enabled: true, Bind: "o"},
			},
			wantErr: nil,
		},
		{
			name: "boundary_whitespace_bind_is_valid",
			k: &Keymaps{
				// The function explicitly checks for exactly "", not strings.TrimSpace
				Open: &menu.Keymap{Enabled: true, Bind: " "},
			},
			wantErr: nil,
		},
		{
			name: "error_missing_bind_first_element",
			k: &Keymaps{
				Edit: &menu.Keymap{Enabled: true, Bind: ""},
			},
			wantErr: ErrInvalidConfigKeymap,
		},
		{
			name: "error_missing_bind_last_element",
			k: &Keymaps{
				Edit: &menu.Keymap{Enabled: true, Bind: "e"},
				Yank: &menu.Keymap{Enabled: true, Bind: ""},
			},
			wantErr: ErrInvalidConfigKeymap,
		},
		{
			name: "error_missing_bind_toggle_all_duplicate_check",
			k: &Keymaps{
				ToggleAll: &menu.Keymap{Enabled: true, Bind: ""},
			},
			wantErr: ErrInvalidConfigKeymap,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.k.Validate()
			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("Validate() expected error %v, got nil", tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Validate() expected error %v, got %v", tt.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("Validate() unexpected error: %v", err)
			}
		})
	}
}
