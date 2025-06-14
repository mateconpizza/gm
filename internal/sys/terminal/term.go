package terminal

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"slices"
	"strings"
	"syscall"

	prompt "github.com/c-bata/go-prompt"
	"golang.org/x/term"

	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/ui/color"
)

// TODO:
// - [ ] check `CancelInterruptHandler` implementation.
// - [ ] check `IsPiped` implementation

var (
	hl  = color.BrightGreen
	dim = color.Gray
)

const termPromptPrefix = "> "

// defaultInterruptFn is the default interrupt function for the terminal.
func defaultInterruptFn(err error) {}

// TermOptFn is an option function for the terminal.
type TermOptFn func(*Options)

// Options represents the options for the terminal.
type Options struct {
	reader      io.Reader
	writer      io.Writer
	PromptStr   string
	InterruptFn func(error)
}

// Term is a struct that represents a terminal.
type Term struct {
	Options
	cancelFn context.CancelFunc
}

// defaultOpts returns the default terminal options.
func defaultOpts() Options {
	return Options{
		reader:    os.Stdin,
		writer:    os.Stdout,
		PromptStr: termPromptPrefix,
	}
}

// WithReader sets the reader for the terminal.
func WithReader(r io.Reader) TermOptFn {
	return func(o *Options) {
		o.reader = r
	}
}

// WithWriter sets the writer for the terminal.
func WithWriter(w io.Writer) TermOptFn {
	return func(o *Options) {
		o.writer = w
	}
}

// WithInterruptFn sets the interrupt function for the terminal.
func WithInterruptFn(fn func(error)) TermOptFn {
	return func(o *Options) {
		o.InterruptFn = fn
	}
}

// SetReader sets the reader for the terminal.
func (t *Term) SetReader(r io.Reader) {
	t.reader = r
}

// SetInterruptFn sets the interrupt function for the terminal.
func (t *Term) SetInterruptFn(fn func(error)) {
	slog.Info("setting interrupt function")
	if t.InterruptFn != nil {
		t.CancelInterruptHandler()
	}
	t.InterruptFn = fn

	ctx, cancel := context.WithCancel(context.Background())
	t.cancelFn = cancel
	setupInterruptHandler(ctx, t.InterruptFn)
}

// Input get the Input data from the user and return it.
func (t *Term) Input(p string) string {
	o, restore := prepareInputState(t.InterruptFn)
	defer restore()
	s := prompt.Input(p, completerDummy(), o...)

	return s
}

func (t *Term) InputPassword() (string, error) {
	// if not a terminal (testing or piped input) - read from configured reader
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		var password string
		if _, err := fmt.Fscanln(t.reader, &password); err != nil {
			return "", fmt.Errorf("reading password: %w", err)
		}

		return password, nil
	}

	if err := saveState(); err != nil {
		return "", err
	}

	t.SetInterruptFn(func(err error) {
		if err := restoreState(); err != nil {
			slog.Error("restoring state", "error", err)
		}
		sys.ErrAndExit(err)
	})

	defer func() {
		if err := restoreState(); err != nil {
			slog.Error("restoring state", "error", err)
		}
	}()

	// Use stdin file descriptor for reading password securely
	fd := int(os.Stdin.Fd())
	p, err := term.ReadPassword(fd)
	if err != nil {
		return "", fmt.Errorf("reading password: %w", err)
	}

	return string(p), nil
}

// Prompt get the input data from the user and return it.
func (t *Term) Prompt(p string) string {
	r := bufio.NewReader(t.reader)
	fmt.Print(p)
	s, _ := r.ReadString('\n')

	return strings.TrimSpace(s)
}

// PromptWithSuggestions prompts the user for input with suggestions based on
// the provided items.
func (t *Term) PromptWithSuggestions(p string, items []string) string {
	return inputWithSuggestions(p, items, t.InterruptFn)
}

// PromptWithFuzzySuggestions prompts the user for input with fuzzy suggestions.
func (t *Term) PromptWithFuzzySuggestions(p string, items []string) string {
	return inputWithFuzzySuggestions(p, items, t.InterruptFn)
}

// ChooseTags prompts the user for input with suggestions based on
// the provided tags.
func (t *Term) ChooseTags(p string, items map[string]int) string {
	return inputWithTags(p, items, t.InterruptFn)
}

// Confirm prompts the user with a question and options.
func (t *Term) Confirm(q, def string) bool {
	err := t.ConfirmErr(q, def)
	if err != nil {
		slog.Debug("terminal confirm", "err", err)
	}

	return err == nil
}

// ConfirmErr prompts the user with a question and options.
func (t *Term) ConfirmErr(q, def string) error {
	if len(def) > 1 {
		// get first char
		def = def[:1]
	}

	opts := []string{"y", "n"}
	if !slices.Contains(opts, def) {
		def = "n"
	}

	choices := fmtChoicesWithDefault(opts, def)
	for i := range len(choices) {
		choices[i] = dim(choices[i]).String()
	}

	chosen, err := t.promptWithChoicesErr(q, choices, def)
	if err != nil {
		return err
	}

	if !strings.EqualFold(chosen, "y") {
		return ErrActionAborted
	}

	return nil
}

// Choose prompts the user to enter one of the given options.
func (t *Term) Choose(q string, opts []string, def string) (string, error) {
	for i := range opts {
		opts[i] = strings.ToLower(opts[i])
	}
	opts = fmtChoicesWithDefaultColor(opts, def)

	return t.promptWithChoicesErr(q, opts, def)
}

// promptWithChoices prompts the user to enter one of the given options.
func (t *Term) promptWithChoicesErr(q string, opts []string, def string) (string, error) {
	sep := dim("/").String()
	s := dim("[").String()
	e := dim("]:").String()

	p := buildPrompt(q, fmt.Sprintf("%s%s%s", s, strings.Join(opts, sep), e))
	return getUserInputWithAttempts(t.reader, t.writer, p, opts, def)
}

// ClearLine deletes n lines in the console.
func (t *Term) ClearLine(n int) {
	if !t.isInteractiveTerminal(n) {
		slog.Debug("clearing line", "error", ErrNotInteractive)
		return
	}
	ClearLine(n)
}

// ReplaceLine deletes n lines in the console and prints the given string.
func (t *Term) ReplaceLine(n int, s string) {
	if !t.isInteractiveTerminal(n) {
		slog.Error("error replacing line", "error", ErrNotInteractive)
		return
	}

	ReplaceLine(n, s)
}

// ClearChars deletes n characters in the console.
func (t *Term) ClearChars(n int) {
	if !t.isInteractiveTerminal(n) {
		slog.Error("error clearing chars", "error", ErrNotInteractive)
		return
	}
	ClearChars(n)
}

// Clear clears the terminal.
func (t *Term) Clear() {
	if !t.isInteractiveTerminal(1) {
		slog.Error("error clearing the term", "error", ErrNotInteractive)
		return
	}
	clearTerminal()
}

// CancelInterruptHandler cancels the interrupt handler.
func (t *Term) CancelInterruptHandler() {
	if t.cancelFn != nil {
		slog.Warn("cancelling interrupt handler")
		t.cancelFn()
	}
}

// IsPiped returns true if the terminal input is piped.
func (t *Term) IsPiped() bool {
	if file, ok := t.reader.(*os.File); ok {
		fileInfo, _ := file.Stat()
		return (fileInfo.Mode() & os.ModeCharDevice) == 0
	}

	// If reader is not an *os.File, assume it's piped (e.g., bytes.Buffer,
	// strings.Reader)
	return true
}

// isInteractiveTerminal checks if the input is valid and the terminal is
// interactive.
func (t *Term) isInteractiveTerminal(n int) bool {
	if n <= 0 {
		return false
	}

	// check if the term's reader is an *os.file and is a terminal
	file, ok := t.reader.(*os.File)

	return ok && term.IsTerminal(int(file.Fd()))
}

func (t *Term) PipedInput(input *[]string) {
	if !t.IsPiped() {
		return
	}
	s := getQueryFromPipe(os.Stdin)
	if s == "" {
		return
	}

	split := strings.Split(s, " ")
	*input = append(*input, split...)
}

// New returns a new terminal.
func New(opts ...TermOptFn) *Term {
	t := &Term{
		Options: defaultOpts(),
	}

	for _, opt := range opts {
		opt(&t.Options)
	}

	// set up the interrupt handler
	if t.InterruptFn == nil {
		t.InterruptFn = defaultInterruptFn
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.cancelFn = cancel
	setupInterruptHandler(ctx, t.InterruptFn)

	return t
}

// setupInterruptHandler handles interruptions.
func setupInterruptHandler(ctx context.Context, onInterrupt func(error)) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan,
		os.Interrupt,    // Ctrl+C (SIGINT)
		syscall.SIGTERM, // Process termination
		syscall.SIGHUP,  // Terminal closed
	)

	go func() {
		select {
		case sig := <-sigChan:
			fmt.Println()
			slog.Debug("interrupt handler called", "signal", sig)
			onInterrupt(sys.ErrActionAborted)
			os.Exit(1)
		case <-ctx.Done():
			slog.Warn("interrupt handler cancelled")
			return
		}
	}()
}
