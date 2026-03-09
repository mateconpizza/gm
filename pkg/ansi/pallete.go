package ansi

import (
	"fmt"
	"math/rand"
	"strings"
)

var ColorEnabled bool = true

func DisableColor() {
	ColorEnabled = false
}

func NewPalette() *Palette {
	return &Palette{
		Reset: Reset,

		// Standard foreground colors (30-37).
		Black:   Black,
		Red:     Red,
		Green:   Green,
		Yellow:  Yellow,
		Blue:    Blue,
		Magenta: Magenta,
		Cyan:    Cyan,
		White:   White,

		// Bright foreground colors (90-97).
		BrightBlack:   BrightBlack,
		BrightRed:     BrightRed,
		BrightGreen:   BrightGreen,
		BrightYellow:  BrightYellow,
		BrightBlue:    BrightBlue,
		BrightMagenta: BrightMagenta,
		BrightCyan:    BrightCyan,
		BrightWhite:   BrightWhite,

		// Standard background colors (40-47).
		BgBlack:   BgBlack,
		BgRed:     BgRed,
		BgGreen:   BgGreen,
		BgYellow:  BgYellow,
		BgBlue:    BgBlue,
		BgMagenta: BgMagenta,
		BgCyan:    BgCyan,
		BgWhite:   BgWhite,

		// Bright background colors (100-107).
		BgBrightBlack:   BgBrightBlack,
		BgBrightRed:     BgBrightRed,
		BgBrightGreen:   BgBrightGreen,
		BgBrightYellow:  BgBrightYellow,
		BgBrightBlue:    BgBrightBlue,
		BgBrightMagenta: BgBrightMagenta,
		BgBrightCyan:    BgBrightCyan,
		BgBrightWhite:   BgBrightWhite,

		// Text styles.
		Bold:          Bold,
		Dim:           Dim,
		Italic:        Italic,
		Underline:     Underline,
		Undercurl:     Undercurl,
		Blink:         Blink,
		BlinkRapid:    BlinkRapid,
		Inverse:       Inverse,
		Hidden:        Hidden,
		Strikethrough: Strikethrough,
	}
}

// Wrap wraps the given text with the provided styles and resets afterwards.
func (s SGR) Wrap(text string, styles ...SGR) string {
	if !ColorEnabled {
		return text
	}

	return string(s) + combine(styles...) + text + string(Reset)
}

// With combines the receiver style with additional styles and returns a new
// SGR value.
func (s SGR) With(styles ...SGR) SGR {
	return SGR(string(s) + combine(styles...))
}

// Sprint wraps the formatted text with the receiver style and returns it as a
// string.
func (s SGR) Sprint(a ...any) string {
	return s.Wrap(fmt.Sprint(a...))
}

// Sprintf wraps the formatted text using the provided format string with the
// receiver style and returns it as a string.
func (s SGR) Sprintf(f string, a ...any) string {
	return s.Wrap(fmt.Sprintf(f, a...))
}

// Print prints styled text to the standard output.
func (s SGR) Print(a ...any) {
	fmt.Print(s.Wrap(s.Sprint(a...)))
}

// Println prints styled text with a newline.
func (s SGR) Println(a ...any) {
	fmt.Println(s.Wrap(s.Sprint(a...)))
}

// Printf prints styled text using a format string.
func (s SGR) Printf(format string, a ...any) {
	fmt.Print(s.Wrap(fmt.Sprintf(format, a...)))
}

// combine merges multiple SGR codes into a single string.
func combine(codes ...SGR) string {
	var sb strings.Builder
	for _, code := range codes {
		sb.WriteString(string(code))
	}
	return sb.String()
}

type Palette struct {
	Reset SGR // Reset all attributes

	// Standard foreground colors (30-37).
	Black   SGR
	Red     SGR
	Green   SGR
	Yellow  SGR
	Blue    SGR
	Magenta SGR
	Cyan    SGR
	White   SGR

	// Bright foreground colors (90-97).
	BrightBlack   SGR
	BrightRed     SGR
	BrightGreen   SGR
	BrightYellow  SGR
	BrightBlue    SGR
	BrightMagenta SGR
	BrightCyan    SGR
	BrightWhite   SGR

	// Standard background colors (40-47).
	BgBlack   SGR
	BgRed     SGR
	BgGreen   SGR
	BgYellow  SGR
	BgBlue    SGR
	BgMagenta SGR
	BgCyan    SGR
	BgWhite   SGR

	// Bright background colors (100-107).
	BgBrightBlack   SGR
	BgBrightRed     SGR
	BgBrightGreen   SGR
	BgBrightYellow  SGR
	BgBrightBlue    SGR
	BgBrightMagenta SGR
	BgBrightCyan    SGR
	BgBrightWhite   SGR

	// Text styles.
	Bold          SGR // Bold or increased intensity
	Dim           SGR // Faint or dim
	Italic        SGR // Italic
	Underline     SGR // Underline
	Undercurl     SGR // Undercurl
	Blink         SGR // Slow blink
	BlinkRapid    SGR // Rapid blink
	Inverse       SGR // Inverse/reverse video
	Hidden        SGR // Conceal/hidden
	Strikethrough SGR // Crossed-out/strikethrough
}

func (p *Palette) Enabled() bool { return ColorEnabled }

// Random returns a random color with the given styles.
func (p *Palette) Random(styles ...SGR) SGR {
	return ColorRand(styles...)
}

// Remover removes ANSI codes from a given string.
func (p *Palette) Remover(s string) string {
	return Remover(s)
}

// ColorRand returns a random color with the given styles.
func ColorRand(styles ...SGR) SGR {
	colors := []SGR{
		Red.With(styles...),
		Green.With(styles...),
		Yellow.With(styles...),
		Blue.With(styles...),
		Magenta.With(styles...),
		Cyan.With(styles...),
		White.With(styles...),
		BrightBlack.With(styles...),
		BrightRed.With(styles...),
		BrightGreen.With(styles...),
		BrightYellow.With(styles...),
		BrightBlue.With(styles...),
		BrightMagenta.With(styles...),
		BrightCyan.With(styles...),
		BrightWhite.With(styles...),
	}

	return colors[rand.Intn(len(colors))]
}
