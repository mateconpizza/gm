package terminal

import (
	"bufio"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	prompt "github.com/c-bata/go-prompt"

	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/format"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/ui/color"
)

// PromptSuggester is a function that generates suggestions for a given prompt.
type PromptSuggester = func(in prompt.Document) []prompt.Suggest

// filterFn is a function that filters suggestions based on a given string.
type filterFn = func(completions []prompt.Suggest, sub string, ignoreCase bool) []prompt.Suggest

// inputWithTags prompts the user for input with suggestions based on
// the provided tags.
func inputWithTags[T comparable, V any](p string, items map[T]V, exitFn func(error)) string {
	o, restore := prepareInputState(exitFn)
	defer restore()

	s := prompt.Input(p, completerTagsWithCount(items, prompt.FilterHasPrefix), o...)

	return s
}

// inputWithSuggestions prompts the user for input with suggestions based on
// the provided items.
func inputWithSuggestions[T any](p string, items []T, exitFn func(error)) string {
	o, restore := prepareInputState(exitFn)
	defer restore()

	s := prompt.Input(p, completerPrefix(items), o...)

	return s
}

// inputWithFuzzySuggestions prompts the user for input with fuzzy suggestions
// based on the provided items and exit function.
func inputWithFuzzySuggestions[T any](p string, items []T, exitFn func(error)) string {
	o, restore := prepareInputState(exitFn)
	defer restore()

	s := prompt.Input(p, completerFuzzy(items), o...)

	return s
}

// Confirm prompts the user with a question and options.
func Confirm(q, def string) bool {
	t := New(WithInterruptFn(sys.ErrAndExit))
	return t.Confirm(q, def)
}

// ConfirmErr prompts the user with a question and options.
func ConfirmErr(q, def string) error {
	t := New(WithInterruptFn(sys.ErrAndExit))
	return t.ConfirmErr(q, def)
}

// Choose prompts the user to enter one of the given options.
func Choose(q string, opts []string, def string) (string, error) {
	t := New(WithInterruptFn(sys.ErrAndExit))
	return t.Choose(q, opts, def)
}

// ReadPipedInput reads the input from a pipe.
func ReadPipedInput(args *[]string) {
	if !IsPiped() {
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
func prepareInputState(exitFn func(error)) (o []prompt.Option, restore func()) {
	// BUG: https://github.com/c-bata/go-prompt/issues/233#issuecomment-1076162632
	if err := saveState(); err != nil {
		exitFn(err)
	}

	// opts
	o = promptOptions(config.App.Color)
	o = append(o, prompt.OptionAddKeyBind(quitKeybind(exitFn)))

	// restores term state
	restore = func() {
		if err := restoreState(); err != nil {
			exitFn(err)
		}
	}

	return o, restore
}

// promptOptions generates default options for prompt.
func promptOptions(c bool) (o []prompt.Option) {
	o = append(o,
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
		o = append(o,
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
	sg := make([]prompt.Suggest, 0)
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
	sg := make([]prompt.Suggest, 0)
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
func getUserInputWithAttempts(
	rd io.Reader,
	w io.Writer,
	p string,
	opts []string,
	def string,
) (string, error) {
	r := bufio.NewReader(rd)
	const attempts = 3
	var count int
	for count < attempts {
		_, _ = fmt.Fprint(w, p)

		input, err := r.ReadString('\n')
		if err != nil {
			slog.Error("error reading input", "error", err)
			return "", fmt.Errorf("%w", err)
		}

		input = strings.ToLower(strings.TrimSpace(input))
		if input == "" && def != "" {
			return def, nil
		}

		if isValidOption(input, opts) {
			return input, nil
		}

		count++
		if count <= attempts-1 {
			ClearLine(format.CountLines(p))
		}
	}

	return "", fmt.Errorf("%d %w", attempts, ErrIncorrectAttempts)
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
		result.WriteString("\n")
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "error reading from pipe:", err)

		return ""
	}

	return result.String()
}

// quitKeybind returns the quitKeybind for the completer.
func quitKeybind(f func(err error)) prompt.KeyBind {
	return prompt.KeyBind{
		Key: prompt.ControlC,
		Fn: func(*prompt.Buffer) {
			if termState != nil {
				if err := restoreState(); err != nil {
					f(err)
				}
			}

			f(sys.ErrActionAborted)
		},
	}
}

// isValidOption checks if input is a valid choice.
func isValidOption(input string, opts []string) bool {
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
		return fmt.Sprintf("%s %s ", q, color.Gray(opts))
	}

	if opts == "" {
		return q + " "
	}

	return fmt.Sprintf("%s %s ", q, color.Gray(opts))
}

// WaitForEnter displays a prompt and waits for the user to press ENTER.
func WaitForEnter() {
	fmt.Print("Press ENTER to continue...")

	var input string
	_, _ = fmt.Scanln(&input)
}
