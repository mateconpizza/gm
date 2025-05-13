package repo

import "errors"

var (
	// database errs.
	ErrDBAlreadyExists      = errors.New("database already exists")
	ErrDBAlreadyInitialized = errors.New("already initialized")
	ErrDBEmpty              = errors.New("database is empty")
	ErrDBNotFound           = errors.New("database not found")
	ErrDBNotInitialized     = errors.New("database not initialized")
	ErrDBsNotFound          = errors.New("no database/s found")
	ErrDBCorrupted          = errors.New("database corrupted")
	ErrDBEncryptedNotFound  = errors.New("encrypted database not found")
	ErrDBEncryted           = errors.New("database already locked")
	ErrDBDecryptFirst       = errors.New("unlock database first")
	ErrDBNotEncrypted       = errors.New("database not encrypted")
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
	ErrBackupAlreadyExists = errors.New("backup already exists")
	ErrBackupDisabled      = errors.New("backups are disabled")
	ErrBackupNoPurge       = errors.New("no backup to purge")
	ErrBackupNotFound      = errors.New("no backup found")
	ErrBackupPathNotSet    = errors.New("backup path not set")
)
