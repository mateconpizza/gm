package color

import (
	"fmt"
	"strings"
)

const _reset = "\x1b[0m"

var enableColorOutput *bool

func Enable(b *bool) {
	enableColorOutput = b
}

type Color struct {
	text   string
	color  string
	styles []string
}

func Text(s ...string) *Color {
	return &Color{text: strings.Join(s, " ")}
}

func (c *Color) Style(styles ...string) *Color {
	c.styles = append(c.styles, styles...)
	return c
}

func (c *Color) String() string {
	if !*enableColorOutput {
		return c.text
	}

	// add styles and colors
	styles := strings.Join(c.styles, "")

	return fmt.Sprintf("%s%s%s%s", styles, c.color, c.text, _reset)
}

func (c *Color) Bold() *Color {
	return c.Style("\x1b[1m")
}

func (c *Color) Dim() *Color {
	return c.Style("\x1b[2m")
}

func (c *Color) Underline() *Color {
	return c.Style("\x1b[4m")
}

func (c *Color) Italic() *Color {
	return c.Style("\x1b[3m")
}

func Black(text ...string) *Color {
	return &Color{text: strings.Join(text, " "), color: "\x1b[30m"}
}

func Blue(text ...string) *Color {
	return &Color{text: strings.Join(text, " "), color: "\x1b[34m"}
}

func Cyan(text ...string) *Color {
	return &Color{text: strings.Join(text, " "), color: "\x1b[36m"}
}

func Gray(text ...string) *Color {
	return &Color{text: strings.Join(text, " "), color: "\x1b[90m"}
}

func Green(text ...string) *Color {
	return &Color{text: strings.Join(text, " "), color: "\x1b[32m"}
}

func Orange(text ...string) *Color {
	return &Color{text: strings.Join(text, " "), color: "\x1b[33m"}
}

func Purple(text ...string) *Color {
	return &Color{text: strings.Join(text, " "), color: "\x1b[35m"}
}

func Red(text ...string) *Color {
	return &Color{text: strings.Join(text, " "), color: "\x1b[31m"}
}

func White(text ...string) *Color {
	return &Color{text: strings.Join(text, " "), color: "\x1b[37m"}
}

func Yellow(text ...string) *Color {
	return &Color{text: strings.Join(text, " "), color: "\x1b[33m"}
}

func Magenta(text ...string) *Color {
	return &Color{text: strings.Join(text, " "), color: "\x1b[35m"}
}

func BrightBlack(text ...string) *Color {
	return &Color{text: strings.Join(text, " "), color: "\x1b[90m"}
}

func BrightBlue(text ...string) *Color {
	return &Color{text: strings.Join(text, " "), color: "\x1b[94m"}
}

func BrightCyan(text ...string) *Color {
	return &Color{text: strings.Join(text, " "), color: "\x1b[96m"}
}

func BrightGray(text ...string) *Color {
	return &Color{text: strings.Join(text, " "), color: "\x1b[37m"}
}

func BrightGreen(text ...string) *Color {
	return &Color{text: strings.Join(text, " "), color: "\x1b[92m"}
}

func BrightMagenta(text ...string) *Color {
	return &Color{text: strings.Join(text, " "), color: "\x1b[95m"}
}

func BrightRed(text ...string) *Color {
	return &Color{text: strings.Join(text, " "), color: "\x1b[91m"}
}

func BrightWhite(text ...string) *Color {
	return &Color{text: strings.Join(text, " "), color: "\x1b[97m"}
}

func BrightYellow(text ...string) *Color {
	return &Color{text: strings.Join(text, " "), color: "\x1b[93m"}
}
