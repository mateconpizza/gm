package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/haaag/gm/internal/bookmark"
	"github.com/haaag/gm/internal/format"
	"github.com/haaag/gm/internal/format/color"
	"github.com/haaag/gm/internal/repo"
	"github.com/haaag/gm/internal/terminal"
	"github.com/haaag/gm/internal/util/frame"
	"github.com/haaag/gm/internal/util/scraper"
	"github.com/haaag/gm/internal/util/spinner"
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

// handleURL retrieves a URL from args or prompts the user for input.
func handleURL(border string, args *[]string) string {
	urlPrompt := color.Blue("+ URL\t:").Bold().String()

	// This checks if URL is provided and returns it
	if len(*args) > 0 {
		url := (*args)[0]
		*args = (*args)[1:]
		url = strings.TrimRight(url, "\n")
		fmt.Println(urlPrompt, url)

		return url
	}

	// Prompt user for URL
	urlPrompt += color.BrightRed("\n " + border).String()
	urlPrompt += color.GetANSI(color.BrightGray)

	return terminal.ReadInput(urlPrompt)
}

// handleTags retrieves tags from args or prompts the user for input.
func handleTags(border string, args *[]string) string {
	tagsPrompt := color.Purple("+ Tags\t:").Bold().String()

	// This checks if tags are provided and returns them
	if len(*args) > 0 {
		tags := (*args)[0]
		*args = (*args)[1:]
		tags = strings.TrimRight(tags, "\n")
		tags = strings.Join(strings.Fields(tags), ",")
		fmt.Println(tagsPrompt, tags)

		return tags
	}

	// Prompt user for tags
	tagsPrompt += color.Gray(" (comma-separated)").Italic().String()
	tagsPrompt += color.BrightRed("\n " + border).String()
	tagsPrompt += color.GetANSI(color.BrightGray)

	return terminal.ReadInput(tagsPrompt)
}

// fetchTitleAndDesc fetch and display title and description.
func fetchTitleAndDesc(url string, minWidth int) (title, desc string) {
	const _indentation = 10

	mesg := color.Yellow("Scraping webpage...").String()
	s := spinner.New(spinner.WithMesg(mesg))
	s.Start()

	sc := scraper.New(url)
	_ = sc.Scrape()

	title = sc.GetTitle()
	desc = sc.GetDesc()

	s.Stop()

	var r strings.Builder
	r.WriteString(color.Green("+ Title\t: ").Bold().String())
	r.WriteString(format.SplitAndAlignLines(title, minWidth, _indentation))
	r.WriteString(color.Yellow("\n+ Desc\t: ").Bold().String())
	r.WriteString(format.SplitAndAlignLines(desc, minWidth, _indentation))
	fmt.Println(r.String())

	return title, desc
}

// handleAdd fetch metadata and adds a new bookmark.
func handleAdd(r *repo.SQLiteRepository, args []string) error {
	if terminal.IsPiped() && len(args) < 2 {
		return fmt.Errorf("%w: URL or TAGS cannot be empty", bookmark.ErrInvalid)
	}

	f := frame.New(frame.WithColorBorder(color.Gray))

	header := color.Yellow("Add Bookmark").Bold().String()
	quit := color.Gray(" (ctrl+c to exit)").Italic().String()
	f.Header(header + quit)
	f.Newline().Render()

	// Retrieve URL
	url := handleURL(f.Border.Mid, &args)
	if url == "" {
		return bookmark.ErrURLEmpty
	}

	// WARN: do we need this trim? why?
	url = strings.TrimRight(url, "/")

	if r.HasRecord(r.Cfg.GetTableMain(), "url", url) {
		item, _ := r.GetByURL(r.Cfg.GetTableMain(), url)
		return fmt.Errorf("%w with id: %d", bookmark.ErrDuplicate, item.ID)
	}

	// Retrieve tags
	tags := handleTags(f.Border.Mid, &args)

	// Fetch title and description
	title, desc := fetchTitleAndDesc(url, terminal.MinWidth)

	// Create a new bookmark
	b := bookmark.New(url, title, bookmark.ParseTags(tags), desc)

	if !terminal.IsPiped() {
		if err := confirmEditOrSave(b); err != nil {
			return fmt.Errorf("%w", err)
		}
	}

	if _, err := r.Insert(r.Cfg.GetTableMain(), b); err != nil {
		return fmt.Errorf("%w", err)
	}

	success := color.Green("successfully").Italic().Bold()
	fmt.Println("\nbookmark added", success)

	return nil
}

func init() {
	rootCmd.AddCommand(addCmd)
}
