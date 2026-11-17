package bookio

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/mateconpizza/gm/pkg/bookmark"
)

func WriteJSON[T any](w io.Writer, v *T) error {
	data, err := json.MarshalIndent(&v, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshalling JSON: %w", err)
	}
	_, err = w.Write(data)
	return err
}

func ReadJSON(r io.Reader) ([]*bookmark.Bookmark, error) {
	var bj []*bookmark.Record

	j := json.NewDecoder(r)
	if err := j.Decode(&bj); err != nil {
		return nil, err
	}

	bs := make([]*bookmark.Bookmark, 0, len(bj))
	for i := range bj {
		bs = append(bs, bj[i].Bookmark())
	}

	return bs, nil
}
