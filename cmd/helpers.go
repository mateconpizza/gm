package cmd

import (
	"errors"
	"fmt"

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
		util.ShortenString(bf.Bookmark.Title.String, bf.MaxLen),
		bf.Bookmark.Tags,
	)
	return s
}

func (bf *BookmarkFormatter) Pretty() string {
	return bf.Bookmark.PrettyColorString()
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

func handleMenu() *menu.Menu {
	menuName, err := rootCmd.Flags().GetString("menu")
	if err != nil {
		fmt.Println("err on getting menu:", err)
	}

	if menuName == "" {
		return nil
	}

	return menu.New(menuName)
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

func checkInitDB(_ *cobra.Command, _ []string) error {
	if _, err := getDB(); err != nil {
		if errors.Is(err, errs.ErrDBNotFound) {
			return fmt.Errorf("%w: use 'init' to initialise a new database", errs.ErrDBNotFound)
		}
		return fmt.Errorf("%w", err)
	}

	return nil
}
