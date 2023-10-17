package main

import (
	"fmt"
	"log"
	"strings"
)

var Menus = make(map[string]Menu)

type Menu struct {
	Command   string
	Arguments []string
}

func (m *Menu) UpdateMessage(message string) {
	m.replaceArg("-mesg", message)
}

func (m *Menu) UpdatePrompt(prompt string) {
	m.replaceArg("-p", prompt)
}

func (m *Menu) replaceArg(argName, newValue string) {
	for i := 0; i < len(m.Arguments); i++ {
		if m.Arguments[i] == argName {
			m.Arguments[i+1] = newValue
		}
	}
}

var rofiMenu = Menu{
	Command: "rofi",
	Arguments: []string{
		"-dmenu",
		"-l", "10",
		"-p", "GoMarks",
		"-mesg", "Welcome to GoMarks",
		"-theme-str", "window {width: 75%; height: 55%;}",
		"-theme-str", "textbox {markup: false;}",
	},
}

var dmenuMenu = Menu{
	Command: "dmenu",
	Arguments: []string{
		"-i",
		"-p", "GoMarks>",
		"-l", "10",
	},
}

func loadMenus() {
	registerMenu(dmenuMenu)
	registerMenu(rofiMenu)
}

func registerMenu(m Menu) {
	Menus[m.Command] = m
}

func getMenu(s string) (Menu, error) {
	menu, ok := Menus[s]
	if !ok {
		return Menu{}, fmt.Errorf("menu '%s' not found", s)
	}
	return menu, nil
}

func Prompt(m *Menu) (string, error) {
	input, err := executeCommand(m, "")
	if err != nil {
		log.Fatal(err)
	}
	return strings.Trim(input, "\n"), nil
}

func Confirm(m *Menu, msg string, pmt string) bool {
	if msg != "" {
		m.UpdateMessage(msg)
	}
	if pmt != "" {
		m.UpdatePrompt(pmt)
	}
	options := []fmt.Stringer{
		Option{"Yes"},
		Option{"No"},
	}
	idx, err := Select(m, options)
	if err != nil {
		log.Fatal(err)
	}
	return idx == 0
}

func Select(m *Menu, items []fmt.Stringer) (int, error) {
	var itemsText []string
	for _, item := range items {
		itemText := item.String()
		itemsText = append(itemsText, itemText)
	}

	itemsString := strings.Join(itemsText, "\n")
	output, err := executeCommand(m, itemsString)
	if err != nil {
		log.Fatal(err)
	}

	selectedStr := strings.TrimSpace(output)
	if !isSelectedTextInItems(selectedStr, itemsText) {
		log.Fatal("invalid selection:", selectedStr)
	}

  index := findSelectedIndex(selectedStr, itemsText)
  if index != -1 {
    return index, nil
  }
	return -1, fmt.Errorf("item not found")
}
