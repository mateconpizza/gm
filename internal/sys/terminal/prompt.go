package terminal

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	prompt "github.com/c-bata/go-prompt"

	"github.com/haaag/gm/internal/format/color"
)

var ErrActionAborted = errors.New("action aborted")

type PromptSuggester = func(in prompt.Document) []prompt.Suggest

type FilterFunc = func(completions []prompt.Suggest, sub string, ignoreCase bool) []prompt.Suggest

const promptPrefix = ">>> "

// Input get the Input data from the user and return it.
func Input(p string, exitFn func(error)) string {
	o, restore := prepareInputState(exitFn)
	defer restore()
	s := prompt.Input(p, completerDummy(), o...)

	return s
}

// Prompt get the input data from the user and return it.
func Prompt(p string) string {
	r := bufio.NewReader(os.Stdin)
	fmt.Print(p)
	s, _ := r.ReadString('\n')

	return strings.TrimSpace(s)
}

// InputTags prompts the user for input with suggestions based on
// the provided tags.
func InputTags[T comparable, V any](terms map[T]V, exitFn func(error)) string {
	o, restore := prepareInputState(exitFn)
	defer restore()
	s := prompt.Input(promptPrefix, completerTagsWithCount(terms, prompt.FilterHasPrefix), o...)

	return s
}

// InputWithSuggestions prompts the user for input with suggestions based on
// the provided items.
func InputWithSuggestions[T any](terms []T, exitFn func(error)) string {
	o, restore := prepareInputState(exitFn)
	defer restore()
	s := prompt.Input(promptPrefix, completerPrefix(terms), o...)

	return s
}

// InputWithFuzzySuggestions prompts the user for input with fuzzy suggestions
// based on the provided items and exit function.
func InputWithFuzzySuggestions[T any](terms []T, exitFn func(error)) string {
	o, restore := prepareInputState(exitFn)
	defer restore()
	s := prompt.Input(promptPrefix, completerFuzzy(terms), o...)

	return s
}

// Confirm prompts the user with a question and options.
func Confirm(q, def string) bool {
	choices := promptWithDefChoice([]string{"y", "n"}, def)
	chosen := promptWithChoices(q, choices, def)

	return strings.EqualFold(chosen, "y")
}

// ConfirmWithChoices prompts the user to enter one of the given options.
func ConfirmWithChoices(q string, opts []string, def string) string {
	for i := 0; i < len(opts); i++ {
		opts[i] = strings.ToLower(opts[i])
	}
	opts = promptWithDefChoice(opts, def)

	return promptWithChoices(q, opts, def)
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

// WaitForEnter displays a prompt and waits for the user to press ENTER.
func WaitForEnter() {
	fmt.Print("Press ENTER to continue...")
	var input string
	_, _ = fmt.Scanln(&input)
}

// prepareInputState prepares the input state and options, handling errors with exitFn.
func prepareInputState(exitFn func(error)) (o []prompt.Option, restore func()) {
	// BUG: https://github.com/c-bata/go-prompt/issues/233#issuecomment-1076162632

	if err := saveState(); err != nil {
		exitFn(err)
	}

	// opts
	o = promptOptions(enabledColor)
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
func promptOptions(c *bool) (o []prompt.Option) {
	o = append(o,
		prompt.OptionPrefixTextColor(prompt.White),
		prompt.OptionInputTextColor(prompt.DefaultColor),
		prompt.OptionSuggestionBGColor(prompt.Black),
		prompt.OptionDescriptionBGColor(prompt.Black),
		prompt.OptionSuggestionTextColor(prompt.White),
		prompt.OptionDescriptionTextColor(prompt.White),
		prompt.OptionSelectedSuggestionTextColor(prompt.Color(prompt.DisplayBold)),
		prompt.OptionSelectedDescriptionTextColor(prompt.Color(prompt.DisplayBold)),
		prompt.OptionSelectedSuggestionBGColor(prompt.White),
		prompt.OptionSelectedDescriptionBGColor(prompt.White),
		prompt.OptionScrollbarBGColor(prompt.DefaultColor),
		prompt.OptionScrollbarThumbColor(prompt.LightGray),
	)

	// color
	if *c {
		o = append(o,
			prompt.OptionPrefixTextColor(prompt.Yellow),
			prompt.OptionPreviewSuggestionTextColor(prompt.Blue),
			prompt.OptionInputTextColor(prompt.DarkGray),
		)
	}

	return
}

// completerCreate creates a PromptSuggester that filters suggestions based on
// the provided terms and filter function.
func completerCreate[T any](terms []T, filter FilterFunc) PromptSuggester {
	sg := make([]prompt.Suggest, 0)
	for _, t := range terms {
		sg = append(sg, prompt.Suggest{Text: fmt.Sprint(t)})
	}

	return func(in prompt.Document) []prompt.Suggest {
		return filter(sg, in.GetWordBeforeCursor(), true)
	}
}

// completerPrefix generates a list of suggestions from a given array of terms
// using prefix matching.
func completerPrefix[T any](terms []T) PromptSuggester {
	return completerCreate(terms, prompt.FilterHasPrefix)
}

// completerFuzzy generates a list of suggestions from a given array of terms using fuzzy matching.
func completerFuzzy[T any](terms []T) PromptSuggester {
	return completerCreate(terms, prompt.FilterFuzzy)
}

// completerDummy generates an empty list of suggestions.
func completerDummy() PromptSuggester {
	return completerCreate([]prompt.Suggest{}, prompt.FilterHasPrefix)
}

// completerTagsWithCount creates a prompt suggester with count as a
// description.
func completerTagsWithCount[T comparable, V any](m map[T]V, filter FilterFunc) PromptSuggester {
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

// promptWithChoices prompts the user to enter one of the given options.
func promptWithChoices(q string, opts []string, def string) string {
	p := buildPrompt(q, fmt.Sprintf("[%s]:", strings.Join(opts, "/")))
	r := bufio.NewReader(os.Stdin)

	for {
		fmt.Print(p)
		s, err := r.ReadString('\n')
		if err != nil {
			log.Println("Error reading input:", err)

			return ""
		}

		s = strings.ToLower(strings.TrimSpace(s))
		if s == "" && def != "" {
			return def
		}

		for _, opt := range opts {
			if strings.EqualFold(s, opt) || strings.EqualFold(s, opt[:1]) {
				return s
			}
		}

		fmt.Printf("invalid response. use: %s", formatOpts(opts))
	}
}

// promptWithDefChoice capitalizes the default option and appends to the end of
// the slice.
func promptWithDefChoice(opts []string, def string) []string {
	for i := 0; i < len(opts); i++ {
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

// formatOpts formats each option in the slice as "[x]option" where x is the
// first letter of the option.
func formatOpts(opts []string) string {
	// FIX: delete me
	n := len(opts)
	if n == 0 {
		return ""
	}

	var s string
	for _, option := range opts {
		s += fmt.Sprintf("[%s]%s ", strings.ToLower(option[:1]), option[1:])
	}

	return s
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

			f(ErrActionAborted)
		},
	}
}
