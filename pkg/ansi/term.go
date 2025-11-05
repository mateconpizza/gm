package ansi

// Cursor control sequences for showing, hiding, and moving the cursor.
type Cursor struct {
	Hide  CursorCode // Hide the cursor
	Show  CursorCode // Show the cursor
	Up    CursorCode // Move cursor up one line
	Down  CursorCode // Move cursor down one line
	Right CursorCode // Move cursor right one column
	Left  CursorCode // Move cursor left one column

	// Absolute positioning.
	Home    CursorCode // Move cursor to home position (0,0)
	ToStart CursorCode // Move cursor to start of current line
	ToEnd   CursorCode // Move cursor to end of current line
	Return  CursorCode // Carriage return to start of line

	// Save and restore cursor position.
	Save    CursorCode // Save cursor position
	Restore CursorCode // Restore cursor position
}

// Erase sequences for clearing parts of the screen or line.
type Erase struct {
	// Line erasing.
	Line        EraseCode // Erase entire line
	LineToEnd   EraseCode // Erase from cursor to end of line
	LineToStart EraseCode // Erase from cursor to start of line

	// Screen erasing.
	Screen        EraseCode // Erase entire screen
	ScreenToEnd   EraseCode // Erase from cursor to end of screen
	ScreenToStart EraseCode // Erase from cursor to start of screen

	// Combined operations.
	ClearScreen     EraseCode // Clear screen and move to home
	ClearLineUp     EraseCode // Move up and clear line
	ClearCharBacksp EraseCode // Backspace, space, backspace (erase char)
}

func NewCursorCodes() *Cursor {
	return &Cursor{
		Hide:  CursorHide,
		Show:  CursorShow,
		Up:    CursorUp,
		Down:  CursorDown,
		Right: CursorRight,
		Left:  CursorLeft,

		// Absolute positioning.
		Home:    CursorHome,
		ToStart: CursorToStart,
		ToEnd:   CursorToEnd,
		Return:  CursorReturn,

		// Save and restore cursor position.
		Save:    CursorSave,
		Restore: CursorRestore,
	}
}

func NewEraseCodes() *Erase {
	return &Erase{
		// Line erasing.
		Line:        EraseLine,
		LineToEnd:   EraseLineToEnd,
		LineToStart: EraseLineToStart,

		// Screen erasing.
		Screen:        EraseScreen,
		ScreenToEnd:   EraseScreenToEnd,
		ScreenToStart: EraseScreenToStart,

		// Combined operations.
		ClearScreen:     ClearScreen,
		ClearLineUp:     ClearLineUp,
		ClearCharBacksp: ClearCharBacksp,
	}
}
