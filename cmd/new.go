/*
Copyright © 2023 haaag <git.haaag@gmail.com>
*/package cmd

import (
	"fmt"

	"gomarks/pkg/bookmark"
	"gomarks/pkg/color"
	"gomarks/pkg/constants"
	"gomarks/pkg/scrape"
	"gomarks/pkg/util"

	"github.com/spf13/cobra"
)

func handleURL(args *[]string) string {
	var url string
	var urlPrompt string

	if len(*args) > 0 {
		url = (*args)[0]
		urlPrompt = fmt.Sprintf(
			"%s%s+ URL\t: %s%s%s",
			color.Bold,
			color.Blue,
			color.White,
			url,
			color.Reset,
		)
		*args = (*args)[1:]
		fmt.Println(urlPrompt)
	} else {
		urlPrompt = fmt.Sprintf("%s%s+ URL:%s", color.Bold, color.Blue, color.Reset)
		url = util.TakeInput(urlPrompt)
	}
	return url
}

func handleTags(args *[]string) string {
	var tags string

	if len(*args) > 0 {
		tags = (*args)[0]
		tagsPrompt := fmt.Sprintf(
			"%s%s+ Tags\t: %s%s%s",
			color.Bold,
			color.Purple,
			color.White,
			tags,
			color.Reset)
		fmt.Println(tagsPrompt)
	} else {
		tagsPrompt := fmt.Sprintf(
			"%s%s+ Tags\t:%s %s(comma separated)%s",
			color.Bold,
			color.Purple,
			color.Reset,
			color.Gray,
			color.Reset,
		)
		tags = util.TakeInput(tagsPrompt)
	}
	return tags
}

func handleTitle(url string) string {
	title, err := scrape.GetTitle(url)
	if err != nil {
		return ""
	}

	titlePrompt := fmt.Sprintf(
		"%s%s+ Title\t:%s %s%s%s",
		color.Bold,
		color.Green,
		color.Reset,
		color.Bold,
		title,
		color.Reset,
	)
	fmt.Println(titlePrompt)
	return title
}

func handleDesc(url string) string {
	maxLen := 80
	desc, err := scrape.GetDescription(url)
	if err != nil {
		return ""
	}
	titlePrompt := fmt.Sprintf(
		"%s%s+ Desc\t:%s %s%s%s",
		color.Bold,
		color.Yellow,
		color.Reset,
		color.Bold,
		util.SplitAndAlignString(desc, maxLen),
		color.Reset,
	)
	fmt.Println(titlePrompt)
	return desc
}

var newCmd = &cobra.Command{
	Use:   "new",
	Short: "add a new bookmark",
	Long:  "add a new bookmark",
	Run: func(_ *cobra.Command, args []string) {
		r := getDB()
		fmt.Printf(
			"%s%s%s: adding a new bookmark\n\t use %s%sctrl+c%s for quit\n\n",
			color.Bold,
			constants.AppName,
			color.Reset,
			color.Bold,
			color.Red,
			color.Reset,
		)

		url := handleURL(&args)

		if r.RecordExists(url, "bookmarks") {
			fmt.Println("Sha existe loco...")
			return
		}

		tags := handleTags(&args)
		title := handleTitle(url)
		desc := handleDesc(url)

		b := bookmark.Create(url, title, tags, desc)

		confirmPrompt := fmt.Sprintf(
			"\n%sSave bookmark?%s %s[Y/n]:%s ",
			color.Bold,
			color.Reset,
			color.Gray,
			color.Reset,
		)
		confirm := util.ConfirmChanges(confirmPrompt)
		if !confirm {
			return
		}
		if !b.IsValid() {
			return
		}
		fmt.Println("TAGS::", b.Tags)
		fmt.Println("Saving bookmark...")
	},
}

func init() {
	var url string
	var tags string
	newCmd.Flags().StringVarP(&url, "url", "u", "", "url for new bookmark")
	newCmd.Flags().StringVarP(&tags, "tags", "t", "", "tags for new bookmark")
	rootCmd.AddCommand(newCmd)
}
