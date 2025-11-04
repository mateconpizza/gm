package frame

func NewBorders(header, row, mid, footer string) *FrameBorders {
	return &FrameBorders{
		Header: header,
		Row:    row,
		Mid:    mid,
		Footer: footer,
	}
}

// WithBordersRoundedCorner borders
//
//	header: ┌─
//	row:    │
//	mid:    ├─
//	footer: └─
func WithBordersRoundedCorner() OptFn {
	return func(o *Options) {
		o.Border = &FrameBorders{
			Header: "╭─ ",
			Row:    "│  ",
			Mid:    "├─ ",
			Footer: "╰─ ",
		}
	}
}

// WithBordersASCII borders
//
//	header: +-
//	row:    |
//	mid:    +-
//	footer: +-
func WithBordersASCII() OptFn {
	return func(o *Options) {
		o.Border = &FrameBorders{
			Header: "+- ",
			Row:    "|  ",
			Mid:    "+- ",
			Footer: "+- ",
		}
	}
}

// WithBordersDotted borders
//
//	header: ..
//	row:    .
//	mid:    ..
//	footer: ..
func WithBordersDotted() OptFn {
	return func(o *Options) {
		o.Border = &FrameBorders{
			Header: ".. ",
			Row:    ".  ",
			Mid:    ".. ",
			Footer: ".. ",
		}
	}
}

// WithBordersMidDotted borders
//
//	header: ··
//	row:    ·
//	mid:    ··
//	footer: ··
func WithBordersMidDotted() OptFn {
	return func(o *Options) {
		o.Border = &FrameBorders{
			Header: "·· ",
			Row:    "·  ",
			Mid:    "·· ",
			Footer: "·· ",
		}
	}
}

// WithBordersDouble borders
//
//	header: ╔═
//	row:    ║
//	mid:    ╠═
//	footer: ╚═
func WithBordersDouble() OptFn {
	return func(o *Options) {
		o.Border = &FrameBorders{
			Header: "╔═ ",
			Row:    "║  ",
			Mid:    "╠═ ",
			Footer: "╚═ ",
		}
	}
}

// WithBordersSingleLine borders
//
//	header: ┌─
//	row:    │
//	mid:    ├─
//	footer: └─
func WithBordersSingleLine() OptFn {
	return func(o *Options) {
		o.Border = &FrameBorders{
			Header: "┌─ ",
			Row:    "│  ",
			Mid:    "├─ ",
			Footer: "└─ ",
		}
	}
}

// WithBordersSimple borders
//
//	header: -
//	row:    |
//	mid:    -
//	footer: -
func WithBordersSimple() OptFn {
	return func(o *Options) {
		o.Border = &FrameBorders{
			Header: "- ",
			Row:    "| ",
			Mid:    "- ",
			Footer: "- ",
		}
	}
}

// WithBordersDashed borders
//
//	header: --
//	row:    |
//	mid:    --
//	footer: --
func WithBordersDashed() OptFn {
	return func(o *Options) {
		o.Border = &FrameBorders{
			Header: "-- ",
			Row:    "|  ",
			Mid:    "-- ",
			Footer: "-- ",
		}
	}
}

// WithBordersArtDeco borders
//
//	header: ╓─
//	row:    ║
//	mid:    ╠─
//	footer: ╙─
func WithBordersArtDeco() OptFn {
	return func(o *Options) {
		o.Border = &FrameBorders{
			Header: "╓─ ",
			Row:    "║  ",
			Mid:    "╠─ ",
			Footer: "╙─ ",
		}
	}
}

// WithBordersHeavy borders
//
//	header: ┏━
//	row:    ┃
//	mid:    ┣━
//	footer: ┗━
func WithBordersHeavy() OptFn {
	return func(o *Options) {
		o.Border = &FrameBorders{
			Header: "┏━ ",
			Row:    "┃  ",
			Mid:    "┣━ ",
			Footer: "┗━ ",
		}
	}
}

// WithBordersSolidSquare borders
//
//	header: ╭
//	row:    │
//	mid:    ├
//	footer: ╰
func WithBordersSolidSquare() OptFn {
	return func(o *Options) {
		o.Border = &FrameBorders{
			Header: "╭ ",
			Row:    "│  ",
			Mid:    "├ ",
			Footer: "╰ ",
		}
	}
}

// WithBordersHollowSquare borders
//
//	header: ╭
//	row:    │
//	mid:    ├
//	footer: ╰
func WithBordersHollowSquare() OptFn {
	return func(o *Options) {
		o.Border = &FrameBorders{
			Header: "╭ ",
			Row:    "│  ",
			Mid:    "├ ",
			Footer: "╰ ",
		}
	}
}

// WithBordersSolidBullet borders
//
//	header: ╭
//	row:    │
//	mid:    ├
//	footer: ╰
func WithBordersSolidBullet() OptFn {
	return func(o *Options) {
		o.Border = &FrameBorders{
			Header: "╭ ",
			Row:    "│  ",
			Mid:    "├ ",
			Footer: "╰ ",
		}
	}
}

// WithBordersHollowBullet borders
//
//	header: ╭
//	row:    │
//	mid:    ├
//	footer: ╰
func WithBordersHollowBullet() OptFn {
	return func(o *Options) {
		o.Border = &FrameBorders{
			Header: "╭ ",
			Row:    "│  ",
			Mid:    "├ ",
			Footer: "╰ ",
		}
	}
}

// WithBordersHollowDiamond borders
//
//	header: ╭
//	row:    │
//	mid:    ├
//	footer: ╰
func WithBordersHollowDiamond() OptFn {
	return func(o *Options) {
		o.Border = &FrameBorders{
			Header: "╭ ",
			Row:    "│  ",
			Mid:    "├ ",
			Footer: "╰ ",
		}
	}
}

// WithBordersSolidDiamond borders
//
//	header: ╭
//	row:    │
//	mid:    ├
//	footer: ╰
func WithBordersSolidDiamond() OptFn {
	return func(o *Options) {
		o.Border = &FrameBorders{
			Header: "╭ ",
			Row:    "│  ",
			Mid:    "├ ",
			Footer: "╰ ",
		}
	}
}

// WithBordersPlusSign borders
//
//	header: +
//	row:    |
//	mid:    +
//	footer: +
func WithBordersPlusSign() OptFn {
	return func(o *Options) {
		o.Border = &FrameBorders{
			Header: "+ ",
			Row:    "| ",
			Mid:    "+ ",
			Footer: "+ ",
		}
	}
}

// WithBordersCustom sets a custom border.
func WithBordersCustom(header, row, mid, footer string) OptFn {
	return func(o *Options) {
		o.Border = &FrameBorders{
			Header: header,
			Row:    row,
			Mid:    mid,
			Footer: footer,
		}
	}
}

// WithNoBorders sets no border.
func WithNoBorders() OptFn {
	return func(o *Options) {
		o.Border = &FrameBorders{
			Header: "",
			Row:    "",
			Mid:    "",
			Footer: "",
		}
	}
}
