package application

import (
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

type Flags struct {
	// Actions
	Menu  bool // Menu mode
	List  bool // List items
	Print bool // Print something
	All   bool // Include all items

	// Output format
	Output  string // Output
	Field   string // Bookmarks fields
	JSON    bool   // JSON output
	Preview string // Menu preview
	Sort    string // Sort by

	// Filtering and pagination
	Head int      // Head limit
	Tags []string // Tags list to filter bookmarks
	Tail int      // Tail limit

	// Bookmark operations
	Limit    int           // Limit to N
	Year     int           // Year
	Update   bool          // Update bookmarks
	Title    string        // Bookmark's title
	TagsStr  string        // Bookmark's tags (tag1,tag2,...)
	Duration time.Duration // Timeout ops

	// Configuration and behavior
	Color    bool   // Application color enable
	ColorStr string // WithColor enable color output
	Force    bool   // Force action without confirmation
	Yes      bool   // Assume "yes" on most questions
	Path     string // Custom database path
	Verbose  int    // Verbose output level
	Version  bool   // App version

	// git
	Reinit bool // Reinitialize existing repository
}

func SetVerbosity(verbose int) {
	levels := []slog.Level{
		slog.LevelError,
		slog.LevelWarn,
		slog.LevelInfo,
		slog.LevelDebug,
	}
	level := levels[min(verbose, len(levels)-1)]

	logger := slog.New(
		slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			AddSource: true,
			Level:     level,
			ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
				if a.Key == "source" {
					if source, ok := a.Value.Any().(*slog.Source); ok {
						dir, file := filepath.Split(source.File)
						source.File = filepath.Join(filepath.Base(filepath.Clean(dir)), file)

						return slog.Attr{Key: "source", Value: slog.AnyValue(source)}
					}
				}

				return a
			},
		}),
	)
	slog.SetDefault(logger)

	slog.Debug("logging", "level", level)
}
