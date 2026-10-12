package database

// The SQLite driver is imported in exactly one place so it is easy to swap.
// To use the pure-Go driver instead (no C compiler needed), replace the import with
//
//	_ "modernc.org/sqlite"
//
// set driverName to "sqlite", and change the DSN options in Open to that driver's
// "_pragma=" style.
import _ "github.com/mattn/go-sqlite3"

const driverName = "sqlite3"
