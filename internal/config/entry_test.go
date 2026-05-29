package config

import (
	"testing"
)

func TestNewEntry(t *testing.T) {
	e := NewEntry("test-id", 0)
	if e.ID != "test-id" {
		t.Errorf("expected ID 'test-id', got %q", e.ID)
	}
	if e.Index != 0 {
		t.Errorf("expected Index 0, got %d", e.Index)
	}
	if len(e.Properties) != 0 {
		t.Errorf("expected empty properties, got %d", len(e.Properties))
	}
}

func TestEntrySetProperty(t *testing.T) {
	e := NewEntry("test", 0)
	e.Properties["name"] = &PropertyValue{Name: "name", Value: "initial"}
	e.Properties["readonly"] = &PropertyValue{Name: "readonly", Value: "cantchange", ReadOnly: true}

	// Test set existing property
	err := e.SetProperty("name", "updated")
	if err != nil {
		t.Errorf("unexpected error setting property: %v", err)
	}
	val, _ := e.GetProperty("name")
	if val != "updated" {
		t.Errorf("expected 'updated', got %v", val)
	}

	// Test set non-existent property
	err = e.SetProperty("nonexistent", "value")
	if err == nil {
		t.Error("expected error for non-existent property")
	}

	// Test set read-only property
	err = e.SetProperty("readonly", "newvalue")
	if err == nil {
		t.Error("expected error for read-only property")
	}
}

func TestEntryGetProperty(t *testing.T) {
	e := NewEntry("test", 0)
	e.Properties["address"] = &PropertyValue{Name: "address", Value: "192.168.1.1/24"}

	val, ok := e.GetProperty("address")
	if !ok {
		t.Error("expected to find property 'address'")
	}
	if val != "192.168.1.1/24" {
		t.Errorf("expected '192.168.1.1/24', got %v", val)
	}

	_, ok = e.GetProperty("nonexistent")
	if ok {
		t.Error("expected not to find non-existent property")
	}
}

func TestEntryGetString(t *testing.T) {
	e := NewEntry("test", 0)
	e.Properties["name"] = &PropertyValue{Name: "name", Value: "hello"}

	if s := e.GetString("name"); s != "hello" {
		t.Errorf("expected 'hello', got %q", s)
	}
	if s := e.GetString("nonexistent"); s != "" {
		t.Errorf("expected empty string, got %q", s)
	}
}

func TestEntryFlagString(t *testing.T) {
	e := NewEntry("test", 0)

	// No flags
	if fs := e.FlagString(); fs != "" {
		t.Errorf("expected empty flags, got %q", fs)
	}

	// All flags
	e.Disabled = true
	e.Invalid = true
	e.Dynamic = true
	e.Slave = true
	if fs := e.FlagString(); fs != "X I D S" {
		t.Errorf("expected 'X I D S', got %q", fs)
	}
}

func TestEntryClone(t *testing.T) {
	e := NewEntry("orig", 0)
	e.Properties["name"] = &PropertyValue{Name: "name", Value: "original"}
	e.Disabled = true
	e.Dynamic = false

	clone := e.Clone()
	if clone.ID != "orig" {
		t.Errorf("expected ID 'orig', got %q", clone.ID)
	}
	if !clone.Disabled {
		t.Error("expected clone to be disabled")
	}
	val, _ := clone.GetProperty("name")
	if val != "original" {
		t.Errorf("expected 'original', got %v", val)
	}

	// Modify original, ensure clone unchanged
	e.Properties["name"].Value = "modified"
	cloneVal, _ := clone.GetProperty("name")
	if cloneVal != "original" {
		t.Errorf("expected clone to still have 'original', got %v", cloneVal)
	}
}

func TestPropertyValueDefaults(t *testing.T) {
	pv := &PropertyValue{
		Name:     "test",
		Type:     "string",
		Value:    "val",
		ReadOnly: true,
		Required: true,
		Default:  "default",
	}
	if pv.Name != "test" {
		t.Errorf("expected Name 'test', got %q", pv.Name)
	}
	if !pv.ReadOnly {
		t.Error("expected ReadOnly true")
	}
	if !pv.Required {
		t.Error("expected Required true")
	}
	if pv.Default != "default" {
		t.Errorf("expected Default 'default', got %v", pv.Default)
	}
}
