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
	Border *FrameBorders
	color  Color
	text   []string
	Icons  *Icons
	writer io.Writer
}

type Frame struct {
	Options
	buf string
}

// defaultOpts returns the default configuration options.
func defaultOpts() Options {
	return Options{
		Border: &FrameBorders{
			Header: "+ ",
			Row:    "| ",
			Mid:    "+ ",
			Footer: "+ ",
		},
		color: nil,
		text:  make([]string, 0),
		Icons: &Icons{
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
			o.color = nil
			return
		}

		o.color = c
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
		o.Icons = i
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
	colorMutex.Lock()
	defer colorMutex.Unlock()

	if colorEnabled && f.color != nil {
		return f.color.Sprint(s)
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

// Header adds a styled header row to the Frame.
func (f *Frame) Header(s ...string) *Frame {
	header := f.applyStyle(f.Border.Header)
	return f.applyBorder(header, s)
}

// Headerln adds a styled header row with a newline to the Frame.
func (f *Frame) Headerln(s ...string) *Frame {
	return f.Header(s...).Ln()
}

// Row adds a styled row to the Frame.
func (f *Frame) Row(s ...string) *Frame {
	row := f.applyStyle(f.Border.Row)
	return f.applyBorder(row, s)
}

// Rowln adds a styled row with a newline to the Frame.
func (f *Frame) Rowln(s ...string) *Frame {
	return f.Row(s...).Ln()
}

// Mid adds a styled mid-row to the Frame.
func (f *Frame) Mid(s ...string) *Frame {
	mid := f.applyStyle(f.Border.Mid)
	return f.applyBorder(mid, s)
}

// Midln adds a styled mid-row with a newline to the Frame.
func (f *Frame) Midln(s ...string) *Frame {
	return f.Mid(s...).Ln()
}

// Footer adds a styled footer row to the Frame.
func (f *Frame) Footer(s ...string) *Frame {
	foo := f.applyStyle(f.Border.Footer)
	return f.applyFooterBorder(foo, s)
}

// Footerln adds a styled footer row with a newline to the Frame.
func (f *Frame) Footerln(s ...string) *Frame {
	return f.Footer(s...).Ln()
}

// Flush writes the current text content of the Frame to the writer and resets
// it.
func (f *Frame) Flush() *Frame {
	// if something is left in the buffer, print it now as footer
	if strings.TrimSpace(f.buf) != "" {
		f.Footerln(strings.TrimSpace(f.buf))
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

// Helper method to format an icon with its color.
func (f *Frame) formatIcon(style IconStyle) string {
	colorMutex.Lock()
	defer colorMutex.Unlock()

	if !colorEnabled || style.Color == nil {
		return style.Symbol + " "
	}

	return style.Color.Sprint(style.Symbol) + " "
}

// Error logs styled error messages with custom icons.
func (f *Frame) Error(s ...string) *Frame {
	mid := f.applyStyle(f.formatIcon(f.Icons.Error))
	return f.applyBorder(mid, s)
}

// Warning logs styled warning messages with custom icons.
func (f *Frame) Warning(s ...string) *Frame {
	mid := f.applyStyle(f.formatIcon(f.Icons.Warning))
	return f.applyBorder(mid, s)
}

// Success logs styled success messages with custom icons.
func (f *Frame) Success(s ...string) *Frame {
	mid := f.applyStyle(f.formatIcon(f.Icons.Success))
	return f.applyBorder(mid, s)
}

// Info logs styled informational messages with custom icons.
func (f *Frame) Info(s ...string) *Frame {
	mid := f.applyStyle(f.formatIcon(f.Icons.Info))
	return f.applyBorder(mid, s)
}

// Question logs styled question messages with custom icons.
func (f *Frame) Question(s string) *Frame {
	mid := f.applyStyle(f.formatIcon(f.Icons.Question))
	return f.applyBorder(mid, []string{s})
}

// String returns the current text content of the Frame.
func (f *Frame) String() string {
	return strings.Join(f.text, "")
}

// StringReset returns the current text content and resets the Frame.
func (f *Frame) StringReset() string {
	s := f.String()
	f.Reset()

	return s
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
