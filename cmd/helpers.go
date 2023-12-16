package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"gomarks/pkg/app"
	"gomarks/pkg/bookmark"
	"gomarks/pkg/database"
	"gomarks/pkg/format"

	"github.com/spf13/cobra"
)

func promptWithOptions(question string, options []string) string {
	p := format.Prompt(question, fmt.Sprintf("[%s]:", strings.Join(options, "/")))
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
		return fmt.Errorf("%w", err)
	}

	return nil
}

func exampleUsage(l ...string) string {
	var s string
	for _, line := range l {
		s += fmt.Sprintf("  %s %s", app.Config.Name, line)
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

func printSliceSummary(bs *bookmark.Slice, msg string) {
	fmt.Println(msg)
	for _, b := range *bs {
		idStr := fmt.Sprintf("[%s]", strconv.Itoa(b.ID))
		fmt.Printf(
			"  + %s %s\n",
			format.Text(idStr).Gray(),
			format.Text(format.ShortenString(b.Title, app.Term.MinWidth)).Purple(),
		)
		fmt.Printf("    %s\n", format.Text("tags:", b.Tags).Gray())
		fmt.Printf("    %s\n\n", format.ShortenString(b.URL, app.Term.MinWidth))
	}
}

func extractIDs(args []string) ([]int, error) {
	ids := make([]int, 0)

	for _, arg := range strings.Fields(strings.Join(args, " ")) {
		id, err := strconv.Atoi(arg)
		if err != nil {
			if errors.Is(err, strconv.ErrSyntax) {
				return nil, fmt.Errorf("%w: '%s'", bookmark.ErrInvalidRecordID, arg)
			}
			return nil, fmt.Errorf("%w", err)
		}
		ids = append(ids, id)
	}

	return ids, nil
}

// confirmProceed prompts the user to confirm or edit the given bookmark slice.
// If the user confirms, the buffer is returned as-is.
// If the user edits the buffer, the edited buffer is returned.
// If the user aborts the action, an error is returned.
func confirmProceed(bs *bookmark.Slice, editFn bookmark.EditFn) (bool, error) {
	options := []string{"Yes", "No", "Edit"}
	s := fmt.Sprintf("Delete %d bookmarks?", len(*bs))
	option := promptWithOptions(s, options)

	switch option {
	case "y", "yes":
		return true, nil
	case "n", "no":
		return false, fmt.Errorf("%w", database.ErrActionAborted)
	case "e", "edit":
		if err := editFn(bs); err != nil {
			return false, fmt.Errorf("%w", err)
		}
		return false, nil
	}

	return false, fmt.Errorf("%w", database.ErrActionAborted)
}
