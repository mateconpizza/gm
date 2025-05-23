package handler

import (
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/haaag/gm/internal/bookmark"
	"github.com/haaag/gm/internal/config"
	"github.com/haaag/gm/internal/format"
	"github.com/haaag/gm/internal/format/color"
	"github.com/haaag/gm/internal/format/frame"
	"github.com/haaag/gm/internal/locker"
	"github.com/haaag/gm/internal/menu"
	"github.com/haaag/gm/internal/repo"
	"github.com/haaag/gm/internal/sys"
	"github.com/haaag/gm/internal/sys/files"
	"github.com/haaag/gm/internal/sys/terminal"
)

// confirmRemove prompts the user to confirm the action.
func confirmRemove(m *menu.Menu[Bookmark], t *terminal.Term, bs *Slice, s string) error {
	for !config.App.Force {
		cs, err := getColorScheme(config.App.Colorscheme)
		if err != nil {
			return err
		}
		n := bs.Len()
		if n == 0 {
			return repo.ErrRecordNotFound
		}
		bs.ForEach(func(b Bookmark) {
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
			items, err := SelectionWithMenu(m, *bs.Items(), fzfFormatter(false))
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
func validateRemove(bs *Slice, force bool) error {
	if bs.Empty() {
		return repo.ErrRecordNotFound
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
		return repo.ErrDBLocked
	}
	i := color.BrightYellow(config.App.Cmd, "init").Italic()
	// check if default db not found
	name := filepath.Base(p)
	if name == config.DefaultDBName {
		slog.Warn("default database not found", "name", name)
		return fmt.Errorf("%w: use '%s' to initialize", repo.ErrDBMainNotFound, i)
	}
	ei := fmt.Errorf("%w %q: use '%s' to initialize", repo.ErrDBNotFound, name, i)
	dbs, err := repo.Databases(filepath.Dir(p))
	if err != nil {
		return ei
	}
	if slices.Contains(dbs, name) {
		return nil
	}

	return ei
}

// FindDB returns the path to the database.
func FindDB(p string) (string, error) {
	slog.Debug("searching db", "path", p)
	if files.Exists(p) {
		return p, nil
	}
	fs, err := repo.Databases(filepath.Dir(p))
	if err != nil {
		return "", fmt.Errorf("%w", err)
	}
	s := filepath.Base(p)
	for _, f := range fs {
		if strings.Contains(f, s) {
			return f, nil
		}
	}

	return "", fmt.Errorf("%w: %q", repo.ErrDBNotFound, format.StripSuffixes(s))
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

// CheckDBNotEncrypted checks if the database is encrypted.
func CheckDBNotEncrypted(p string) error {
	err := locker.IsLocked(p)
	if err != nil {
		if errors.Is(err, locker.ErrFileEncrypted) {
			return repo.ErrDBUnlockFirst
		}

		return fmt.Errorf("%w", err)
	}

	return nil
}
