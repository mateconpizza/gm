package application

import (
	"time"
)

type Flags struct {
	// Actions
	Menu      bool   // Menu mode
	List      bool   // List items
	Print     bool   // Print something
	All       bool   // Include all items
	Rename    string // Rename action
	Configure bool   // Configure action

	// Output format
	Output  string // Output
	Field   string // Bookmarks fields
	JSON    bool   // JSON output
	Preview string // Menu preview
	Sort    string // Sort by
	Reverse bool   // Reverse output
	Filter  string // Filter by
	Format  string // Format type

	// Filtering and pagination
	Head int      // Head limit
	Tags []string // Tags list to filter bookmarks
	Tail int      // Tail limit

	// Bookmark operations
	Limit   int           // Limit to N
	Year    int           // Year
	Update  bool          // Update bookmarks
	Title   string        // Bookmark's title
	TagsStr string        // Bookmark's tags (tag1,tag2,...)
	Timeout time.Duration // Timeout ops

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

	// Export markdown
	Type    string
	Summary string
	Glyphs  string
}

func ColorEnabled(mode string, stdinPiped, stdoutPiped, stderrPiped, noColor func() bool) bool {
	switch mode {
	case "always":
		return true
	case "never":
		return false
	default: // auto
		return !stdinPiped() &&
			!stdoutPiped() &&
			!stderrPiped() &&
			!noColor()
	}
}
