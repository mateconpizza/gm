package cli

import (
	"fmt"
	c "gomarks/pkg/constants"
	"gomarks/pkg/database"
)

var Version = fmt.Sprintf("%s v%s", c.AppName, c.Version)

func HandleFormat(f string, bookmarks []database.Bookmark) error {
	switch f {
	case "json":
		j := database.ToJSON(&bookmarks)
		fmt.Println(j)
		return nil
	case "pretty":
		for _, b := range bookmarks {
			fmt.Println(b)
		}
		return nil
	case "pretty-color":
		fmt.Println("Pretty Print. Not implemented yet")
		return nil
	default:
		errMsg := fmt.Sprintf(
			"'%s' is not a valid format. supported formats: json, pretty, pretty-color",
			f,
		)
		fmt.Println(errMsg)
		return fmt.Errorf(errMsg)
	}
}
