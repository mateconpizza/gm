package ansi

import (
	"fmt"
	"io"
	"math/rand"
	"strings"
)

type Style struct {
	code    SGR
	enabled bool
}

func (s Style) Enabled() bool { return s.enabled }

func newStyle(code SGR, enabled bool) Style {
	return Style{code: code, enabled: enabled}
}

// Wrap wraps the given text with the style and resets afterward, or
// returns text unchanged if disabled.
func (s Style) Wrap(text string, styles ...Style) string {
	if !s.enabled || text == "" {
		return text
	}
	return string(s.code) + combineStyles(styles...) + text + string(Reset)
}

// With combines the receiver style with additional styles, carrying the
// same enabled flag.
func (s Style) With(styles ...Style) Style {
	return Style{code: SGR(string(s.code) + combineCodes(styles...)), enabled: s.enabled}
}

func (s Style) Sprint(a ...any) string                      { return s.Wrap(fmt.Sprint(a...)) }
func (s Style) Sprintf(f string, a ...any) string           { return s.Wrap(fmt.Sprintf(f, a...)) }
func (s Style) Fprint(w io.Writer, a ...any)                { fmt.Fprint(w, s.Sprint(a...)) }
func (s Style) Fprintln(w io.Writer, a ...any)              { fmt.Fprintln(w, s.Sprint(a...)) }
func (s Style) Printf(w io.Writer, format string, a ...any) { fmt.Fprint(w, s.Sprintf(format, a...)) }

func combineStyles(styles ...Style) string {
	var sb strings.Builder
	for _, st := range styles {
		sb.WriteString(string(st.code))
	}
	return sb.String()
}

func combineCodes(styles ...Style) string {
	return combineStyles(styles...)
}

type Palette struct {
	enabled bool

	Reset  Style
	Normal Style

	// Standard foreground colors (30-37).
	Black, Red, Green, Yellow, Blue, Magenta, Cyan, White, Gray, Orange Style

	// Bright foreground colors (90-97).
	BrightBlack, BrightRed, BrightGreen, BrightYellow,
	BrightBlue, BrightMagenta, BrightCyan, BrightWhite Style

	// Standard background colors (40-47).
	BgBlack, BgRed, BgGreen, BgYellow, BgBlue, BgMagenta, BgCyan, BgWhite Style

	// Bright background colors (100-107).
	BgBrightBlack, BgBrightRed, BgBrightGreen, BgBrightYellow,
	BgBrightBlue, BgBrightMagenta, BgBrightCyan, BgBrightWhite Style

	// Text styles.
	Bold, Dim, Italic, Underline, Undercurl, Blink, BlinkRapid, Inverse, Hidden, Strikethrough Style
}

func NewPalette(enabled bool) *Palette {
	s := func(code SGR) Style { return newStyle(code, enabled) }
	return &Palette{
		enabled: enabled,

		Reset:  s(Reset),
		Normal: s(Normal),

		// Standard foreground colors (30-37).
		Black:   s(Black),
		Red:     s(Red),
		Green:   s(Green),
		Yellow:  s(Yellow),
		Blue:    s(Blue),
		Magenta: s(Magenta),
		Cyan:    s(Cyan),
		White:   s(White),
		Gray:    s(Gray),
		Orange:  s(Orange),

		// Bright foreground colors (90-97).
		BrightBlack:   s(BrightBlack),
		BrightRed:     s(BrightRed),
		BrightGreen:   s(BrightGreen),
		BrightYellow:  s(BrightYellow),
		BrightBlue:    s(BrightBlue),
		BrightMagenta: s(BrightMagenta),
		BrightCyan:    s(BrightCyan),
		BrightWhite:   s(BrightWhite),

		// Standard background colors (40-47).
		BgBlack:   s(BgBlack),
		BgRed:     s(BgRed),
		BgGreen:   s(BgGreen),
		BgYellow:  s(BgYellow),
		BgBlue:    s(BgBlue),
		BgMagenta: s(BgMagenta),
		BgCyan:    s(BgCyan),
		BgWhite:   s(BgWhite),

		// Bright background colors (100-107).
		BgBrightBlack:   s(BgBrightBlack),
		BgBrightRed:     s(BgBrightRed),
		BgBrightGreen:   s(BgBrightGreen),
		BgBrightYellow:  s(BgBrightYellow),
		BgBrightBlue:    s(BgBrightBlue),
		BgBrightMagenta: s(BgBrightMagenta),
		BgBrightCyan:    s(BgBrightCyan),
		BgBrightWhite:   s(BgBrightWhite),

		// Text styles.
		Bold:          newStyle(Bold, true),
		Dim:           s(Dim),
		Italic:        newStyle(Italic, true),
		Underline:     s(Underline),
		Undercurl:     s(Undercurl),
		Blink:         s(Blink),
		BlinkRapid:    s(BlinkRapid),
		Inverse:       s(Inverse),
		Hidden:        s(Hidden),
		Strikethrough: s(Strikethrough),
	}
}

func (p *Palette) Enabled() bool { return p.enabled }

// Remover removes ANSI codes from a given string.
func (p *Palette) Remover(s string) string {
	return Remover(s)
}

// StyleAll applies styles to all elements in the slice.
func (p *Palette) StyleAll(a []string, styles ...Style) []string {
	for i := range a {
		for _, c := range styles {
			a[i] = c.Sprint(a[i])
		}
	}

	return a
}

// Random returns a random color Style from the palette, combined with the
// given additional styles.
func (p *Palette) Random(styles ...Style) Style {
	colors := []Style{
		p.Red.With(styles...),
		p.Green.With(styles...),
		p.Yellow.With(styles...),
		p.Blue.With(styles...),
		p.Magenta.With(styles...),
		p.Cyan.With(styles...),
		p.White.With(styles...),
		p.BrightRed.With(styles...),
		p.BrightGreen.With(styles...),
		p.BrightYellow.With(styles...),
		p.BrightBlue.With(styles...),
		p.BrightMagenta.With(styles...),
		p.BrightCyan.With(styles...),
	}

	return colors[rand.Intn(len(colors))]
}

// Random returns a random color with the given styles.
func Random(styles ...SGR) SGR {
	colors := []SGR{
		Red.With(styles...),
		Green.With(styles...),
		Yellow.With(styles...),
		Blue.With(styles...),
		Magenta.With(styles...),
		Cyan.With(styles...),
		White.With(styles...),
		BrightRed.With(styles...),
		BrightGreen.With(styles...),
		BrightYellow.With(styles...),
		BrightBlue.With(styles...),
		BrightMagenta.With(styles...),
		BrightCyan.With(styles...),
	}

	return colors[rand.Intn(len(colors))]
}
