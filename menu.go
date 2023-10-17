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
		"rofi", "-dmenu", "-p", "GoMarks>", "-l", "10", "-mesg", " Welcome to GoMarks",
		"-theme-str", "window {width: 75%; height: 55%;}"})
}

func registerMenu(s string, command []string) {
	Menus[s] = command
}

func getMenu(s string) ([]string, error) {
	menu, ok := Menus[s]
	if !ok {
		return nil, fmt.Errorf("menu '%s' not found", s)
	}
	return menu, nil
}

func Prompt(menuArgs []string) (string, error) {
	input, err := executeCommand(menuArgs, "")
	if err != nil {
		log.Fatal(err)
	}
	return strings.Trim(input, "\n"), nil
}

func Confirm(menuArgs []string, message string) bool {
	options := []fmt.Stringer{
		Option{"Yes"},
		Option{"No"},
	}
	idx, err := Select(menuArgs, options)
	if err != nil {
		log.Fatal(err)
	}
	return idx == 0
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
