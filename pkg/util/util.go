package util

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/haaag/gm/pkg/format"
)

// FilterEntries returns a list of backups
func FilterEntries(name, path string) ([]fs.DirEntry, error) {
	var filtered []fs.DirEntry
	files, err := os.ReadDir(path)

	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	for _, entry := range files {
		if entry.IsDir() {
			continue
		}
		if strings.Contains(entry.Name(), name) {
			filtered = append(filtered, entry)
		}
	}
	return filtered, nil
}

// GetEnv retrieves an environment variable
func GetEnv(key, def string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}

	return def
}

// binPath returns the path of the binary
func BinPath(binaryName string) string {
	cmd := exec.Command("which", binaryName)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	c := strings.TrimRight(string(out), "\n")
	log.Printf("which %s = %s", binaryName, c)
	return c
}

// binExists checks if the binary exists in $PATH
func BinExists(binaryName string) bool {
	cmd := exec.Command("which", binaryName)
	err := cmd.Run()
	return err == nil
}

func Spinner(done chan bool, mesg string) {
	spinner := []string{" ", "▁", "▂", "▃", "▄", "▅", "▆", "▇"}
	for i := 0; ; i++ {
		select {
		case <-done:
			fmt.Printf("\r%-*s\r", len(mesg)+2, " ")
			return
		default:
			fmt.Printf("\r%s %s", spinner[i%len(spinner)], mesg)
			time.Sleep(110 * time.Millisecond)
		}
	}
}

// ParseUniqueStrings returns a slice of unique strings
func ParseUniqueStrings(input *[]string, sep string) *[]string {
	uniqueItems := make([]string, 0)
	uniqueMap := make(map[string]struct{})

	/* for _, item := range *input {
		i := strings.TrimSpace(item)
		if i != "" {
			uniqueMap[i] = struct{}{}
		}
	} */

	for _, tags := range *input {
		tagList := strings.Split(tags, sep)
		for _, tag := range tagList {
			tag = strings.TrimSpace(tag)
			if tag != "" {
				uniqueMap[tag] = struct{}{}
			}
		}
	}

	for tag := range uniqueMap {
		uniqueItems = append(uniqueItems, tag)
	}

	return &uniqueItems
}

func HeavyBox() error {
	var (
		sb     strings.Builder
		up     = format.Color("┏━").Dim().String()
		mid    = format.Color("┃ ").Dim().String()
		midd   = format.Color("┣━").Dim().String()
		bottom = format.Color("┗━").Dim().String()
	)

	sb.WriteString(up + " New bookmark\n")
	sb.WriteString(mid + "\n")
	sb.WriteString(mid + "\n")
	sb.WriteString(mid + "\n")
	sb.WriteString(midd + "\n")
	sb.WriteString(bottom)
	fmt.Println(sb.String())
	return nil
}

func LightBox() error {
	var (
		sb     strings.Builder
		up     = format.Color("┌─").Dim().String()
		mid    = format.Color("│ ").Dim().String()
		midd   = format.Color("├─").Dim().String()
		bottom = format.Color("└─").Dim().String()
	)

	sb.WriteString(up + format.Color(" New bookmark\n").Yellow().Bold().String())
	sb.WriteString(mid + "\n")
	sb.WriteString(mid + "\n")
	sb.WriteString(mid + "\n")
	sb.WriteString(midd + "\n")
	sb.WriteString(mid + "\n")
	sb.WriteString(bottom)
	fmt.Println(sb.String())
	return nil
}

func ColorBox() error {
	var sb strings.Builder

	sangriaMid := format.Color("├─ ").Gray().String()
	sangriaBottom := format.Color("└─ ").Gray().String()
	mid := format.Color("│").Gray().String()
	midNewLine := format.Color("│\n").Gray().String()

	sb.WriteString(format.Color("┌─").Gray().String())
	sb.WriteString(format.Color("").Yellow().Bold().String())
	sb.WriteString(format.Color(" New bookmark\n").Yellow().Bold().String())
	sb.WriteString(midNewLine)
	sb.WriteString(sangriaMid)
	sb.WriteString(format.Color("URL:\n").Blue().String())
	sb.WriteString(mid)
	sb.WriteString(format.Color("    https://pkg.go.dev/\n").Gray().String())
	sb.WriteString(sangriaMid)
	sb.WriteString(format.Color("Tags:\n").Purple().String())
	sb.WriteString(mid)
	sb.WriteString(format.Color("    golang,packages,standard\n").Gray().String())
	sb.WriteString(sangriaMid)
	sb.WriteString(format.Color("Title:\n").Cyan().String())
	sb.WriteString(mid)
	sb.WriteString(format.Color("    Go Packages - Go Packages\n").Gray().String())
	sb.WriteString(sangriaMid)
	sb.WriteString(format.Color("Description:\n").Yellow().String())
	sb.WriteString(mid)
	sb.WriteString(format.Color("    Go is an open source programming language that makes it easy to build simple,\n").Gray().String())
	sb.WriteString(mid)
	sb.WriteString(format.Color("    reliable, and efficient software.\n").Gray().String())
	sb.WriteString(midNewLine)
	sb.WriteString(sangriaBottom)
	sb.WriteString(format.Color("Save?").Green().Bold().String())
	sb.WriteString(format.Color(" [no/edit/Yes]: ").Gray().String())
	fmt.Println(sb.String())
	_ = `
┌─ New bookmark
│
├─ URL:
│    https://pkg.go.dev/
├─ Tags:
│    golang,packages,standar
├─ Title:
│    Go Packages - Go Packages
├─ Description:
│    Go is an open source programming language that makes it easy to build simple,
│    reliable, and efficient software.
│
└─ Save? [no/edit/Yes]: y
`

	return nil
}
