package frame

import (
	"fmt"
	"strings"

	"github.com/haaag/gm/internal/format/color"
)

// OptFn is an option function for the frame.
type OptFn func(*Options)

type FrameBorders struct {
	Header, Row, Mid, Footer string
}

type Options struct {
	Border  *FrameBorders
	color   color.ColorFn
	text    []string
	newLine bool
}

type Frame struct {
	Options
}

// defaultOpts returns the default frame options.
func defaultOpts() Options {
	return Options{
		Border:  defaultBorders,
		color:   nil,
		text:    make([]string, 0),
		newLine: true,
	}
}

func WithColorBorder(c color.ColorFn) OptFn {
	return func(o *Options) {
		o.color = c
	}
}

func WithNoNewLine() OptFn {
	return func(o *Options) {
		o.newLine = false
	}
}

func (f *Frame) Text(t ...string) *Frame {
	f.text = append(f.text, t...)
	return f
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
		if f.newLine {
			return f.Text(border, "", "\n")
		}

		return f.Text(border, "")
	}

	// append first element
	if f.newLine {
		f.Text(border, s[0], "\n")
	} else {
		f.Text(border, s[0])
	}

	if n == 1 {
		return f
	}

	// the rest as Row
	for _, line := range s[1:] {
		f.Row(line)
	}

	return f
}

func (f *Frame) Header(s ...string) *Frame {
	header := f.applyStyle(f.Border.Header)
	return f.applyBorder(header, s)
}

func (f *Frame) Row(s ...string) *Frame {
	row := f.applyStyle(f.Border.Row)
	return f.applyBorder(row, s)
}

func (f *Frame) Mid(s ...string) *Frame {
	mid := f.applyStyle(f.Border.Mid)
	return f.applyBorder(mid, s)
}

func (f *Frame) Footer(s ...string) *Frame {
	foo := f.applyStyle(f.Border.Footer)
	return f.applyBorder(foo, s)
}

func (f *Frame) Render() {
	fmt.Print(strings.Join(f.text, ""))
}

// Clean clears the frame.
func (f *Frame) Clean() *Frame {
	f.text = make([]string, 0)
	return f
}

func (f *Frame) String() string {
	return strings.Join(f.text, "")
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
