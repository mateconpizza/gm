package menu

import (
	"fmt"
	u "gomarks/pkg/utils"
	"io"
	"log"
	"os/exec"
	"strings"
)

type Option struct {
	Label string
}

func (o Option) String() string {
	return o.Label
}

type MenuCollection map[string]Menu

var Menus = make(MenuCollection)

func (mc MenuCollection) Register(m Menu) {
	mc[m.Command] = m
}

func (mc MenuCollection) Get(s string) (Menu, error) {
	menu, ok := mc[s]
	if !ok {
		return Menu{}, fmt.Errorf("menu '%s' not found", s)
	}
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

func (m *Menu) Confirm(msg, prompt string) bool {
	m.UpdateMessage(msg)
	m.UpdatePrompt(prompt)
	options := []fmt.Stringer{
		Option{"Yes"},
		Option{"No"},
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
	if err != nil {
		log.Fatal(err)
	}
	return strings.TrimRight(input, "\n"), nil
}

func (m *Menu) Select(items []fmt.Stringer) (int, error) {
	var itemsText []string
	for _, item := range items {
		itemText := item.String()
		itemsText = append(itemsText, itemText)
	}

	itemsString := strings.Join(itemsText, "\n")
	output, err := m.Run(itemsString)
	if err != nil {
		log.Fatal(err)
	}

	selectedStr := strings.TrimSpace(output)
	if !u.IsSelectedTextInItems(selectedStr, itemsText) {
		log.Fatal("invalid selection:", selectedStr)
	}

	index := u.FindSelectedIndex(selectedStr, itemsText)
	if index != -1 {
		return index, nil
	}
	return -1, fmt.Errorf("item not found")
}

func (m *Menu) Run(s string) (string, error) {
	cmd := exec.Command(m.Command, m.Arguments...)

	if s != "" {
		cmd.Stdin = strings.NewReader(s)
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatal("Error creating output pipe:", err)
	}

	err = cmd.Start()
	if err != nil {
		log.Fatal("Error starting dmenu:", err)
	}

	output, err := io.ReadAll(stdoutPipe)
	if err != nil {
		log.Fatal("Error reading output:", err)
	}

	err = cmd.Wait()
	if err != nil {
		return "", fmt.Errorf("program exited with non-zero status: %s", err)
	}
  outputStr := string(output)
	return strings.TrimRight(outputStr, "\n"), nil
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
