package config

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

	// Output format
	Field     string // Field to print
	JSON      bool   // JSON output
	Oneline   bool   // Oneline output
	Multiline bool   // Multiline output

	// Filtering and pagination
	Head int      // Head limit
	Tags []string // Tags list to filter bookmarks
	Tail int      // Tail limit

	// Bookmark operations
	Export   bool // Exports the bookmarks into a Netscape HTML file
	Snapshot bool // Fetches snapshot from Wayback Machine
	Update   bool // Update bookmarks
	Status   bool // Status checks URLs status code

	// Configuration and behavior
	Color    bool   // Application color enable
	ColorStr string // WithColor enable color output
	Force    bool   // Force action without confirmation
	Path     string // Custom database path
	Redo     bool   // Redo last action
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
	Status     bool // Show tracked databases status
	Track      bool // Track database in git
	Untrack    bool // Untrack database in git
}

func NewFlags() *Flags {
	return &Flags{}
}
