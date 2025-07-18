package handler

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/locker"
	"github.com/mateconpizza/gm/internal/slice"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/color"
	"github.com/mateconpizza/gm/internal/ui/menu"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
)

// confirmRemove prompts the user to confirm the action.
func confirmRemove(c *ui.Console, m *menu.Menu[bookmark.Bookmark], bs *slice.Slice[bookmark.Bookmark]) error {
	for !config.App.Flags.Force {
		n := bs.Len()
		if n == 0 {
			return db.ErrRecordNotFound
		}

		bs.ForEach(func(b bookmark.Bookmark) {
			fmt.Println(txt.Frame(&b))
		})

		s := color.BrightRed("remove").Bold().String()

		opts := []string{"yes", "no"}
		if bs.Len() > 1 {
			opts = append(opts, "select")
		}

		opt, err := c.Choose(fmt.Sprintf("%s %d bookmark/s?", s, n), opts, "n")
		if err != nil {
			return fmt.Errorf("%w", err)
		}

		switch strings.ToLower(opt) {
		case "n", "no":
			return sys.ErrActionAborted
		case "y", "yes":
			return nil
		case "s", "select":
			items, err := selectionWithMenu(m, *bs.Items(), txt.Oneline)
			if err != nil {
				return err
			}

			bs.Set(&items)
			fmt.Println()
		}
	}

	if bs.Empty() {
		return db.ErrRecordNotFound
	}

	return nil
}

// confirmUserLimit prompts the user to confirm the exceeding limit.
func confirmUserLimit(c *ui.Console, count, maxItems int, q string) error {
	if config.App.Flags.Force || count < maxItems {
		return nil
	}

	if !c.Confirm(q+", continue?", "n") {
		return sys.ErrActionAborted
	}

	c.ReplaceLine(c.F.Midln(q).StringReset())

	return nil
}

// extractIDsFrom extracts IDs from a argument slice.
func extractIDsFrom(args []string) ([]int, error) {
	ids := make([]int, 0)
	if len(args) == 0 {
		return ids, nil
	}

	for _, arg := range strings.Fields(strings.Join(args, " ")) {
		id, err := strconv.Atoi(arg)
		if err != nil {
			if errors.Is(err, strconv.ErrSyntax) {
				continue
			}

			return nil, fmt.Errorf("%w", err)
		}

		ids = append(ids, id)
	}

	return ids, nil
}

// validateRemove checks if the remove operation is valid.
func validateRemove(bs []*bookmark.Bookmark, force bool) error {
	if len(bs) == 0 {
		return db.ErrRecordNotFound
	}

	if terminal.IsPiped() && !force {
		return fmt.Errorf(
			"%w: input from pipe is not supported yet. use --force",
			sys.ErrActionAborted,
		)
	}

	return nil
}

// passwordConfirm prompts user for password input.
func passwordConfirm(c *ui.Console) (string, error) {
	s, err := c.InputPassword("Password: ")
	if err != nil {
		return "", fmt.Errorf("%w", err)
	}

	fmt.Println()

	s2, err := c.InputPassword("Confirm Password: ")
	if err != nil {
		return "", fmt.Errorf("%w", err)
	}

	fmt.Println()

	if s != s2 {
		return "", locker.ErrPassphraseMismatch
	}

	return s, nil
}

// CheckDBLocked checks if the database is locked.
func CheckDBLocked(p string) error {
	err := locker.IsLocked(p)
	if err != nil {
		if errors.Is(err, locker.ErrItemLocked) {
			return db.ErrDBUnlockFirst
		}

		return fmt.Errorf("%w", err)
	}

	return nil
}

// validURL checks if a string is a valid URL.
func validURL(s string) bool {
	parsedURL, err := url.Parse(s)
	if err != nil {
		return false
	}

	return parsedURL.Scheme != "" && parsedURL.Host != ""
}
