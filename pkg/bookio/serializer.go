package bookio

import (
	"bytes"
	"encoding/json"

	yaml "gopkg.in/yaml.v3"

	"github.com/mateconpizza/gm/pkg/bookmark"
)

// Serializer turns a bookmark record into bytes and back, in one format.
type Serializer interface {
	Marshal(r *bookmark.Record) ([]byte, error)
	Unmarshal(data []byte) (*bookmark.Record, error)
	Ext() string // file extension for this format, e.g. ".json"
}

type JSONSerializer struct{}

func (JSONSerializer) Ext() string { return JSONExtension }
func (JSONSerializer) Marshal(r *bookmark.Record) ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}

func (JSONSerializer) Unmarshal(data []byte) (*bookmark.Record, error) {
	var r bookmark.Record
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

type YAMLSerializer struct{}

func (YAMLSerializer) Ext() string { return ".yaml" }
func (YAMLSerializer) Marshal(r *bookmark.Record) ([]byte, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(r); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil { // flushes the encoder
		return nil, err
	}
	return buf.Bytes(), nil
}

func (YAMLSerializer) Unmarshal(data []byte) (*bookmark.Record, error) {
	var r bookmark.Record
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true) // typos in a hand-edited file are errors
	if err := dec.Decode(&r); err != nil {
		return nil, err
	}
	return &r, nil
}
