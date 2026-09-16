package terminal

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"regexp"
	"strings"

	prompt "github.com/c-bata/go-prompt"

	"github.com/mateconpizza/gm/internal/application"
)

const (
	cursorUp       = "\x1b[1A"
	cursorReturn   = "\r"
	eraseLineToEnd = "\x1b[0K"
	cursorHide     = "\x1b[?25l"
	cursorShow     = "\x1b[?25h"
)

var ansiEscapeRe = regexp.MustCompile(`\x1b\[[0-9;]*m`)

type highlightFn func(string) string

type color interface {
	Sprint(a ...any) string
}

type Colorizer struct {
	enabled bool

	muted    color
	success  color
	selected color
	error    color
	hotkey   color
}

func NewColorizer(enabled bool) *Colorizer {
	return &Colorizer{enabled: enabled}
}

func (cz *Colorizer) WithError(c color) *Colorizer {
	cz.error = c
	return cz
}

func (cz *Colorizer) WithSuccess(c color) *Colorizer {
	cz.success = c
	return cz
}

func (cz *Colorizer) WithSelected(c color) *Colorizer {
	cz.selected = c
	return cz
}

func (cz *Colorizer) WithMuted(c color) *Colorizer {
	cz.muted = c
	return cz
}

func (cz *Colorizer) WithHotkey(c color) *Colorizer {
	cz.hotkey = c
	return cz
}

func (cz *Colorizer) Error(s string) string    { return cz.apply(s, cz.error) }
func (cz *Colorizer) Hotkey(s string) string   { return cz.apply(s, cz.hotkey) }
func (cz *Colorizer) Muted(s string) string    { return cz.apply(s, cz.muted) }
func (cz *Colorizer) Selected(s string) string { return cz.apply(s, cz.selected) }
func (cz *Colorizer) Success(s string) string  { return cz.apply(s, cz.success) }
func (cz *Colorizer) apply(s string, col color) string {
	if !cz.enabled || col == nil {
		return s
	}
	return col.Sprint(s)
}

// PromptInput contains all the information needed for a user prompt.
type PromptInput struct {
	reader     *bufio.Reader
	writer     io.Writer
	rompt      string
	options    []string
	def        string
	colorizer  *Colorizer
	maxRetries int // maxRetries specifies the maximum number of retries allowed for user input.
}

// PromptSuggester is a function that generates suggestions for a given prompt.
type PromptSuggester = func(in prompt.Document) []prompt.Suggest

// filterFn is a function that filters suggestions based on a given string.
type filterFn = func(completions []prompt.Suggest, sub string, ignoreCase bool) []prompt.Suggest

// inputWithTags prompts the user for input with suggestions based on
// the provided tags.
func inputWithTags[T comparable, V any](t *Term, p string, items map[T]V) string {
	o, restore := prepareInputState(t)
	defer restore()

	s := prompt.Input(p, completerTagsWithCount(items, prompt.FilterHasPrefix), o...)

	return s
}

// inputWithSuggestions prompts the user for input with suggestions based on
// the provided items.
func inputWithSuggestions[T any](t *Term, p string, items []T) string {
	o, restore := prepareInputState(t)
	defer restore()

	s := prompt.Input(p, completerPrefix(items), o...)
	return s
}

// inputWithFuzzySuggestions prompts the user for input with fuzzy suggestions
// based on the provided items and exit function.
func inputWithFuzzySuggestions[T any](t *Term, p string, items []T) string {
	o, restore := prepareInputState(t)
	defer restore()

	s := prompt.Input(p, completerFuzzy(items), o...)
	return s
}

// Confirm prompts the user with a question and options.
func Confirm(ctx context.Context, q, def string) bool {
	t := New(WithInterruptFn(application.Exit))
	return t.Confirm(ctx, q, def)
}

// ConfirmErr prompts the user with a question and options.
func ConfirmErr(ctx context.Context, q, def string) error {
	t := New(WithInterruptFn(application.Exit))
	return t.ConfirmErr(ctx, q, def)
}

// Choose prompts the user to enter one of the given options.
func Choose(ctx context.Context, q string, opts []string, def string) (string, error) {
	t := New(WithInterruptFn(application.Exit))
	return t.Choose(ctx, q, opts, def)
}

func Password(ctx context.Context) (string, error) {
	t := New(WithInterruptFn(application.Exit))
	return t.InputPassword(ctx)
}

// ReadPipedInput reads the input from a pipe.
func ReadPipedInput(args *[]string) {
	if !StdinPiped() {
		return
	}

	s := getQueryFromPipe(os.Stdin)
	if s == "" {
		return
	}

	split := strings.Split(s, " ")
	*args = append(*args, split...)
}

// prepareInputState prepares the input state and options, handling errors with
// exitFn.
func prepareInputState(t *Term) (o []prompt.Option, restore func()) {
	// BUG: https://github.com/c-bata/go-prompt/issues/233#issuecomment-1076162632
	if err := t.saveTermState(); err != nil {
		t.interruptFn(err)
	}

	// opts
	o = promptOptions(t.colorizer.enabled)
	o = append(o, prompt.OptionAddKeyBind(quitKeybind(t)))

	// restores term state
	restore = func() {
		if err := t.restoreTermState(); err != nil {
			t.interruptFn(err)
		}
	}

	return o, restore
}

// promptOptions generates default options for prompt.
func promptOptions(c bool) (o []prompt.Option) {
	o = append(
		o,
		prompt.OptionPrefixTextColor(prompt.White),
		prompt.OptionInputTextColor(prompt.DefaultColor),
		prompt.OptionSuggestionBGColor(prompt.Black),
		prompt.OptionDescriptionBGColor(prompt.Black),
		prompt.OptionSuggestionTextColor(prompt.DefaultColor),
		prompt.OptionDescriptionTextColor(prompt.White),
		prompt.OptionSelectedSuggestionTextColor(prompt.Color(prompt.DisplayBold)),
		prompt.OptionSelectedDescriptionTextColor(prompt.Color(prompt.DisplayBold)),
		prompt.OptionSelectedSuggestionBGColor(prompt.White),
		prompt.OptionSelectedDescriptionBGColor(prompt.White),
		prompt.OptionScrollbarBGColor(prompt.DefaultColor),
		prompt.OptionScrollbarThumbColor(prompt.LightGray),
	)

	// color
	if c {
		o = append(
			o,
			prompt.OptionPrefixTextColor(prompt.DarkGray),
			prompt.OptionPreviewSuggestionTextColor(prompt.Blue),
			prompt.OptionInputTextColor(prompt.DarkGray),
		)
	}

	return o
}

// completerHelper creates a PromptSuggester that filters suggestions based on
// the provided items and filter function.
func completerHelper[T any](items []T, filter filterFn) PromptSuggester {
	sg := make([]prompt.Suggest, 0, len(items))
	for _, t := range items {
		sg = append(sg, prompt.Suggest{Text: fmt.Sprint(t)})
	}

	return func(in prompt.Document) []prompt.Suggest {
		return filter(sg, in.GetWordBeforeCursor(), true)
	}
}

// completerPrefix generates a list of suggestions from a given array of items
// using prefix matching.
func completerPrefix[T any](items []T) PromptSuggester {
	return completerHelper(items, prompt.FilterHasPrefix)
}

// completerFuzzy generates a list of suggestions from a given array of items
// using fuzzy matching.
func completerFuzzy[T any](items []T) PromptSuggester {
	return completerHelper(items, prompt.FilterFuzzy)
}

// completerDummy generates an empty list of suggestions.
func completerDummy() PromptSuggester {
	return completerHelper([]prompt.Suggest{}, prompt.FilterHasPrefix)
}

// completerTagsWithCount creates a prompt suggester with count as a
// description.
func completerTagsWithCount[T comparable, V any](m map[T]V, filter filterFn) PromptSuggester {
	sg := make([]prompt.Suggest, 0, len(m))
	for t, v := range m {
		sg = append(sg, prompt.Suggest{
			Text:        fmt.Sprint(t),
			Description: fmt.Sprintf("(%v)", v),
		})
	}

	return func(in prompt.Document) []prompt.Suggest {
		return filter(sg, in.GetWordBeforeCursor(), true)
	}
}

// getUserInputWithAttempts reads user input and validates against the options,
// with a limited number of attempts (3).
func getUserInputWithAttempts(ctx context.Context, pi *PromptInput) (string, error) {
	var count int
	for count < pi.maxRetries {
		_, _ = fmt.Fprint(pi.writer, pi.rompt)

		// ch to receive input result
		type inputResult struct {
			input string
			err   error
		}
		resultChan := make(chan inputResult, 1)

		// read in a goroutine so context can interrupt it
		go func() {
			userInput, err := pi.reader.ReadString('\n')
			resultChan <- inputResult{input: userInput, err: err}
		}()

		// wait for input, context cancellation, or timeout
		select {
		case <-ctx.Done():
			return "", application.ErrActionAborted
		case result := <-resultChan:
			if result.err != nil {
				slog.Error("error reading input", "error", result.err)
				return "", result.err
			}

			userInput := strings.ToLower(strings.TrimSpace(result.input))

			// user accepted the default
			if userInput == "" && pi.def != "" || userInput == pi.def {
				redrawPromptWithSelection(pi.writer, pi.rompt, pi.def, pi.options, pi.colorizer.Success)
				return pi.def, nil
			}

			// user typed a specific valid option
			if isValidOption(userInput, pi.options) {
				redrawPromptWithSelection(pi.writer, pi.rompt, userInput, pi.options, pi.colorizer.Selected)
				return userInput, nil
			}

			count++
			// user ran out of retries
			if count <= pi.maxRetries-1 {
				ClearLine(pi.writer, len(strings.Split(pi.rompt, "\n")))
			}
		}
	}

	redrawPromptWithSelection(pi.writer, pi.rompt, "error", []string{"error"}, pi.colorizer.Error)
	return "", fmt.Errorf("%d %w", pi.maxRetries, ErrIncorrectAttempts)
}

// fmtChoicesWithDefaultColor capitalizes and highlights the default option,
// and highlights the first letter of each option in red.
func fmtChoicesWithDefaultColor(cz *Colorizer, opts []string, def string) []string {
	if def == "" {
		for i := range opts {
			opts[i] = cz.Muted(opts[i])
		}

		return opts
	}

	var (
		formatted  []string
		defaultOpt string
	)

	for _, opt := range opts {
		if strings.HasPrefix(opt, def) {
			// Capitalize and color the first letter of the default
			colored := cz.Hotkey(strings.ToUpper(opt[:1])) + cz.Muted(opt[1:])
			defaultOpt = colored
		} else {
			// Highlight first letter of non-default
			colored := cz.Hotkey(opt[:1]) + cz.Muted(opt[1:])
			formatted = append(formatted, colored)
		}
	}

	if defaultOpt != "" {
		formatted = append(formatted, defaultOpt)
	}

	return formatted
}

// fmtChoicesWithDefault capitalizes the default option and appends to the end of
// the slice.
func fmtChoicesWithDefault(opts []string, def string) []string {
	if def == "" {
		return opts
	}

	for i := range opts {
		if strings.HasPrefix(opts[i], def) {
			w := opts[i]
			// append to the end of the slice
			opts[i] = opts[len(opts)-1]
			opts = opts[:len(opts)-1]
			opts = append(opts, strings.ToUpper(w[:1])+w[1:])
		}
	}

	return opts
}

// getQueryFromPipe reads the input from the pipe.
func getQueryFromPipe(r io.Reader) string {
	var result strings.Builder

	scanner := bufio.NewScanner(bufio.NewReader(r))
	for scanner.Scan() {
		line := scanner.Text()
		result.WriteString(line)
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "error reading from pipe:", err)

		return ""
	}

	return result.String()
}

// quitKeybind returns the quitKeybind for the completer.
func quitKeybind(t *Term) prompt.KeyBind {
	return prompt.KeyBind{
		Key: prompt.ControlC,
		Fn: func(*prompt.Buffer) {
			if t.state.Current() != nil {
				if err := t.restoreTermState(); err != nil {
					t.interruptFn(err)
				}
			}

			t.interruptFn(application.ErrActionAborted)
		},
	}
}

// isValidOption checks if input is a valid choice.
func isValidOption(input string, opts []string) bool {
	for i := range opts {
		opts[i] = ansiRemover(opts[i])
	}

	for _, opt := range opts {
		if strings.EqualFold(input, opt) || strings.EqualFold(input, opt[:1]) {
			return true
		}
	}

	return false
}

// buildPrompt returns a formatted string with a question and options.
func buildPrompt(q, opts string) string {
	if q == "" {
		return fmt.Sprintf("%s %s ", q, opts)
	}

	if opts == "" {
		return q + " "
	}

	return fmt.Sprintf("%s %s ", q, opts)
}

// WaitForEnter displays a prompt and waits for the user to press ENTER.
func WaitForEnter(ctx context.Context, mesg string) error {
	fmt.Fprint(os.Stdout, mesg)

	done := make(chan struct{})

	go func() {
		var input string
		_, _ = fmt.Scanln(&input)
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// redrawPromptWithSelection replaces the prompt line with the given prompt `q`
// and highlights the selected option with the given color.
func redrawPromptWithSelection(w io.Writer, q, selected string, opts []string, c highlightFn) {
	var result string
	selected = strings.TrimSpace(selected)

	for _, o := range opts {
		o = ansiRemover(o)
		if strings.EqualFold(selected, o) || strings.EqualFold(selected, o[:1]) {
			result = strings.Title(strings.ToLower(o))
			break
		}
	}

	// Fallback if no match found
	if result == "" {
		result = strings.Title(strings.ToLower(selected))
	}

	// Redraw line
	_, _ = fmt.Fprint(w, cursorUp, cursorReturn)
	_, _ = fmt.Fprint(w, q)
	_, _ = fmt.Fprint(w, eraseLineToEnd)
	_, _ = fmt.Fprintln(w, c(result))
}

// ansiRemover removes ANSI codes from a given string.
func ansiRemover(s string) string {
	return ansiEscapeRe.ReplaceAllString(s, "")
}
