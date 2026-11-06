package bookio

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	files "github.com/mateconpizza/gofiles"

	"github.com/mateconpizza/gm/pkg/bookmark"
)

const (
	JSONExtension   = ".json"
	summaryFile     = "summary.json"  // Git and database metadata
	trackerFilepath = ".tracked.json" // Tracked databases in Git
)

var JSONStrategy = &RepositoryLoader{
	Func:   jsonLoader,
	Prefix: "Loading JSON bookmarks",
	FileFilter: And(
		IsFile,
		HasExtension(JSONExtension),
		NotNamed(summaryFile, trackerFilepath),
	),
}

func jsonLoader(ctx context.Context, path string) (*bookmark.Bookmark, error) {
	bj := bookmark.NewRecord()
	if err := files.ReadJSON(path, bj); err != nil {
		return nil, fmt.Errorf("%w: %s", err, path)
	}

	return bj.Bookmark(), nil
}

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
