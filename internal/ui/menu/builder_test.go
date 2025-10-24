package menu

import (
	"reflect"
	"testing"
)

func TestBuildHeaderStrings(t *testing.T) {
	t.Parallel()

	t.Run("success with visible keybinds", func(t *testing.T) {
		t.Parallel()
		m := &Menu[any]{
			Options: Options{
				cfg: &Config{
					Header: Header{Sep: " | ", Enabled: true},
				},
				keymaps: &keyManager{},
			},
		}

		keys := []*Keymap{
			{Bind: "a", Desc: "Add", Enabled: true},
			{Bind: "x", Desc: "Hidden", Enabled: true, Hidden: true},
			{Bind: "d", Desc: "Delete", Enabled: true},
		}
		m.keymaps.register(keys...)

		got := m.buildHeaderStrings()
		want := []string{"a:Add", "d:Delete"}

		if !reflect.DeepEqual(got, want) {
			t.Fatalf("want %v, got %v", want, got)
		}
	})

	t.Run("uses custom header when provided", func(t *testing.T) {
		t.Parallel()

		want := []string{"custom header"}
		m := New[any](
			WithHeaderOnly(want[0]),
			WithKeybinds([]*Keymap{
				{Bind: "a", Desc: "Add", Enabled: true},
				{Bind: "x", Desc: "Hidden", Enabled: true, Hidden: true},
				{Bind: "d", Desc: "Delete", Enabled: true},
			}...))

		got := m.buildHeaderStrings()

		if !reflect.DeepEqual(got, want) {
			t.Fatalf("want %v, got %v", want, got)
		}
	})
}

func TestFormatHeaderArg(t *testing.T) {
	t.Parallel()
	m := &Menu[any]{
		Options: Options{
			cfg: &Config{
				Header: Header{Sep: " | "},
			},
		},
	}

	headers := []string{"a:Add", "d:Delete"}
	got, err := m.formatHeaderArgs(headers)
	if err != nil {
		t.Fatal(err)
	}

	want := `--header=a:Add | d:Delete`
	if got != want {
		t.Fatalf("want %q, got %q", want, got)
	}
}

func TestBuildHeader_Integration(t *testing.T) {
	t.Parallel()
	m := &Menu[any]{
		Options: Options{
			keymaps: &keyManager{},
			cfg: &Config{
				Header: Header{Sep: " | ", Enabled: true},
			},
		},
	}

	keys := []*Keymap{
		{Bind: "a", Desc: "Add", Enabled: true},
		{Bind: "d", Desc: "Delete", Enabled: true},
	}
	m.keymaps.register(keys...)

	err := m.buildHeaderArgs()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{`--header=a:Add | d:Delete`}
	got := m.arguments

	if len(got) != len(want) {
		t.Fatalf("length mismatch: want %d, got %d", len(want), len(got))
	}

	for i := range want {
		if want[i] != got[i] {
			t.Fatalf("mismatch at index %d:\nwant: %q\ngot:  %q", i, want[i], got[i])
		}
	}
}

func TestBuildPreview(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name       string
		previewCmd string
		previewKey *Keymap
		wantArgs   int
		wantError  bool
	}{
		{
			name:       "generates args from template with valid keymap",
			previewCmd: "echo {1}",
			previewKey: &Keymap{Bind: "ctrl-/", Enabled: true},
			wantArgs:   2,
		},
		{
			name:       "returns empty args for disabled preview keymap",
			previewCmd: "echo {1}",
			previewKey: &Keymap{},
			wantArgs:   0,
		},
		{
			name:       "handles command without placeholders",
			previewCmd: "echo preview",
			previewKey: &Keymap{Bind: "ctrl-/", Enabled: true},
			wantArgs:   2,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			m := &Menu[any]{
				Options: Options{
					withColor:  true,
					previewCmd: tc.previewCmd,
					cfg: &Config{
						BuiltinKeymaps: &BuiltinKeymaps{Preview: tc.previewKey},
					},
					keymaps: &keyManager{},
				},
			}

			err := m.buildPreviewArgs()
			if tc.wantError && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tc.wantError && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got := len(m.arguments); got != tc.wantArgs {
				t.Fatalf("want %d args, got %d, args: %v", tc.wantArgs, got, m.arguments)
			}
		})
	}
}
