package db

import "errors"

var (
	// database errs.
	ErrDBExists             = errors.New("database exists")
	ErrDBAlreadyInitialized = errors.New("already initialized")
	ErrDBExistsAndInit      = errors.New("database exists and initialized")
	ErrDBEmpty              = errors.New("database is empty")
	ErrDBNotFound           = errors.New("database not found")
	ErrDBsNotFound          = errors.New("no database/s found")
	ErrDBCorrupted          = errors.New("database corrupted")
	ErrDBLocked             = errors.New("database is locked")
	ErrDBUnlockFirst        = errors.New("unlock database first")
	ErrDBMainNameReserved   = errors.New("name reserved")
	ErrDBMainNotFound       = errors.New("main database not found")
)

var (
	// records errs.
	ErrRecordDuplicate        = errors.New("record already exists")
	ErrRecordIDNotProvided    = errors.New("no id provided")
	ErrCommit                 = errors.New("commit error")
	ErrRecordNoMatch          = errors.New("no match found")
	ErrRecordNotFound         = errors.New("no record found")
	ErrRecordQueryNotProvided = errors.New("no id or query provided")
	ErrRecordScan             = errors.New("scan record")
)

var (
	// backups errs.
	ErrBackupExists   = errors.New("backup already exists")
	ErrBackupNotFound = errors.New("no backup found")
)
