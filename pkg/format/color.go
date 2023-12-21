package format

import (
	"fmt"
	"strings"
)

var WithColor bool = true

const reset = "\x1b[0m"

type ColoredText struct {
	text   string
	color  string
	styles []string
}

func (c *ColoredText) Style(styles ...string) *ColoredText {
	c.styles = append(c.styles, styles...)
	return c
}

func Text(s ...string) *ColoredText {
	return &ColoredText{text: strings.Join(s, " ")}
}

func (c *ColoredText) String() string {
	styles := strings.Join(c.styles, "")
	if WithColor {
		return fmt.Sprintf("%s%s%s%s", styles, c.color, c.text, reset)
	}
	return c.text
}

// styles
func (c *ColoredText) Bold() *ColoredText {
	return c.Style("\x1b[1m")
}

func (c *ColoredText) Dim() *ColoredText {
	return c.Style("\x1b[2m")
}

func (c *ColoredText) Underline() *ColoredText {
	return c.Style("\x1b[4m")
}

// colors
func (c *ColoredText) Blue() *ColoredText {
	c.color = "\x1b[34m"
	return c
}

func (c *ColoredText) Cyan() *ColoredText {
	c.color = "\x1b[36m"
	return c
}

func (c *ColoredText) Gray() *ColoredText {
	c.color = "\x1b[38;5;242m"
	return c
}

func (c *ColoredText) Green() *ColoredText {
	c.color = "\x1b[32m"
	return c
}

func (c *ColoredText) Red() *ColoredText {
	c.color = "\x1b[31m"
	return c
}

func (c *ColoredText) Purple() *ColoredText {
	c.color = "\x1b[35m"
	return c
}

func (c *ColoredText) White() *ColoredText {
	c.color = ""
	return c
}

func (c *ColoredText) Yellow() *ColoredText {
	c.color = "\x1b[33m"
	return c
}
