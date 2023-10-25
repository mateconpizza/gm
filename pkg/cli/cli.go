package cli

import (
	"errors"
	"fmt"
	c "gomarks/pkg/constants"
	db "gomarks/pkg/database"
)

var Version = fmt.Sprintf("%s v%s", c.AppName, c.Version)
var ErrInvalidInput = errors.New("invalid input type")

func HandleFormat(f string, bookmarks []db.Bookmark) error {
	switch f {
	case "json":
		j := db.ToJSON(&bookmarks)
		fmt.Println(j)
		return nil
	case "pretty":
		for _, b := range bookmarks {
			fmt.Println(b.PrettyColorString())
		}
	default:
		for _, b := range bookmarks {
			fmt.Println(b)
		}
		return nil
	}
	return ErrInvalidInput
}
