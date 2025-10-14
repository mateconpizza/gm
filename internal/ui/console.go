// Package ui provides console interaction utilities with styled output.
// It wraps terminal operations with colored frames and user prompts.
package ui

import (
	"context"
	"io"

	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui/color"
	"github.com/mateconpizza/gm/internal/ui/frame"
)

type Console struct {
	Term  *terminal.Term
	Frame *frame.Frame
}

// Option is a function type for configuring Console.
type Option func(*Console)

// NewConsole creates a new Console with the given options.
func NewConsole(opts ...Option) *Console {
	c := &Console{}
	for _, opt := range opts {
		opt(c)
	}

	if c.Term == nil {
		c.Term = terminal.New()
	}

	if c.Frame == nil {
		c.Frame = frame.New()
	}

	return c
}

func NewDefaultConsole(ctx context.Context, f func(error)) *Console {
	return NewConsole(
		WithFrame(frame.New(frame.WithColorBorder(color.Gray))),
		WithDefaultTerminal(ctx, f),
	)
}

// WithTerminal sets a custom terminal.
func WithTerminal(t *terminal.Term) Option {
	return func(c *Console) {
		c.Term = t
	}
}

// WithFrame sets a custom frame.
func WithFrame(f *frame.Frame) Option {
	return func(c *Console) {
		c.Frame = f
	}
}

func WithDefaultTerminal(ctx context.Context, f func(error)) Option {
	return WithTerminal(terminal.New(
		terminal.WithContext(ctx),
		terminal.WithInterruptFn(f),
	))
}

// ConfirmErr prompts the user with a question and options.
func (c *Console) ConfirmErr(q, def string) error {
	return c.Term.ConfirmErr(c.Frame.Reset().Question(q).StringReset(), def)
}

func (c *Console) Confirm(q, def string) bool {
	return c.Term.Confirm(c.Frame.Reset().Question(q).StringReset(), def)
}

func (c *Console) Choose(q string, opts []string, def string) (string, error) {
	return c.Term.Choose(c.Frame.Reset().Question(q).StringReset(), opts, def)
}

func (c *Console) Input(p string) string {
	return c.Term.Input(c.Frame.Reset().Info(p).StringReset())
}

func (c *Console) InputPassword(s string) (string, error) {
	c.Frame.Reset().Question(s).Flush()
	return c.Term.InputPassword()
}

// Prompt get the input data from the user and return it.
func (c *Console) Prompt(p string) string {
	return c.Term.Prompt(c.Frame.Reset().Question(p).StringReset())
}

func (c *Console) PromptWithSuggestions(p string, items []string) string {
	return c.Term.PromptWithSuggestions(p, items)
}

func (c *Console) ReplaceLine(s string) {
	c.Term.ReplaceLine(1, s)
}

func (c *Console) ReplaceLines(n int, s string) {
	c.Term.ReplaceLine(n, s)
}

func (c *Console) ClearLine(n int) {
	c.Term.ClearLine(n)
}

func (c *Console) SetReader(r io.Reader) {
	c.Term.SetReader(r)
}

func (c *Console) SetWriter(w io.Writer) {
	c.Term.SetWriter(w)
}

// SuccessMesg returns a prettified success message.
func (c *Console) SuccessMesg(s string) string {
	success := color.BrightGreen("Successfully ").Italic().String()
	message := success + color.Text(s).Italic().String()

	return c.Frame.Reset().Success(message).StringReset()
}

func (c *Console) Success(s string) *frame.Frame {
	return c.Frame.Reset().Success(s)
}

// ErrorMesg returns a prettified error message.
func (c *Console) ErrorMesg(s string) string {
	err := color.BrightRed("Error ").Italic().String()
	message := err + color.Text(s).Italic().String()

	return c.Frame.Reset().Error(message).StringReset()
}

func (c *Console) Error(s string) *frame.Frame {
	return c.Frame.Reset().Error(s)
}

// WarningMesg returns a prettified warning message.
func (c *Console) WarningMesg(s string) string {
	warning := color.BrightYellow("Warning ").Italic().String()
	message := warning + color.Text(s).Italic().String()

	return c.Frame.Reset().Warning(message).StringReset()
}

func (c *Console) Warning(s string) *frame.Frame {
	return c.Frame.Reset().Warning(s)
}

// InfoMesg returns a prettified info message.
func (c *Console) InfoMesg(s string) string {
	info := color.BrightBlue("Info ").Italic().String()
	message := info + color.Text(s).Italic().String()

	return c.Frame.Reset().Info(message).StringReset()
}

func (c *Console) Info(s string) *frame.Frame {
	return c.Frame.Reset().Info(s)
}
