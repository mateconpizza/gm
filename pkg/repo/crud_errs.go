package repo

import "errors"

var (
	// database errs.
	ErrDBAlreadyExists      = errors.New("database already exists")
	ErrDBAlreadyInitialized = errors.New("database already initialized")
	ErrDBDefault            = errors.New("default database not found")
	ErrDBDrop               = errors.New("dropping database")
	ErrDBEmpty              = errors.New("database is empty")
	ErrDBNameSpecify        = errors.New("database name not specified")
	ErrDBNotFound           = errors.New("database not found")
	ErrDBNotInitialized     = errors.New("database not initialized")
	ErrDBResetSequence      = errors.New("resetting sqlite_sequence")
	ErrDBsNotFound          = errors.New("no database/s found")
	ErrSQLQuery             = errors.New("executing query")
)

var (
	// records errs.
	ErrRecordActionAborted    = errors.New("action aborted")
	ErrRecordDelete           = errors.New("error delete record")
	ErrRecordDuplicate        = errors.New("record already exists")
	ErrRecordIDInvalid        = errors.New("invalid id")
	ErrRecordIDNotProvided    = errors.New("no id provided")
	ErrRecordInsert           = errors.New("inserting record")
	ErrRecordNoMatch          = errors.New("no match found")
	ErrRecordNotExists        = errors.New("row not exists")
	ErrRecordNotFound         = errors.New("no record found")
	ErrRecordQueryNotProvided = errors.New("no id or query provided")
	ErrRecordScan             = errors.New("scan record")
	ErrRecordUpdate           = errors.New("update failed")
	ErrRecordRestoreTable     = errors.New("restoring from main table")
)

var (
	// backups errs.
	ErrBackupDisabled      = errors.New("backups are disabled")
	ErrBackupNotFound      = errors.New("no backup found")
	ErrBackupNoPurge       = errors.New("no backup to purge")
	ErrBackupAlreadyExists = errors.New("backup already exists")
	ErrBackupCreate        = errors.New("could not create backup")
	ErrBackupRemove        = errors.New("could not remove backup")
	ErrBackupRestore       = errors.New("could not restore backup")
	ErrBackupStatus        = errors.New("could not get backup status")
)
