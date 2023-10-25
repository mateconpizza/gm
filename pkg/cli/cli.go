package cli

import (
	"errors"
	"fmt"
	db "gomarks/pkg/database"
)

var ErrInvalidInput = errors.New("invalid input type")

func HandleFormat(f string, bookmarks []db.Bookmark) error {
	switch f {
	case "json":
		j := db.ToJSON(&bookmarks)
		fmt.Println(j)
	case "pretty":
		for _, b := range bookmarks {
			fmt.Println(b.PrettyColorString())
		}
	case "plain":
		for _, b := range bookmarks {
			fmt.Println(b)
		}
	default:
		return fmt.Errorf("invalid output format: %s", f)
	}
	return nil
}
