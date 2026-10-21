package terminal

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"unicode"
	"unicode/utf8"

	prompt "github.com/c-bata/go-prompt"
)

const (
	cursorUp       = "\x1b[1A"
	cursorReturn   = "\r"
	eraseLineToEnd = "\x1b[0K"
	cursorHide     = "\x1b[?25l"
	cursorShow     = "\x1b[?25h"
)

type (
	HighlightFn func(string) string

	Colorizer struct {
		Muted    HighlightFn
		Success  HighlightFn
		Selected HighlightFn
		Error    HighlightFn
		Hotkey   HighlightFn

		Enabled bool
	}
)

var defaultColorizer = &Colorizer{
	Muted:    func(s string) string { return s },
	Success:  func(s string) string { return s },
	Selected: func(s string) string { return s },
	Error:    func(s string) string { return s },
	Hotkey:   func(s string) string { return s },
	Enabled:  false,
}

// PromptInput contains all the information needed for a user prompt.
type PromptInput struct {
	reader           *bufio.Reader
	writer           io.Writer
	rompt            string
	options          []string
	def              string
	colorizer        *Colorizer
	maxRetries       int // maxRetries specifies the maximum number of retries allowed for user input.
	cancelationError error
}

type (
	// PromptSuggester is a function that generates suggestions for a given prompt.
	PromptSuggester = func(in prompt.Document) []prompt.Suggest

	// filterFn is a function that filters suggestions based on a given string.
	filterFn = func(completions []prompt.Suggest, sub string, ignoreCase bool) []prompt.Suggest
)

// inputWithTags prompts the user for input with suggestions based on
// the provided tags.
func inputWithTags[T comparable, V any](ctx context.Context, t *Term, p string, items map[T]V) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	o, restore := prepareInputState(t)
	defer restore()

	s := prompt.Input(p, completerTagsWithCount(items, prompt.FilterHasPrefix), o...)

	if err := ctx.Err(); err != nil {
		return "", err
	}

	return s, nil
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

func Confirm(ctx context.Context, q, def string) bool     { return New().Confirm(ctx, q, def) }
func Password(ctx context.Context) (string, error)        { return New().InputPassword(ctx) }
func ConfirmErr(ctx context.Context, q, def string) error { return New().ConfirmErr(ctx, q, def) }
func Choose(ctx context.Context, q string, opts []string, def string) (string, error) {
	return New().Choose(ctx, q, opts, def)
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
	o = promptOptions(t.colorizer.Enabled)
	o = append(o, quitKeybind(t), selectKeybind())

	// restores term state
	restore = func() {
		if err := t.restoreTermState(); err != nil {
			t.interruptFn(err)
		}
	}

	return o, restore
}

// promptOptions generates default options for prompt.
func promptOptions(color bool) []prompt.Option {
	// plain: default text, gray selection
	prefix, selectedBG, thumb := prompt.DefaultColor, prompt.DarkGray, prompt.DarkGray
	if color {
		prefix, selectedBG, thumb = prompt.Cyan, prompt.Blue, prompt.LightGray
	}

	opts := []prompt.Option{
		prompt.OptionMaxSuggestion(8),
		prompt.OptionInputBGColor(prompt.DefaultColor),
		prompt.OptionPrefixBackgroundColor(prompt.DefaultColor),
		prompt.OptionPreviewSuggestionBGColor(prompt.DefaultColor),

		prompt.OptionCompletionOnDown(),            // Down opens/moves the suggestions, not only Tab
		prompt.OptionShowCompletionAtStart(),       // show suggestions before typing anything
		prompt.OptionCompletionWordSeparator(" ,"), // complete each tag after a comma or space

		prompt.OptionPrefixTextColor(prefix),
		prompt.OptionInputTextColor(prompt.DefaultColor),

		prompt.OptionSuggestionBGColor(prompt.DefaultColor),
		prompt.OptionDescriptionBGColor(prompt.DefaultColor),

		prompt.OptionSuggestionTextColor(prompt.DefaultColor),
		prompt.OptionDescriptionTextColor(prompt.DefaultColor),

		prompt.OptionSelectedSuggestionBGColor(selectedBG),
		prompt.OptionSelectedDescriptionBGColor(selectedBG),

		prompt.OptionSelectedSuggestionTextColor(prompt.White),
		prompt.OptionSelectedDescriptionTextColor(prompt.White),

		prompt.OptionScrollbarBGColor(prompt.DefaultColor),
		prompt.OptionScrollbarThumbColor(thumb),

		prompt.OptionSwitchKeyBindMode(prompt.EmacsKeyBind),

		prompt.OptionSuggestionBGColor(prompt.DefaultColor),
		prompt.OptionScrollbarBGColor(prompt.DefaultColor),
		prompt.OptionScrollbarThumbColor(prompt.DefaultColor),
	}

	// only set with color, the plain case keeps go-prompt's default
	if color {
		opts = append(opts, prompt.OptionPreviewSuggestionTextColor(prompt.Cyan))
	}

	return opts
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
			return "", pi.cancelationError
		case result := <-resultChan:
			if result.err != nil {
				slog.Error("error reading input", "error", result.err)
				return "", result.err
			}

			userInput := strings.ToLower(strings.TrimSpace(result.input))

			// user accepted the default
			if userInput == "" && pi.def != "" || userInput == pi.def {
				redrawPromptWithSelection(pi.writer, pi.rompt, pi.def, pi.colorizer.Success)
				return pi.def, nil
			}

			// exact match or unique prefix
			if opt, ok := resolveOption(userInput, pi.options); ok {
				c := pi.colorizer.Selected
				if opt == pi.def {
					c = pi.colorizer.Success
				}
				redrawPromptWithSelection(pi.writer, pi.rompt, opt, c)
				return opt, nil
			}

			count++
			// user ran out of retries
			if count <= pi.maxRetries-1 {
				ClearLine(pi.writer, len(strings.Split(pi.rompt, "\n")))
			}
		}
	}

	redrawPromptWithSelection(pi.writer, pi.rompt, "error", pi.colorizer.Error)
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
func quitKeybind(t *Term) prompt.Option {
	return prompt.OptionAddKeyBind(prompt.KeyBind{
		Key: prompt.ControlC,
		Fn: func(*prompt.Buffer) {
			if t.state.Current() != nil {
				if err := t.restoreTermState(); err != nil {
					t.interruptFn(err)
				}
			}

			t.interruptFn(t.cancelationError)
		},
	})
}

func selectKeybind() prompt.Option {
	return prompt.OptionAddKeyBind(prompt.KeyBind{
		Key: prompt.ControlY,
		Fn: func(buf *prompt.Buffer) {
			// the highlighted suggestion is accepted by this keypress;
			// add a separator to start the next word (tags example)
			buf.InsertText(", ", false, true)
		},
	})
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
func WaitForEnter(ctx context.Context, msg string) error {
	fmt.Fprint(os.Stdout, msg)

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
func redrawPromptWithSelection(w io.Writer, q, selected string, c HighlightFn) {
	result := capitalize(strings.ToLower(strings.TrimSpace(selected)))
	_, _ = fmt.Fprint(w, cursorUp, cursorReturn)
	_, _ = fmt.Fprint(w, q)
	_, _ = fmt.Fprint(w, eraseLineToEnd)
	_, _ = fmt.Fprintln(w, c(result))
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	r, size := utf8.DecodeRuneInString(s)
	return string(unicode.ToUpper(r)) + s[size:]
}
