package bookmark

import (
	"fmt"
	"gomarks/pkg/config"
	"gomarks/pkg/format"
	"gomarks/pkg/util"
	"log"
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
		fmt.Println(urlPrompt, url)
		return url
	}

	return util.GetInputFromPrompt(urlPrompt.String())
}

func HandleTags(args *[]string) string {
	tagsPrompt := format.Text("+ Tags\t:").Purple().Bold().String()

	if len(*args) > 0 {
		tags := (*args)[0]
		*args = (*args)[1:]
		fmt.Println(tagsPrompt, tags)
		return tags
	}

	c := format.Text(" (comma-separated)").Gray().String()
	return util.GetInputFromPrompt(tagsPrompt + c)
}

func HandleDesc(url string) string {
	desc, err := Description(url)
	if err != nil {
		return BookmarkDefaultDesc
	}

	descPrompt := format.Text("+ Desc\t:").Yellow()
	descColor := format.SplitAndAlignString(desc, config.Term.MinWidth)
	fmt.Println(descPrompt, descColor)
	return desc
}

func HandleTitle(url string) string {
	title, err := Title(url)
	if err != nil {
		return BookmarkDefaultTitle
	}

	titlePrompt := format.Text("+ Title\t:").Green().Bold()
	titleColor := format.SplitAndAlignString(title, config.Term.MinWidth)
	fmt.Println(titlePrompt, titleColor)
	return title
}

func Format(f string, bs []Bookmark) error {
	switch f {
	case "json":
		j := format.ToJSON(bs)
		fmt.Println(j)
	case "pretty":
		for _, b := range bs {
			fmt.Println(b.String())
		}
	case "menu":
		maxIDLen := 5
		maxTagsLen := 18
		maxLine := config.Term.MaxWidth - maxIDLen
		tagsPercentage := 30
		template := "%-*d%-*s%-*s\n"

		for _, b := range bs {
			lenTags := maxLine * tagsPercentage / 100
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
