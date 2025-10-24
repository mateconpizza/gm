package color

import (
	"fmt"
	"strings"
)

type Palette struct{}

// Normal Colors.

func (p *Palette) Black(s ...any) string         { return Black(s...).String() }
func (p *Palette) BlackItalic(s ...any) string   { return Black(s...).Italic().String() }
func (p *Palette) BlackBold(s ...any) string     { return Black(s...).Bold().String() }
func (p *Palette) Blue(s ...any) string          { return Blue(s...).String() }
func (p *Palette) BlueItalic(s ...any) string    { return Blue(s...).Italic().String() }
func (p *Palette) BlueBold(s ...any) string      { return Blue(s...).Bold().String() }
func (p *Palette) Cyan(s ...any) string          { return Cyan(s...).String() }
func (p *Palette) CyanItalic(s ...any) string    { return Cyan(s...).Italic().String() }
func (p *Palette) CyanBold(s ...any) string      { return Cyan(s...).Bold().String() }
func (p *Palette) Gray(s ...any) string          { return Gray(s...).String() }
func (p *Palette) GrayItalic(s ...any) string    { return Gray(s...).Italic().String() }
func (p *Palette) GrayBold(s ...any) string      { return Gray(s...).Bold().String() }
func (p *Palette) Green(s ...any) string         { return Green(s...).String() }
func (p *Palette) GreenItalic(s ...any) string   { return Green(s...).Italic().String() }
func (p *Palette) GreenBold(s ...any) string     { return Green(s...).Bold().String() }
func (p *Palette) Magenta(s ...any) string       { return Magenta(s...).String() }
func (p *Palette) MagentaItalic(s ...any) string { return Magenta(s...).Italic().String() }
func (p *Palette) MagentaBold(s ...any) string   { return Magenta(s...).Bold().String() }
func (p *Palette) Orange(s ...any) string        { return Orange(s...).String() }
func (p *Palette) OrangeItalic(s ...any) string  { return Orange(s...).Italic().String() }
func (p *Palette) OrangeBold(s ...any) string    { return Orange(s...).Bold().String() }
func (p *Palette) Purple(s ...any) string        { return Purple(s...).String() }
func (p *Palette) PurpleItalic(s ...any) string  { return Purple(s...).Italic().String() }
func (p *Palette) PurpleBold(s ...any) string    { return Purple(s...).Bold().String() }
func (p *Palette) Red(s ...any) string           { return Red(s...).String() }
func (p *Palette) RedItalic(s ...any) string     { return Red(s...).Italic().String() }
func (p *Palette) RedBold(s ...any) string       { return Red(s...).Bold().String() }
func (p *Palette) White(s ...any) string         { return White(s...).String() }
func (p *Palette) WhiteItalic(s ...any) string   { return White(s...).Italic().String() }
func (p *Palette) WhiteBold(s ...any) string     { return White(s...).Bold().String() }
func (p *Palette) Yellow(s ...any) string        { return Yellow(s...).String() }
func (p *Palette) YellowItalic(s ...any) string  { return Yellow(s...).Italic().String() }
func (p *Palette) YellowBold(s ...any) string    { return Yellow(s...).Bold().String() }

// Bright colors.

func (p *Palette) BrightBlack(s ...any) string         { return BrightBlack(s...).String() }
func (p *Palette) BrightBlackItalic(s ...any) string   { return BrightBlack(s...).Italic().String() }
func (p *Palette) BrightBlackBold(s ...any) string     { return BrightBlack(s...).Bold().String() }
func (p *Palette) BrightBlue(s ...any) string          { return BrightBlue(s...).String() }
func (p *Palette) BrightBlueItalic(s ...any) string    { return BrightBlue(s...).Italic().String() }
func (p *Palette) BrightBlueBold(s ...any) string      { return BrightBlue(s...).Bold().String() }
func (p *Palette) BrightCyan(s ...any) string          { return BrightCyan(s...).String() }
func (p *Palette) BrightCyanItalic(s ...any) string    { return BrightCyan(s...).Italic().String() }
func (p *Palette) BrightCyanBold(s ...any) string      { return BrightCyan(s...).Bold().String() }
func (p *Palette) BrightGray(s ...any) string          { return BrightGray(s...).String() }
func (p *Palette) BrightGrayItalic(s ...any) string    { return BrightGray(s...).Italic().String() }
func (p *Palette) BrightGrayBold(s ...any) string      { return BrightGray(s...).Bold().String() }
func (p *Palette) BrightGreen(s ...any) string         { return BrightGreen(s...).String() }
func (p *Palette) BrightGreenItalic(s ...any) string   { return BrightGreen(s...).Italic().String() }
func (p *Palette) BrightGreenBold(s ...any) string     { return BrightGreen(s...).Bold().String() }
func (p *Palette) BrightMagenta(s ...any) string       { return BrightMagenta(s...).String() }
func (p *Palette) BrightMagentaItalic(s ...any) string { return BrightMagenta(s...).Italic().String() }
func (p *Palette) BrightMagentaBold(s ...any) string   { return BrightMagenta(s...).Bold().String() }
func (p *Palette) BrightOrange(s ...any) string        { return BrightOrange(s...).String() }
func (p *Palette) BrightOrangeItalic(s ...any) string  { return BrightOrange(s...).Italic().String() }
func (p *Palette) BrightOrangeBold(s ...any) string    { return BrightOrange(s...).Bold().String() }
func (p *Palette) BrightPurple(s ...any) string        { return BrightPurple(s...).String() }
func (p *Palette) BrightPurpleItalic(s ...any) string  { return BrightPurple(s...).Italic().String() }
func (p *Palette) BrightPurpleBold(s ...any) string    { return BrightPurple(s...).Bold().String() }
func (p *Palette) BrightRed(s ...any) string           { return BrightRed(s...).String() }
func (p *Palette) BrightRedItalic(s ...any) string     { return BrightRed(s...).Italic().String() }
func (p *Palette) BrightRedBold(s ...any) string       { return BrightRed(s...).Bold().String() }
func (p *Palette) BrightWhite(s ...any) string         { return BrightWhite(s...).String() }
func (p *Palette) BrightWhiteItalic(s ...any) string   { return BrightWhite(s...).Italic().String() }
func (p *Palette) BrightWhiteBold(s ...any) string     { return BrightWhite(s...).Bold().String() }
func (p *Palette) BrightYellow(s ...any) string        { return BrightYellow(s...).String() }
func (p *Palette) BrightYellowItalic(s ...any) string  { return BrightYellow(s...).Italic().String() }
func (p *Palette) BrightYellowBold(s ...any) string    { return BrightYellow(s...).Bold().String() }

// Styles

func (p *Palette) Bold(s ...any) string    { return Text(anyToString(s...)).Bold().String() }
func (p *Palette) Dim(s ...any) string     { return Text(anyToString(s...)).Dim().String() }
func (p *Palette) Inverse(s ...any) string { return Text(anyToString(s...)).Inverse().String() }
func (p *Palette) Italic(s ...any) string  { return Text(anyToString(s...)).Italic().String() }
func (p *Palette) Strikethrough(s ...any) string {
	return Text(anyToString(s...)).Strikethrough().String()
}
func (p *Palette) Underline(s ...any) string { return Text(anyToString(s...)).Underline().String() }
func (p *Palette) Undercurl(s ...any) string { return Text(anyToString(s...)).Undercurl().String() }

// Helper

func anyToString(s ...any) string {
	var b strings.Builder
	for i, v := range s {
		if i > 0 {
			b.WriteByte(' ')
		}
		fmt.Fprint(&b, v)
	}
	return b.String()
}

func (p *Palette) Enabled() bool { return IsEnabled }

func NewPalette() *Palette {
	return &Palette{}
}
