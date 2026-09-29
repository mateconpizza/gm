package application

import "github.com/mateconpizza/gm/internal/ui/formatter"

type UI struct {
	Format    string              `json:"format"                  yaml:"format"`
	GlyphMode formatter.GlyphMode `json:"glyphs"                  yaml:"glyphs"`
	Custom    *formatter.Glyphs   `json:"custom_glyphs,omitempty" yaml:"custom_glyphs,omitempty"`

	formatter formatter.Formatter
	glyphs    *formatter.Glyphs
}

func newUI() *UI {
	fm, _ := formatter.New(formatter.Format(OutputFormat))
	return &UI{
		Format:    OutputFormat,
		GlyphMode: formatter.GlyphDefault,
		formatter: fm,
		glyphs:    formatter.NewGlyphs(formatter.GlyphDefault, nil),
	}
}

func (u *UI) WithFormatter(f formatter.Formatter) *UI {
	u.formatter = f
	return u
}

func (u *UI) WithGlyphs(g *formatter.Glyphs) *UI {
	u.glyphs = g
	return u
}

func loadGlyphs(app *App) error {
	if err := app.UI.GlyphMode.Valid(); err != nil {
		return err
	}

	app.WithGlyphs(
		formatter.NewGlyphs(
			app.UI.GlyphMode,
			app.UI.Custom,
		),
	)

	return nil
}
