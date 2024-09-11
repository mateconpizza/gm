package repo

// Database/backups related constants.
const (
	// DefaultDBName the default name of the SQLite database.
	DefaultDBName string = "bookmarks.db"

	// MaxBytesSize the maximum size in bytes of the SQLite database before
	// vacuum.
	MaxBytesSize int64 = 1000000

	// DatabaseMainTable the name of the main bookmarks table.
	DatabaseMainTable string = "bookmarks"

	// DatabaseDeletedTable the name of the deleted bookmarks table.
	DatabaseDeletedTable string = "deleted_bookmarks"

	// DatabaseDateFormat the string formatting of datetimes in the database.
	DatabaseDateFormat string = "2006-01-02 15:04:05"

	// DatabaseBackcupDateFormat the string formatting of datetimes for files
	// (backups).
	DatabaseBackcupDateFormat string = "2006-01-02_15-04"

	// DatabaseBackupMaxBackups the maximum number of backups allowed.
	DatabaseBackupMaxBackups int = 3
)
