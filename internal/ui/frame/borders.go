package frame

var (
	roundedCorner = NewBorders("╭─ ", "│  ", "├─ ", "╰─ ")
	asciiBorder   = NewBorders("+- ", "|  ", "+- ", "+- ")
	dottedBorder  = NewBorders(".. ", ".  ", ".. ", ".. ")
	midDotted     = NewBorders("·· ", "·  ", "·· ", "·· ")
	doubleBorder  = NewBorders("╔═ ", "║  ", "╠═ ", "╚═ ")
	singleBorder  = NewBorders("┌─ ", "│  ", "├─ ", "└─ ")
	simpleBorder  = NewBorders("- ", "| ", "- ", "- ")

	dashedBorder        = NewBorders("-- ", "|  ", "-- ", "-- ")
	artDecoBorder       = NewBorders("╓─ ", "║  ", "╠─ ", "╙─ ")
	heavyBorder         = NewBorders("┏━ ", "┃  ", "┣━ ", "┗━ ")
	solidSquareBorder   = NewBorders("╭ ", "│  ", "├ ", "╰ ")
	hollowSquareBorder  = NewBorders("╭ ", "│  ", "├ ", "╰ ")
	solidBulletBorder   = NewBorders("╭ ", "│  ", "├ ", "╰ ")
	hollowBulletBorder  = NewBorders("╭ ", "│  ", "├ ", "╰ ")
	hollowDiamondBorder = NewBorders("╭ ", "│  ", "├ ", "╰ ")
	solidDiamondBorder  = NewBorders("╭ ", "│  ", "├ ", "╰ ")
	plusSignBorder      = NewBorders("+ ", "| ", "+ ", "+ ")
	noBorder            = NewBorders("", "", "", "")
)

func setBorders(fb *FrameBorders) OptFn {
	return func(o *Options) {
		o.Border = fb
	}
}

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
//	header: ╭─
//	row:    │
//	mid:    ├─
//	footer: ╰─
func WithBordersRoundedCorner() OptFn { return setBorders(roundedCorner) }

// WithBordersASCII borders
//
//	header: +-
//	row:    |
//	mid:    +-
//	footer: +-
func WithBordersASCII() OptFn { return setBorders(asciiBorder) }

// WithBordersDotted borders
//
//	header: ..
//	row:    .
//	mid:    ..
//	footer: ..
func WithBordersDotted() OptFn { return setBorders(dottedBorder) }

// WithBordersMidDotted borders
//
//	header: ··
//	row:    ·
//	mid:    ··
//	footer: ··
func WithBordersMidDotted() OptFn { return setBorders(midDotted) }

// WithBordersDouble borders
//
//	header: ╔═
//	row:    ║
//	mid:    ╠═
//	footer: ╚═
func WithBordersDouble() OptFn { return setBorders(doubleBorder) }

// WithBordersSingleLine borders
//
//	header: ┌─
//	row:    │
//	mid:    ├─
//	footer: └─
func WithBordersSingleLine() OptFn { return setBorders(singleBorder) }

// WithBordersSimple borders
//
//	header: -
//	row:    |
//	mid:    -
//	footer: -
func WithBordersSimple() OptFn { return setBorders(simpleBorder) }

// WithBordersDashed borders
//
//	header: --
//	row:    |
//	mid:    --
//	footer: --
func WithBordersDashed() OptFn { return setBorders(dashedBorder) }

// WithBordersArtDeco borders
//
//	header: ╓─
//	row:    ║
//	mid:    ╠─
//	footer: ╙─
func WithBordersArtDeco() OptFn { return setBorders(artDecoBorder) }

// WithBordersHeavy borders
//
//	header: ┏━
//	row:    ┃
//	mid:    ┣━
//	footer: ┗━
func WithBordersHeavy() OptFn { return setBorders(heavyBorder) }

// WithBordersSolidSquare borders
//
//	header: ╭
//	row:    │
//	mid:    ├
//	footer: ╰
func WithBordersSolidSquare() OptFn { return setBorders(solidSquareBorder) }

// WithBordersHollowSquare borders
//
//	header: ╭
//	row:    │
//	mid:    ├
//	footer: ╰
func WithBordersHollowSquare() OptFn { return setBorders(hollowSquareBorder) }

// WithBordersSolidBullet borders
//
//	header: ╭
//	row:    │
//	mid:    ├
//	footer: ╰
func WithBordersSolidBullet() OptFn { return setBorders(solidBulletBorder) }

// WithBordersHollowBullet borders
//
//	header: ╭
//	row:    │
//	mid:    ├
//	footer: ╰
func WithBordersHollowBullet() OptFn { return setBorders(hollowBulletBorder) }

// WithBordersHollowDiamond borders
//
//	header: ╭
//	row:    │
//	mid:    ├
//	footer: ╰
func WithBordersHollowDiamond() OptFn { return setBorders(hollowDiamondBorder) }

// WithBordersSolidDiamond borders
//
//	header: ╭
//	row:    │
//	mid:    ├
//	footer: ╰
func WithBordersSolidDiamond() OptFn { return setBorders(solidDiamondBorder) }

// WithBordersPlusSign borders
//
//	header: +
//	row:    |
//	mid:    +
//	footer: +
func WithBordersPlusSign() OptFn { return setBorders(plusSignBorder) }

// WithBordersCustom sets a custom border.
func WithBordersCustom(header, row, mid, footer string) OptFn {
	return setBorders(NewBorders(header, row, mid, footer))
}

// WithNoBorders sets no border.
func WithNoBorders() OptFn { return setBorders(noBorder) }
