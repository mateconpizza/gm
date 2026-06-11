package application

import "io"

type Git struct {
	Enabled bool   `json:"enabled" yaml:"enabled"` // Enable git
	Log     bool   `json:"logging" yaml:"logging"` // Enable logging
	Remote  string `json:"remote"  yaml:"remote"`  // Remote repo

	writer io.Writer // Writer for logging
}

func (g *Git) SetWriter(w io.Writer) { g.writer = w }
func (g *Git) Writer() io.Writer     { return g.writer }

func (g *Git) Status() string {
	if !g.Enabled {
		return "disabled"
	}
	return "enabled"
}

func (g *Git) Logging() string {
	if !g.Log {
		return "disabled"
	}
	return "enabled"
}

func (g *Git) Load() {
	if !g.Log {
		g.writer = io.Discard
	}
}
