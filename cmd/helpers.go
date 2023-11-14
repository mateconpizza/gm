package cmd

import (
	"errors"
	"fmt"
	"strconv"

	"gomarks/pkg/bookmark"
	"gomarks/pkg/color"
	"gomarks/pkg/constants"
	"gomarks/pkg/database"
	"gomarks/pkg/errs"
	"gomarks/pkg/format"

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
		format.ShortenString(bf.Bookmark.Title, bf.MaxLen),
		bf.Bookmark.Tags,
	)
	return s
}

func (bf *BookmarkFormatter) Pretty() string {
	return bf.Bookmark.String()
}

func checkInitDB(_ *cobra.Command, _ []string) error {
	if _, err := getDB(); err != nil {
		if errors.Is(err, errs.ErrDBNotFound) {
			return fmt.Errorf("%w: use 'init' to initialize a new database", errs.ErrDBNotFound)
		}
		return fmt.Errorf("%w", err)
	}

	return nil
}

func exampleUsage(l []string) string {
	var s string
	for _, line := range l {
		s += fmt.Sprintf("  %s %s", constants.AppName, line)
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
