package cmd

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/haaag/gm/pkg/bookmark"
	"github.com/haaag/gm/pkg/editor"
	"github.com/haaag/gm/pkg/format"
	"github.com/haaag/gm/pkg/repo"
	"github.com/haaag/gm/pkg/terminal"
	"github.com/haaag/gm/pkg/util"
)

var (
	ErrActionAborted  = errors.New("action aborted")
	ErrURLNotProvided = errors.New("URL not provided")
	ErrUnknownField   = errors.New("field unknown")
)

var C = format.Color

// handleByField prints the selected field
func handleByField(bs *Slice) error {
	if Field == "" {
		return nil
	}

	var printer = func(b Bookmark) error {
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

// handleFormat prints the bookmarks in different formats
func handleFormat(bs *Slice) error {
	if !Prettify || Exit {
		return nil
	}
	bs.ForEach(func(b Bookmark) {
		fmt.Println(format.PrettyWithURLPath(&b, terminal.MaxWidth))
	})
	return nil
}

// handleOneline formats the bookmarks in oneline
func handleOneline(bs *Slice) error {
	if !Oneline {
		return nil
	}
	bs.ForEach(func(b Bookmark) {
		fmt.Print(format.Oneline(&b, terminal.Color, terminal.MaxWidth))
	})
	return nil
}

// handleJsonFormat formats the bookmarks in JSON
func handleJsonFormat(bs *Slice) error {
	if !Json {
		return nil
	}
	fmt.Println(string(format.ToJSON(bs.GetAll())))
	return nil
}

// handleHeadAndTail returns a slice of bookmarks with limited elements
func handleHeadAndTail(bs *Slice) error {
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

// handleByQuery
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

// handleByTags returns a slice of bookmarks based on the provided tags
func handleByTags(r *Repo, bs *Slice) error {
	if Tags == "" {
		return nil
	}
	if err := r.GetByTags(r.Cfg.GetTableMain(), Tags, bs); err != nil {
		return fmt.Errorf("byTags :%w", err)
	}
	if bs.Len() == 0 {
		return fmt.Errorf("%w tag: '%s'", repo.ErrRecordNoMatch, Tags)
	}
	bs.Filter(func(b Bookmark) bool {
		for _, s := range strings.Split(b.Tags, ",") {
			if strings.TrimSpace(s) == Tags {
				return true
			}
		}
		return false
	})
	return nil
}

// handleAdd fetch metadata and adds a new bookmark
func handleAdd(r *Repo, args []string) error {
	if !Add {
		return nil
	}

	if terminal.Piped && len(args) < 2 {
		return fmt.Errorf("%w: URL or tags cannot be empty", bookmark.ErrInvalidInput)
	}

	fmt.Println(C("New bookmark\n").Yellow().Bold().String())
	url := bookmark.HandleURL(&args)
	if url == "" {
		return ErrURLNotProvided
	}

	// WARN: do we need this trim? why?
	url = strings.TrimRight(url, "/")

	if r.RecordExists(r.Cfg.GetTableMain(), "url", url) {
		item, _ := r.GetByURL(r.Cfg.GetTableMain(), url)
		return fmt.Errorf("%w with id: %d", bookmark.ErrBookmarkDuplicate, item.ID)
	}

	tags := bookmark.HandleTags(&args)
	title, desc := bookmark.HandleTitleAndDesc(url, terminal.MinWidth)
	b := bookmark.New(url, title, format.ParseTags(tags), desc)

	if !terminal.Piped {
		if err := confirmEditOrSave(b); err != nil {
			return fmt.Errorf("%w", err)
		}
	}

	if _, err := r.Insert(r.Cfg.GetTableMain(), b); err != nil {
		return fmt.Errorf("%w", err)
	}

	fmt.Println(C("new bookmark added successfully").Green())
	Exit = true
	return nil
}

// handleEdition renders the edition interface
func handleEdition(r *Repo, bs *Slice) error {
	if !Edit {
		return nil
	}

	var n = bs.Len()
	if n == 0 {
		return repo.ErrRecordQueryNotProvided
	}

	var header = "## [%d] %s\n## (%d/%d) bookmarks/s:\n\n"
	var edition = func(i int, b Bookmark) error {
		var buf = b.Buffer()
		var shortTitle = format.ShortenString(b.Title, terminal.MinWidth)
		editor.Append(fmt.Sprintf(header, b.ID, shortTitle, i+1, n), &buf)
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

		fmt.Printf("%s: id: [%d] %s\n", App.GetName(), b.ID, C("updated").Blue())
		return nil
	}

	if err := bs.ForEachIdx(edition); err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}

// handleRemove prompts the user the records to remove
func handleRemove(r *Repo, bs *Slice) error {
	if !Remove {
		return nil
	}

	if bs.Len() == 0 {
		return repo.ErrRecordNotFound
	}

	if terminal.Piped && !Force {
		return fmt.Errorf(
			"%w: input from pipe is not supported yet. use --force",
			ErrActionAborted,
		)
	}

	for !Force {
		var prompt string
		n := bs.Len()
		if n == 0 {
			return repo.ErrRecordNotFound
		}

		bs.ForEach(func(b Bookmark) {
			prompt += format.Delete(&b, terminal.MaxWidth) + "\n"
		})

		prompt += C("remove").Bold().Red().String() + fmt.Sprintf(" %d bookmark/s?", n)
		opt := terminal.ConfirmOrEdit(prompt, []string{"yes", "no", "edit"}, "n")

		switch opt {
		case "n", "no":
			return ErrActionAborted
		case "y", "yes":
			Force = true
		case "e", "edit":
			if err := filterBookmarkSelection(bs); err != nil {
				return err
			}
			terminal.Clear()
		}
	}

	if bs.Len() == 0 {
		return repo.ErrRecordNotFound
	}

	chDone := make(chan bool)
	go util.Spinner(chDone, C("removing record/s...").Gray().String())
	if err := r.DeleteAndReorder(bs, r.Cfg.GetTableMain(), r.Cfg.GetTableDeleted()); err != nil {
		return fmt.Errorf("deleting and reordering records: %w", err)
	}
	time.Sleep(time.Second * 1)
	chDone <- true

	fmt.Println(C("bookmark/s removed successfully").Green())
	return nil
}

// handleCheckStatus prints the status code of the bookmark URL
func handleCheckStatus(bs *Slice) error {
	if !Status {
		return nil
	}
	n := bs.Len()
	if n == 0 {
		return repo.ErrRecordQueryNotProvided
	}

	status := C("status").Green().Bold().String()
	if n > 15 && !terminal.Confirm(fmt.Sprintf("checking %s of %d, continue?", status, n), "y") {
		return ErrActionAborted
	}
	if err := bookmark.CheckStatus(bs); err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}

// handleCopyOpen performs an action on the bookmark
func handleCopyOpen(bs *Slice) error {
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

// handleBookmarksFromArgs retrieves records from the database based on either an
// ID or a query string.
func handleIDsFromArgs(r *Repo, bs *Slice, args []string) error {
	ids, err := extractIDsFromStr(args)
	if !errors.Is(err, bookmark.ErrInvalidRecordID) && err != nil {
		return fmt.Errorf("%w", err)
	}

	if len(ids) == 0 {
		return nil
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

// confirmEditOrSave confirms if the user wants to save the bookmark
func confirmEditOrSave(b *Bookmark) error {
	save := C("\nsave").Green().Bold().String() + " bookmark?"
	opt := terminal.ConfirmOrEdit(save, []string{"yes", "no", "edit"}, "y")

	switch opt {
	case "n":
		return fmt.Errorf("%w", ErrActionAborted)
	case "e":
		if err := bookmarkEdition(b); err != nil {
			return fmt.Errorf("%w", err)
		}
	}

	return nil
}

// handleRestore restores record/s from the deleted table
func handleRestore(r *Repo, bs *Slice) error {
	// TODO: extract logic, DRY in `handleRemove` too
	if !Restore {
		return nil
	}

	if !Deleted {
		err := repo.ErrRecordRestoreTable
		return fmt.Errorf("%w: use %s", err, C("--deleted").Bold().Red().String())
	}

	for !Force {
		var prompt string
		n := bs.Len()
		if n == 0 {
			return repo.ErrRecordNotFound
		}

		bs.ForEach(func(b Bookmark) {
			prompt += format.Other(&b, terminal.MaxWidth) + "\n"
		})

		prompt += C("restore").Bold().Yellow().String() + fmt.Sprintf(" %d bookmark/s?", n)
		opt := terminal.ConfirmOrEdit(prompt, []string{"yes", "no", "edit"}, "n")

		switch opt {
		case "n", "no":
			return ErrActionAborted
		case "y", "yes":
			Force = true
		case "e", "edit":
			if err := filterBookmarkSelection(bs); err != nil {
				return err
			}
			terminal.Clear()
		}
	}

	chDone := make(chan bool)
	go util.Spinner(chDone, C("restoring record/s...").Yellow().String())
	if err := r.Restore(bs); err != nil {
		return fmt.Errorf("%w: restoring bookmark", err)
	}
	time.Sleep(time.Second * 1)
	chDone <- true
	fmt.Println(C("bookmark/s restored successfully").Green())
	return nil
}
