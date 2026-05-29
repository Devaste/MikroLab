package core

// Entry represents a single row in a List node — a set of property values
// with optional flags (disabled, dynamic, invalid, slave).
type Entry interface {
	// ID returns the unique identifier (.id) for this entry.
	ID() string

	// Index returns the position of this entry within its parent list.
	Index() int

	// Properties returns all property values keyed by property name.
	Properties() map[string]interface{}

	// Property returns the value of a single property, or false if not found.
	Property(name string) (interface{}, bool)

	// SetProperty updates a property value. Returns an error if the property
	// is read-only or does not exist.
	SetProperty(name string, value interface{}) error

	// Flags returns a map of flag names to their boolean state.
	// Common flags: "disabled", "dynamic", "invalid", "slave".
	Flags() map[string]bool

	// Disabled returns whether this entry is disabled.
	Disabled() bool

	// Dynamic returns whether this entry was created automatically (e.g., by DHCP).
	Dynamic() bool

	// Invalid returns whether this entry has an invalid configuration.
	Invalid() bool

	// Slave returns whether the entry's interface is a slave port.
	Slave() bool
}

// SettingsDirectory is a list-like directory that holds an ordered collection
// of entries. Every entry shares the same property schema.
//
// In RouterOS 7, list nodes (e.g., /ip/address, /interface/bridge/vlan)
// are represented as settings directories. They support full CRUD operations
// and emit events on changes.
type SettingsDirectory interface {
	Directory

	// Add creates a new entry with the given properties, applies defaults
	// for missing properties, and returns the created entry.
	Add(props map[string]interface{}) (Entry, error)

	// Set updates one or more properties of an existing entry identified by its ID.
	// Returns an error if the entry does not exist or a property is read-only.
	Set(id string, props map[string]interface{}) error

	// Remove deletes the entry with the given ID.
	// Returns an error if the entry does not exist or is dynamic.
	Remove(id string) error

	// List returns all entries in order.
	List() []Entry

	// Get returns the entry with the given ID, or false if not found.
	Get(id string) (Entry, bool)
}
