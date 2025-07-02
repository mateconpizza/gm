// Package frame provides a customizable text framing and styling utility for
// console output, including borders and icons.
package frame

import (
	"fmt"
	"strings"

	"github.com/mateconpizza/gm/internal/ui/color"
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
	color  color.ColorFn
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
		color:  nil,
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

func WithColorBorder(c color.ColorFn) OptFn {
	return func(o *Options) {
		o.color = c
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
	if f.color != nil {
		return f.color(s).String()
	}

	return s
}

// applyBorder applies the border to the first element. The rest elements are
// Row.
func (f *Frame) applyBorder(border string, s []string) *Frame {
	n := len(s)
	if n == 0 {
		return f.Text(border, "")
	}
	// append first element
	f.Text(border, s[0])

	if n == 1 {
		return f
	}
	// the rest as Row
	for _, line := range s[1:] {
		f.Ln().Row(line)
	}

	return f
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
	return f.applyBorder(foo, s)
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
	e := color.BrightRed(f.icon.error + " ").Bold().String()
	mid := f.applyStyle(e)

	return f.applyBorder(mid, s)
}

func (f *Frame) Warning(s ...string) *Frame {
	e := color.BrightYellow(f.icon.warning + " ").Bold().String()
	mid := f.applyStyle(e)

	return f.applyBorder(mid, s)
}

func (f *Frame) Success(s ...string) *Frame {
	e := color.BrightGreen(f.icon.success + " ").Bold().String()
	mid := f.applyStyle(e)

	return f.applyBorder(mid, s)
}

func (f *Frame) Info(s ...string) *Frame {
	e := color.BrightBlue(f.icon.info + " ").Bold().String()
	mid := f.applyStyle(e)

	return f.applyBorder(mid, s)
}

func (f *Frame) Question(s ...string) *Frame {
	q := color.BrightGreen(f.icon.question + " ").Bold().String()
	mid := f.applyStyle(q)

	return f.applyBorder(mid, color.ApplyMany(s, color.StyleBold))
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
