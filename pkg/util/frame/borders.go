package frame

var defaultBorders = &FrameBorders{
	Header: "+ ",
	Row:    "| ",
	Mid:    "+ ",
	Footer: "+ ",
}

func SetDefaultBorders(b *FrameBorders) {
	defaultBorders = b
}

func NewBorders(header, row, mid, footer string) *FrameBorders {
	return &FrameBorders{
		Header: header,
		Row:    row,
		Mid:    mid,
		Footer: footer,
	}
}

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

func WithBordersAscii() OptFn {
	return func(o *Options) {
		o.Border = &FrameBorders{
			Header: "+- ",
			Row:    "|  ",
			Mid:    "+- ",
			Footer: "+- ",
		}
	}
}

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

func WithDefaultBorders() OptFn {
	return func(o *Options) {
		o.Border = defaultBorders
	}
}

func WithBordersCustom(header, row, mid, footer string) OptFn {
	return func(o *Options) {
		o.Border.Header = header
		o.Border.Row = row
		o.Border.Mid = mid
		o.Border.Footer = footer
	}
}

func WithNoBorders() OptFn {
	return func(o *Options) {
		o.Border.Header = ""
		o.Border.Row = ""
		o.Border.Mid = ""
		o.Border.Footer = ""
	}
}
