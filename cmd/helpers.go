package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"gomarks/pkg/bookmark"
	"gomarks/pkg/color"
	"gomarks/pkg/config"
	"gomarks/pkg/database"
	"gomarks/pkg/errs"
	"gomarks/pkg/format"

	"github.com/spf13/cobra"
)

func promptWithOptions(question string, options []string) string {
	p := format.Prompt(question, fmt.Sprintf("[%s]: ", strings.Join(options, "/")))
	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Print(p)
		input, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println("Error reading input:", err)
			return ""
		}

		input = strings.TrimSpace(input)
		input = strings.ToLower(input)

		for _, opt := range options {
			if strings.EqualFold(input, opt) || strings.EqualFold(input, opt[:1]) {
				return input
			}
		}

		fmt.Printf("Invalid response. Please enter one of: %s\n", strings.Join(options, ", "))
	}
}

func checkInitDB(_ *cobra.Command, _ []string) error {
	if _, err := getDB(); err != nil {
		if errors.Is(err, errs.ErrDBNotFound) {
			init := color.ColorizeBold("init", color.Yellow)
			return fmt.Errorf("%w: use %s to initialize a new database", errs.ErrDBNotFound, init)
		}
		return fmt.Errorf("%w", err)
	}

	return nil
}

func exampleUsage(l []string) string {
	var s string
	for _, line := range l {
		s += fmt.Sprintf("  %s %s", config.App.Name, line)
	}

	return s
}

func getDB() (*database.SQLiteRepository, error) {
	r, err := database.GetDB()
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}
	return r, nil
}

func printSliceSummary(bs *bookmark.Slice) {
	for _, b := range *bs {
		idStr := fmt.Sprintf("[%s]", strconv.Itoa(b.ID))
		fmt.Printf(
			"\t+ %s %s\n",
			color.Colorize(idStr, color.Gray),
			format.ShortenString(b.URL, maxLen),
		)
	}
}
