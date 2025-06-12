package color

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
)

var (
	ErrColorSchemeColorValue = errors.New("missing value")
	ErrColorSchemeInvalid    = errors.New("invalid colorscheme")
	ErrColorSchemeName       = errors.New("missing colorscheme name")
	ErrColorSchemePalette    = errors.New("missing palette")
	ErrColorSchemeUnknown    = errors.New("unknown colorscheme")
	ErrColorSchemePath       = errors.New("missing colorscheme path")
)

var DefaultSchemes = map[string]*Scheme{
	"gruvbox-light-medium": GruvboxLightMedium(),
	"gruvbox-dark-medium":  GruvboxDarkMedium(),
	"default":              DefaultColorScheme(),
}

// Palette defines a full 16-color palette.
type Palette struct {
	Color0  string `yaml:"color0"`  // Black|Background (COLOR0)
	Color1  string `yaml:"color1"`  // Red (COLOR1)
	Color2  string `yaml:"color2"`  // Green (COLOR2)
	Color3  string `yaml:"color3"`  // Yellow (COLOR3)
	Color4  string `yaml:"color4"`  // Blue (COLOR4)
	Color5  string `yaml:"color5"`  // Magenta (COLOR5)
	Color6  string `yaml:"color6"`  // Cyan (COLOR6)
	Color7  string `yaml:"color7"`  // White (COLOR7)
	Color8  string `yaml:"color8"`  // BrightBlack (COLOR8)
	Color9  string `yaml:"color9"`  // BrightRed (COLOR9)
	Color10 string `yaml:"color10"` // BrightGreen (COLOR10)
	Color11 string `yaml:"color11"` // BrightYellow (COLOR11)
	Color12 string `yaml:"color12"` // BrightBlue (COLOR12)
	Color13 string `yaml:"color13"` // BrightMagenta (COLOR13)
	Color14 string `yaml:"color14"` // BrightCyan (COLOR14)
	Color15 string `yaml:"color15"` // BrightWhite (COLOR15)
}

// Black or Background (COLOR0).
func (p *Palette) Black(s ...any) *Color {
	return HexRGB(p.Color0)(s...)
}

// Red (COLOR1).
func (p *Palette) Red(s ...any) *Color {
	return HexRGB(p.Color1)(s...)
}

// Green (COLOR2).
func (p *Palette) Green(s ...any) *Color {
	return HexRGB(p.Color2)(s...)
}

// Yellow (COLOR3).
func (p *Palette) Yellow(s ...any) *Color {
	return HexRGB(p.Color3)(s...)
}

// Blue (COLOR4).
func (p *Palette) Blue(s ...any) *Color {
	return HexRGB(p.Color4)(s...)
}

// Magenta (COLOR5).
func (p *Palette) Magenta(s ...any) *Color {
	return HexRGB(p.Color5)(s...)
}

// Cyan (COLOR6).
func (p *Palette) Cyan(s ...any) *Color {
	return HexRGB(p.Color6)(s...)
}

// White (COLOR7).
func (p *Palette) White(s ...any) *Color {
	return HexRGB(p.Color7)(s...)
}

// BrightBlack (COLOR8).
func (p *Palette) BrightBlack(s ...any) *Color {
	if p.Color8 == "" {
		return p.Black(s...)
	}

	return HexRGB(p.Color8)(s...)
}

// BrightRed (COLOR9).
func (p *Palette) BrightRed(s ...any) *Color {
	if p.Color9 == "" {
		return p.Red(s...)
	}

	return HexRGB(p.Color9)(s...)
}

// BrightGreen (COLOR10).
func (p *Palette) BrightGreen(s ...any) *Color {
	if p.Color10 == "" {
		return p.Green(s...)
	}

	return HexRGB(p.Color10)(s...)
}

// BrightYellow (COLOR11).
func (p *Palette) BrightYellow(s ...any) *Color {
	if p.Color11 == "" {
		return p.Yellow(s...)
	}

	return HexRGB(p.Color11)(s...)
}

// BrightBlue (COLOR12).
func (p *Palette) BrightBlue(s ...any) *Color {
	if p.Color12 == "" {
		return p.Blue(s...)
	}

	return HexRGB(p.Color12)(s...)
}

// BrightMagenta (COLOR13).
func (p *Palette) BrightMagenta(s ...any) *Color {
	if p.Color13 == "" {
		return p.Magenta(s...)
	}

	return HexRGB(p.Color13)(s...)
}

// BrightCyan (COLOR14).
func (p *Palette) BrightCyan(s ...any) *Color {
	if p.Color14 == "" {
		return p.Cyan(s...)
	}

	return HexRGB(p.Color14)(s...)
}

// BrightWhite or Foreground (COLOR15).
func (p *Palette) BrightWhite(s ...any) *Color {
	if p.Color15 == "" {
		return p.White(s...)
	}

	return HexRGB(p.Color15)(s...)
}

// Foreground (COLOR15).
func (p *Palette) Foreground(s ...any) *Color {
	return HexRGB(p.Color15)(s...)
}

// Background (COLOR0).
func (p *Palette) Background(s ...any) *Color {
	return HexRGB(p.Color0)(s...)
}

type Scheme struct {
	Name     string `yaml:"name"` // Name of the scheme
	*Palette `yaml:",inline"`
	Enabled  bool `yaml:"-"`
}

// Len returns the number of colors found in the scheme.
func (p *Palette) Len() int {
	v := reflect.ValueOf(*p)
	count := 0

	for i := range v.NumField() {
		val := v.Field(i).String()
		if strings.TrimSpace(val) != "" {
			count++
		}
	}

	return count
}

// Validate checks if the color scheme is valid.
func (s *Scheme) Validate() error {
	if s == nil {
		return ErrColorSchemeInvalid
	}

	if s.Name == "" {
		return ErrColorSchemeName
	}

	if s.Palette == nil {
		return ErrColorSchemePalette
	}

	missing := []string{}
	check := map[string]string{
		"color0": s.Color0,
		"color1": s.Color1,
		"color2": s.Color2,
		"color3": s.Color3,
		"color4": s.Color4,
		"color5": s.Color5,
		"color6": s.Color6,
		"color7": s.Color7,
	}

	for k, v := range check {
		if strings.TrimSpace(v) == "" {
			missing = append(missing, k)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("%w: %s", ErrColorSchemeColorValue, strings.Join(missing, ", "))
	}

	return nil
}

func NewScheme(s string, cs *Palette) *Scheme {
	return &Scheme{
		Name:    s,
		Palette: cs,
	}
}

// DefaultColorScheme returns the default color scheme.
func DefaultColorScheme() *Scheme {
	return NewScheme("default", &Palette{
		Color0:  black,
		Color1:  red,
		Color2:  green,
		Color3:  yellow,
		Color4:  blue,
		Color5:  magenta,
		Color6:  cyan,
		Color7:  white,
		Color8:  brightBlack,
		Color9:  brightRed,
		Color10: brightGreen,
		Color11: brightYellow,
		Color12: brightBlue,
		Color13: brightMagenta,
		Color14: brightCyan,
		Color15: brightWhite,
	})
}

// GruvboxDarkMedium returns a Gruvbox Dark Medium color scheme.
func GruvboxDarkMedium() *Scheme {
	return NewScheme("gruvbox-dark-medium", &Palette{
		Color0:  "#282828",
		Color1:  "#cc241d",
		Color2:  "#98971a",
		Color3:  "#d79921",
		Color4:  "#458588",
		Color5:  "#b16286",
		Color6:  "#689d6a",
		Color7:  "#a89984",
		Color8:  "#928374",
		Color9:  "#fb4934",
		Color10: "#b8bb26",
		Color11: "#fabd2f",
		Color12: "#83a598",
		Color13: "#d3869b",
		Color14: "#8ec07c",
		Color15: "#ebdbb2",
	})
}

// GruvboxLightMedium returns a Gruvbox Light Medium color scheme.
func GruvboxLightMedium() *Scheme {
	return NewScheme("gruvbox-light-medium", &Palette{
		Color0:  "#fbf1c7",
		Color1:  "#cc241d",
		Color2:  "#98971a",
		Color3:  "#d79921",
		Color4:  "#458588",
		Color5:  "#b16286",
		Color6:  "#689d6a",
		Color7:  "#7c6f64",
		Color8:  "#928374",
		Color9:  "#9d0006",
		Color10: "#79740e",
		Color11: "#b57614",
		Color12: "#076678",
		Color13: "#8f3f71",
		Color14: "#427b58",
		Color15: "#3c3836",
	})
}
