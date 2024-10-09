package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/haaag/gm/internal/bookmark"
	"github.com/haaag/gm/internal/bookmark/scraper"
	"github.com/haaag/gm/internal/format"
	"github.com/haaag/gm/internal/format/color"
	"github.com/haaag/gm/internal/format/frame"
	"github.com/haaag/gm/internal/repo"
	"github.com/haaag/gm/internal/sys/spinner"
	"github.com/haaag/gm/internal/sys/terminal"
)

// addCmd represents the add command.
var addCmd = &cobra.Command{
	Use:   "add",
	Short: "add a new bookmark",
	PreRunE: func(cmd *cobra.Command, args []string) error {
		return verifyDatabase(Cfg)
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		r, err := repo.New(Cfg)
		if err != nil {
			return fmt.Errorf("%w", err)
		}
		defer r.Close()

		return handleAdd(r, args)
	},
}

// handleAdd adds a new bookmark.
func handleAdd(r *repo.SQLiteRepository, args []string) error {
	if terminal.IsPiped() && len(args) < 2 {
		return fmt.Errorf("%w: URL or TAGS cannot be empty", bookmark.ErrInvalid)
	}

	// header
	f := frame.New(frame.WithColorBorder(color.Gray))
	header := color.Yellow("Add Bookmark").Bold().String()
	q := color.Gray(" (ctrl+c to exit)").Italic().String()
	f.Header(header + q)
	f.Newline().Render()

	b := bookmark.New()
	if err := parseNewBookmark(r, b, args); err != nil {
		return err
	}

	if err := confirmEditSave(b); err != nil {
		if !errors.Is(err, bookmark.ErrBufferUnchanged) {
			return fmt.Errorf("%w", err)
		}
	}

	// insert new bookmark
	if _, err := r.Insert(r.Cfg.TableMain, b); err != nil {
		return fmt.Errorf("%w", err)
	}

	success := color.BrightGreen("Successfully").Italic().Bold()
	fmt.Printf("\n%s bookmark created\n", success)

	return nil
}

// handleURL retrieves a URL from args or prompts the user for input.
func handleURL(r *repo.SQLiteRepository, args *[]string) string {
	urlPrompt := color.Blue("+ URL\t:").Bold().String()

	// This checks if URL is provided and returns it
	if len(*args) > 0 {
		url := strings.TrimRight((*args)[0], "\n")
		fmt.Println(urlPrompt, url)
		*args = (*args)[1:]

		return url
	}

	fmt.Println(urlPrompt)

	return terminal.Input(func(err error) {
		r.Close()
		logErrAndExit(err)
	})
}

// handleTags retrieves the Tags from args or prompts the user for input.
func handleTags(r *repo.SQLiteRepository, args *[]string) string {
	tagsPrompt := color.Purple("+ Tags\t:").Bold().String()

	// This checks if tags are provided and returns them
	if len(*args) > 0 {
		tag := strings.TrimRight((*args)[0], "\n")
		tag = strings.Join(strings.Fields(tag), ",")
		fmt.Println(tagsPrompt, tag)

		*args = (*args)[1:]

		return tag
	}

	tagsPrompt += color.Gray(" (spaces|comma separated)").Italic().String()
	fmt.Println(tagsPrompt)

	tags, _ := repo.TagsCounter(r)

	quit := func(err error) {
		r.Close()
		logErrAndExit(err)
	}

	return terminal.InputTags(tags, quit)
}

// parseNewBookmark fetch metadata and parses the new bookmark.
func parseNewBookmark(r *repo.SQLiteRepository, b *Bookmark, args []string) error {
	// retrieve url
	url, err := parseURL(r, &args)
	if err != nil {
		return err
	}
	// retrieve tags
	tags := handleTags(r, &args)
	// fetch title and description
	title, desc := fetchTitleAndDesc(url, terminal.MinWidth)

	b.URL = url
	b.Title = title
	b.Tags = bookmark.ParseTags(tags)
	b.Desc = desc

	return nil
}

// fetchTitleAndDesc fetch and display title and description.
func fetchTitleAndDesc(url string, minWidth int) (title, desc string) {
	const _indentation = 10

	s := spinner.New(
		spinner.WithMesg(color.Yellow("scraping webpage...").String()),
		spinner.WithColor(color.BrightMagenta),
	)
	s.Start()

	sc := scraper.New(url)
	_ = sc.Scrape()

	title = sc.Title()
	desc = sc.Desc()

	s.Stop()

	var r strings.Builder
	r.WriteString(color.Green("+ Title\t: ").Bold().String())
	r.WriteString(format.SplitAndAlign(title, minWidth, _indentation))
	r.WriteString(color.Yellow("\n+ Desc\t: ").Bold().String())
	r.WriteString(format.SplitAndAlign(desc, minWidth, _indentation))
	fmt.Println(r.String())

	return title, desc
}

// parseURL parse URL from args.
func parseURL(r *repo.SQLiteRepository, args *[]string) (string, error) {
	url := handleURL(r, args)
	if url == "" {
		return url, bookmark.ErrURLEmpty
	}

	// WARN: do we need this trim? why?
	url = strings.TrimRight(url, "/")

	if r.HasRecord(r.Cfg.TableMain, "url", url) {
		item, _ := r.ByURL(r.Cfg.TableMain, url)
		return "", fmt.Errorf("%w with id: %d", bookmark.ErrDuplicate, item.ID)
	}

	return url, nil
}

func init() {
	rootCmd.AddCommand(addCmd)
}
