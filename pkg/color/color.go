package color

import (
	"math/rand"
	"time"
)

var Blue = "\033[34m"
var Cyan = "\033[36m"
var Gray = "\033[37;2m"
var Green = "\033[32m"
var Purple = "\033[35m"
var Red = "\033[31m"
var White = "\033[97m"
var Yellow = "\033[33m"
var Bold = "\033[1m"
var Reset = "\033[0m"

func GetRandomColor() string {
	colors := []string{Red, Green, Yellow, Blue, Purple, Cyan, Gray, White}
	source := rand.NewSource(time.Now().UnixNano())
	random := rand.New(source)
	randomIndex := random.Intn(len(colors))
	return colors[randomIndex]
}
