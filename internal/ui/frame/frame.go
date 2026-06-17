// Package frame provides a customizable text framing and styling utility for
// console output, including borders and icons.
package frame

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
)

var (
	colorMutex   sync.Mutex
	colorEnabled bool = true
)

// Color defines the minimal interface for coloring text.
type Color interface {
	Sprint(args ...any) string
}

type IconStyle struct {
	Symbol string
	Color  Color // Applies color/style
}

type Icons struct {
	Error    IconStyle
	Info     IconStyle
	Question IconStyle
	Success  IconStyle
	Warning  IconStyle
}

// OptFn is an option function for the frame.
type OptFn func(*Options)

type FrameBorders struct {
	Header, Row, Mid, Footer string
}

// Options represents the configuration options for a Frame.
type Options struct {
	border      *FrameBorders
	borderColor Color
	text        []string
	icons       *Icons
	writer      io.Writer
}

type Frame struct {
	Options
	buf string
}

// defaultOpts returns the default configuration options.
func defaultOpts() Options {
	return Options{
		border:      defaultBorder,
		borderColor: nil,
		text:        make([]string, 0),
		icons: &Icons{
			Error:    IconStyle{Symbol: "✗"},
			Info:     IconStyle{Symbol: "i"},
			Question: IconStyle{Symbol: "?"},
			Success:  IconStyle{Symbol: "✓"},
			Warning:  IconStyle{Symbol: "!"},
		},
	}
}

func WithColorBorder(c Color) OptFn {
	return func(o *Options) {
		if !colorEnabled {
			o.borderColor = nil
			return
		}

		o.borderColor = c
	}
}

func WithWriter(w io.Writer) OptFn {
	return func(o *Options) {
		o.writer = w
	}
}

// WithIcons creates an option function to customize icons in the Frame.
func WithIcons(i *Icons) OptFn {
	return func(o *Options) {
		o.icons = i
	}
}

func WithBorders(b *FrameBorders) OptFn {
	return func(o *Options) {
		o.border = b
	}
}

// Ln adds a new line.
func (f *Frame) Ln() *Frame { return f.Text("\n") }

func (f *Frame) Text(t ...string) *Frame {
	f.text = append(f.text, t...)
	return f
}

func (f *Frame) Textln(t ...string) *Frame {
	f.text = append(f.text, t...)
	return f.Ln()
}

// build handles the core logic of applying styles/colors and formatting the border.
func (f *Frame) build(borderStr string, c Color, s []string, applyFn func(string, []string) *Frame) *Frame {
	if c == nil {
		return applyFn(f.applyStyle(borderStr), s)
	}
	return applyFn(c.Sprint(borderStr), s)
}

func (f *Frame) Header(s ...string) *Frame             { return f.HeaderC(nil, s...) }
func (f *Frame) Headerln(s ...string) *Frame           { return f.HeaderC(nil, s...).Ln() }
func (f *Frame) HeaderCln(c Color, s ...string) *Frame { return f.HeaderC(c, s...).Ln() }
func (f *Frame) HeaderC(c Color, s ...string) *Frame   { return f.build(f.border.Header, c, s, f.apply) }
func (f *Frame) Row(s ...string) *Frame                { return f.RowC(nil, s...) }
func (f *Frame) Rowln(s ...string) *Frame              { return f.RowC(nil, s...).Ln() }
func (f *Frame) RowCln(c Color, s ...string) *Frame    { return f.RowC(c, s...).Ln() }
func (f *Frame) RowC(c Color, s ...string) *Frame      { return f.build(f.border.Row, c, s, f.apply) }
func (f *Frame) Mid(s ...string) *Frame                { return f.MidC(nil, s...) }
func (f *Frame) Midln(s ...string) *Frame              { return f.MidC(nil, s...).Ln() }
func (f *Frame) MidCln(c Color, s ...string) *Frame    { return f.MidC(c, s...).Ln() }
func (f *Frame) MidC(c Color, s ...string) *Frame      { return f.build(f.border.Mid, c, s, f.apply) }
func (f *Frame) Footer(s ...string) *Frame             { return f.FooterC(nil, s...) }
func (f *Frame) Footerln(s ...string) *Frame           { return f.FooterC(nil, s...).Ln() }
func (f *Frame) FooterCln(c Color, s ...string) *Frame { return f.FooterC(c, s...).Ln() }
func (f *Frame) FooterC(c Color, s ...string) *Frame {
	return f.build(f.border.Footer, c, s, f.applyFooter)
}

func (f *Frame) Error(s ...string) *Frame   { return f.applyIcon(f.icons.Error, s) }
func (f *Frame) Warning(s ...string) *Frame { return f.applyIcon(f.icons.Warning, s) }
func (f *Frame) Success(s ...string) *Frame { return f.applyIcon(f.icons.Success, s) }
func (f *Frame) Info(s ...string) *Frame    { return f.applyIcon(f.icons.Info, s) }
func (f *Frame) Question(s string) *Frame   { return f.applyIcon(f.icons.Question, []string{s}) }

// CustomBorderFunc defines a function that returns a border string.
type CustomBorderFunc func() string

// Custom applies a custom border string to the provided lines.
func (f *Frame) Custom(border string, s ...string) *Frame { return f.apply(border, s) }

// CustomFunc applies a dynamically generated border to the provided lines.
func (f *Frame) CustomFunc(fn CustomBorderFunc, s ...string) *Frame {
	if fn == nil {
		return f.apply("", s)
	}

	return f.apply(fn(), s)
}

// Write implements the io.Writer interface.
func (f *Frame) Write(p []byte) (int, error) {
	content := f.buf + string(p)
	lines := strings.Split(content, "\n")

	// last element may be incomplete
	f.buf = lines[len(lines)-1]
	lines = lines[:len(lines)-1]

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		f.Rowln(line)
	}

	return len(p), nil
}

func (f *Frame) SetWriter(w io.Writer)  { f.writer = w }
func (f *Frame) SetBorders(opt OptFn)   { opt(&f.Options) }
func (f *Frame) Borders() *FrameBorders { return f.border }

// Flush writes the current text content of the Frame to the writer and resets
// it.
func (f *Frame) Flush() *Frame {
	// if something is left in the buffer, print it now as footer
	if line := strings.TrimSpace(f.buf); line != "" {
		f.Footerln(line)
		f.buf = ""
	}
	fmt.Fprint(f.writer, strings.Join(f.text, ""))
	f.text = nil
	return f
}

// Reset clears the frame.
func (f *Frame) Reset() *Frame {
	f.text = make([]string, 0)
	return f
}

// String returns the current text content of the Frame.
func (f *Frame) String() string { return strings.Join(f.text, "") }

// StringReset returns the current text content and resets the Frame.
func (f *Frame) StringReset() string {
	s := f.String()
	f.Reset()

	return s
}

// Bytes returns the frame content as a byte slice.
func (f *Frame) Bytes() []byte { return fmt.Appendf(nil, `%s`, f.StringReset()) }

func (f *Frame) applyStyle(s string) string {
	colorMutex.Lock()
	defer colorMutex.Unlock()

	if colorEnabled && f.borderColor != nil {
		return f.borderColor.Sprint(s)
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

// apply applies the border to the first element. The rest elements are Row.
func (f *Frame) apply(border string, s []string) *Frame {
	return f.applyBorderGeneric(border, s, false)
}

// applyFooter applies the border to the first element,
// and centers the last line.
func (f *Frame) applyFooter(border string, s []string) *Frame {
	return f.applyBorderGeneric(border, s, true)
}

func (f *Frame) applyIcon(icon IconStyle, s []string) *Frame {
	mid := f.applyStyle(f.formatIcon(icon))
	return f.apply(mid, s)
}

// Helper method to format an icon with its color.
func (f *Frame) formatIcon(style IconStyle) string {
	colorMutex.Lock()
	defer colorMutex.Unlock()

	if !colorEnabled || style.Color == nil {
		return style.Symbol + " "
	}

	return style.Color.Sprint(style.Symbol) + " "
}

// New creates a new Frame instance with the provided options.
func New(opts ...OptFn) *Frame {
	o := defaultOpts()
	for _, fn := range opts {
		fn(&o)
	}

	if o.writer == nil {
		o.writer = os.Stdout
	}

	return &Frame{
		Options: o,
	}
}

// NewIcons creates a new Icons instance with custom icon values.
func NewIcons() *Icons {
	return &Icons{}
}

// DisableColor disables text coloring globally.
func DisableColor() {
	colorMutex.Lock()
	defer colorMutex.Unlock()
	colorEnabled = false
}
