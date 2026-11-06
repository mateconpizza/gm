package bookio

import (
	"io"

	yaml "gopkg.in/yaml.v3"
)

const YAMLExtension = ".yaml"

// WriteYAML writes v as YAML to provided wrtier.
func WriteYAML[T any](w io.Writer, v *T) error {
	enc := yaml.NewEncoder(w)
	enc.SetIndent(2)
	if err := enc.Encode(&v); err != nil {
		return err
	}
	return enc.Close()
}

// ReadYAML decodes YAML from r into v, rejecting unknown fields.
func ReadYAML[T any](r io.Reader, v *T) error {
	dec := yaml.NewDecoder(r)
	dec.KnownFields(true)
	return dec.Decode(&v)
}
