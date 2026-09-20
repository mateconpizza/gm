package git

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
)

type LoggerFunc func(s string) string

type LogStyle struct {
	PreProcessor func(msg string) string
	Hash         LoggerFunc
	Repo         LoggerFunc
	Message      LoggerFunc
	Status       LoggerFunc
	Info         LoggerFunc
}

func defaultLoggerFunc(s string) string { return s }

var defaultStyler = LogStyle{
	Hash:    defaultLoggerFunc,
	Repo:    defaultLoggerFunc,
	Message: defaultLoggerFunc,
	Status:  defaultLoggerFunc,
	Info:    defaultLoggerFunc,
}

// LogEntry holds the parsed components of a single log line.
//
//	hash [repo] message (status)
type LogEntry struct {
	Hash   string // e.g. "e7e92be"
	Repo   string // e.g. "[main]"
	Mesg   string // e.g. "http status updated"
	Status string // e.g. "(~mod:1)"

	styler *LogStyle
}

func NewLogEntry() *LogEntry {
	return &LogEntry{styler: &defaultStyler}
}

func (e *LogEntry) WithStyler(c *LogStyle) *LogEntry {
	e.styler = c
	return e
}

// Colored returns the LogEntry formatted with ANSI colors.
func (e *LogEntry) Colored() string {
	var sb strings.Builder

	f := func(s string) {
		sb.WriteString(s)
		sb.WriteByte(' ')
	}

	//	hash [repo] message (status)
	f(e.styler.Hash(e.Hash))
	f(e.styler.Repo(e.Repo))
	f(e.styler.Message(e.Mesg))
	if e.Status != "" {
		f(e.styler.Status(e.Status))
	}

	return sb.String()
}

// parseLogLine parses a single line into a LogEntry.
func parseLogLine(e *LogEntry, l string) bool {
	// hash [repo] message (status)
	l = strings.TrimSpace(l)
	if l == "" {
		return false
	}

	// extract hash
	hashSpace := strings.IndexByte(l, ' ')
	if hashSpace == -1 {
		return false
	}
	e.Hash = l[:hashSpace]
	l = strings.TrimSpace(l[hashSpace+1:])

	// extract repo
	if strings.HasPrefix(l, "[") {
		repoEnd := strings.IndexByte(l, ']')
		if repoEnd != -1 {
			e.Repo = l[1:repoEnd]
			l = strings.TrimSpace(l[repoEnd+1:])
		}
	}

	// extract message and optional status
	statusStart := strings.LastIndexByte(l, '(')
	if statusStart != -1 && strings.HasSuffix(l, ")") {
		e.Mesg = strings.TrimSpace(l[:statusStart])
		e.Status = l[statusStart:]
	} else {
		e.Mesg = l
	}

	return true
}

// StreamLogs reads from an io.Reader line-by-line, parsing and printing the
// colored output immediately.
func StreamLogs(ctx context.Context, r io.Reader, w io.Writer, entry *LogEntry) error {
	scanner := bufio.NewScanner(r)

	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return err
		}

		line := scanner.Text()

		if ok := parseLogLine(entry, line); ok {
			if entry.styler.PreProcessor != nil {
				entry.Mesg = entry.styler.PreProcessor(entry.Mesg)
			}
			fmt.Fprintln(w, entry.Colored())
		}
	}

	return scanner.Err()
}

func CommitMsg(verb, object string, s Stats) string {
	return fmt.Sprintf("%s %s (%s)", verb, object, s)
}
