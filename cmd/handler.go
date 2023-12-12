package cmd

import (
	"errors"
	"fmt"
	"math"
	"os"
	"strconv"

	"gomarks/pkg/app"
	"gomarks/pkg/bookmark"
	"gomarks/pkg/database"
	"gomarks/pkg/errs"
	"gomarks/pkg/format"
	"gomarks/pkg/util"

	"github.com/spf13/cobra"
)

func handleQuery(args []string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("%w", errs.ErrNoIDorQueryProvided)
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

	maxIDLen := 5
	maxTagsLen := 10
	maxURLLen := app.Term.Max - (maxIDLen + maxTagsLen)

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
			fmt.Printf(
				"%-*d %-*s %-*s\n",
				maxIDLen,
				b.ID,
				maxURLLen,
				format.ShortenString(b.URL, maxURLLen),
				maxTagsLen,
				b.Tags,
			)
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

// handleGetRecords retrieves records from the database based on either an ID or a query string.
func handleGetRecords(r *database.SQLiteRepository, args []string) (*bookmark.Slice, error) {
	ids, err := extractIDs(args)
	if !errors.Is(err, errs.ErrIDInvalid) && err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	if len(ids) > 0 {
		bookmarks := make(bookmark.Slice, 0, len(ids))

		for _, id := range ids {
			b, err := r.GetRecordByID(app.DB.Table.Main, id)
			if err != nil {
				return nil, fmt.Errorf("getting record by ID '%d': %w", id, err)
			}
			bookmarks = append(bookmarks, *b)
		}

		return &bookmarks, nil
	}

	queryOrID, err := handleQuery(args)
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	var id int
	var b *bookmark.Bookmark

	if id, err = strconv.Atoi(queryOrID); err == nil {
		b, err = r.GetRecordByID(app.DB.Table.Main, id)
		if err != nil {
			return nil, fmt.Errorf("getting record by id '%d': %w", id, err)
		}
		return bookmark.NewSlice(b), nil
	}

	bs, err := r.GetRecordsByQuery(app.DB.Table.Main, queryOrID)
	if err != nil {
		return nil, fmt.Errorf("getting records by query '%s': %w", queryOrID, err)
	}
	return bs, nil
}

func handleTitle(url string) string {
	title, err := bookmark.FetchTitle(url)
	if err != nil {
		return ""
	}

	titlePrompt := format.Text("+ Title\t:").Green().Bold()
	titleColor := format.SplitAndAlignString(title, app.Term.Min)
	fmt.Println(titlePrompt, titleColor)
	return title
}

func handleDesc(url string) string {
	desc, err := bookmark.FetchDescription(url)
	if err != nil {
		return ""
	}

	descPrompt := format.Text("+ Desc\t:").Yellow()
	descColor := format.SplitAndAlignString(desc, app.Term.Min)
	fmt.Println(descPrompt, descColor)
	return desc
}

func handleURL(args *[]string) string {
	urlPrompt := format.Text("+ URL\t:").Blue().Bold()

	if len(*args) > 0 {
		url := (*args)[0]
		*args = (*args)[1:]
		fmt.Println(urlPrompt, url)
		return url
	}

	return util.GetInput(urlPrompt.String())
}

func handleTags(args *[]string) string {
	tagsPrompt := format.Text("+ Tags\t:").Purple().Bold().String()

	if len(*args) > 0 {
		tags := (*args)[0]
		*args = (*args)[1:]
		fmt.Println(tagsPrompt, tags)
		return tags
	}

	c := format.Text(" (comma-separated)").Gray().String()
	return util.GetInput(tagsPrompt + c)
}

func handleInfoFlag(r *database.SQLiteRepository) {
	lastMainID := r.GetMaxID(app.DB.Table.Main)
	lastDeletedID := r.GetMaxID(app.DB.Table.Deleted)

	fmt.Println(app.ShowInfo(lastMainID, lastDeletedID))
}

func handleMaxTermLen() error {
	w, h, err := util.GetConsoleSize()
	if err != nil && !errors.Is(err, errs.ErrNotTTY) {
		return fmt.Errorf("getting console size: %w", err)
	}

	app.Term.Width = w
	app.Term.Height = h

	if w < app.Term.Max && w > app.Term.Min {
		safeReduction := 2
		app.Term.Max = w - (app.Term.Min / safeReduction)
	}

	return nil
}

func parseArgsAndExit(cmd *cobra.Command, r *database.SQLiteRepository) {
	version, _ := cmd.Flags().GetBool("version")
	infoFlag, _ := cmd.Flags().GetBool("info")

	if version {
		name := format.Text(app.Info.Title).Blue().Bold()
		fmt.Printf("%s v%s\n", name, app.Config.Version)
		os.Exit(0)
	}

	if infoFlag {
		handleInfoFlag(r)
		os.Exit(0)
	}
}

func logErrAndExit(err error) {
	if err != nil {
		fmt.Printf("%s: %s\n", app.Config.Name, err)
		os.Exit(1)
	}
}
