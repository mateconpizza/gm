package handler

import (
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/haaag/gm/internal/bookmark"
	"github.com/haaag/gm/internal/config"
	"github.com/haaag/gm/internal/format/color"
	"github.com/haaag/gm/internal/format/frame"
	"github.com/haaag/gm/internal/menu"
	"github.com/haaag/gm/internal/repo"
	"github.com/haaag/gm/internal/sys/terminal"
)

var ErrActionAborted = errors.New("action aborted")

// force is used to force the action, dont ask for confirmation.
var force *bool

// Force sets the force flag.
func Force(f *bool) {
	force = f
}

// Confirmation prompts the user to confirm the action.
func Confirmation(
	m *menu.Menu[Bookmark],
	t *terminal.Term,
	bs *Slice,
	prompt string,
	colors color.ColorFn,
) error {
	for !*force {
		n := bs.Len()
		if n == 0 {
			return repo.ErrRecordNotFound
		}
		bs.ForEach(func(b Bookmark) {
			fmt.Println(bookmark.FrameFormatted(&b, colors))
		})
		// render frame
		f := frame.New(frame.WithColorBorder(colors))
		q := f.Footer(prompt + fmt.Sprintf(" %d bookmark/s?", n)).String()
		opts := []string{"yes", "no"}
		if bs.Len() > 1 {
			opts = append(opts, "select")
		}
		opt := t.Choose(q, opts, "n")
		switch strings.ToLower(opt) {
		case "n", "no":
			return ErrActionAborted
		case "y", "yes":
			return nil
		case "s", "select":
			items, err := Selection(m, *bs.Items(), bookmark.FzfFormatter(false))
			if err != nil {
				return err
			}
			bs.Set(&items)
			fmt.Println()
		}
	}

	if bs.Empty() {
		return repo.ErrRecordNotFound
	}

	return nil
}

// confirmUserLimit prompts the user to confirm the exceeding limit.
func confirmUserLimit(count, maxItems int, q string) error {
	if *force || count < maxItems {
		return nil
	}
	defer terminal.ClearLine(1)
	f := frame.New(frame.WithColorBorder(color.BrightBlue)).Header(q)
	if !terminal.Confirm(f.String(), "n") {
		return ErrActionAborted
	}

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
func validateRemove(bs *Slice, force bool) error {
	if bs.Empty() {
		return repo.ErrRecordNotFound
	}

	if terminal.IsPiped() && !force {
		return fmt.Errorf(
			"%w: input from pipe is not supported yet. use --force",
			ErrActionAborted,
		)
	}

	return nil
}

// URLValid checks if a string is a valid URL.
func URLValid(s string) bool {
	parsedUrl, err := url.Parse(s)
	if err != nil {
		return false
	}

	return parsedUrl.Scheme != "" && parsedUrl.Host != ""
}

// ValidateDB verifies if the database exists.
func ValidateDB(cmd *cobra.Command, c *repo.SQLiteConfig) error {
	if c.Exists() {
		return nil
	}
	s := color.BrightYellow(config.App.Cmd, "init").Italic()
	init := fmt.Errorf("%w '%s': use '%s' to initialize", repo.ErrDBNotFound, c.Name, s)
	databases, err := repo.Databases(c.Path)
	if err != nil {
		return init
	}
	dbName, err := cmd.Flags().GetString("name")
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	// find with no|other extension
	databases.ForEachMut(func(r *repo.SQLiteRepository) {
		s := strings.TrimSuffix(r.Cfg.Name, filepath.Ext(r.Cfg.Name))
		if s == dbName {
			c.Name = r.Cfg.Name
		}
	})

	if c.Exists() {
		return nil
	}

	return init
}
