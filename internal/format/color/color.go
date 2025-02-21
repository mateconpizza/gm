// Package color provides utilities for formatting and coloring text
// output in the terminal
package color

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"github.com/haaag/gm/internal/config"
)

type ColorFn func(arg ...any) *Color

const (
	// normal colors.
	black   = "\x1b[30m"
	blue    = "\x1b[34m"
	cyan    = "\x1b[36m"
	gray    = "\x1b[90m"
	green   = "\x1b[32m"
	magenta = "\x1b[95m"
	orange  = "\x1b[33m"
	purple  = "\x1b[35m"
	red     = "\x1b[31m"
	white   = "\x1b[37m"
	yellow  = "\x1b[93m"

	// bright colors.
	brightBlack   = "\x1b[90m"
	brightBlue    = "\x1b[94m"
	brightCyan    = "\x1b[96m"
	brightGray    = "\x1b[37m"
	brightGreen   = "\x1b[92m"
	brightMagenta = "\x1b[95m"
	brightOrange  = "\x1b[38;5;214m"
	brightPurple  = "\x1b[38;5;135m"
	brightRed     = "\x1b[91m"
	brightWhite   = "\x1b[97m"
	brightYellow  = "\x1b[93m"

	// styles.
	bold          = "\x1b[1m"
	dim           = "\x1b[2m"
	inverse       = "\x1b[7m"
	italic        = "\x1b[3m"
	strikethrough = "\x1b[9m"
	underline     = "\x1b[4m"

	// reset colors.
	reset = "\x1b[0m"
)

// ANSICode returns the ANSI code from a Color function.
func ANSICode(f ColorFn) string {
	c := f()
	v := reflect.ValueOf(c).Elem().FieldByName("color")
	return v.String()
}

// Color represents styled text with a specific color and formatting styles.
type Color struct {
	text   string
	color  string
	styles []string
}

func Text(s ...string) *Color {
	return &Color{text: strings.Join(s, " ")}
}

func (c *Color) applyStyle(styles ...string) *Color {
	c.styles = append(c.styles, styles...)
	return c
}

func (c *Color) Bold() *Color {
	return c.applyStyle(bold)
}

func (c *Color) Dim() *Color {
	return c.applyStyle(dim)
}

func (c *Color) Inverse() *Color {
	return c.applyStyle(inverse)
}

func (c *Color) Italic() *Color {
	return c.applyStyle(italic)
}

func (c *Color) Strikethrough() *Color {
	return c.applyStyle(strikethrough)
}

func (c *Color) Underline() *Color {
	return c.applyStyle(underline)
}

func (c *Color) String() string {
	if !config.App.Color {
		return c.text
	}
	// apply styles
	styles := strings.Join(c.styles, "")

	return fmt.Sprintf("%s%s%s%s", styles, c.color, c.text, reset)
}

func Reset() string {
	return reset
}

func Black(arg ...any) *Color {
	return addColor(black, arg...)
}

func Blue(arg ...any) *Color {
	return addColor(blue, arg...)
}

func Cyan(arg ...any) *Color {
	return addColor(cyan, arg...)
}

func Gray(arg ...any) *Color {
	return addColor(gray, arg...)
}

func Green(arg ...any) *Color {
	return addColor(green, arg...)
}

func Magenta(arg ...any) *Color {
	return addColor(magenta, arg...)
}

func Orange(arg ...any) *Color {
	return addColor(orange, arg...)
}

func Purple(arg ...any) *Color {
	return addColor(purple, arg...)
}

func Red(arg ...any) *Color {
	return addColor(red, arg...)
}

func White(arg ...any) *Color {
	return addColor(white, arg...)
}

func Yellow(arg ...any) *Color {
	return addColor(yellow, arg...)
}

func BrightBlack(arg ...any) *Color {
	return addColor(brightBlack, arg...)
}

func BrightBlue(arg ...any) *Color {
	return addColor(brightBlue, arg...)
}

func BrightCyan(arg ...any) *Color {
	return addColor(brightCyan, arg...)
}

func BrightGray(arg ...any) *Color {
	return addColor(brightGray, arg...)
}

func BrightGreen(arg ...any) *Color {
	return addColor(brightGreen, arg...)
}

func BrightMagenta(arg ...any) *Color {
	return addColor(brightMagenta, arg...)
}

func BrightOrange(arg ...any) *Color {
	return addColor(brightOrange, arg...)
}

func BrightPurple(arg ...any) *Color {
	return addColor(brightPurple, arg...)
}

func BrightRed(arg ...any) *Color {
	return addColor(brightRed, arg...)
}

func BrightWhite(arg ...any) *Color {
	return addColor(brightWhite, arg...)
}

func BrightYellow(arg ...any) *Color {
	return addColor(brightYellow, arg...)
}

func Default(arg ...any) *Color {
	return addColor(brightWhite, arg...)
}

func addColor(c string, arg ...any) *Color {
	return &Color{text: join(arg...), color: c}
}

func join(text ...any) string {
	str := make([]string, 0, len(text))
	for _, t := range text {
		str = append(str, fmt.Sprint(t))
	}

	return strings.Join(str, " ")
}

// ApplyMany applies a color to a slice of strings returning new slice of
// strings.
func ApplyMany(s []string, c ColorFn) []string {
	for i := 0; i < len(s); i++ {
		s[i] = c(s[i]).String()
	}

	return s
}

// RemoveANSICodes removes ANSI codes from a given string.
func RemoveANSICodes(s string) string {
	re := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	return re.ReplaceAllString(s, "")
}
