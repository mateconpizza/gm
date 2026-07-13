package menu

import (
	"errors"
	"testing"
)

func TestBuiltinKeymaps_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		k       *BuiltinKeymaps
		wantErr error
	}{
		{
			name: "all_valid_and_enabled",
			k: &BuiltinKeymaps{
				Edit:      &Keymap{Enabled: true, Bind: "e"},
				EditNotes: &Keymap{Enabled: true, Bind: "n"},
				Open:      &Keymap{Enabled: true, Bind: "o"},
				Preview:   &Keymap{Enabled: true, Bind: "p"},
				QR:        &Keymap{Enabled: true, Bind: "q"},
				OpenQR:    &Keymap{Enabled: true, Bind: "O"},
				ToggleAll: &Keymap{Enabled: true, Bind: "t"},
				Yank:      &Keymap{Enabled: true, Bind: "y"},
			},
			wantErr: nil,
		},
		{
			name:    "all_nil_keymaps",
			k:       &BuiltinKeymaps{},
			wantErr: nil,
		},
		{
			name: "all_disabled_with_empty_binds",
			k: &BuiltinKeymaps{
				Edit:      &Keymap{Enabled: false, Bind: ""},
				EditNotes: &Keymap{Enabled: false, Bind: ""},
				Open:      &Keymap{Enabled: false, Bind: ""},
				Preview:   &Keymap{Enabled: false, Bind: ""},
				QR:        &Keymap{Enabled: false, Bind: ""},
				OpenQR:    &Keymap{Enabled: false, Bind: ""},
				ToggleAll: &Keymap{Enabled: false, Bind: ""},
				Yank:      &Keymap{Enabled: false, Bind: ""},
			},
			wantErr: nil,
		},
		{
			name: "mixed_nil_disabled_and_enabled",
			k: &BuiltinKeymaps{
				Edit:      nil,
				EditNotes: &Keymap{Enabled: false, Bind: ""},
				Open:      &Keymap{Enabled: true, Bind: "o"},
			},
			wantErr: nil,
		},
		{
			name: "boundary_whitespace_bind_is_valid",
			k: &BuiltinKeymaps{
				// The function explicitly checks for exactly "", not strings.TrimSpace
				Open: &Keymap{Enabled: true, Bind: " "},
			},
			wantErr: nil,
		},
		{
			name: "error_missing_bind_first_element",
			k: &BuiltinKeymaps{
				Edit: &Keymap{Enabled: true, Bind: ""},
			},
			wantErr: ErrInvalidConfigKeymap,
		},
		{
			name: "error_missing_bind_last_element",
			k: &BuiltinKeymaps{
				Edit: &Keymap{Enabled: true, Bind: "e"},
				Yank: &Keymap{Enabled: true, Bind: ""},
			},
			wantErr: ErrInvalidConfigKeymap,
		},
		{
			name: "error_missing_bind_toggle_all_duplicate_check",
			k: &BuiltinKeymaps{
				ToggleAll: &Keymap{Enabled: true, Bind: ""},
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
