package cmd

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
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
	PreRunE: func(cmd *cobra.Command, _ []string) error {
		return handler.CheckDBNotEncrypted(config.App.DBPath)
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		r, err := repo.New(config.App.DBPath)
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
	f.Header(h + color.Gray(" (ctrl+c to exit)\n").Italic().String()).Row("\n").Flush()

	b := bookmark.New()
	if err := parserNewBookmark(t, r, b, args); err != nil {
		return err
	}
	// ask confirmation
	fmt.Println()
	if !config.App.Force && !t.IsPiped() {
		if err := addHandleConfirmation(r, t, b); err != nil {
			if !errors.Is(err, bookmark.ErrBufferUnchanged) {
				return fmt.Errorf("%w", err)
			}
		}
	}
	// validate
	if err := bookmark.Validate(b); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}
	// insert new bookmark
	if err := r.InsertOne(context.Background(), b); err != nil {
		return fmt.Errorf("%w", err)
	}
	success := color.BrightGreen("Successfully").Italic().String()
	f.Success(success + " bookmark created\n").Flush()

	return nil
}

// addHandleConfirmation confirms if the user wants to save the bookmark.
func addHandleConfirmation(r *Repo, t *terminal.Term, b *Bookmark) error {
	f := frame.New(frame.WithColorBorder(color.Gray))
	opt, err := t.Choose(f.Clear().Question("save bookmark?").String(), []string{"yes", "no", "edit"}, "y")
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	switch strings.ToLower(opt) {
	case "n", "no":
		return fmt.Errorf("%w", sys.ErrActionAborted)
	case "e", "edit":
		t.ClearLine(1)
		if err := bookmarkEdition(r, t, b); err != nil {
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

	mTags, _ := repo.TagsCounter(r)
	tags := bookmark.ParseTags(t.ChooseTags(f.Border.Mid, mTags))

	f.Clear().Mid(color.BrightBlue("Tags\t:").String()).
		Text(" " + color.Gray(tags).String()).Ln()

	t.ClearLine(format.CountLines(f.String()))
	f.Flush()

	return tags
}

// parserNewBookmark fetch metadata and parses the new bookmark.
func parserNewBookmark(t *terminal.Term, r *Repo, b *Bookmark, args []string) error {
	// retrieve url
	url, err := parseNewURL(t, &args)
	if err != nil {
		return err
	}
	if b, exists := r.Has(url); exists {
		return fmt.Errorf("%w with id=%d", bookmark.ErrDuplicate, b.ID)
	}
	// retrieve tags
	tags := addHandleTags(t, r, &args)
	// fetch title and description
	title, desc := parseTitleAndDescription(url)
	b.URL = url
	b.Title = title
	b.Tags = bookmark.ParseTags(tags)
	b.Desc = strings.Join(format.SplitIntoChunks(desc, terminal.MinWidth), "\n")

	return nil
}

// parseTitleAndDescription fetch and display title and description.
func parseTitleAndDescription(url string) (title, desc string) {
	const indentation int = 10
	f := frame.New(frame.WithColorBorder(color.Gray))
	width := terminal.MinWidth - len(f.Border.Row)

	if titleFlag != "" {
		t := color.Gray(format.SplitAndAlign(titleFlag, width, indentation)).String()
		f.Mid(color.BrightCyan("Title\t: ").String()).Text(t).Ln().Flush()
		return titleFlag, desc
	}

	sp := rotato.New(
		rotato.WithMesg("scraping webpage..."),
		rotato.WithMesgColor(rotato.ColorYellow),
		rotato.WithSpinnerColor(rotato.ColorBrightMagenta),
	)
	sp.Start()
	// scrape data
	sc := scraper.New(url)
	if err := sc.Scrape(); err != nil {
		return title, desc
	}
	title = sc.Title()
	desc = sc.Desc()
	sp.Done()

	t := color.Gray(format.SplitAndAlign(title, width, indentation)).String()
	f.Mid(color.BrightCyan("Title\t: ").String()).Text(t).Ln()
	if desc != "" {
		descColor := color.Gray(format.SplitAndAlign(desc, width, indentation)).String()
		f.Mid(color.BrightOrange("Desc\t: ").String()).Text(descColor).Ln()
	}

	f.Flush()

	return title, desc
}

// parseNewURL parse URL from args.
func parseNewURL(t *terminal.Term, args *[]string) (string, error) {
	url := addHandleURL(t, args)
	if url == "" {
		return url, bookmark.ErrURLEmpty
	}

	// Trim trailing slash for future comparisons
	return strings.TrimRight(url, "/"), nil
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
	opt, err := t.Choose(f.Question("continue?").String(), []string{"yes", "no"}, "y")
	if err != nil {
		slog.Error("choosing", "error", err)
		return ""
	}

	lines += format.CountLines(f.String())
	f.Clear()

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
func bookmarkEdition(r *Repo, t *terminal.Term, b *Bookmark) error {
	te, err := files.NewEditor(config.App.Env.Editor)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	const spaces = 10
	buf := b.Buffer()
	sep := format.CenteredLine(terminal.MinWidth-spaces, "bookmark addition")
	format.BufferAppendEnd(" [New]", &buf)
	format.BufferAppend("#\n# "+sep+"\n\n", &buf)
	format.BufferAppend(fmt.Sprintf("# database: %q\n", r.Name()), &buf)
	format.BufferAppend(fmt.Sprintf("# %s:\tv%s\n", "version", config.App.Version), &buf)

	if err := bookmark.Edit(te, t, buf, b); err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}
