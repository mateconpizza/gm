// Package ansi provides ANSI escape sequences for terminal control, including
// cursor manipulation, screen clearing, colors, and text styles.
package ansi

import (
	"fmt"
	"regexp"
	"strings"
)

var ansiEscapeRe = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// CursorCode control sequences for showing, hiding, and moving the cursor.
type CursorCode string

const (
	CursorHide  CursorCode = "\x1b[?25l"
	CursorShow  CursorCode = "\x1b[?25h"
	CursorUp    CursorCode = "\x1b[1A"
	CursorDown  CursorCode = "\x1b[1B"
	CursorRight CursorCode = "\x1b[1C"
	CursorLeft  CursorCode = "\x1b[1D"

	// Absolute positioning.
	CursorHome    CursorCode = "\x1b[H"
	CursorToStart CursorCode = "\x1b[1G"
	CursorToEnd   CursorCode = "\x1b[999C"
	CursorReturn  CursorCode = "\r"

	// Save and restore cursor position.
	CursorSave    CursorCode = "\x1b[s"
	CursorRestore CursorCode = "\x1b[u"
)

// EraseCode sequences for clearing parts of the screen or line.
type EraseCode string

const (
	// Line erasing.
	EraseLine        EraseCode = "\x1b[2K"
	EraseLineToEnd   EraseCode = "\x1b[0K"
	EraseLineToStart EraseCode = "\x1b[1K"

	// Screen erasing.
	EraseScreen        EraseCode = "\x1b[2J"
	EraseScreenToEnd   EraseCode = "\x1b[0J"
	EraseScreenToStart EraseCode = "\x1b[1J"
	EraseScrollback    EraseCode = "\x1b[3J" // Clear scrollback buffer (not always supported)

	// Combined operations.
	ClearScreen           EraseCode = "\x1b[H\x1b[2J"
	ClearScreenScrollback EraseCode = "\x1b[H\x1b[2J\x1b[3J" // Clear screen + scrollback
	ClearLineUp           EraseCode = "\x1b[F\x1b[K"
	ClearCharBacksp       EraseCode = "\b \b"
)

// SGR (Select Graphic Rendition) sequences for colors and text styles.
type SGR string

const (
	Reset  SGR = "\x1b[0m" // Reset all attributes
	Normal SGR = "\x1b[39m"

	// Standard foreground colors (30-37).
	Black   SGR = "\x1b[30m"
	Red     SGR = "\x1b[31m"
	Green   SGR = "\x1b[32m"
	Yellow  SGR = "\x1b[33m"
	Blue    SGR = "\x1b[34m"
	Magenta SGR = "\x1b[35m"
	Cyan    SGR = "\x1b[36m"
	White   SGR = "\x1b[37m"
	Gray    SGR = "\x1b[97m\x1b[2m"
	Orange  SGR = "\x1b[38;5;208m"

	// Bright foreground colors (90-97).
	BrightBlack   SGR = "\x1b[90m"
	BrightRed     SGR = "\x1b[91m"
	BrightGreen   SGR = "\x1b[92m"
	BrightYellow  SGR = "\x1b[93m"
	BrightBlue    SGR = "\x1b[94m"
	BrightMagenta SGR = "\x1b[95m"
	BrightCyan    SGR = "\x1b[96m"
	BrightWhite   SGR = "\x1b[97m"

	// Standard background colors (40-47).
	BgBlack   SGR = "\x1b[40m"
	BgRed     SGR = "\x1b[41m"
	BgGreen   SGR = "\x1b[42m"
	BgYellow  SGR = "\x1b[43m"
	BgBlue    SGR = "\x1b[44m"
	BgMagenta SGR = "\x1b[45m"
	BgCyan    SGR = "\x1b[46m"
	BgWhite   SGR = "\x1b[47m"

	// Bright background colors (100-107).
	BgBrightBlack   SGR = "\x1b[100m"
	BgBrightRed     SGR = "\x1b[101m"
	BgBrightGreen   SGR = "\x1b[102m"
	BgBrightYellow  SGR = "\x1b[103m"
	BgBrightBlue    SGR = "\x1b[104m"
	BgBrightMagenta SGR = "\x1b[105m"
	BgBrightCyan    SGR = "\x1b[106m"
	BgBrightWhite   SGR = "\x1b[107m"

	// Text styles.
	Bold          SGR = "\x1b[1m"   // Bold
	Dim           SGR = "\x1b[2m"   // Faint or dim
	Italic        SGR = "\x1b[3m"   // Italic
	Underline     SGR = "\x1b[4m"   // Underline
	Undercurl     SGR = "\x1b[4:3m" // Undercurl
	Blink         SGR = "\x1b[5m"   // Slow blink
	BlinkRapid    SGR = "\x1b[6m"   // Rapid blink
	Inverse       SGR = "\x1b[7m"   // Inverse/reverse video
	Hidden        SGR = "\x1b[8m"   // Conceal/hidden
	Strikethrough SGR = "\x1b[9m"   // Crossed-out/strikethrough

	// Style resets.
	NormalIntensity SGR = "\x1b[22m" // Normal intensity (not bold/dim)
	NoItalic        SGR = "\x1b[23m" // Not italic
	NoUnderline     SGR = "\x1b[24m" // Not underlined
	NoBlink         SGR = "\x1b[25m" // Not blinking
	NoInverse       SGR = "\x1b[27m" // Not inverse
	NoHidden        SGR = "\x1b[28m" // Not hidden
	NoStrikethrough SGR = "\x1b[29m" // Not crossed out
	DefaultFgColor  SGR = "\x1b[39m" // Default foreground color
	DefaultBgColor  SGR = "\x1b[49m" // Default background color
)

// Wrap wraps the given text with the provided styles and resets afterwards.
func (s SGR) Wrap(text string, styles ...SGR) string {
	if text == "" {
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

// Remover removes ANSI codes from a given string.
func Remover(s string) string {
	return ansiEscapeRe.ReplaceAllString(s, "")
}

type Codes struct {
	*Palette
	*Cursor
	*Erase
}

func NewCodes(enabled bool) *Codes {
	return &Codes{
		Palette: NewPalette(enabled),
		Cursor:  NewCursorCodes(),
		Erase:   NewEraseCodes(),
	}
}
