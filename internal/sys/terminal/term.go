package terminal

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"slices"
	"strings"

	prompt "github.com/c-bata/go-prompt"
	"golang.org/x/term"

	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/pkg/ansi"
)

// defaultInterruptFn is the default interrupt function for the terminal.
func defaultInterruptFn(err error) { slog.Debug("InterruptFn not set") }

type termSize struct {
	width    int
	maxWidth int
	minWidth int
	height   int
}

// TermOptFn is an option function for the terminal.
type TermOptFn func(*Options)

// Options represents the options for the terminal.
type Options struct {
	reader      io.Reader
	writer      io.Writer
	PromptStr   string
	interruptFn func(error) // interruptFn handles cancellation (Ctrl-C, ESC, etc.)
}

// Term is a struct that represents a terminal.
type Term struct {
	Options
	cancelFn context.CancelFunc
	size     *termSize
}

// defaultOpts returns the default terminal options.
func defaultOpts() Options {
	return Options{
		reader: os.Stdin,
		writer: os.Stdout,
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

// WithInterruptFn sets a callback that executes on terminal interruption.
func WithInterruptFn(fn func(error)) TermOptFn {
	return func(o *Options) {
		o.interruptFn = fn
	}
}

// SetReader sets the reader for the terminal.
func (t *Term) SetReader(r io.Reader) {
	t.reader = r
}

// SetWriter sets the writer for the terminal.
func (t *Term) SetWriter(w io.Writer) {
	t.writer = w
}

// Input get the Input data from the user and return it.
func (t *Term) Input(p string) string {
	o, restore := prepareInputState(t.interruptFn)
	defer restore()

	s := prompt.Input(p, completerDummy(), o...)

	return s
}

func (t *Term) InputPassword(ctx context.Context) (string, error) {
	fd := int(os.Stdin.Fd())

	// if not a terminal (piped or test), read plain input
	if !term.IsTerminal(fd) {
		var password string
		if _, err := fmt.Fscanln(t.reader, &password); err != nil {
			return "", fmt.Errorf("reading password: %w", err)
		}
		return password, nil
	}

	// Save and restore terminal state
	if err := saveState(); err != nil {
		return "", err
	}
	defer func() {
		if err := restoreState(); err != nil {
			slog.Warn("restoring terminal state", "error", err)
		}
	}()

	// channel to receive password result
	type passwordResult struct {
		password string
		err      error
	}
	resultChan := make(chan passwordResult, 1)

	go func() {
		p, err := term.ReadPassword(fd)
		resultChan <- passwordResult{password: string(p), err: err}
	}()

	select {
	case <-ctx.Done():
		return "", sys.ErrActionAborted
	case result := <-resultChan:
		if result.err != nil {
			return "", fmt.Errorf("reading password: %w", result.err)
		}
		return result.password, nil
	}
}

// Prompt get the input data from the user and return it.
func (t *Term) Prompt(ctx context.Context, p string) (string, error) {
	r := bufio.NewReader(t.reader)
	fmt.Fprint(t.writer, p)

	type inputResult struct {
		input string
		err   error
	}

	resultChan := make(chan inputResult, 1)

	go func() {
		userInput, err := r.ReadString('\n')
		resultChan <- inputResult{input: userInput, err: err}
	}()

	select {
	case <-ctx.Done():
		return "", sys.ErrActionAborted
	case result := <-resultChan:
		if result.err != nil {
			return "", result.err
		}
		return strings.TrimSpace(result.input), nil
	}
}

// PromptWithSuggestions prompts the user for input with suggestions based on
// the provided items.
func (t *Term) PromptWithSuggestions(p string, items []string) string {
	return inputWithSuggestions(p, items, t.interruptFn)
}

// PromptWithFuzzySuggestions prompts the user for input with fuzzy suggestions.
func (t *Term) PromptWithFuzzySuggestions(p string, items []string) string {
	return inputWithFuzzySuggestions(p, items, t.interruptFn)
}

// ChooseTags prompts the user for input with suggestions based on
// the provided tags.
func (t *Term) ChooseTags(p string, items map[string]int) string {
	return inputWithTags(p, items, t.interruptFn)
}

// Confirm prompts the user with a question and options.
func (t *Term) Confirm(ctx context.Context, q, def string) bool {
	err := t.ConfirmErr(ctx, q, def)
	if err != nil {
		slog.Debug("terminal confirm", "err", err)
	}

	return err == nil
}

// ConfirmErr prompts the user with a question and options.
func (t *Term) ConfirmErr(ctx context.Context, q, def string) error {
	if force {
		slog.Debug("force", "def", def)
		return nil
	}

	if len(def) > 1 {
		// get first char
		def = def[:1]
	}

	opts := []string{"y", "n"}
	if !slices.Contains(opts, def) {
		def = "n"
	}

	h := &highlighter{}
	choices := fmtChoicesWithDefault(opts, def)
	for i := range len(choices) {
		choices[i] = h.dim(choices[i])
	}

	chosen, err := t.promptWithChoicesErr(ctx, q, choices, def)
	if err != nil {
		return err
	}

	if !strings.EqualFold(chosen, "y") {
		return sys.ErrExitFailure
	}

	return nil
}

// Choose prompts the user to enter one of the given options.
func (t *Term) Choose(ctx context.Context, q string, opts []string, def string) (string, error) {
	if force {
		slog.Debug("choose", "def", def)
		return def, nil
	}

	for i := range opts {
		opts[i] = strings.ToLower(opts[i])
	}

	opts = fmtChoicesWithDefaultColor(opts, def)

	return t.promptWithChoicesErr(ctx, q, opts, def)
}

// WaitForEnter displays a prompt and waits for the user to press ENTER.
func (t *Term) WaitForEnter(ctx context.Context, mesg string) error { return WaitForEnter(ctx, mesg) }

// promptWithChoices prompts the user to enter one of the given options.
func (t *Term) promptWithChoicesErr(ctx context.Context, q string, opts []string, def string) (string, error) {
	h := &highlighter{}
	dimmer := h.dim
	sep := dimmer("/")
	s := dimmer("[")
	e := dimmer("]:")

	p := buildPrompt(q, fmt.Sprintf("%s%s%s", s, strings.Join(opts, sep), e))

	return getUserInputWithAttempts(ctx, &PromptInput{
		Reader:  t.reader,
		Writer:  t.writer,
		Prompt:  p,
		Options: opts,
		Default: def,
	})
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
		slog.Warn("error replacing line", "error", ErrNotInteractive)
		return
	}

	ReplaceLine(n, s)
}

// ClearChars deletes n characters in the console.
func (t *Term) ClearChars(n int) {
	if !t.isInteractiveTerminal(n) {
		slog.Warn("error clearing chars", "error", ErrNotInteractive)
		return
	}

	ClearChars(n)
}

// Clear clears the terminal.
func (t *Term) Clear() {
	if !t.isInteractiveTerminal(1) {
		slog.Warn("error clearing the term", "error", ErrNotInteractive)
		return
	}

	clearTerminal()
}

// SetInterruptFn sets the interrupt function for the terminal, canceling the
// interrupt handler if it is already set.
//
// If fn is nil, the interrupt handler is disabled.
func (t *Term) SetInterruptFn(fn func(error)) {
	// FIX: remove
	slog.Info("setting interrupt function")
	t.interruptFn = fn
}

// InterruptFn returns current interruptFn.
func (t *Term) InterruptFn() func(error) {
	return t.interruptFn
}

// CancelInterruptHandler cancels the interrupt handler.
func (t *Term) CancelInterruptHandler() {
	if t.cancelFn != nil {
		slog.Warn("cancelling interrupt handler")
		t.cancelFn()
	}
}

// StdinPiped returns true if the terminal input is piped.
func (t *Term) StdinPiped() bool {
	if file, ok := t.reader.(*os.File); ok {
		fileInfo, _ := file.Stat()
		return (fileInfo.Mode() & os.ModeCharDevice) == 0
	}

	// If reader is not an *os.File, assume it's piped (e.g., bytes.Buffer,
	// strings.Reader)
	return true
}

// StdoutPiped reports whether stdout is redirected or piped.
func (t *Term) StdoutPiped() bool {
	if file, ok := t.writer.(*os.File); ok {
		fileInfo, _ := file.Stat()
		return (fileInfo.Mode() & os.ModeCharDevice) == 0
	}

	// If writer is not an *os.File, assume it's redirected (e.g., bytes.Buffer).
	return true
}

func (t *Term) IsPiped() bool { return t.StdinPiped() || t.StdoutPiped() }

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

// HideCursor hides cursor.
func (t *Term) HideCursor() error {
	_, err := fmt.Fprint(t.writer, ansi.CursorHide)
	return err
}

// ShowCursor unhide cursor.
func (t *Term) ShowCursor() error {
	_, err := fmt.Fprint(t.writer, ansi.CursorShow)
	return err
}

func (t *Term) Height() int   { return t.size.height }
func (t *Term) Width() int    { return t.size.width }
func (t *Term) MaxWidth() int { return t.size.maxWidth }
func (t *Term) MinWidth() int { return t.size.minWidth }

// Print writes content to the terminal, paginating if the output
// exceeds the terminal height.
func (t *Term) Print(ctx context.Context, content string) error {
	if t.needsPager(content) {
		return t.paginate(ctx, content)
	}
	_, err := fmt.Fprint(t.writer, content)
	return err
}

// needsPager returns true if content exceeds the terminal height.
func (t *Term) needsPager(content string) bool {
	file, ok := t.writer.(*os.File)
	if !ok {
		return false
	}
	if !term.IsTerminal(int(file.Fd())) {
		return false
	}
	return strings.Count(content, "\n") >= t.Height()
}

// paginate pipes content through $PAGER (default: less).
func (t *Term) paginate(ctx context.Context, content string) error {
	pager, ok := os.LookupEnv("PAGER")

	// user explicitly disabled paging
	if ok && pager == "" {
		_, err := fmt.Fprint(t.writer, content)
		return err
	}

	// unset case
	if pager == "" {
		pager = "less -RFX"
	}

	args := strings.Fields(pager)
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Stdin = strings.NewReader(content)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := withRestoredTerminal(cmd.Run); err != nil {
		_, err = fmt.Fprint(t.writer, content)
		return err
	}
	return nil
}

// New returns a new terminal with the provided options.
func New(opts ...TermOptFn) *Term {
	t := &Term{
		Options: defaultOpts(),
		size: &termSize{
			maxWidth: maxWidth,
			minWidth: minWidth,
			width:    width,
			height:   height,
		},
	}

	for _, opt := range opts {
		opt(&t.Options)
	}

	// Set default interrupt handler if not provided
	if t.interruptFn == nil {
		t.interruptFn = defaultInterruptFn
	}

	return t
}
