package handler

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/mateconpizza/gm/internal/deps"
	"github.com/mateconpizza/gm/internal/picker"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui/formatter"
	"github.com/mateconpizza/gm/internal/ui/menu"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
)

// confirmRemove prompts the user to confirm the action.
func confirmRemove(ctx context.Context, d *deps.Deps, bs []*bookmark.Bookmark) ([]*bookmark.Bookmark, error) {
	app, err := d.Application(ctx)
	if err != nil {
		return nil, err
	}
	m := picker.New[*bookmark.Bookmark](app, menu.WithMultiSelection())

	for !app.Flags.Yes {
		n := len(bs)
		if n == 0 {
			return nil, db.ErrRecordNotFound
		}

		for i := range n {
			fmt.Fprintln(d.Writer(), formatter.FrameFunc(d.Console(), bs[i]))
		}

		opts := []string{"yes", "no"}
		if n > 1 {
			opts = append(opts, "select")
		}

		c, p := d.Console(), d.Console().Palette()
		c.ClearLine(1)            // clean empty line from FrameFunc
		c.Frame().Rowln().Flush() // connect FrameFunc with prompt
		name := p.Bold.Sprint(app.DBBaseName())

		s := fmt.Sprintf("%s [%d] bookmark/s from %s?", p.BrightRed.Wrap("remove", p.Bold), n, name)
		opt, err := c.Choose(ctx, s, opts, "n")
		if err != nil {
			return nil, err
		}

		switch strings.ToLower(opt) {
		case "n", "no":
			return nil, sys.ErrActionAborted
		case "y", "yes":
			return bs, nil
		case "s", "select":
			m.SetFormatter(func(b **bookmark.Bookmark) string {
				return formatter.OnelineFunc(d.Console(), *b)
			})

			bs, err = m.Select(bs)
			if err != nil {
				return nil, err
			}

			fmt.Fprintln(d.Writer())
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

// ValidURL checks if a string is a valid URL.
func ValidURL(s string) bool {
	parsedURL, err := url.Parse(s)
	if err != nil {
		return false
	}

	return parsedURL.Scheme != "" && parsedURL.Host != ""
}
