package terminal

import (
	"errors"
	"testing"

	"golang.org/x/term"
)

func TestState_Save(t *testing.T) {
	t.Parallel()

	errSaveFailed := errors.New("save failed")
	dummyState := &term.State{}

	tests := []struct {
		name      string
		saveFunc  saveStateFunc
		wantState *term.State
		wantErr   error
	}{
		{
			name: "normal_save",
			saveFunc: func() (*term.State, error) {
				return dummyState, nil
			},
			wantState: dummyState,
			wantErr:   nil,
		},
		{
			name: "save_fails_with_error",
			saveFunc: func() (*term.State, error) {
				return nil, errSaveFailed
			},
			wantState: nil,
			wantErr:   errSaveFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := &State{
				saveFunc: tt.saveFunc,
			}
			err := s.Save()

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("Save() expected error %v, got nil", tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Save() expected error %v, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Save() unexpected error: %v", err)
			}
			if s.Current() != tt.wantState {
				t.Fatalf("Save() state = %v; want %v", s.Current(), tt.wantState)
			}
		})
	}
}

func TestState_Restore(t *testing.T) {
	t.Parallel()

	errRestoreFailed := errors.New("restore failed")
	dummyState := &term.State{}

	tests := []struct {
		name        string
		current     *term.State
		restoreFunc restoreStateFunc
		wantErr     error
	}{
		{
			name:    "normal_restore",
			current: dummyState,
			restoreFunc: func(state *term.State) error {
				return nil
			},
			wantErr: nil,
		},
		{
			name:    "no_state_to_restore_nil",
			current: nil,
			restoreFunc: func(state *term.State) error {
				return nil
			},
			wantErr: ErrNoStateToRestore,
		},
		{
			name:    "restore_fails_with_error",
			current: dummyState,
			restoreFunc: func(state *term.State) error {
				return errRestoreFailed
			},
			wantErr: errRestoreFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := &State{
				current:     tt.current,
				restoreFunc: tt.restoreFunc,
			}
			err := s.Restore()

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("Restore() expected error %v, got nil", tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Restore() expected error %v, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Restore() unexpected error: %v", err)
			}
		})
	}
}

func TestState_Configuration(t *testing.T) {
	t.Parallel()

	t.Run("new_state_defaults_not_nil", func(t *testing.T) {
		t.Parallel()

		s := NewState()
		if s.saveFunc == nil {
			t.Error("NewState() saveFunc is nil, expected default function")
		}
		if s.restoreFunc == nil {
			t.Error("NewState() restoreFunc is nil, expected default function")
		}
	})

	t.Run("fluent_chaining_overrides_funcs", func(t *testing.T) {
		t.Parallel()

		saveCalled := false
		restoreCalled := false

		s := NewState().
			WithSaveFunc(func() (*term.State, error) {
				saveCalled = true
				return nil, nil
			}).WithRestoreFunc(func(state *term.State) error {
			restoreCalled = true
			return nil
		})

		_, err := s.saveFunc()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		_ = s.restoreFunc(nil)

		if !saveCalled {
			t.Error("WithSaveFunc() didn't set the correct function")
		}
		if !restoreCalled {
			t.Error("WithRestoreFunc() didn't set the correct function")
		}
	})

	t.Run("current_getter", func(t *testing.T) {
		t.Parallel()
		dummyState := &term.State{}
		s := &State{current: dummyState}

		if s.Current() != dummyState {
			t.Errorf("Current() = %v; want %v", s.Current(), dummyState)
		}
	})
}
