package color

import (
	"fmt"
)

var (
	Blue   = "\033[34m"
	Cyan   = "\033[36m"
	Gray   = "\033[37;2m"
	Green  = "\033[32m"
	Purple = "\033[35m"
	Red    = "\033[31m"
	White  = "\033[97m"
	Yellow = "\033[33m"
	Bold   = "\033[1m"
	Reset  = "\033[0m"
)

func Colorize(s, color string) string {
	return fmt.Sprintf("%s%s%s", color, s, Reset)
}

func ColorizeBold(s, color string) string {
	return fmt.Sprintf("%s%s%s%s", Bold, color, s, Reset)
}
