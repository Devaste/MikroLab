package config

import (
	"fmt"
	"strings"
)

// Flag represents a boolean state marker attached to an entry.
type Flag struct {
	Letter      string `json:"letter"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Value       bool   `json:"-"`
}

// PropertyValue stores a single property value with metadata.
type PropertyValue struct {
	Name     string      `json:"name"`
	Type     string      `json:"type"`
	Value    interface{} `json:"value"`
	ReadOnly bool        `json:"readOnly"`
	Computed bool        `json:"computed"`
	Required bool        `json:"required"`
	Default  interface{} `json:"default,omitempty"`
}

// Entry represents a single row in a List node.
type Entry struct {
	ID         string                    `json:"id"`
	Index      int                       `json:"index"`
	Properties map[string]*PropertyValue `json:"properties"`
	Flags      map[string]*Flag          `json:"flags"`
	Disabled   bool                      `json:"disabled"`
	Invalid    bool                      `json:"invalid"`
	Dynamic    bool                      `json:"dynamic"`
	Slave      bool                      `json:"slave"`
}

// NewEntry creates a new entry with the given ID and index.
func NewEntry(id string, index int) *Entry {
	return &Entry{
		ID:         id,
		Index:      index,
		Properties: make(map[string]*PropertyValue),
		Flags:      make(map[string]*Flag),
	}
}

// GetProperty returns a property value by name.
func (e *Entry) GetProperty(name string) (interface{}, bool) {
	prop, ok := e.Properties[name]
	if !ok {
		return nil, false
	}
	return prop.Value, true
}

// SetProperty sets a property value with sanitization.
func (e *Entry) SetProperty(name string, value interface{}) error {
	prop, ok := e.Properties[name]
	if !ok {
		return fmt.Errorf("property %q not found in entry %s", name, e.ID)
	}
	if prop.ReadOnly {
		return fmt.Errorf("property %q is read-only", name)
	}
	// Sanitize string values
	if strVal, ok := value.(string); ok && prop.Type == "string" {
		value = SanitizeString(strVal, MaxPropertyStringLen)
	}
	if commentVal, ok := value.(string); ok && name == "comment" {
		value = SanitizeComment(commentVal, MaxCommentLen)
	}
	prop.Value = value
	return nil
}

// GetString returns a property as string.
func (e *Entry) GetString(name string) string {
	val, ok := e.GetProperty(name)
	if !ok {
		return ""
	}
	s, _ := val.(string)
	return s
}

// FlagString returns the flags string (e.g., "XID S") for display.
func (e *Entry) FlagString() string {
	var flags []string
	if e.Disabled {
		flags = append(flags, "X")
	}
	if e.Invalid {
		flags = append(flags, "I")
	}
	if e.Dynamic {
		flags = append(flags, "D")
	}
	if e.Slave {
		flags = append(flags, "S")
	}
	return strings.Join(flags, " ")
}

// Clone creates a deep copy of the entry.
func (e *Entry) Clone() *Entry {
	clone := NewEntry(e.ID, e.Index)
	clone.Disabled = e.Disabled
	clone.Invalid = e.Invalid
	clone.Dynamic = e.Dynamic
	clone.Slave = e.Slave
	for k, v := range e.Properties {
		prop := *v
		clone.Properties[k] = &prop
	}
	return clone
}
