package cmd

import (
	"errors"
	"fmt"
	"math"

	"gomarks/pkg/bookmark"
	"gomarks/pkg/color"
	"gomarks/pkg/constants"
	"gomarks/pkg/database"
	"gomarks/pkg/errs"
	"gomarks/pkg/menu"
	"gomarks/pkg/util"

	"github.com/spf13/cobra"
)

type Formatter interface {
	Format() string
	Pretty()
}

type BookmarkFormatter struct {
	Bookmark *bookmark.Bookmark
	MaxLen   int
}

func (bf *BookmarkFormatter) Format() string {
	s := fmt.Sprintf(
		"%-4d %-*s %-10s",
		bf.Bookmark.ID,
		bf.MaxLen,
		util.ShortenString(bf.Bookmark.Title, bf.MaxLen),
		bf.Bookmark.Tags,
	)
	return s
}

func (bf *BookmarkFormatter) Pretty() string {
	return bf.Bookmark.String()
}

func exampleUsage(l []string) string {
	var s string
	for _, line := range l {
		s += fmt.Sprintf("  %s %s", constants.AppName, line)
	}
	return s
}

func cmdTitle(s string) {
	fmt.Printf(
		"%s%s%s: %s, use %s%sctrl+c%s for quit\n\n",
		color.Bold,
		constants.AppName,
		color.Reset,
		s,
		color.Bold,
		color.Red,
		color.Reset,
	)
}

func getDB() (*database.SQLiteRepository, error) {
	r, err := database.GetDB()
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}
	return r, nil
}

func handleMenu() (*menu.Menu, error) {
	menuName, err := rootCmd.Flags().GetString("menu")
	if err != nil {
		fmt.Println("err on getting menu:", err)
	}

	if menuName == "" {
		return nil, nil
	}

	m, err := menu.New(menuName)
	if err != nil {
		return nil, fmt.Errorf("error creating menu: %w", err)
	}

	return m, nil
}

func handleQuery(args []string) string {
	var query string
	if len(args) == 0 {
		query = ""
	} else {
		query = args[0]
	}
	return query
}

func handleFormatOutput() (string, error) {
	json, err := rootCmd.Flags().GetBool("json")
	if err != nil {
		return "", fmt.Errorf("error getting json flag: %w", err)
	}

	if json {
		return "json", nil
	}

	return "pretty", nil
}

func checkInitDB(_ *cobra.Command, _ []string) error {
	if _, err := getDB(); err != nil {
		if errors.Is(err, errs.ErrDBNotFound) {
			return fmt.Errorf("%w: use 'init' to initialise a new database", errs.ErrDBNotFound)
		}
		return fmt.Errorf("%w", err)
	}

	return nil
}

func handlePicker() (string, error) {
	picker, err := rootCmd.Flags().GetString("pick")
	if err != nil {
		return "", fmt.Errorf("error getting picker flag: %w", err)
	}

	return picker, nil
}

func handleHeadAndTail(cmd *cobra.Command, bs *bookmark.Slice) {
	head, _ := cmd.Flags().GetInt("head")
	tail, _ := cmd.Flags().GetInt("tail")

	if head > 0 {
		head = int(math.Min(float64(head), float64(bs.Len())))
		*bs = (*bs)[:head]
	}

	if tail > 0 {
		tail = int(math.Min(float64(tail), float64(bs.Len())))
		*bs = (*bs)[len(*bs)-tail:]
	}
}
