package db

import "sync"

// SQLiteWriteMutex is a global mutex for serializing SQLite write operations.
//
// SQLite only allows 1 writer at a time, even with WAL mode enabled.
// All code that performs write operations (INSERT, UPDATE, DELETE) to the
// SQLite database MUST acquire this lock to prevent SQLITE_BUSY errors.
//
// Usage:
//
//	db.SQLiteWriteMutex.Lock()
//	defer db.SQLiteWriteMutex.Unlock()
//	// ... perform database write operation ...
var SQLiteWriteMutex sync.Mutex
