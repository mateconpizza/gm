package format

import (
	"fmt"
	"strings"

	"github.com/haaag/gm/pkg/terminal"
)

const _reset = "\x1b[0m"

type ColoredText struct {
	text   string
	color  string
	styles []string
}

func (c *ColoredText) Style(styles ...string) *ColoredText {
	c.styles = append(c.styles, styles...)
	return c
}

func Color(s ...string) *ColoredText {
	return &ColoredText{text: strings.Join(s, " ")}
}

func (c *ColoredText) String() string {
	if terminal.Color {
		styles := strings.Join(c.styles, "")
		return fmt.Sprintf("%s%s%s%s", styles, c.color, c.text, _reset)
	}

	return c.text
}

func (c *ColoredText) Bold() *ColoredText {
	return c.Style("\x1b[1m")
}

func (c *ColoredText) Dim() *ColoredText {
	return c.Style("\x1b[2m")
}

func (c *ColoredText) Underline() *ColoredText {
	return c.Style("\x1b[4m")
}

func (c *ColoredText) Italic() *ColoredText {
	return c.Style("\x1b[3m")
}

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

func (c *ColoredText) Orange() *ColoredText {
	c.color = "\x1b[38;5;208m"
	return c
}

func (c *ColoredText) White() *ColoredText {
	c.color = "\x1b[97m"
	return c
}

func (c *ColoredText) Yellow() *ColoredText {
	c.color = "\x1b[33m"
	return c
}
