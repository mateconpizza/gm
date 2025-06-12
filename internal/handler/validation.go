package handler

import (
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/mateconpizza/gm/internal/bookmark"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/db"
	"github.com/mateconpizza/gm/internal/format/color"
	"github.com/mateconpizza/gm/internal/format/frame"
	"github.com/mateconpizza/gm/internal/locker"
	"github.com/mateconpizza/gm/internal/menu"
	"github.com/mateconpizza/gm/internal/slice"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/files"
	"github.com/mateconpizza/gm/internal/sys/terminal"
)

// confirmRemove prompts the user to confirm the action.
func confirmRemove(
	m *menu.Menu[bookmark.Bookmark],
	t *terminal.Term,
	bs *slice.Slice[bookmark.Bookmark],
	s string,
) error {
	for !config.App.Force {
		cs, err := getColorScheme(config.App.Colorscheme)
		if err != nil {
			return err
		}
		slog.Info("colorscheme loaded", "name", cs.Name)
		n := bs.Len()
		if n == 0 {
			return db.ErrRecordNotFound
		}
		bs.ForEach(func(b bookmark.Bookmark) {
			fmt.Println(bookmark.Frame(&b, cs))
		})

		f := frame.New(frame.WithColorBorder(color.BrightRed))
		opts := []string{"yes", "no"}
		if bs.Len() > 1 {
			opts = append(opts, "select")
		}
		opt, err := t.Choose(f.Question(fmt.Sprintf("%s %d bookmark/s?", s, n)).String(), opts, "n")
		if err != nil {
			return fmt.Errorf("%w", err)
		}
		switch strings.ToLower(opt) {
		case "n", "no":
			return sys.ErrActionAborted
		case "y", "yes":
			return nil
		case "s", "select":
			items, err := selectionWithMenu(m, *bs.Items(), fzfFormatter(false))
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
func confirmUserLimit(count, maxItems int, q string) error {
	if config.App.Force || count < maxItems {
		return nil
	}
	defer terminal.ClearLine(1)
	f := frame.New(frame.WithColorBorder(color.BrightBlue)).Header(q)
	if !terminal.Confirm(f.String(), "n") {
		return sys.ErrActionAborted
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
func validateRemove(bs *slice.Slice[bookmark.Bookmark], force bool) error {
	if bs.Empty() {
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

// ValidateDBExists verifies if the database exists.
func ValidateDBExists(p string) error {
	slog.Debug("validating database", "path", p)
	if files.Exists(p) {
		return nil
	}
	if err := locker.IsLocked(p); err != nil {
		return db.ErrDBLocked
	}
	i := color.BrightYellow(config.App.Cmd, "init").Italic()
	o := color.BrightYellow(config.App.Cmd, "new").Italic()
	// check if default db not found
	name := filepath.Base(p)
	if name == config.DefaultDBName {
		slog.Warn("default database not found", "name", name)
		return fmt.Errorf("%w: use %s to initialize", db.ErrDBMainNotFound, i)
	}
	ei := fmt.Errorf("%w: use %s or %s", db.ErrDBNotFound, i, o)
	dbs, err := db.Databases(filepath.Dir(p))
	if err != nil {
		return ei
	}
	if slices.Contains(dbs, name) {
		return nil
	}

	return ei
}

// passwordConfirm prompts user for password input.
func passwordConfirm(t *terminal.Term, f *frame.Frame) (string, error) {
	f.Question("Password: ").Flush()
	s, err := t.InputPassword()
	if err != nil {
		return "", fmt.Errorf("%w", err)
	}

	f.Ln().Question("Confirm Password: ").Flush()
	s2, err := t.InputPassword()
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

// AssertDefaultDatabaseExists checks if the default database exists.
func AssertDefaultDatabaseExists() error {
	p := filepath.Join(config.App.Path.Data, config.DefaultDBName)
	if !files.Exists(p) {
		i := color.BrightYellow(config.App.Cmd, "init").Italic()
		return fmt.Errorf("%w: use '%s' to initialize", db.ErrDBMainNotFound, i)
	}

	return nil
}

// URLValid checks if a string is a valid URL.
func URLValid(s string) bool {
	parsedURL, err := url.Parse(s)
	if err != nil {
		return false
	}

	return parsedURL.Scheme != "" && parsedURL.Host != ""
}
