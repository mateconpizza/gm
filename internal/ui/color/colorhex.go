package color

import (
	"fmt"
	"strconv"
	"strings"
)

// HexToANSI converts a hex color code to an ANSI escape sequence.
func HexToANSI(hex string) string {
	if !strings.HasPrefix(hex, "#") {
		return hex
	}
	// remove the leading '#' if present
	hex = strings.TrimPrefix(hex, "#")
	// convert the hex code to an integer
	i, err := strconv.ParseInt(hex, 16, 32)
	if err != nil {
		return ""
	}
	// convert the integer to an ansi escape sequence
	return fmt.Sprintf("\x1b[38;2;%d;%d;%dm", (i>>16)&0xFF, (i>>8)&0xFF, i&0xFF)
}

// HexRGB creates a new ColorFn that can be used to create Color instances
// with the specified hex color.
func HexRGB(hex string) ColorFn {
	return func(arg ...any) *Color {
		text := fmt.Sprint(arg...)

		return &Color{
			text:   text,
			color:  HexToANSI(hex),
			styles: []string{},
		}
	}
}

func NewColor(color string) ColorFn {
	return func(arg ...any) *Color {
		return &Color{
			color:  color,
			styles: []string{},
		}
	}
}
