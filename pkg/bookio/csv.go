package bookio

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"

	"github.com/mateconpizza/gm/pkg/bookmark"
)

var (
	ErrInvalidHeader = errors.New("invalid header")
	ErrInvalidField  = errors.New("invalid field")
	ErrURLMissing    = errors.New("missing URL")
)

var CSVDefaultHeader = []string{"id", "url", "title", "desc", "created_at", "favorite", "notes"}

func ExportToCSV(bs []*bookmark.Bookmark, writer io.Writer, fields []string) error {
	if len(fields) == 0 {
		fields = CSVDefaultHeader
	}

	w := csv.NewWriter(writer)
	defer w.Flush()

	// build field map: db tag -> struct field index
	fieldMap := make(map[string]int)
	t := reflect.TypeFor[bookmark.Bookmark]()

	for i := 0; i < t.NumField(); i++ {
		tag := t.Field(i).Tag.Get("db")
		if tag != "" {
			fieldMap[tag] = i
		}
	}

	// validate requested fields
	for _, f := range fields {
		if _, ok := fieldMap[f]; !ok {
			return fmt.Errorf("%w: %q", ErrInvalidField, f)
		}
	}

	// write header
	if err := w.Write(fields); err != nil {
		return err
	}

	// write rows
	for _, b := range bs {
		v := reflect.ValueOf(b).Elem()

		row := make([]string, len(fields))

		for i, f := range fields {
			f = strings.ToLower(strings.TrimSpace(f))
			fv := v.Field(fieldMap[f])

			switch fv.Kind() {
			case reflect.String:
				row[i] = fv.String()

			case reflect.Int:
				row[i] = strconv.Itoa(int(fv.Int()))

			case reflect.Bool:
				row[i] = strconv.FormatBool(fv.Bool())

			default:
				row[i] = fmt.Sprintf("%v", fv.Interface())
			}
		}

		if err := w.Write(row); err != nil {
			return err
		}
	}

	return w.Error()
}

func ImportFromCSV(r io.Reader) ([]*bookmark.Bookmark, error) {
	cr := csv.NewReader(r)

	header, err := cr.Read()
	if err != nil {
		return nil, fmt.Errorf("reading header: %w", err)
	}

	// header index
	idx := make(map[string]int, len(header))
	for i, h := range header {
		idx[h] = i
	}

	if _, ok := idx["url"]; !ok {
		return nil, fmt.Errorf("%w: missing required field 'url'", ErrInvalidHeader)
	}

	var out []*bookmark.Bookmark

	for line := 2; ; line++ {
		record, err := cr.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", line, err)
		}

		b := bookmark.New()
		v := reflect.ValueOf(b).Elem()
		t := v.Type()

		get := func(name string) string {
			if i, ok := idx[name]; ok && i < len(record) {
				return record[i]
			}
			return ""
		}

		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			dbTag := field.Tag.Get("db")
			if dbTag == "" {
				continue
			}

			raw := get(dbTag)
			if raw == "" {
				continue // optional field
			}

			fv := v.Field(i)

			switch fv.Kind() {
			case reflect.String:
				fv.SetString(raw)

			case reflect.Int:
				n, err := strconv.Atoi(raw)
				if err != nil {
					return nil, fmt.Errorf("line %d: invalid %s %q: %w", line, dbTag, raw, err)
				}
				fv.SetInt(int64(n))

			case reflect.Bool:
				bv, err := strconv.ParseBool(raw)
				if err != nil {
					return nil, fmt.Errorf("line %d: invalid %s %q: %w", line, dbTag, raw, err)
				}
				fv.SetBool(bv)
			}
		}

		if b.URL == "" {
			return nil, fmt.Errorf("line %d: %w", line, ErrURLMissing)
		}

		out = append(out, b)
	}

	return out, nil
}
