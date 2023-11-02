package color

import (
	"math/rand"
	"time"
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

func GetRandomColor() string {
	colors := []string{Red, Green, Yellow, Blue, Purple, Cyan, Gray, White}
	source := rand.NewSource(time.Now().UnixNano())
	random := rand.New(source)
	randomIndex := random.Intn(len(colors))
	return colors[randomIndex]
}
