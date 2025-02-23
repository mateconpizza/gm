package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/haaag/rotato"
	"github.com/spf13/cobra"

	"github.com/haaag/gm/internal/bookmark"
	"github.com/haaag/gm/internal/bookmark/scraper"
	"github.com/haaag/gm/internal/config"
	"github.com/haaag/gm/internal/format"
	"github.com/haaag/gm/internal/format/color"
	"github.com/haaag/gm/internal/format/frame"
	"github.com/haaag/gm/internal/handler"
	"github.com/haaag/gm/internal/repo"
	"github.com/haaag/gm/internal/sys"
	"github.com/haaag/gm/internal/sys/files"
	"github.com/haaag/gm/internal/sys/terminal"
)

// newRecordCmd represents the add command.
var newRecordCmd = &cobra.Command{
	Use:     "new",
	Short:   "Add a new bookmark",
	Aliases: []string{"add"},
	RunE: func(cmd *cobra.Command, args []string) error {
		r, err := repo.New(Cfg)
		if err != nil {
			return fmt.Errorf("%w", err)
		}
		defer r.Close()
		// setup terminal and interrupt func handler (ctrl+c,esc handler)
		t := terminal.New(terminal.WithInterruptFn(func(err error) {
			r.Close()
			sys.ErrAndExit(err)
		}))
		defer t.CancelInterruptHandler()
		t.PipedInput(&args)

		return add(t, r, args)
	},
}

// add adds a new bookmark.
func add(t *terminal.Term, r *Repo, args []string) error {
	if t.IsPiped() && len(args) < 2 {
		return fmt.Errorf("%w: URL or TAGS cannot be empty", bookmark.ErrInvalid)
	}
	// header
	f := frame.New(frame.WithColorBorder(color.Gray))
	h := color.BrightYellow("Add Bookmark").String()
	f.Warning(h + color.Gray(" (ctrl+c to exit)\n").Italic().String()).Row("\n").Flush()

	b := bookmark.New()
	if err := addParseNewBookmark(t, r, b, args); err != nil {
		return err
	}
	// ask confirmation
	fmt.Println()
	if !config.App.Force && !t.IsPiped() {
		if err := addHandleConfirmation(t, b); err != nil {
			if !errors.Is(err, bookmark.ErrBufferUnchanged) {
				return fmt.Errorf("%w", err)
			}
		}
		t.ClearLine(1)
	}
	// insert new bookmark
	if err := r.Insert(b); err != nil {
		return fmt.Errorf("%w", err)
	}
	success := color.BrightGreen("Successfully").Italic().String()
	f.Success(success + " bookmark created\n").Flush()

	return nil
}

// addHandleConfirmation confirms if the user wants to save the bookmark.
func addHandleConfirmation(t *terminal.Term, b *Bookmark) error {
	f := frame.New(frame.WithColorBorder(color.Gray))
	save := color.BrightGreen("save").String()
	opt := t.Choose(f.Success(save).String()+" bookmark?", []string{"yes", "no", "edit"}, "y")

	switch strings.ToLower(opt) {
	case "n", "no":
		return fmt.Errorf("%w", sys.ErrActionAborted)
	case "e", "edit":
		if err := bookmarkEdition(b); err != nil {
			return fmt.Errorf("%w", err)
		}
	}

	return nil
}

// addHandleURL retrieves a URL from args or prompts the user for input.
func addHandleURL(t *terminal.Term, args *[]string) string {
	f := frame.New(frame.WithColorBorder(color.Gray))
	f.Header(color.BrightMagenta("URL\t:").String())
	// checks if url is provided
	if len(*args) > 0 {
		url := strings.TrimRight((*args)[0], "\n")
		f.Text(" " + color.Gray(url).String()).Ln().Flush()
		*args = (*args)[1:]

		return url
	}
	// checks clipboard
	c := addHandleClipboard(t)
	if c != "" {
		return c
	}

	f.Ln().Flush()
	url := t.Input(f.Border.Mid)
	f.Mid(color.BrightMagenta("URL\t:").String()).Text(" " + color.Gray(url).String()).Ln()
	// clean 'frame' lines
	t.ClearLine(format.CountLines(f.String()))
	f.Flush()

	return url
}

// addHandleTags retrieves the Tags from args or prompts the user for input.
func addHandleTags(t *terminal.Term, r *Repo, args *[]string) string {
	f := frame.New(frame.WithColorBorder(color.Gray))
	f.Header(color.BrightBlue("Tags\t:").String())
	// this checks if tags are provided, parses them and return them
	if len(*args) > 0 {
		tags := strings.TrimRight((*args)[0], "\n")
		tags = strings.Join(strings.Fields(tags), ",")
		tags = bookmark.ParseTags(tags)
		f.Text(" " + color.Gray(tags).String()).Ln().Flush()

		*args = (*args)[1:]

		return tags
	}
	// prompt for tags
	f.Text(color.Gray(" (spaces|comma separated)").Italic().String()).Ln().Flush()

	mTags, _ := repo.CounterTags(r)
	tags := bookmark.ParseTags(t.ChooseTags(f.Border.Mid, mTags))

	f.Clear().Mid(color.BrightBlue("Tags\t:").String()).
		Text(" " + color.Gray(tags).String()).Ln()

	t.ClearLine(format.CountLines(f.String()))
	f.Flush()

	return tags
}

// addParseNewBookmark fetch metadata and parses the new bookmark.
func addParseNewBookmark(t *terminal.Term, r *Repo, b *Bookmark, args []string) error {
	// retrieve url
	url, err := addParseURL(t, r, &args)
	if err != nil {
		return err
	}
	// retrieve tags
	tags := addHandleTags(t, r, &args)
	// fetch title and description
	title, desc := addTitleAndDesc(url)
	b.URL = url
	b.Title = title
	b.Tags = bookmark.ParseTags(tags)
	b.Desc = strings.Join(format.SplitIntoChunks(desc, terminal.MinWidth), "\n")

	return nil
}

// addTitleAndDesc fetch and display title and description.
func addTitleAndDesc(url string) (title, desc string) {
	sp := rotato.New(
		rotato.WithMesg("scraping webpage..."),
		rotato.WithMesgColor(rotato.ColorYellow),
		rotato.WithSpinnerColor(rotato.ColorBrightMagenta),
	)
	sp.Start()
	// scrape data
	sc := scraper.New(url)
	_ = sc.Scrape()
	title = sc.Title()
	desc = sc.Desc()
	sp.Done()

	const indentation int = 10

	f := frame.New(frame.WithColorBorder(color.Gray))

	width := terminal.MinWidth - len(f.Border.Row)
	titleColor := color.Gray(format.SplitAndAlign(title, width, indentation)).String()
	descColor := color.Gray(format.SplitAndAlign(desc, width, indentation)).String()
	f.Mid(color.BrightCyan("Title\t: ").String()).Text(titleColor).Ln().
		Mid(color.BrightOrange("Desc\t: ").String()).Text(descColor).Ln().
		Flush()

	return title, desc
}

// addParseURL parse URL from args.
func addParseURL(t *terminal.Term, r *Repo, args *[]string) (string, error) {
	url := addHandleURL(t, args)
	if url == "" {
		return url, bookmark.ErrURLEmpty
	}
	// WARN: do we need this trim? why?
	url = strings.TrimRight(url, "/")
	if r.HasRecord(r.Cfg.Tables.Main, "url", url) {
		item, _ := r.ByURL(r.Cfg.Tables.Main, url)
		return "", fmt.Errorf("%w with id: %d", bookmark.ErrDuplicate, item.ID)
	}

	return url, nil
}

// addHandleClipboard checks if there a valid URL in the clipboard.
func addHandleClipboard(t *terminal.Term) string {
	c := sys.ReadClipboard()
	if !handler.URLValid(c) {
		return ""
	}

	f := frame.New(frame.WithColorBorder(color.Gray))
	f.Mid(color.BrightCyan("found valid URL in clipboard\n").Italic().String()).Flush()
	lines := format.CountLines(f.String())
	bURL := f.Mid(color.BrightMagenta("URL\t:").String()).
		Text(" " + color.Gray(c).String() + "\n").Row("\n").
		Flush()
	opt := t.Choose(f.Mid("continue?").String(), []string{"yes", "no"}, "y")
	lines += format.CountLines(f.String())

	defer t.ClearLine(lines)

	switch opt {
	case "n", "no":
		t.ClearLine(lines)
		return ""
	default:
		fmt.Print(bURL)
		return c
	}
}

// bookmarkEdition edits a bookmark with a text editor.
func bookmarkEdition(b *Bookmark) error {
	te, err := files.GetEditor(config.App.Env.Editor)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	if err := bookmark.Edit(te, b.Buffer(), b); err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}
