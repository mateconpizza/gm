package color

import (
	"errors"
	"testing"
)

func setupInvalidColorScheme(t *testing.T) *Scheme {
	t.Helper()

	return NewScheme("", &Palette{
		Color0: "#282828",
		Color1: "#cc241d",
		Color2: "#98971a",
		Color5: "#b16286",
		Color6: "#689d6a",
		Color7: "#a89984",
	})
}

func TestValidateColorScheme(t *testing.T) {
	t.Parallel()

	cs := setupInvalidColorScheme(t)
	// err on empty name
	err := cs.Validate()
	if err == nil || !errors.Is(err, ErrColorSchemeName) {
		t.Errorf("Expected ErrColorSchemeName, got %v", err)
	}

	cs.Name = "testing-color-scheme"
	// err on missing colors (3 & 4)
	err = cs.Validate()
	if err == nil {
		t.Error("Expected error for missing colors, got nil")
	}

	if !errors.Is(err, ErrColorSchemeColorValue) {
		t.Errorf("Expected ErrColorSchemeColorValue, got %v", err)
	}

	// err on missing palette
	cs.Palette = nil
	if cs.Palette != nil {
		t.Error("Expected Palette to be nil")
	}
	err = cs.Validate()
	if err == nil {
		t.Error("Expected error for missing palette, got nil")
	}
	if !errors.Is(err, ErrColorSchemePalette) {
		t.Errorf("Expected ErrColorSchemePalette, got %v", err)
	}

	// err on nil colorscheme
	cs = nil
	err = cs.Validate()
	if err == nil {
		t.Error("Expected error for nil colorscheme, got nil")
	}
	if !errors.Is(err, ErrColorSchemeInvalid) {
		t.Errorf("Expected ErrColorSchemeInvalid, got %v", err)
	}
}
