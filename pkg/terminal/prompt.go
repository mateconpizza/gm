package terminal

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/haaag/gm/pkg/format/color"
)

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

// promptWithOptions prompts the user to enter one of the given options.
func promptWithOptions(question string, options []string, defaultValue string) string {
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

		fmt.Printf("invalid response. please enter one of: %s\n", strings.Join(options, ", "))
	}
}

// promptWithOptsAndDef capitalizes the default option and appends to the end of
// the slice.
func promptWithOptsAndDef(options []string, def string) []string {
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

	return strings.Trim(s, "\n")
}

// Confirm prompts the user to enter yes or no.
func Confirm(question, def string) bool {
	options := promptWithOptsAndDef([]string{"y", "n"}, def)
	chosen := promptWithOptions(question, options, def)

	return strings.EqualFold(chosen, "y")
}

// ConfirmOrEdit prompts the user to enter one of the given options.
func ConfirmOrEdit(question string, options []string, def string) string {
	for i := 0; i < len(options); i++ {
		options[i] = strings.ToLower(options[i])
	}
	options = promptWithOptsAndDef(options, def)

	return promptWithOptions(question, options, def)
}

// Prompt returns a formatted string with a question and options.
func Prompt(question, options string) string {
	return fmt.Sprintf("%s %s ", question, color.Gray(options))
}
