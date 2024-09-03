package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/haaag/gm/pkg/bookmark"
	"github.com/haaag/gm/pkg/editor"
	"github.com/haaag/gm/pkg/format"
	"github.com/haaag/gm/pkg/format/color"
	"github.com/haaag/gm/pkg/qr"
	"github.com/haaag/gm/pkg/repo"
	"github.com/haaag/gm/pkg/terminal"
	"github.com/haaag/gm/pkg/util/spinner"
)

var (
	ErrActionAborted  = errors.New("action aborted")
	ErrURLNotProvided = errors.New("URL not provided")
	ErrUnknownField   = errors.New("field unknown")
)

// handleByField prints the selected field.
func handleByField(bs *Slice) error {
	if Field == "" {
		return nil
	}

	printer := func(b Bookmark) error {
		switch Field {
		case "id":
			fmt.Println(b.ID)
		case "url":
			fmt.Println(b.URL)
		case "title":
			fmt.Println(b.Title)
		case "tags":
			fmt.Println(b.Tags)
		case "desc":
			fmt.Println(b.Desc)
		default:
			return fmt.Errorf("%w: '%s'", ErrUnknownField, Field)
		}

		return nil
	}

	if err := bs.ForEachErr(printer); err != nil {
		return fmt.Errorf("%w", err)
	}
	Prettify = false

	return nil
}

// handleFormat prints the bookmarks in different formats.
func handleFormat(bs *Slice) error {
	if !Prettify || Exit {
		return nil
	}

	bs.ForEach(func(b Bookmark) {
		fmt.Println(format.PrettyWithURLPath(&b, terminal.MaxWidth) + "\n")
	})

	return nil
}

// handleOneline formats the bookmarks in oneline.
func handleOneline(bs *Slice) error {
	if !Oneline {
		return nil
	}

	bs.ForEach(func(b Bookmark) {
		fmt.Print(format.Oneline(&b, terminal.Color, terminal.MaxWidth))
	})

	return nil
}

// handleJSONFormat formats the bookmarks in JSON.
func handleJSONFormat(bs *Slice) error {
	if !JSON {
		return nil
	}

	fmt.Println(string(format.ToJSON(bs.GetAll())))

	return nil
}

// handleHeadAndTail returns a slice of bookmarks with limited
// elements.
func handleHeadAndTail(bs *Slice) error {
	if Head == 0 && Tail == 0 {
		return nil
	}

	if Head < 0 || Tail < 0 {
		return fmt.Errorf("%w: head=%d tail=%d", format.ErrInvalidOption, Head, Tail)
	}

	bs.Head(Head)
	bs.Tail(Tail)

	return nil
}

// handleListAll retrieves records from the database based on either an ID or a
// query string.
func handleListAll(r *Repo, bs *Slice) error {
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
func handleByQuery(r *Repo, bs *Slice, args []string) error {
	if bs.Len() != 0 || len(args) == 0 {
		return nil
	}

	query := strings.Join(args, "%")
	if err := r.GetByQuery(r.Cfg.GetTableMain(), query, bs); err != nil {
		return fmt.Errorf("%w: '%s'", err, strings.Join(args, " "))
	}

	return nil
}

// handleByTags returns a slice of bookmarks based on the
// provided tags.
func handleByTags(r *Repo, bs *Slice) error {
	if Tags == nil {
		return nil
	}

	for _, tag := range Tags {
		if err := r.GetByTags(r.Cfg.GetTableMain(), tag, bs); err != nil {
			return fmt.Errorf("byTags :%w", err)
		}
	}

	if bs.Len() == 0 {
		return fmt.Errorf("%w by tag: '%s'", repo.ErrRecordNoMatch, strings.Join(Tags, ", "))
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
func handleEdition(r *Repo, bs *Slice) error {
	if !Edit {
		return nil
	}

	n := bs.Len()
	if n == 0 {
		return repo.ErrRecordQueryNotProvided
	}

	header := "# [%d/%d] | %d | %s\n\n"

	// edition edits the bookmark with a text editor.
	edition := func(i int, b Bookmark) error {
		buf := b.Buffer()
		shortTitle := format.ShortenString(b.Title, terminal.MinWidth-10)
		editor.Append(fmt.Sprintf(header, i+1, n, b.ID, shortTitle), &buf)
		editor.AppendVersion(App.Name, App.Version, &buf)
		bufCopy := make([]byte, len(buf))
		copy(bufCopy, buf)

		if err := editor.Edit(&buf); err != nil {
			return fmt.Errorf("%w", err)
		}

		if editor.IsSameContentBytes(&buf, &bufCopy) {
			return nil
		}

		content := editor.Content(&buf)
		if err := editor.Validate(&content); err != nil {
			return fmt.Errorf("%w", err)
		}

		editedB := bookmark.ParseContent(&content)
		editedB.ID = b.ID
		b = *editedB

		if _, err := r.Update(r.Cfg.GetTableMain(), &b); err != nil {
			return fmt.Errorf("handle edition: %w", err)
		}

		fmt.Printf("%s: id: [%d] %s\n", App.GetName(), b.ID, color.Blue("updated").Bold())

		return nil
	}

	if err := bs.ForEachErrIdx(edition); err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}

// handleRemove prompts the user the records to remove.
func handleRemove(r *Repo, bs *Slice) error {
	if !Remove {
		return nil
	}

	if err := validateRemove(bs); err != nil {
		return err
	}

	prompt := color.Red("remove").Bold().String()
	if err := confirmAction(bs, prompt, color.Red); err != nil {
		return err
	}

	return removeRecords(r, bs)
}

// handleCheckStatus prints the status code of the bookmark
// URL.
func handleCheckStatus(bs *Slice) error {
	if !Status {
		return nil
	}

	n := bs.Len()
	if n == 0 {
		return repo.ErrRecordQueryNotProvided
	}

	status := color.Green("status").Bold().String()
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
		if err := copyToClipboard(b.URL); err != nil {
			return fmt.Errorf("%w", err)
		}
	}

	if Open {
		if err := openBrowser(b.URL); err != nil {
			return fmt.Errorf("%w", err)
		}
	}

	return nil
}

// handleBookmarksFromArgs retrieves records from the database
// based on either an ID or a query string.
func handleIDsFromArgs(r *Repo, bs *Slice, args []string) error {
	ids, err := extractIDsFromStr(args)
	if len(ids) == 0 {
		return nil
	}

	if !errors.Is(err, bookmark.ErrInvalidRecordID) && err != nil {
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

// handleRestore restores record/s from the deleted table.
func handleRestore(r *Repo, bs *Slice) error {
	if !Restore {
		return nil
	}

	if !Deleted {
		err := repo.ErrRecordRestoreTable
		del := color.Red("--deleted").Bold().String()
		return fmt.Errorf("%w: use %s to read from deleted records", err, del)
	}

	prompt := color.Yellow("restore").Bold().String()
	if err := confirmAction(bs, prompt, color.Yellow); err != nil {
		return err
	}

	s := spinner.New()
	s.Mesg = color.Yellow("restoring record/s...").String()
	s.Start()

	if err := r.Restore(bs); err != nil {
		return fmt.Errorf("%w: restoring bookmark", err)
	}

	s.Stop()
	fmt.Println("bookmark/s restored", color.Yellow("successfully").Bold())

	return nil
}

// handleQR handles creation, rendering or opening of
// QR-Codes.
func handleQR(bs *Slice) error {
	if !QR {
		return nil
	}

	Exit = true
	b := bs.Get(0)

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

// handleFrame prints the bookmarks in a frame.
func handleFrame(bs *Slice) error {
	if !Frame {
		return nil
	}

	bs.ForEachIdx(func(i int, b Bookmark) {
		format.WithFrame(&b, terminal.MinWidth)
		if i != bs.Len()-1 {
			// do not print the last line
			fmt.Println()
		}
	})

	return nil
}
