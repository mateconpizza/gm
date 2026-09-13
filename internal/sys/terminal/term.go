package terminal

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"slices"
	"strings"
	"sync"

	prompt "github.com/c-bata/go-prompt"
	"golang.org/x/term"

	"github.com/mateconpizza/gm/internal/sys"
)

// defaultInterruptFn is the default interrupt function for the terminal.
func defaultInterruptFn(err error) { slog.Debug("InterruptFn not set") }

type pagerRunFunc func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error

type isTerminalFunc func(fd int) bool

type readPasswordFunc func(fd int) ([]byte, error)

// TermOptFn is an option function for the terminal.
type TermOptFn func(*Options)

// Options represents the options for the terminal.
type Options struct {
	reader       io.Reader
	writer       io.Writer
	interruptFn  func(error) // interruptFn handles cancellation (Ctrl-C, ESC, etc.)
	inputRetries int         // retries specifies the maximum number of retries allowed for user input.
	state        *State

	isTerminal   isTerminalFunc
	readPassword readPasswordFunc

	pagerFunc pagerRunFunc

	colorizer *Colorizer
}

// Term is a struct that represents a terminal.
type Term struct {
	Options

	mu       sync.Mutex
	br       *bufio.Reader
	cancelFn context.CancelFunc
	size     *TermSize
}

// New returns a new terminal with the provided options.
func New(opts ...TermOptFn) *Term {
	t := &Term{
		Options: Options{
			reader:       os.Stdin,
			writer:       os.Stdout,
			isTerminal:   term.IsTerminal,
			readPassword: term.ReadPassword,
			state:        NewState(),
			pagerFunc:    defaultPagerRun,
			inputRetries: 3,
			colorizer:    &Colorizer{},
		},
		size: NewSize(),
	}

	for _, opt := range opts {
		opt(&t.Options)
	}

	// Set default interrupt handler if not provided
	if t.interruptFn == nil {
		t.interruptFn = defaultInterruptFn
	}

	t.br = bufio.NewReader(t.reader)

	return t
}

func WithColorizer(c *Colorizer) TermOptFn     { return func(o *Options) { o.colorizer = c } }
func WithInterruptFn(fn func(error)) TermOptFn { return func(o *Options) { o.interruptFn = fn } }
func WithMaxRetries(n int) TermOptFn           { return func(o *Options) { o.inputRetries = n } }
func WithReader(r io.Reader) TermOptFn         { return func(o *Options) { o.reader = r } }
func WithTermState(s *State) TermOptFn         { return func(o *Options) { o.state = s } }
func WithWriter(w io.Writer) TermOptFn         { return func(o *Options) { o.writer = w } }

// SetReader sets the reader for the terminal.
func (t *Term) SetReader(r io.Reader) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.reader = r
	t.br = bufio.NewReader(r)
}

// SetWriter sets the writer for the terminal.
func (t *Term) SetWriter(w io.Writer) {
	t.writer = w
}

// Input get the Input data from the user and return it.
func (t *Term) Input(p string) string {
	o, restore := prepareInputState(t)
	defer restore()

	s := prompt.Input(p, completerDummy(), o...)

	return s
}

func (t *Term) InputPassword(ctx context.Context) (string, error) {
	fd := int(os.Stdin.Fd())

	// if not a terminal (piped or test), read plain input
	if !t.isTerminal(fd) {
		password, err := t.currentReader().ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return "", fmt.Errorf("reading password: %w", err)
		}
		return strings.TrimSuffix(password, "\n"), nil
	}

	// Save and restore terminal state
	if err := t.saveTermState(); err != nil {
		return "", err
	}
	defer func() {
		if err := t.restoreTermState(); err != nil {
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
		p, err := t.readPassword(fd)
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
	br := t.currentReader()
	fmt.Fprint(t.writer, p)

	type inputResult struct {
		input string
		err   error
	}

	resultChan := make(chan inputResult, 1)

	go func() {
		userInput, err := br.ReadString('\n')
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
	return inputWithSuggestions(t, p, items)
}

// PromptWithFuzzySuggestions prompts the user for input with fuzzy suggestions.
func (t *Term) PromptWithFuzzySuggestions(p string, items []string) string {
	return inputWithFuzzySuggestions(t, p, items)
}

// ChooseTags prompts the user for input with suggestions based on
// the provided tags.
func (t *Term) ChooseTags(p string, items map[string]int) string {
	return inputWithTags(t, p, items)
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

	choices := fmtChoicesWithDefault(opts, def)
	for i := range choices {
		choices[i] = t.colorizer.Muted(choices[i])
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

	opts = fmtChoicesWithDefaultColor(t.colorizer, opts, def)

	return t.promptWithChoicesErr(ctx, q, opts, def)
}

// WaitForEnter displays a prompt and waits for the user to press ENTER.
func (t *Term) WaitForEnter(ctx context.Context, mesg string) error { return WaitForEnter(ctx, mesg) }

// ClearLine deletes n lines in the console.
func (t *Term) ClearLine(n int) {
	if !t.isInteractiveTerminal(n) {
		slog.Debug("clearing line", "error", ErrNotInteractive)
		return
	}

	ClearLine(t.writer, n)
}

// ReplaceLine deletes n lines in the console and prints the given string.
func (t *Term) ReplaceLine(n int, s string) {
	if !t.isInteractiveTerminal(n) {
		slog.Warn("error replacing line", "error", ErrNotInteractive)
		return
	}

	ReplaceLine(t.writer, n, s)
}

// ClearChars deletes n characters in the console.
func (t *Term) ClearChars(n int) {
	if !t.isInteractiveTerminal(n) {
		slog.Warn("error clearing chars", "error", ErrNotInteractive)
		return
	}

	ClearChars(t.writer, n)
}

// Clear clears the terminal.
func (t *Term) Clear() {
	if !t.isInteractiveTerminal(1) {
		slog.Warn("error clearing the term", "error", ErrNotInteractive)
		return
	}

	clearTerminal(t.writer)
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

// HideCursor hides cursor.
func (t *Term) HideCursor() error {
	_, err := fmt.Fprint(t.writer, cursorHide)
	return err
}

// ShowCursor unhide cursor.
func (t *Term) ShowCursor() error {
	_, err := fmt.Fprint(t.writer, cursorShow)
	return err
}

func (t *Term) Height() int   { return t.size.height }
func (t *Term) Width() int    { return t.size.width }
func (t *Term) MaxWidth() int { return t.size.maxWidth }
func (t *Term) MinWidth() int { return t.size.minWidth }

// Print writes content to the terminal, paginating if the output
// exceeds the terminal height.
func (t *Term) Print(ctx context.Context, content string) error {
	// FIX: stream output to the pager instead of buffering the entire content in
	// memory.
	if t.needsPager(content) {
		return t.paginate(ctx, content)
	}
	_, err := fmt.Fprint(t.writer, content)
	return err
}

// currentReader returns the active buffered reader under a short lock.
func (t *Term) currentReader() *bufio.Reader {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.br
}

// promptWithChoices prompts the user to enter one of the given options.
func (t *Term) promptWithChoicesErr(ctx context.Context, q string, opts []string, def string) (string, error) {
	dimmer := t.colorizer.Muted
	sep := dimmer("/")
	s := dimmer("[")
	e := dimmer("]:")

	p := buildPrompt(q, fmt.Sprintf("%s%s%s", s, strings.Join(opts, sep), e))

	return getUserInputWithAttempts(ctx, &PromptInput{
		reader:     t.currentReader(),
		writer:     t.writer,
		rompt:      p,
		options:    opts,
		def:        def,
		maxRetries: t.inputRetries,
		colorizer:  t.colorizer,
	})
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
	if len(args) == 0 {
		_, err := fmt.Fprint(t.writer, content)
		return err
	}

	run := func() error {
		return t.pagerFunc(ctx, args, strings.NewReader(content), os.Stdout, os.Stderr)
	}

	if err := t.withRestoredTerminal(run); err != nil {
		_, err = fmt.Fprint(t.writer, content)
		return err
	}
	return nil
}

// withRestoredTerminal saves the terminal state, runs fn, then restores it
// regardless of how fn exits. Safe to call even if stdin is not a terminal.
func (t *Term) withRestoredTerminal(fn func() error) error {
	if !t.isTerminal(int(os.Stdin.Fd())) {
		return fn()
	}

	if err := t.state.Save(); err != nil {
		slog.Debug("failed to save terminal state", "err", err)
		return fn()
	}

	defer func() {
		if err := t.state.Restore(); err != nil {
			slog.Debug("failed to restore terminal state", "err", err)
		}
	}()

	return fn()
}

func (t *Term) saveTermState() error {
	slog.Debug("saving terminal state")
	if !t.isTerminal(int(os.Stdin.Fd())) {
		slog.Debug("not a terminal, skipping saveState")
		return nil
	}
	return t.state.Save()
}

func (t *Term) restoreTermState() error {
	slog.Debug("restoring terminal state")
	if !t.isTerminal(int(os.Stdin.Fd())) {
		slog.Debug("not a terminal, skipping restoreState")
		return nil
	}
	return t.state.Restore()
}

func defaultPagerRun(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}
