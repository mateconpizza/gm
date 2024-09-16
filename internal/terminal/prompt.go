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
func Confirm(q, d string) bool {
	options := PromptWithOptsAndDef([]string{"y", "n"}, d)
	chosen := PromptWithOptions(q, options, d)

	return strings.EqualFold(chosen, "y")
}

// ConfirmWithOpts prompts the user to enter one of the given options.
func ConfirmWithOpts(q string, o []string, d string) string {
	for i := 0; i < len(o); i++ {
		o[i] = strings.ToLower(o[i])
	}
	o = PromptWithOptsAndDef(o, d)

	return PromptWithOptions(q, o, d)
}

// Prompt returns a formatted string with a question and options.
func Prompt(question, options string) string {
	return fmt.Sprintf("%s %s ", question, color.Gray(options))
}

// PromptWithOptions prompts the user to enter one of the given options.
func PromptWithOptions(question string, options []string, defaultValue string) string {
	p := Prompt(question, fmt.Sprintf("[%s]:", strings.Join(options, "/")))
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

		if s == "" && defaultValue != "" {
			return defaultValue
		}

		for _, opt := range options {
			if strings.EqualFold(s, opt) || strings.EqualFold(s, opt[:1]) {
				return s
			}
		}

		fmt.Printf("invalid response. valid: %s\n", formatOpts(options))
	}
}

// PromptWithOptsAndDef capitalizes the default option and appends to the end of
// the slice.
func PromptWithOptsAndDef(options []string, def string) []string {
	for i := 0; i < len(options); i++ {
		if strings.HasPrefix(options[i], def) {
			w := options[i]

			// append to the end of the slice
			options[i] = options[len(options)-1]
			options = options[:len(options)-1]
			options = append(options, strings.ToUpper(w[:1])+w[1:])
		}
	}

	return options
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

// formatOpts formats each option in the slice as "[x]option" where x is the first letter of the option.
func formatOpts(o []string) string {
	n := len(o)
	if n == 0 {
		return ""
	}

	var s string
	for _, option := range o {
		s += fmt.Sprintf("[%s]%s ", strings.ToLower(option[:1]), option[1:])
	}

	return s
}
