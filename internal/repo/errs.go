package repo

import "errors"

var (
	// database errs.
	ErrDBAlreadyExists      = errors.New("database already exists")
	ErrDBAlreadyInitialized = errors.New("already initialized")
	ErrDBExistsAndInit      = errors.New("database already exists and initialized")
	ErrDBExistsNotInit      = errors.New("database already exists and not initialized")
	ErrDBEmpty              = errors.New("database is empty")
	ErrDBNotFound           = errors.New("database not found")
	ErrDBNotInitialized     = errors.New("database not initialized")
	ErrDBsNotFound          = errors.New("no database/s found")
	ErrDBCorrupted          = errors.New("database corrupted")
	ErrDBLockedNotFound     = errors.New("encrypted database not found")
	ErrDBLocked             = errors.New("database is locked")
	ErrDBUnlockFirst        = errors.New("unlock database first")
	ErrDBUnlocked           = errors.New("database not encrypted")
	ErrDBNameRequired       = errors.New("name required")
	ErrDBMainNameReserved   = errors.New("name reserved")
	ErrDBMainNotFound       = errors.New("main database not found")
)

var (
	// records errs.
	ErrRecordActionAborted    = errors.New("action aborted")
	ErrRecordDuplicate        = errors.New("record already exists")
	ErrRecordIDNotProvided    = errors.New("no id provided")
	ErrRecordInsert           = errors.New("inserting record")
	ErrCommit                 = errors.New("commit error")
	ErrRecordNoMatch          = errors.New("no match found")
	ErrRecordNotFound         = errors.New("no record found")
	ErrRecordQueryNotProvided = errors.New("no id or query provided")
	ErrRecordScan             = errors.New("scan record")
)

var (
	// backups errs.
	ErrBackupExists     = errors.New("backup already exists")
	ErrBackupDisabled   = errors.New("backups are disabled")
	ErrBackupNoPurge    = errors.New("no backup to purge")
	ErrBackupNotFound   = errors.New("no backup found")
	ErrBackupPathNotSet = errors.New("backup path not set")
)
