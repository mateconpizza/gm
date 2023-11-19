package errs

import "errors"

var (
	// bookmark
	ErrActionAborted       = errors.New("action aborted")
	ErrBookmarkDuplicate   = errors.New("bookmark already exists")
	ErrBookmarkEdition     = errors.New("")
	ErrBookmarkInvalid     = errors.New("bookmark invalid")
	ErrBookmarkNotFound    = errors.New("no bookmarks found")
	ErrBookmarkNotSelected = errors.New("no bookmarks selected")
	ErrBookmarkUnchaged    = errors.New("buffer unchanged")
	ErrEditorNotFound      = errors.New("editor not found")
	ErrItemNotFound        = errors.New("item not found")
	ErrOptionInvalid       = errors.New("invalid option")
	ErrTagsEmpty           = errors.New("tags cannot be empty")
	ErrURLEmpty            = errors.New("URL cannot be empty")
	ErrNoIDProvided        = errors.New("no id provided")
	ErrNoIDorQueryPrivided = errors.New("no id or query provided")

	// database
	ErrRecordNotExists      = errors.New("row not exists")
	ErrRecordDelete         = errors.New("error delete record")
	ErrRecordDuplicate      = errors.New("record already exists")
	ErrRecordInsert         = errors.New("inserting record")
	ErrRecordScan           = errors.New("scan record")
	ErrRecordUpdate         = errors.New("update failed")
	ErrSQLQuery             = errors.New("executing query")
	ErrDBNotFound           = errors.New("database not found")
	ErrDBAlreadyInitialized = errors.New("database already initialized")
	ErrDBSequence           = errors.New("resetting sqlite_sequence")
)
