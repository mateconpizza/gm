package config

import (
	"log/slog"
	"os"
	"path/filepath"
)

type Flags struct {
	// Actions
	Copy   bool // Copy URL into clipboard
	Edit   bool // Edit mode
	Menu   bool // Menu mode
	Notes  bool // Record notes
	Open   bool // Open URL in default browser
	QR     bool // QR code generator
	Remove bool // Remove bookmarks
	List   bool // List items
	Create bool // Action create

	// Output format
	Field     string // Field to print
	JSON      bool   // JSON output
	Oneline   bool   // Oneline output
	Multiline bool   // Multiline output
	Format    string

	// Filtering and pagination
	Head int      // Head limit
	Tags []string // Tags list to filter bookmarks
	Tail int      // Tail limit

	// Bookmark operations
	Export   bool   // Exports the bookmarks into a Netscape HTML file
	Snapshot bool   // Fetches lastets snapshot from Wayback Machine
	Limit    int    // Limit to N
	Year     int    // Year
	Update   bool   // Update bookmarks
	Status   bool   // Status checks URLs status code
	Title    string // Bookmark's title
	TagsStr  string // Bookmark's tags (tag1,tag2,...)

	// Configuration and behavior
	Color    bool   // Application color enable
	ColorStr string // WithColor enable color output
	Force    bool   // Force action without confirmation
	Yes      bool   // Assume "yes" on most questions
	Path     string // Custom database path
	Verbose  int    // Verbose output level

	// Subcmds
	Database
	GitFlags
}

// Database operations.
type Database struct {
	Info    bool // Database info
	List    bool // List database items
	Lock    bool // Lock a database
	Reorder bool // Reorder table IDs
	Unlock  bool // Unlock a database
	Vacuum  bool // Rebuild the database file
}

// GitFlags tracking operations.
type GitFlags struct {
	Management bool // Git repository management
	Track      bool // Track database in git
	Untrack    bool // Untrack database in git
	Redo       bool // Redo
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
