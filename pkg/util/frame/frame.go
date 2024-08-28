package frame

import (
	"fmt"
	"strings"

	"github.com/haaag/gm/pkg/format/color"
)

// OptFn is an option function for the frame.
type OptFn func(*Options)

type FrameBorders struct {
	Header, Row, Mid, Footer string
}

type Options struct {
	Border   *FrameBorders
	color    color.ColorFn
	text     []string
	maxWidth int
}

type Frame struct {
	Options
}

// defaultOpts returns the default frame options.
func defaultOpts() Options {
	return Options{
		Border:   defaultBorders,
		color:    nil,
		text:     make([]string, 0),
		maxWidth: 80,
	}
}

func WithColorBorder(c color.ColorFn) OptFn {
	return func(o *Options) {
		o.color = c
	}
}

func WithMaxWidth(n int) OptFn {
	return func(o *Options) {
		o.maxWidth = n
	}
}

func (f *Frame) Text(t ...string) *Frame {
	f.text = append(f.text, t...)
	return f
}

func (f *Frame) Newline() *Frame {
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
		return f.Text(border, "", "\n")
	}

	// append first element
	f.Text(border, s[0], "\n")
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

func (f *Frame) Clean() {
	f.text = make([]string, 0)
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
