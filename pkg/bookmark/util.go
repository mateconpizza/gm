package bookmark

import (
	"fmt"
	"log"
	"os/exec"
	"strings"

	"gomarks/pkg/format"
	"gomarks/pkg/terminal"
)

func Validate(b *Bookmark) error {
	if b.URL == "" {
		log.Print("bookmark is invalid. URL is empty")
		return ErrBookmarkURLEmpty
	}

	if b.Tags == "," || b.Tags == "" {
		log.Print("bookmark is invalid. Tags are empty")
		return ErrBookmarkTagsEmpty
	}

	log.Print("bookmark is valid")

	return nil
}

func HandleURL(args *[]string) string {
	urlPrompt := format.Text("+ URL\t:").Blue().Bold()

	if len(*args) > 0 {
		url := (*args)[0]
		*args = (*args)[1:]
		url = strings.TrimRight(url, "\n")
		fmt.Println(urlPrompt, url)
		return url
	}

	return terminal.InputFromUserPrompt(urlPrompt.String())
}

func HandleTags(args *[]string) string {
	tagsPrompt := format.Text("+ Tags\t:").Purple().Bold().String()

	if len(*args) > 0 {
		tags := (*args)[0]
		*args = (*args)[1:]
		tags = strings.TrimRight(tags, "\n")
		t := strings.Fields(tags)
		tags = strings.Join(t, ",")
		fmt.Println(tagsPrompt, tags)
		return tags
	}

	c := format.Text(" (comma-separated)").Gray().String()
	return terminal.InputFromUserPrompt(tagsPrompt + c)
}

func HandleDesc(url string) string {
	indentation := 10
	fmt.Print(format.Text("+ Desc\t: ").Yellow())
	sc := NewScraper(url)
	desc, _ := sc.Description()
	fmt.Println(format.SplitAndAlignString(desc, terminal.Settings.MinWidth, indentation))
	return desc
}

func HandleTitle(url string) string {
	indentation := 10
	fmt.Print(format.Text("+ Title\t: ").Green().Bold())
	sc := NewScraper(url)
	title, _ := sc.Title()
	fmt.Println(format.SplitAndAlignString(title, terminal.Settings.MinWidth, indentation))
	return title
}

func Format(f string, bs []Bookmark) error {
	switch f {
	case "json":
		j := string(format.ToJSON(bs))
		fmt.Println(j)
	case "pretty":
		for _, b := range bs {
			fmt.Println(b.String())
		}
	case "beta":
		for _, b := range bs {
			fmt.Println(b.BetaString())
		}
	case "menu":
		const maxIDLen = 5
		const maxTagsLen = 18
		const tagsPercentage = 30
		const totalPercentage = 100
		maxLine := terminal.Settings.MaxWidth - maxIDLen
		template := "%-*d%-*s%-*s\n"

		for _, b := range bs {
			lenTags := maxLine * tagsPercentage / totalPercentage
			lenUrls := maxLine - lenTags
			fmt.Printf(
				template,
				maxIDLen,
				b.ID,
				maxLine-lenTags,
				format.ShortenString(b.URL, lenUrls),
				maxTagsLen,
				format.ShortenString(b.Tags, lenTags),
			)
		}
	default:
		return fmt.Errorf("%w: %s", format.ErrInvalidOption, f)
	}

	return nil
}

func ExtractIDs(bs *[]Bookmark) []int {
	ids := make([]int, 0, len(*bs))
	for _, b := range *bs {
		ids = append(ids, b.ID)
	}
	return ids
}

func FilterSliceByIDs(bs *[]Bookmark, ids ...int) {
	keepMap := make(map[int]bool)
	for _, id := range ids {
		keepMap[id] = true
	}

	filteredSlice := make([]Bookmark, 0, len(*bs))
	for _, b := range *bs {
		if keepMap[b.ID] {
			filteredSlice = append(filteredSlice, b)
		}
	}

	*bs = filteredSlice
}

func RemoveItemByID(bs *[]Bookmark, idToRemove int) {
	var updatedBookmarks []Bookmark

	for _, bookmark := range *bs {
		if bookmark.ID != idToRemove {
			updatedBookmarks = append(updatedBookmarks, bookmark)
		}
	}

	*bs = updatedBookmarks
}

func logItemsNotFound(items *[]Bookmark, ids []int) {
	itemsFound := make(map[int]bool)
	for _, b := range *items {
		itemsFound[b.ID] = true
	}

	for _, item := range ids {
		if !itemsFound[item] {
			log.Printf("item with ID '%d' not found.\n", item)
		}
	}
}

func binaryExists(binaryName string) bool {
	cmd := exec.Command("which", binaryName)
	err := cmd.Run()

	return err == nil
}
