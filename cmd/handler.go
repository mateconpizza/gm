package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/haaag/gm/internal/bookmark"
	"github.com/haaag/gm/internal/config"
	"github.com/haaag/gm/internal/format"
	"github.com/haaag/gm/internal/format/color"
	"github.com/haaag/gm/internal/menu"
	"github.com/haaag/gm/internal/qr"
	"github.com/haaag/gm/internal/repo"
	"github.com/haaag/gm/internal/terminal"
	"github.com/haaag/gm/internal/util"
	"github.com/haaag/gm/internal/util/files"
)

var (
	ErrActionAborted = errors.New("action aborted")
	ErrInvalidOption = errors.New("invalid option")
)

// handleByField prints the selected field.
func handleByField(bs *Slice) error {
	if Field == "" {
		return nil
	}

	printer := func(b Bookmark) error {
		f, err := b.Field(Field)
		if err != nil {
			return fmt.Errorf("%w", err)
		}

		fmt.Println(f)

		return nil
	}

	if err := bs.ForEachErr(printer); err != nil {
		return fmt.Errorf("%w", err)
	}
	Exit = true

	return nil
}

// handlePrintOut prints the bookmarks in different formats.
func handlePrintOut(bs *Slice) error {
	if Exit {
		return nil
	}

	n := terminal.MinWidth
	lastIdx := bs.Len() - 1

	bs.ForEachIdx(func(i int, b Bookmark) {
		var output string
		if Prettify {
			output = bookmark.PrettyWithURLPath(&b, n) + "\n"
		}

		if Frame {
			output = bookmark.FormatWithFrame(&b, n)
		}

		if output != "" {
			fmt.Print(output)
			if i != lastIdx {
				fmt.Println()
			}
		}
	})

	return nil
}

// handleOneline formats the bookmarks in oneline.
func handleOneline(bs *Slice) error {
	if !Oneline {
		return nil
	}

	bs.ForEach(func(b Bookmark) {
		fmt.Print(bookmark.FormatOneline(&b, terminal.MaxWidth))
	})

	return nil
}

// handleJSONFormat formats the bookmarks in JSON.
func handleJSONFormat(bs *Slice) error {
	if !JSON {
		return nil
	}

	if bs.Len() == 0 {
		fmt.Println(string(format.ToJSON(config.App)))
		Exit = true
		return nil
	}

	fmt.Println(string(format.ToJSON(bs.GetAll())))

	return nil
}

// handleHeadAndTail returns a slice of bookmarks with limited elements.
func handleHeadAndTail(bs *Slice) error {
	if Head == 0 && Tail == 0 {
		return nil
	}

	if Head < 0 || Tail < 0 {
		return fmt.Errorf("%w: head=%d tail=%d", ErrInvalidOption, Head, Tail)
	}

	bs.Head(Head)
	bs.Tail(Tail)

	return nil
}

// handleListAll retrieves records from the database based on either an ID or a
// query string.
func handleListAll(r *repo.SQLiteRepository, bs *Slice) error {
	if !List {
		return nil
	}

	if err := r.GetAll(r.Cfg.GetTableMain(), bs); err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}

// handleByQuery executes a search query on the given repository based on
// provided arguments.
func handleByQuery(r *repo.SQLiteRepository, bs *Slice, args []string) error {
	if bs.Len() != 0 || len(args) == 0 {
		return nil
	}

	query := strings.Join(args, "%")
	if err := r.GetByQuery(r.Cfg.GetTableMain(), query, bs); err != nil {
		return fmt.Errorf("%w: '%s'", err, strings.Join(args, " "))
	}

	return nil
}

// handleByTags returns a slice of bookmarks based on the provided tags.
func handleByTags(r *repo.SQLiteRepository, bs *Slice) error {
	if Tags == nil {
		return nil
	}

	for _, tag := range Tags {
		if err := r.GetByTags(r.Cfg.GetTableMain(), tag, bs); err != nil {
			return fmt.Errorf("byTags :%w", err)
		}
	}

	if bs.Len() == 0 {
		t := strings.Join(Tags, ", ")
		return fmt.Errorf("%w by tag: '%s'", repo.ErrRecordNoMatch, t)
	}

	bs.Filter(func(b Bookmark) bool {
		for _, tag := range Tags {
			if !strings.Contains(b.Tags, tag) {
				return false
			}
		}

		return true
	})

	return nil
}

// handleEdition renders the edition interface.
func handleEdition(r *repo.SQLiteRepository, bs *Slice) error {
	if !Edit {
		return nil
	}

	n := bs.Len()
	if n == 0 {
		return repo.ErrRecordQueryNotProvided
	}

	header := "# [%d/%d] | %d | %s\n\n"
	editor, err := files.GetEditor(config.App.Env.Editor)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	// edition edits the bookmark with a text editor.
	edition := func(i int, b Bookmark) error {
		buf := bookmark.FormatBuffer(&b)
		shortTitle := format.ShortenString(b.Title, terminal.MinWidth-10)
		format.BufferAppend(fmt.Sprintf(header, i+1, n, b.ID, shortTitle), &buf)
		format.BufferApendVersion(config.App.Name, config.App.Version, &buf)
		bufCopy := make([]byte, len(buf))
		copy(bufCopy, buf)

		if err := files.Edit(editor, &buf); err != nil {
			return fmt.Errorf("%w", err)
		}

		if format.IsSameContentBytes(&buf, &bufCopy) {
			return nil
		}

		content := format.ByteSliceToLines(&buf)
		if err := bookmark.BufferValidate(&content); err != nil {
			return fmt.Errorf("%w", err)
		}

		editedB := bookmark.ParseContent(&content)
		editedB = bookmark.ScrapeAndUpdate(editedB)
		editedB.ID = b.ID
		b = *editedB

		if _, err := r.Update(r.Cfg.GetTableMain(), &b); err != nil {
			return fmt.Errorf("handle edition: %w", err)
		}

		fmt.Printf("%s: id: [%d] %s\n", config.App.Name, b.ID, color.Blue("updated").Bold())

		return nil
	}

	if err := bs.ForEachErrIdx(edition); err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}

// handleRemove prompts the user the records to remove.
func handleRemove(r *repo.SQLiteRepository, bs *Slice) error {
	if !Remove {
		return nil
	}

	if err := validateRemove(bs); err != nil {
		return err
	}

	prompt := color.BrightRed("remove").Bold().String()
	if err := confirmAction(bs, prompt, color.BrightRed); err != nil {
		return err
	}

	return removeRecords(r, bs)
}

// handleCheckStatus prints the status code of the bookmark URL.
func handleCheckStatus(bs *Slice) error {
	if !Status {
		return nil
	}

	n := bs.Len()
	if n == 0 {
		return repo.ErrRecordQueryNotProvided
	}

	status := color.BrightGreen("status").Bold().String()
	if n > 15 && !terminal.Confirm(fmt.Sprintf("checking %s of %d, continue?", status, n), "y") {
		return ErrActionAborted
	}

	if err := bookmark.CheckStatus(bs); err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}

// handleCopyOpen performs an action on the bookmark.
func handleCopyOpen(bs *Slice) error {
	if Exit {
		return nil
	}

	b := bs.Get(0)
	if Copy {
		if err := util.CopyClipboard(b.URL); err != nil {
			return fmt.Errorf("%w", err)
		}
	}

	if Open {
		if err := util.OpenInBrowser(b.URL); err != nil {
			return fmt.Errorf("%w", err)
		}
	}

	return nil
}

// handleBookmarksFromArgs retrieves records from the database based on either
// an ID or a query string.
func handleIDsFromArgs(r *repo.SQLiteRepository, bs *Slice, args []string) error {
	ids, err := extractIDsFromStr(args)
	if len(ids) == 0 {
		return nil
	}

	if !errors.Is(err, bookmark.ErrInvalidID) && err != nil {
		return fmt.Errorf("%w", err)
	}

	if err := r.GetByIDList(r.Cfg.GetTableMain(), ids, bs); err != nil {
		return fmt.Errorf("records from args: %w", err)
	}

	if bs.Len() == 0 {
		a := strings.TrimRight(strings.Join(args, " "), "\n")
		return fmt.Errorf("%w by id/s: %s", repo.ErrRecordNotFound, a)
	}

	return nil
}

// handleQR handles creation, rendering or opening of QR-Codes.
func handleQR(bs *Slice) error {
	if !QR {
		return nil
	}

	Exit = true

	qrFn := func(b Bookmark) error {
		qrcode := qr.New(b.GetURL())
		if err := qrcode.Generate(); err != nil {
			return fmt.Errorf("%w", err)
		}

		if Open {
			return openQR(qrcode, &b)
		}

		fmt.Println(b.GetTitle())
		qrcode.Render()
		fmt.Println(b.GetURL())

		return nil
	}

	if err := bs.ForEachErr(qrFn); err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}

// handleMenu allows the user to select multiple records.
func handleMenu(bs *Slice) error {
	if !Menu {
		return nil
	}
	if bs.Len() == 0 {
		return repo.ErrRecordNoMatch
	}
	// menu options
	options := []menu.OptFn{
		menu.WithDefaultKeybinds(),
		menu.WithKeybindEdit(),
		menu.WithKeybindOpen(),
		menu.WithPreview(),
		menu.WithMultiSelection(),
	}

	if Multiline {
		options = append(options, menu.WithMultilineView())
	}

	formatter := func(b Bookmark) string {
		if Multiline {
			return bookmark.Multiline(&b, terminal.MaxWidth)
		}

		return bookmark.FormatOneline(&b, terminal.MaxWidth)
	}

	m := menu.New[Bookmark](options...)

	result, err := m.Select(bs.GetAll(), formatter)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	n := len(result)
	if n == 0 {
		return nil
	}

	bs.Set(&result)

	return nil
}
