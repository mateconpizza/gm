package terminal

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"slices"
	"strings"
	"syscall"

	prompt "github.com/c-bata/go-prompt"
	"golang.org/x/term"

	"github.com/haaag/gm/internal/sys"
)

// TODO:
// - [ ] check `CancelInterruptHandler` implementation.
// - [ ] check `IsPiped` implementation

const termPromptPrefix = "> "

// defaultInterruptFn is the default interrupt function for the terminal.
func defaultInterruptFn(err error) {}

// TermOptFn is an option function for the terminal.
type TermOptFn func(*Options)

// Options represents the options for the terminal.
type Options struct {
	reader      io.Reader
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
		PromptStr: termPromptPrefix,
	}
}

// WithReader sets the reader for the terminal.
func WithReader(r io.Reader) TermOptFn {
	return func(o *Options) {
		o.reader = r
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
	log.Print("setting interrupt function")
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
	if len(def) > 1 {
		// get first char
		def = def[:1]
	}
	opts := []string{"y", "n"}
	if !slices.Contains(opts, def) {
		def = "n"
	}
	choices := fmtChoicesWithDefault(opts, def)
	chosen := t.promptWithChoices(q, choices, def)

	return strings.EqualFold(chosen, "y")
}

// Choose prompts the user to enter one of the given options.
func (t *Term) Choose(q string, opts []string, def string) string {
	for i := 0; i < len(opts); i++ {
		opts[i] = strings.ToLower(opts[i])
	}
	opts = fmtChoicesWithDefault(opts, def)

	return t.promptWithChoices(q, opts, def)
}

// promptWithChoices prompts the user to enter one of the given options.
func (t *Term) promptWithChoices(q string, opts []string, def string) string {
	prompt := buildPrompt(q, fmt.Sprintf("[%s]:", strings.Join(opts, "/")))
	return getUserInput(t.reader, prompt, opts, def)
}

// ClearLine deletes n lines in the console.
func (t *Term) ClearLine(n int) {
	if !t.isInteractiveTerminal(n) {
		log.Printf("error clearing line: %s", ErrNotInteractive)
		return
	}
	ClearLine(n)
}

// ReplaceLine deletes n lines in the console and prints the given string.
func (t *Term) ReplaceLine(n int, s string) {
	if !t.isInteractiveTerminal(n) {
		log.Printf("error replacing line: %s", ErrNotInteractive)
		return
	}

	ReplaceLine(n, s)
}

// ClearChars deletes n characters in the console.
func (t *Term) ClearChars(n int) {
	if !t.isInteractiveTerminal(n) {
		log.Printf("error clearing chars: %s", ErrNotInteractive)
		return
	}
	ClearChars(n)
}

// Clear clears the terminal.
func (t *Term) Clear() {
	if !t.isInteractiveTerminal(1) {
		log.Printf("error clearing the term: %s", ErrNotInteractive)
		return
	}
	clearTerminal()
}

// CancelInterruptHandler cancels the interrupt handler.
func (t *Term) CancelInterruptHandler() {
	if t.cancelFn != nil {
		log.Print("cancelling interrupt handler")
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
			log.Printf("received signal: '%v', cleaning up...", sig)
			onInterrupt(sys.ErrActionAborted)
			os.Exit(1)
		case <-ctx.Done():
			log.Print("interrupt handler cancelled")
			return
		}
	}()
}
