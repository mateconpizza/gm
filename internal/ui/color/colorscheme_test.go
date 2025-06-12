package color

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
	assert.ErrorIs(t, cs.Validate(), ErrColorSchemeName)
	cs.Name = "testing-color-scheme"
	// err on missing colors (3 & 4)
	assert.Error(t, cs.Validate())
	assert.ErrorIs(t, cs.Validate(), ErrColorSchemeColorValue)
	// err on missing palette
	cs.Palette = nil
	assert.Nil(t, cs.Palette)
	assert.Error(t, cs.Validate())
	assert.ErrorIs(t, cs.Validate(), ErrColorSchemePalette)
	// err on nil colorscheme
	cs = nil
	assert.Nil(t, cs)
	assert.Error(t, cs.Validate())
	assert.ErrorIs(t, cs.Validate(), ErrColorSchemeInvalid)
}
