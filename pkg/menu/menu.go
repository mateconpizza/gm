package menu

import (
	"fmt"
	u "gomarks/pkg/util"
	"io"
	"log"
	"os/exec"
	"strings"
)

func GetMenu(s string) Menu {
	mc := make(MenuCollection)
	mc.Load()
	menu, err := mc.Get(s)
	if err != nil {
		log.Fatalf("Error: %v\n", err)
	}
	return menu
}

type Option struct {
	Label string
}

func (o Option) String() string {
	return o.Label
}

type MenuCollection map[string]Menu

func (mc MenuCollection) Register(m Menu) {
	log.Println("Registering menu:", m.Command)
	mc[m.Command] = m
}

func (mc MenuCollection) Get(s string) (Menu, error) {
	menu, ok := mc[s]
	if !ok {
		return Menu{}, fmt.Errorf("menu '%s' not found", s)
	}
	log.Println("Got menu:", menu.Command)
	return menu, nil
}

func (mc MenuCollection) Load() {
	mc.Register(rofiMenu)
	mc.Register(dmenuMenu)
}

type Menu struct {
	Command   string
	Arguments []string
}

func (m *Menu) UpdateMessage(message string) {
	u.ReplaceArg(m.Arguments, "-mesg", message)
}

func (m *Menu) UpdatePrompt(prompt string) {
	u.ReplaceArg(m.Arguments, "-p", prompt)
}

func (m *Menu) Confirm(msg, prompt string) bool {
	m.UpdateMessage(msg)
	m.UpdatePrompt(prompt)
	options := []fmt.Stringer{
		Option{"No"},
		Option{"Yes"},
	}
	idx, err := m.Select(options)
	if err != nil {
		log.Fatal(err)
	}
	return idx == 0
}

func (m *Menu) Prompt(msg, prompt string) (string, error) {
	m.UpdateMessage(msg)
	m.UpdatePrompt(prompt)
	input, err := m.Run("")
	return strings.TrimRight(input, "\n"), err
}

func (m *Menu) Select(items []fmt.Stringer) (int, error) {
	var itemsText []string
	for _, item := range items {
		itemsText = append(itemsText, item.String())
	}

	itemsString := strings.Join(itemsText, "\n")
	output, err := m.Run(itemsString)
	if err != nil {
		log.Fatal(err)
	}
	selectedStr := strings.TrimSpace(output)

	if !u.IsSelectedTextInItems(selectedStr, itemsText) {
		return -1, fmt.Errorf("invalid selection: %s", selectedStr)
	}
	return u.FindSelectedIndex(selectedStr, itemsText), nil
}

func (m *Menu) Run(s string) (string, error) {
	log.Println("Running menu:", m.Command, m.Arguments)
	cmd := exec.Command(m.Command, m.Arguments...)

	if s != "" {
		cmd.Stdin = strings.NewReader(s)
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("error creating output pipe: %s", err)
	}

	err = cmd.Start()
	if err != nil {
		return "", fmt.Errorf("error starting dmenu: %s", err)
	}

	output, err := io.ReadAll(stdoutPipe)
	if err != nil {
		return "", fmt.Errorf("error reading output: %s", err)
	}

	err = cmd.Wait()
	if err != nil {
		return "", fmt.Errorf("user hit scape: %s", err)
	}
	outputStr := string(output)
	outputStr = strings.TrimRight(outputStr, "\n")
	log.Println("Output:", outputStr)
	return outputStr, nil
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
