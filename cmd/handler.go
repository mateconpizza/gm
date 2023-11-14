package cmd

import (
	"fmt"
	"math"

	"gomarks/pkg/bookmark"
	"gomarks/pkg/errs"
	"gomarks/pkg/format"
	"gomarks/pkg/menu"

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

func handleQuery(args []string) string {
	var query string
	if len(args) == 0 {
		query = ""
	} else {
		query = args[0]
	}
	return query
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
		default:
			return fmt.Errorf("%w: %s", errs.ErrOptionInvalid, picker)
		}
	}

	return nil
}

func handleHeadAndTail(cmd *cobra.Command, bs *bookmark.Slice) error {
	head, _ := cmd.Flags().GetInt("head")
	tail, _ := cmd.Flags().GetInt("tail")

	if head < 0 || tail < 0 {
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
