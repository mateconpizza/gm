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
	"github.com/haaag/gm/internal/format"
	"github.com/haaag/gm/internal/format/color"
	"github.com/haaag/gm/internal/format/frame"
	"github.com/haaag/gm/internal/repo"
	"github.com/haaag/gm/internal/sys/files"
	"github.com/haaag/gm/internal/sys/terminal"
)

var ErrActionAborted = errors.New("action aborted")

var (
	// force is used to force the action, dont ask for confirmation.
	force *bool

	// subCommandCalled is used to check if the subcommand was called, to modify
	// some aspects of the program flow, and menu options.
	subCommandCalled bool
)

// Force sets the force flag.
func Force(f *bool) {
	force = f
}

func OnSubcommand() {
	subCommandCalled = true
}

// Confirmation prompts the user to confirm the action.
func Confirmation(t *terminal.Term, bs *Slice, prompt string, colors color.ColorFn) error {
	for !*force {
		n := bs.Len()
		if n == 0 {
			return repo.ErrRecordNotFound
		}

		bs.ForEachIdx(func(i int, b Bookmark) {
			fmt.Println(bookmark.FrameFormatted(&b, terminal.MinWidth, colors))
		})

		// render frame
		f := frame.New(frame.WithColorBorder(colors), frame.WithNoNewLine())
		q := f.Footer(prompt + fmt.Sprintf(" %d bookmark/s?", n)).String()
		opt := t.Choose(q, []string{"yes", "no", "edit"}, "n")
		opt = strings.ToLower(opt)
		switch opt {
		case "n", "no":
			return ErrActionAborted
		case "y", "yes":
			return nil
		case "e", "edit":
			if err := filterSlice(bs); err != nil {
				return err
			}
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
	f := frame.New(frame.WithColorBorder(color.BrightBlue), frame.WithNoNewLine()).Header(q)
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

// filterSlice select which item to remove from a slice using the
// text editor.
func filterSlice(bs *Slice) error {
	buf := bookmark.BufferSlice(bs)
	format.BufferAppendVersion(config.App.Name, config.App.Version, &buf)

	editor, err := files.Editor(config.App.Env.Editor)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	if err := files.Edit(editor, &buf); err != nil {
		return fmt.Errorf("on editing slice buffer: %w", err)
	}

	c := format.ByteSliceToLines(&buf)
	urls := bookmark.ExtractContentLine(&c)
	if len(urls) == 0 {
		return ErrActionAborted
	}

	bs.Filter(func(b Bookmark) bool {
		_, exists := urls[b.URL]
		return exists
	})

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
	init := fmt.Errorf("%w: use '%s' to initialize", repo.ErrDBNotFound, s)
	databases, err := repo.Databases(c.Path)
	if err != nil {
		return init
	}

	dbName, err := cmd.Flags().GetString("name")
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	// find with no|other extension
	databases.ForEach(func(r repo.SQLiteRepository) {
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
