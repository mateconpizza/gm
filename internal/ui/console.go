package ui

import (
	"io"

	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui/color"
	"github.com/mateconpizza/gm/internal/ui/frame"
)

type Console struct {
	T *terminal.Term
	F *frame.Frame
}

// ConsoleOpt is a function type for configuring Console.
type ConsoleOpt func(*Console)

// NewConsole creates a new Console with the given options.
func NewConsole(opts ...ConsoleOpt) *Console {
	c := &Console{}
	for _, opt := range opts {
		opt(c)
	}

	if c.T == nil {
		c.T = terminal.New()
	}

	if c.F == nil {
		c.F = frame.New()
	}

	return c
}

// WithTerminal sets a custom terminal.
func WithTerminal(t *terminal.Term) ConsoleOpt {
	return func(c *Console) {
		c.T = t
	}
}

// WithFrame sets a custom frame.
func WithFrame(f *frame.Frame) ConsoleOpt {
	return func(c *Console) {
		c.F = f
	}
}

// ConfirmErr prompts the user with a question and options.
func (c *Console) ConfirmErr(q, def string) error {
	return c.T.ConfirmErr(c.F.Reset().Question(q).StringReset(), def)
}

func (c *Console) Confirm(q, def string) bool {
	return c.T.Confirm(c.F.Reset().Question(q).StringReset(), def)
}

func (c *Console) Choose(q string, opts []string, def string) (string, error) {
	return c.T.Choose(c.F.Reset().Question(q).StringReset(), opts, def)
}

func (c *Console) Input(p string) string {
	return c.T.Input(c.F.Reset().Info(p).StringReset())
}

func (c *Console) InputPassword(s string) (string, error) {
	c.F.Reset().Question(s).Flush()
	return c.T.InputPassword()
}

// Prompt get the input data from the user and return it.
func (c *Console) Prompt(p string) string {
	return c.T.Prompt(c.F.Reset().Question(p).StringReset())
}

func (c *Console) PromptWithSuggestions(p string, items []string) string {
	return c.T.PromptWithSuggestions(p, items)
}

func (c *Console) ReplaceLine(s string) {
	c.T.ReplaceLine(1, s)
}

func (c *Console) ReplaceLines(n int, s string) {
	c.T.ReplaceLine(n, s)
}

func (c *Console) ClearLine(n int) {
	c.T.ClearLine(n)
}

func (c *Console) SetReader(r io.Reader) {
	c.T.SetReader(r)
}

func (c *Console) SetWriter(w io.Writer) {
	c.T.SetWriter(w)
}

// SuccessMesg returns a prettified success message.
func (c *Console) SuccessMesg(s string) string {
	success := color.BrightGreen("Successfully ").Italic().String()
	message := success + color.Text(s).Italic().String()

	return c.F.Reset().Success(message).StringReset()
}

func (c *Console) Success(s string) *frame.Frame {
	return c.F.Reset().Success(s)
}

// ErrorMesg returns a prettified error message.
func (c *Console) ErrorMesg(s string) string {
	err := color.BrightRed("Error ").Italic().String()
	message := err + color.Text(s).Italic().String()

	return c.F.Reset().Error(message).StringReset()
}

func (c *Console) Error(s string) *frame.Frame {
	return c.F.Reset().Error(s)
}

// WarningMesg returns a prettified warning message.
func (c *Console) WarningMesg(s string) string {
	warning := color.BrightYellow("Warning ").Italic().String()
	message := warning + color.Text(s).Italic().String()

	return c.F.Reset().Warning(message).StringReset()
}

func (c *Console) Warning(s string) *frame.Frame {
	return c.F.Reset().Warning(s)
}

// InfoMesg returns a prettified info message.
func (c *Console) InfoMesg(s string) string {
	info := color.BrightBlue("Info ").Italic().String()
	message := info + color.Text(s).Italic().String()

	return c.F.Reset().Info(message).StringReset()
}

func (c *Console) Info(s string) *frame.Frame {
	return c.F.Reset().Info(s)
}
