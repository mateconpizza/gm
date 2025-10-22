// Package frame provides a customizable text framing and styling utility for
// console output, including borders and icons.
package frame

import (
	"fmt"
	"strings"
)

var colorEnabled bool = false

type color string

var (
	// normal colors.
	ColorBlack   color = "\x1b[30m"
	ColorBlue    color = "\x1b[34m"
	ColorCyan    color = "\x1b[36m"
	ColorGray    color = "\x1b[90m"
	ColorGreen   color = "\x1b[32m"
	ColorMagenta color = "\x1b[95m"
	ColorOrange  color = "\x1b[33m"
	ColorPurple  color = "\x1b[35m"
	ColorRed     color = "\x1b[31m"
	ColorWhite   color = "\x1b[37m"
	ColorYellow  color = "\x1b[93m"

	// bright colors.
	ColorBrightBlack   color = "\x1b[90m"
	ColorBrightBlue    color = "\x1b[94m"
	ColorBrightCyan    color = "\x1b[96m"
	ColorBrightGray    color = "\x1b[37m"
	ColorBrightGreen   color = "\x1b[92m"
	ColorBrightMagenta color = "\x1b[95m"
	ColorBrightOrange  color = "\x1b[38;5;214m"
	ColorBrightPurple  color = "\x1b[38;5;135m"
	ColorBrightRed     color = "\x1b[91m"
	ColorBrightWhite   color = "\x1b[97m"
	ColorBrightYellow  color = "\x1b[93m"

	// styles.
	StyleBold          color = "\x1b[1m"
	StyleDim           color = "\x1b[2m"
	StyleInverse       color = "\x1b[7m"
	StyleItalic        color = "\x1b[3m"
	StyleStrikethrough color = "\x1b[9m"
	StyleUnderline     color = "\x1b[4m"
	StyleBlink         color = "\x1b[5m"

	// reset.
	reset color = "\x1b[0m"
)

var defaultBorders = &FrameBorders{
	Header: "+ ",
	Row:    "| ",
	Mid:    "+ ",
	Footer: "+ ",
}

// OptFn is an option function for the frame.
type OptFn func(*Options)

type FrameBorders struct {
	Header, Row, Mid, Footer string
}

type Options struct {
	Border *FrameBorders
	color  color
	text   []string
	icon   *icon
}

type Frame struct {
	Options
}

type icon struct {
	error    string
	info     string
	question string
	success  string
	warning  string
}

// defaultOpts returns the default frame options.
func defaultOpts() Options {
	return Options{
		Border: defaultBorders,
		color:  "",
		text:   make([]string, 0),
		icon: &icon{
			error:    "✗",
			info:     "i",
			question: "?",
			success:  "✓",
			warning:  "!",
		},
	}
}

func WithColorBorder(c ...color) OptFn {
	colorEnabled = true

	return func(o *Options) {
		var sb strings.Builder
		for _, clr := range c {
			sb.WriteString(string(clr))
		}
		o.color = color(sb.String())
	}
}

func WithIconSuccess(i string) OptFn {
	return func(o *Options) {
		o.icon.success = i
	}
}

func (f *Frame) Text(t ...string) *Frame {
	f.text = append(f.text, t...)
	return f
}

func (f *Frame) Textln(t ...string) *Frame {
	f.text = append(f.text, t...)
	return f.Ln()
}

// Ln adds a new line.
func (f *Frame) Ln() *Frame {
	return f.Text("\n")
}

func (f *Frame) applyStyle(s string) string {
	if f.color != "" {
		return string(f.color) + s + string(reset)
	}

	return s
}

// applyBorderGeneric applies a border to the first element,
// renders intermediate lines as Row, and optionally the last one as footer.
func (f *Frame) applyBorderGeneric(border string, s []string, footer bool) *Frame {
	n := len(s)
	if n == 0 {
		return f.Text(border, "")
	}

	// first line
	f.Text(border, s[0])

	if n == 1 {
		return f
	}

	// middle lines
	limit := n
	if footer {
		limit = n - 1
	}
	for _, line := range s[1:limit] {
		f.Ln().Row(line)
	}

	// last line
	if footer {
		f.Ln().Mid(s[n-1])
	}

	return f
}

// applyBorder applies the border to the first element. The rest elements are Row.
func (f *Frame) applyBorder(border string, s []string) *Frame {
	return f.applyBorderGeneric(border, s, false)
}

// applyFooterBorder applies the border to the first element,
// and centers the last line.
func (f *Frame) applyFooterBorder(border string, s []string) *Frame {
	return f.applyBorderGeneric(border, s, true)
}

func (f *Frame) Header(s ...string) *Frame {
	header := f.applyStyle(f.Border.Header)
	return f.applyBorder(header, s)
}

func (f *Frame) Headerln(s ...string) *Frame {
	return f.Header(s...).Ln()
}

func (f *Frame) Row(s ...string) *Frame {
	row := f.applyStyle(f.Border.Row)
	return f.applyBorder(row, s)
}

func (f *Frame) Rowln(s ...string) *Frame {
	return f.Row(s...).Ln()
}

func (f *Frame) Mid(s ...string) *Frame {
	mid := f.applyStyle(f.Border.Mid)
	return f.applyBorder(mid, s)
}

func (f *Frame) Midln(s ...string) *Frame {
	return f.Mid(s...).Ln()
}

func (f *Frame) Footer(s ...string) *Frame {
	foo := f.applyStyle(f.Border.Footer)
	return f.applyFooterBorder(foo, s)
}

func (f *Frame) Footerln(s ...string) *Frame {
	return f.Footer(s...).Ln()
}

func (f *Frame) Flush() *Frame {
	fmt.Print(strings.Join(f.text, ""))
	return f.Reset()
}

// Reset clears the frame.
func (f *Frame) Reset() *Frame {
	f.text = make([]string, 0)
	return f
}

func (f *Frame) Error(s ...string) *Frame {
	mid := f.applyStyle(applyColorAndBold(ColorBrightRed, f.icon.error))
	return f.applyBorder(mid, s)
}

func (f *Frame) Warning(s ...string) *Frame {
	mid := f.applyStyle(applyColorAndBold(ColorBrightYellow, f.icon.warning))
	return f.applyBorder(mid, s)
}

func (f *Frame) Success(s ...string) *Frame {
	mid := f.applyStyle(applyColorAndBold(ColorBrightGreen, f.icon.success))
	return f.applyBorder(mid, s)
}

func (f *Frame) Info(s ...string) *Frame {
	mid := f.applyStyle(applyColorAndBold(ColorBrightBlue, f.icon.info))
	return f.applyBorder(mid, s)
}

func (f *Frame) Question(s string) *Frame {
	mid := f.applyStyle(applyColorAndBold(ColorBrightGreen, f.icon.question))
	return f.applyBorder(mid, []string{string(StyleBold) + s + string(reset)})
}

func (f *Frame) String() string {
	return strings.Join(f.text, "")
}

func (f *Frame) StringReset() string {
	s := f.String()
	f.Reset()

	return s
}

// Write implements the io.Writer interface.
func (f *Frame) Write(p []byte) (int, error) {
	defer f.Flush()

	content := string(p)
	// Handle carriage returns by splitting on \r and taking the last part
	if strings.Contains(content, "\r") {
		lines := strings.Split(content, "\r")
		// Only process the last line after \r (this simulates overwriting)
		content = lines[len(lines)-1]
	}

	// Collect all non-empty lines first
	var lines []string

	for line := range strings.SplitSeq(content, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}

	for i, line := range lines {
		if i == len(lines)-1 {
			f.Footerln(line)
		} else {
			f.Rowln(line)
		}
	}

	return len(p), nil
}

// New returns a new frame.
func New(opts ...OptFn) *Frame {
	o := defaultOpts()
	for _, fn := range opts {
		fn(&o)
	}

	return &Frame{
		Options: o,
	}
}

func applyColorAndBold(c color, s ...string) string {
	f := strings.Join(s, " ")
	if !colorEnabled {
		return f + " "
	}
	return fmt.Sprintf("%s%s%s %s", StyleBold, c, f, reset)
}
