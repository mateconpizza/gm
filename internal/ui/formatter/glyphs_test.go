package formatter

import (
	"errors"
	"reflect"
	"testing"
)

func TestGlyphMode_Valid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		mode    GlyphMode
		wantErr error
	}{
		{"valid_ascii", GlyphModeASCII, nil},
		{"valid_unicode", GlyphModeUnicode, nil},
		{"valid_custom", GlyphModeCustom, nil},
		{"valid_custom_unicode", GlyphModeCustomUnicode, nil},
		{"invalid_empty_string", GlyphMode(""), ErrGlyphModeInvalid},
		{"invalid_unknown_mode", GlyphMode("unknown-mode"), ErrGlyphModeInvalid},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.mode.Valid()
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Valid() error = %v; wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewGlyphs(t *testing.T) {
	t.Parallel()

	// setup partial overrides for testing custom merging
	customPartial := &Glyphs{
		Ellipsis: "***",
		Warning:  "W",
		Add:      "+",
	}

	expectedCustomASCII := *ASCII
	expectedCustomASCII.Ellipsis = "***"
	expectedCustomASCII.Warning = "W"
	expectedCustomASCII.Add = "+"

	expectedCustomUnicode := *Unicode
	expectedCustomUnicode.Ellipsis = "***"
	expectedCustomUnicode.Warning = "W"
	expectedCustomUnicode.Add = "+"

	tests := []struct {
		name   string
		mode   GlyphMode
		custom *Glyphs
		want   *Glyphs
	}{
		{"ascii_mode_default", GlyphModeASCII, nil, ASCII},
		{"unicode_mode", GlyphModeUnicode, nil, Unicode},
		{"custom_mode_nil_custom", GlyphModeCustom, nil, ASCII},
		{"custom_mode_empty_struct", GlyphModeCustom, &Glyphs{}, ASCII},
		{"custom_mode_partial_override", GlyphModeCustom, customPartial, &expectedCustomASCII},
		{"custom_unicode_mode_partial", GlyphModeCustomUnicode, customPartial, &expectedCustomUnicode},
		{"unknown_mode_fallback", GlyphMode("unknown"), nil, ASCII},
		{"unknown_mode_with_custom_ignored", GlyphMode("unknown"), customPartial, ASCII},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := NewGlyphs(tt.mode, tt.custom)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("NewGlyphs() = %+v; want %+v", got, tt.want)
			}
		})
	}
}
