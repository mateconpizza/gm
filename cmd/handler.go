package cmd

import (
	"errors"
	"fmt"
	"math"
	"os"
	"strconv"

	"gomarks/pkg/bookmark"
	"gomarks/pkg/config"
	"gomarks/pkg/format"
	"gomarks/pkg/util"

	"github.com/spf13/cobra"
)

func handleQuery(args []string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("%w", bookmark.ErrNoIDorQueryProvided)
	}

	queryOrID, err := util.NewGetInput(args)
	if err != nil {
		return "", fmt.Errorf("%w", err)
	}

	return queryOrID, nil
}

func handleFormat(cmd *cobra.Command, bs *[]bookmark.Bookmark) error {
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

func handlePicker(cmd *cobra.Command, bs *[]bookmark.Bookmark) error {
	picker, _ := cmd.Flags().GetString("pick")

	if picker == "" {
		return nil
	}

	maxIDLen := 5
	maxTagsLen := 10
	maxURLLen := config.Term.MaxWidth - (maxIDLen + maxTagsLen)

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
			return fmt.Errorf("%w: %s", format.ErrInvalidOption, picker)
		}
	}

	return nil
}

func handleAllFlag(cmd *cobra.Command, args []string) []string {
	all, _ := cmd.Flags().GetBool("all")

	if all {
		args = append(args, "")
	}

	return args
}

func handleHeadAndTail(cmd *cobra.Command, bs *[]bookmark.Bookmark) error {
	head, _ := cmd.Flags().GetInt("head")
	tail, _ := cmd.Flags().GetInt("tail")

	if head < 0 {
		return fmt.Errorf("%w: %d %d", format.ErrInvalidOption, head, tail)
	}

	if tail < 0 {
		return fmt.Errorf("%w: %d %d", format.ErrInvalidOption, head, tail)
	}

	if head > 0 {
		head = int(math.Min(float64(head), float64(len(*bs))))
		*bs = (*bs)[:head]
	}

	if tail > 0 {
		tail = int(math.Min(float64(tail), float64(len(*bs))))
		*bs = (*bs)[len(*bs)-tail:]
	}

	return nil
}

// handleGetRecords retrieves records from the database based on either an ID or a query string.
func handleGetRecords(r *bookmark.SQLiteRepository, args []string) (*[]bookmark.Bookmark, error) {
	// FIX: split into more functions
	ids, err := extractIDs(args)
	if !errors.Is(err, bookmark.ErrInvalidRecordID) && err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	if len(ids) > 0 {
		bookmarks := make([]bookmark.Bookmark, 0, len(ids))

		for _, id := range ids {
			b, err := r.GetByID(config.DB.Table.Main, id)
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
		b, err = r.GetByID(config.DB.Table.Main, id)
		if err != nil {
			return nil, fmt.Errorf("getting record by id '%d': %w", id, err)
		}
		return &[]bookmark.Bookmark{*b}, nil
	}

	bs, err := r.GetByQuery(config.DB.Table.Main, queryOrID)
	if err != nil {
		return nil, fmt.Errorf("getting records by query '%s': %w", queryOrID, err)
	}
	return bs, nil
}

func handleInfoFlag(r *bookmark.SQLiteRepository) {
	lastMainID := r.GetMaxID(config.DB.Table.Main)
	lastDeletedID := r.GetMaxID(config.DB.Table.Deleted)

	if formatFlag == "json" {
		config.DB.Records.Main = lastMainID
		config.DB.Records.Deleted = lastDeletedID
		fmt.Println(format.ToJSON(config.AppConf))
	} else {
		fmt.Println(config.ShowInfo(lastMainID, lastDeletedID))
	}
}

func handleTermOptions() error {
	if util.IsOutputRedirected() {
		config.Term.Color = false
	}

	width, height, err := util.GetConsoleSize()
	if errors.Is(err, config.ErrNotTTY) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("getting console size: %w", err)
	}

	if width < config.Term.MinWidth {
		return fmt.Errorf("%w: %d. Min: %d", config.ErrTermWidthTooSmall, width, config.Term.MinWidth)
	}

	if height < config.Term.MinHeight {
		return fmt.Errorf("%w: %d. Min: %d", config.ErrTermHeightTooSmall, height, config.Term.MinHeight)
	}

	if width < config.Term.MaxWidth {
		config.Term.MaxWidth = width
	}

	return nil
}

func parseBookmarksAndExit(cmd *cobra.Command, bs *[]bookmark.Bookmark) {
	if status, _ := cmd.Flags().GetBool("status"); status {
		logErrAndExit(handleCheckStatus(cmd, bs))
		os.Exit(0)
	}

	if edition, _ := cmd.Flags().GetBool("edition"); edition {
		logErrAndExit(handleEdition(cmd, bs))
		os.Exit(0)
	}
}

func parseArgsAndExit(r *bookmark.SQLiteRepository) {
	if versionFlag {
		name := format.Text(config.App.Data.Title).Blue().Bold()
		fmt.Printf("%s v%s\n", name, config.App.Version)
		os.Exit(0)
	}

	if infoFlag {
		handleInfoFlag(r, formatFlag)
		os.Exit(0)
	}
}

func logErrAndExit(err error) {
	if err != nil {
		fmt.Printf("%s: %s\n", config.App.Name, err)
		os.Exit(1)
	}
}

func handleCheckStatus(cmd *cobra.Command, bs *[]bookmark.Bookmark) error {
	if len(*bs) == 0 {
		return bookmark.ErrBookmarkNotSelected
	}

	status, _ := cmd.Flags().GetBool("status")
	if !status {
		return nil
	}

	if err := bookmark.CheckStatus(bs); err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}

func handleEdition(cmd *cobra.Command, bs *[]bookmark.Bookmark) error {
	edition, _ := cmd.Flags().GetBool("edition")
	if !edition {
		return nil
	}

	if err := editAndDisplayBookmarks(bs); err != nil {
		return fmt.Errorf("error during editing: %w", err)
	}

	s := format.Text("\nexperimental:").Red().Bold()
	m := format.Text("nothing was updated").Blue()
	fmt.Println(s, m)

	return nil
}
