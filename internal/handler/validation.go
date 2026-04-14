package handler

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/mateconpizza/gm/internal/app"
	"github.com/mateconpizza/gm/internal/locker"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/menu"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
)

// confirmRemove prompts the user to confirm the action.
func confirmRemove(
	a *app.Context,
	m *menu.Menu[bookmark.Bookmark],
	bs []bookmark.Bookmark,
) ([]bookmark.Bookmark, error) {
	for !a.Cfg.Flags.Yes {
		n := len(bs)
		if n == 0 {
			return nil, db.ErrRecordNotFound
		}

		for i := range n {
			fmt.Println(txt.Frame(a.Console(), &bs[i]))
		}

		opts := []string{"yes", "no"}
		if n > 1 {
			opts = append(opts, "select")
		}

		c, p := a.Console(), a.Console().Palette()
		opt, err := c.Choose(fmt.Sprintf("%s %d bookmark/s?", p.BrightRed.Wrap("remove", p.Bold), n), opts, "n")
		if err != nil {
			return nil, err
		}

		switch strings.ToLower(opt) {
		case "n", "no":
			return nil, sys.ErrActionAborted
		case "y", "yes":
			return bs, nil
		case "s", "select":
			items, err := selectionWithMenu(m, bs, func(b *bookmark.Bookmark) string {
				return txt.Oneline(a.Console(), b)
			})
			if err != nil {
				return nil, err
			}

			bs = items
			fmt.Println()
		}
	}

	if len(bs) == 0 {
		return nil, db.ErrRecordNotFound
	}

	return bs, nil
}

// extractIDsFrom extracts IDs from a argument slice.
func extractIDsFrom(args []string) ([]int, error) {
	ids := make([]int, 0)
	if len(args) == 0 {
		return ids, nil
	}

	for arg := range strings.FieldsSeq(strings.Join(args, " ")) {
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

// validURL checks if a string is a valid URL.
func validURL(s string) bool {
	parsedURL, err := url.Parse(s)
	if err != nil {
		return false
	}

	return parsedURL.Scheme != "" && parsedURL.Host != ""
}
