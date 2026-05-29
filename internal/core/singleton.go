package core

// SingletonDirectory is a directory containing exactly one entry.
// In RouterOS 7, singleton directories represent nodes where the
// configuration is a single set of properties rather than a list
// of entries (e.g., /system/identity, /system/clock).
//
// Unlike SettingsDirectory, there is no Add or Remove — the singleton
// entry always exists. Use Get to read its current state and Set to
// modify writable properties.
type SingletonDirectory interface {
	Directory

	// Get returns the single entry managed by this directory.
	Get() Entry

	// Set updates properties on the singleton entry.
	// Returns an error if any of the given properties are read-only
	// or do not exist in the schema.
	Set(props map[string]interface{}) error
}
