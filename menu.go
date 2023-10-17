package main

import (
	"fmt"
	"log"
	"strings"
)

var Menus = make(map[string][]string)

func loadMenus() {
	registerMenu("dmenu", []string{"dmenu", "-i", "-p", "GoMarks>", "-l", "10"})
	registerMenu("rofi", []string{
		"rofi", "-dmenu", "-p", "GoMarks>", "-l", "10", "-mesg",
		" Welcome to GoMarks", "-theme-str",
		"window {width: 75%; height: 55%;}", "-kb-custom-1", "Alt-o"})
}

func Prompt(menuArgs []string) (string, error) {
	input, err := executeCommand(menuArgs, "")
	if err != nil {
		log.Fatal(err)
	}
	return strings.TrimSpace(input), nil
}

func Confirm(menuArgs []string) (bool, error) {
	return true, nil
}

func Select(menuArgs []string, items []fmt.Stringer) (int, error) {
	var itemsText []string
	for _, item := range items {
		itemText := item.String()
		itemsText = append(itemsText, itemText)
	}

	itemsString := strings.Join(itemsText, "\n")
	output, err := executeCommand(menuArgs, itemsString)
	if err != nil {
		log.Fatal(err)
	}

	selectedStr := strings.TrimSpace(output)
	if !isSelectedTextInItems(selectedStr, itemsText) {
		log.Fatal("invalid selection:", selectedStr)
	}

	for index, itemText := range itemsText {
		if strings.Contains(selectedStr, itemText) {
			return index, nil
		}
	}
	return -1, fmt.Errorf("item not found")
}

func SelectBookmark(menuArgs []string, bookmarks *[]Bookmark) (Bookmark, error) {
	var itemsText []string
	for _, bm := range *bookmarks {
		itemText := fmt.Sprintf(
			"%-4d %-80s %-10s",
			bm.ID,
			shortenString(bm.URL, 80),
			bm.Tags,
		)
		itemsText = append(itemsText, itemText)
	}

	itemsString := strings.Join(itemsText, "\n")
	output, err := executeCommand(menuArgs, itemsString)
	if err != nil {
		log.Fatal(err)
	}

	selectedStr := strings.TrimSpace(output)
	index := findSelectedIndex(selectedStr, itemsText)
	if index != -1 {
		return (*bookmarks)[index], nil
	}
  return Bookmark{}, fmt.Errorf("item not found: %s", selectedStr)
}
