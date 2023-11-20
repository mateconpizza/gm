package cmd

import (
	"fmt"
	"math"
	"strconv"

	"gomarks/pkg/bookmark"
	"gomarks/pkg/color"
	"gomarks/pkg/config"
	"gomarks/pkg/database"
	"gomarks/pkg/errs"
	"gomarks/pkg/format"
	"gomarks/pkg/menu"
	"gomarks/pkg/scrape"
	"gomarks/pkg/util"

	"github.com/spf13/cobra"
)

func handleMenu() (*menu.Menu, error) {
	menuName, err := rootCmd.Flags().GetString("menu")
	if err != nil {
		fmt.Println("err on getting menu:", err)
	}

	if menuName == "" {
		return nil, nil
	}

	m, err := menu.New(menuName)
	if err != nil {
		return nil, fmt.Errorf("error creating menu: %w", err)
	}

	return m, nil
}

func handleQuery(args []string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("%w", errs.ErrNoIDorQueryPrivided)
	}

	queryOrID, err := util.NewGetInput(args)
	if err != nil {
		return "", fmt.Errorf("%w", err)
	}

	return queryOrID, nil
}

func handleFormat(cmd *cobra.Command, bs *bookmark.Slice) error {
	formatFlag, _ := cmd.Flags().GetString("format")
	picker, _ := cmd.Flags().GetString("pick")

	if picker != "" {
		return nil
	}

	if err := bookmark.Format(formatFlag, bs); err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}

func handlePicker(cmd *cobra.Command, bs *bookmark.Slice) error {
	picker, _ := cmd.Flags().GetString("pick")

	if picker == "" {
		return nil
	}

	maxLen := 80

	for _, b := range *bs {
		switch picker {
		case "id":
			fmt.Println(b.ID)
		case "url":
			fmt.Println(b.URL)
		case "title":
			fmt.Println(b.Title)
		case "tags":
			fmt.Println(b.Tags)
		case "menu":
			i := fmt.Sprintf(
				"%-4d %-80s %-10s",
				b.ID,
				format.ShortenString(b.URL, maxLen),
				b.Tags,
			)
			fmt.Println(i)
		default:
			return fmt.Errorf("%w: %s", errs.ErrOptionInvalid, picker)
		}
	}

	return nil
}

func handleHeadAndTail(cmd *cobra.Command, bs *bookmark.Slice) error {
	head, _ := cmd.Flags().GetInt("head")
	tail, _ := cmd.Flags().GetInt("tail")

	if head < 0 {
		return fmt.Errorf("%w: %d %d", errs.ErrOptionInvalid, head, tail)
	}

	if tail < 0 {
		return fmt.Errorf("%w: %d %d", errs.ErrOptionInvalid, head, tail)
	}

	if head > 0 {
		head = int(math.Min(float64(head), float64(bs.Len())))
		*bs = (*bs)[:head]
	}

	if tail > 0 {
		tail = int(math.Min(float64(tail), float64(bs.Len())))
		*bs = (*bs)[len(*bs)-tail:]
	}

	return nil
}

func handleNoConfirmation(cmd *cobra.Command) bool {
	noConfirm, err := cmd.Flags().GetBool("no-confirm")
	if err != nil {
		return false
	}
	return noConfirm
}

/**
 * Retrieves records from the database based on either an ID or a query string.
 *
 * @param r The SQLite repository to use for accessing the database.
 * @param args An array of strings containing either an ID or a query string.
 * @return A pointer to a `bookmark.Slice` containing the retrieved records, or an error if any occurred.
 */
func handleGetRecords(r *database.SQLiteRepository, args []string) (*bookmark.Slice, error) {
	queryOrID, err := handleQuery(args)
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	var id int
	var b *bookmark.Bookmark

	if id, err = strconv.Atoi(queryOrID); err == nil {
		b, err = r.GetRecordByID(config.DB.Table.Main, id)
		if err != nil {
			return nil, fmt.Errorf("getting record by id '%d': %w", id, err)
		}
		return bookmark.NewSlice(b), nil
	}

	bs, err := r.GetRecordsByQuery(config.DB.Table.Main, queryOrID)
	if err != nil {
		return nil, fmt.Errorf("getting records by query '%s': %w", queryOrID, err)
	}
	return bs, nil
}

func handleTitle(url string) string {
	maxLen := 80
	title, err := scrape.GetTitle(url)
	if err != nil {
		return ""
	}

	titlePrompt := color.ColorizeBold("+ Title\t:", color.Green)
	titleColor := color.ColorizeBold(format.SplitAndAlignString(title, maxLen), color.White)
	fmt.Println(titlePrompt, titleColor)
	return title
}

func handleDesc(url string) string {
	maxLen := 80
	desc, err := scrape.GetDescription(url)
	if err != nil {
		return ""
	}

	descPrompt := color.ColorizeBold("+ Desc\t:", color.Yellow)
	descColor := color.ColorizeBold(format.SplitAndAlignString(desc, maxLen), color.White)
	fmt.Println(descPrompt, descColor)
	return desc
}

func handleURL(args *[]string) string {
	urlPrompt := color.ColorizeBold("+ URL\t:", color.Blue)

	if len(*args) > 0 {
		url := (*args)[0]
		*args = (*args)[1:]
		fmt.Println(urlPrompt, url)
		return url
	}

	return util.GetInput(urlPrompt)
}

func handleTags(args *[]string) string {
	tagsPrompt := color.ColorizeBold("+ Tags\t:", color.Purple)

	if len(*args) > 0 {
		tags := (*args)[0]
		*args = (*args)[1:]
		fmt.Println(tagsPrompt, tags)
		return tags
	}

	c := color.Colorize(" (comma separated)", color.Gray)
	return util.GetInput(tagsPrompt + c)
}
