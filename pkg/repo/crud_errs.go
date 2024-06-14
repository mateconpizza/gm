package repo

import "errors"

var (
	// database
	ErrDBAlreadyExists      = errors.New("database already exists")
	ErrDBAlreadyInitialized = errors.New("database already initialized")
	ErrDBNotInitialized     = errors.New("database not initialized")
	ErrDBNotFound           = errors.New("database not found")
	ErrDBDefault            = errors.New("default database not found")
	ErrDBsNotFound          = errors.New("no database/s found")
	ErrDBResetSequence      = errors.New("resetting sqlite_sequence")
	ErrDBNameSpecify        = errors.New("database name not specified")
	ErrDBDrop               = errors.New("dropping database")
	ErrSQLQuery             = errors.New("executing query")
	ErrDBEmpty              = errors.New("database is empty")
)

var (
	// records
	ErrRecordDelete           = errors.New("error delete record")
	ErrRecordDuplicate        = errors.New("record already exists")
	ErrRecordInsert           = errors.New("inserting record")
	ErrRecordNotExists        = errors.New("row not exists")
	ErrRecordScan             = errors.New("scan record")
	ErrRecordUpdate           = errors.New("update failed")
	ErrRecordNotFound         = errors.New("no record found")
	ErrRecordIDNotProvided    = errors.New("no id provided")
	ErrRecordActionAborted    = errors.New("action aborted")
	ErrRecordQueryNotProvided = errors.New("no id or query provided")
	ErrRecordInvalidID        = errors.New("invalid id")
)
