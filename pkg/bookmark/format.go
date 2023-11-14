package bookmark

import (
	"encoding/json"
	"fmt"
	"log"

	"gomarks/pkg/color"
	"gomarks/pkg/errs"
	"gomarks/pkg/util"
)

func Format(f string, bs *Slice) error {
	switch f {
	case "json":
		j := ToJSON(bs)
		fmt.Println(j)
	case "pretty":
		for _, b := range *bs {
			fmt.Println(b.String())
		}
		total := color.Colorize(fmt.Sprintf("total [%d]", bs.Len()), color.Gray)
		fmt.Println(total)
	default:
		return fmt.Errorf("%w: %s", errs.ErrOptionInvalid, f)
	}

	return nil
}

func ToJSON(data interface{}) string {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		log.Fatal("Error marshaling to JSON:", err)
	}

	jsonString := string(jsonData)

	return jsonString
}
