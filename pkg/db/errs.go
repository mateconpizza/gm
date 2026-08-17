package db

import "errors"

var (
	// database errs.
	ErrDBExists             = errors.New("database exists")
	ErrDBAlreadyInitialized = errors.New("already initialized")
	ErrDBExistsAndInit      = errors.New("database exists and initialized")
	ErrDBEmpty              = errors.New("database is empty")
	ErrDBNotFound           = errors.New("database not found")
	ErrDBCorrupted          = errors.New("database corrupted")
	ErrDBEmptyPath          = errors.New("database path cannot be empty")
)

var (
	// records errs.
	ErrRecordDuplicate     = errors.New("record already exists")
	ErrRecordIDNotProvided = errors.New("no id provided")
	ErrCommit              = errors.New("commit error")
	ErrRecordNoMatch       = errors.New("no match found")
	ErrRecordNotFound      = errors.New("no record found")
	ErrRecordScan          = errors.New("scan record")
	ErrInvalidSortBy       = errors.New("invalid sort")
	ErrChecksumEmpty       = errors.New("checksum cannot be empty")
)

var (
	// backups errs.
	ErrBackupExists   = errors.New("backup already exists")
	ErrBackupNotFound = errors.New("no backup found")
)

// migration errs.
var (
	ErrMigrationInvalidFilename = errors.New("invalid migration filename")
	ErrMigrationDuplicate       = errors.New("duplicate migration")
	ErrMigrationGap             = errors.New("migration gap")
)
