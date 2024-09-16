package terminal

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/haaag/gm/internal/format/color"
)

// Confirm prompts the user with a question and options.
func Confirm(q, def string) bool {
	options := PromptWithOptsAndDef([]string{"y", "n"}, def)
	chosen := PromptWithOptions(q, options, def)

	return strings.EqualFold(chosen, "y")
}

// ConfirmWithOpts prompts the user to enter one of the given options.
func ConfirmWithOpts(q string, opts []string, def string) string {
	for i := 0; i < len(opts); i++ {
		opts[i] = strings.ToLower(opts[i])
	}
	opts = PromptWithOptsAndDef(opts, def)

	return PromptWithOptions(q, opts, def)
}

// Prompt returns a formatted string with a question and options.
func Prompt(q, opts string) string {
	return fmt.Sprintf("%s %s ", q, color.Gray(opts))
}

// PromptWithOptions prompts the user to enter one of the given options.
func PromptWithOptions(q string, opts []string, def string) string {
	p := Prompt(q, fmt.Sprintf("[%s]:", strings.Join(opts, "/")))
	r := bufio.NewReader(os.Stdin)

	for {
		fmt.Print(p)
		s, err := r.ReadString('\n')
		if err != nil {
			fmt.Println("Error reading input:", err)

			return ""
		}

		s = strings.TrimSpace(s)
		s = strings.ToLower(s)

		if s == "" && def != "" {
			return def
		}

		for _, opt := range opts {
			if strings.EqualFold(s, opt) || strings.EqualFold(s, opt[:1]) {
				return s
			}
		}

		fmt.Printf("invalid response. valid: %s\n", formatOpts(opts))
	}
}

// PromptWithOptsAndDef capitalizes the default option and appends to the end of
// the slice.
func PromptWithOptsAndDef(opts []string, def string) []string {
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

// ReadInput prompts the user for input.
func ReadInput(prompt string) string {
	var s string
	fmt.Print(prompt)
	reader := bufio.NewReader(os.Stdin)
	s, err := reader.ReadString('\n')
	if err != nil {
		return ""
	}

	fmt.Print(color.Reset())

	return strings.Trim(s, "\n")
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
