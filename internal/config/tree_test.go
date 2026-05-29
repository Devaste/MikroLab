package config

import (
	"testing"
)

func TestNewConfigTree(t *testing.T) {
	ct := NewConfigTree()
	if ct == nil {
		t.Fatal("expected non-nil ConfigTree")
	}
	if ct.Root == nil {
		t.Fatal("expected non-nil root node")
	}
	if ct.Root.Path != "/" {
		t.Errorf("expected root path '/', got %q", ct.Root.Path)
	}
	if ct.Root.Type != NodeTypeDirectory {
		t.Errorf("expected root type directory, got %v", ct.Root.Type)
	}
}

func TestNavigateRoot(t *testing.T) {
	ct := NewConfigTree()
	node, err := ct.Navigate("")
	if err != nil {
		t.Errorf("unexpected error navigating root: %v", err)
	}
	if node != ct.Root {
		t.Error("expected root node")
	}

	node, err = ct.Navigate("/")
	if err != nil {
		t.Errorf("unexpected error navigating '/': %v", err)
	}
	if node != ct.Root {
		t.Error("expected root node")
	}
}

func TestEnsurePath(t *testing.T) {
	ct := NewConfigTree()
	node, err := ct.EnsurePath("/ip/address", NodeTypeList, "IP Addresses")
	if err != nil {
		t.Fatalf("unexpected error ensuring path: %v", err)
	}

	if node.Path != "/ip/address" {
		t.Errorf("expected path '/ip/address', got %q", node.Path)
	}
	if node.Type != NodeTypeList {
		t.Errorf("expected type list, got %v", node.Type)
	}
	if node.Title != "IP Addresses" {
		t.Errorf("expected title 'IP Addresses', got %q", node.Title)
	}

	// Check intermediate node
	ipNode, err := ct.Navigate("/ip")
	if err != nil {
		t.Fatalf("unexpected error navigating '/ip': %v", err)
	}
	if ipNode.Type != NodeTypeDirectory {
		t.Errorf("expected /ip to be directory, got %v", ipNode.Type)
	}
}

func TestNavigateExistingPath(t *testing.T) {
	ct := NewConfigTree()
	ct.EnsurePath("/ip/address", NodeTypeList, "IP Addresses")

	node, err := ct.Navigate("/ip/address")
	if err != nil {
		t.Fatalf("unexpected error navigating: %v", err)
	}
	if node.Path != "/ip/address" {
		t.Errorf("expected '/ip/address', got %q", node.Path)
	}
}

func TestNavigateNonExistentPath(t *testing.T) {
	ct := NewConfigTree()
	_, err := ct.Navigate("/nonexistent")
	if err == nil {
		t.Error("expected error navigating non-existent path")
	}
}

func TestAddEntry(t *testing.T) {
	ct := NewConfigTree()
	ct.EnsurePath("/test/list", NodeTypeList, "Test List")

	entry := NewEntry("", 0)
	entry.Properties["name"] = &PropertyValue{Name: "name", Value: "test-entry"}

	err := ct.AddEntry("/test/list", entry)
	if err != nil {
		t.Fatalf("unexpected error adding entry: %v", err)
	}

	if entry.ID == "" {
		t.Error("expected entry ID to be set")
	}
	if entry.Index != 0 {
		t.Errorf("expected index 0, got %d", entry.Index)
	}

	entries, err := ct.GetEntries("/test/list")
	if err != nil {
		t.Fatalf("unexpected error getting entries: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].ID != entry.ID {
		t.Errorf("expected entry ID %q, got %q", entry.ID, entries[0].ID)
	}
}

func TestAddEntryToNonList(t *testing.T) {
	ct := NewConfigTree()
	ct.EnsurePath("/test", NodeTypeDirectory, "Test")

	entry := NewEntry("", 0)
	err := ct.AddEntry("/test", entry)
	if err == nil {
		t.Error("expected error adding entry to non-list node")
	}
}

func TestAddMultipleEntries(t *testing.T) {
	ct := NewConfigTree()
	ct.EnsurePath("/test", NodeTypeList, "Test")

	e1 := NewEntry("", 0)
	e2 := NewEntry("", 0)

	ct.AddEntry("/test", e1)
	ct.AddEntry("/test", e2)

	entries, _ := ct.GetEntries("/test")
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Index != 0 {
		t.Errorf("expected first entry index 0, got %d", entries[0].Index)
	}
	if entries[1].Index != 1 {
		t.Errorf("expected second entry index 1, got %d", entries[1].Index)
	}
}

func TestRemoveEntry(t *testing.T) {
	ct := NewConfigTree()
	ct.EnsurePath("/test", NodeTypeList, "Test")

	e1 := NewEntry("", 0)
	e2 := NewEntry("", 0)
	ct.AddEntry("/test", e1)
	ct.AddEntry("/test", e2)

	err := ct.RemoveEntry("/test", e1.ID)
	if err != nil {
		t.Fatalf("unexpected error removing entry: %v", err)
	}

	entries, _ := ct.GetEntries("/test")
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry after removal, got %d", len(entries))
	}
	if entries[0].ID != e2.ID {
		t.Errorf("expected remaining entry to be e2, got %q", entries[0].ID)
	}
}

func TestRemoveEntryNotFound(t *testing.T) {
	ct := NewConfigTree()
	ct.EnsurePath("/test", NodeTypeList, "Test")
	err := ct.RemoveEntry("/test", "nonexistent-id")
	if err == nil {
		t.Error("expected error removing non-existent entry")
	}
}

func TestRemoveDynamicEntry(t *testing.T) {
	ct := NewConfigTree()
	ct.EnsurePath("/test", NodeTypeList, "Test")

	e := NewEntry("", 0)
	e.Dynamic = true
	ct.AddEntry("/test", e)

	err := ct.RemoveEntry("/test", e.ID)
	if err == nil {
		t.Error("expected error removing dynamic entry")
	}
}

func TestGetEntryByID(t *testing.T) {
	ct := NewConfigTree()
	ct.EnsurePath("/test", NodeTypeList, "Test")

	entry := NewEntry("", 0)
	ct.AddEntry("/test", entry)

	found, err := ct.GetEntryByID("/test", entry.ID)
	if err != nil {
		t.Fatalf("unexpected error getting entry: %v", err)
	}
	if found.ID != entry.ID {
		t.Errorf("expected entry ID %q, got %q", entry.ID, found.ID)
	}
}

func TestGetEntryByIDNotFound(t *testing.T) {
	ct := NewConfigTree()
	ct.EnsurePath("/test", NodeTypeList, "Test")
	_, err := ct.GetEntryByID("/test", "nonexistent")
	if err == nil {
		t.Error("expected error for non-existent entry")
	}
}

func TestSetEntry(t *testing.T) {
	ct := NewConfigTree()
	ct.EnsurePath("/test", NodeTypeList, "Test")

	entry := NewEntry("", 0)
	entry.Properties["name"] = &PropertyValue{Name: "name", Value: "original"}
	entry.Properties["comment"] = &PropertyValue{Name: "comment", Value: ""}
	ct.AddEntry("/test", entry)

	err := ct.SetEntry("/test", entry.ID, map[string]interface{}{
		"name":    "updated",
		"comment": "test comment",
	})
	if err != nil {
		t.Fatalf("unexpected error setting entry: %v", err)
	}

	updated, _ := ct.GetEntryByID("/test", entry.ID)
	if updated.GetString("name") != "updated" {
		t.Errorf("expected 'updated', got %q", updated.GetString("name"))
	}
	if updated.GetString("comment") != "test comment" {
		t.Errorf("expected 'test comment', got %q", updated.GetString("comment"))
	}
}

func TestListNodes(t *testing.T) {
	ct := NewConfigTree()
	ct.EnsurePath("/ip/address", NodeTypeList, "IP Addresses")
	ct.EnsurePath("/ip/arp", NodeTypeList, "ARP")

	names, err := ct.ListNodes("/ip")
	if err != nil {
		t.Fatalf("unexpected error listing nodes: %v", err)
	}

	expected := map[string]bool{"address": true, "arp": true}
	for _, n := range names {
		if !expected[n] {
			t.Errorf("unexpected child node: %s", n)
		}
		delete(expected, n)
	}
	if len(expected) > 0 {
		t.Errorf("missing children: %v", expected)
	}
}

func TestGetEntriesOnNonList(t *testing.T) {
	ct := NewConfigTree()
	ct.EnsurePath("/test", NodeTypeDirectory, "Test")
	_, err := ct.GetEntries("/test")
	if err == nil {
		t.Error("expected error getting entries from directory node")
	}
}

func TestConcurrentAccess(t *testing.T) {
	ct := NewConfigTree()
	ct.EnsurePath("/test", NodeTypeList, "Test")

	done := make(chan bool)

	// Add entries concurrently
	go func() {
		for i := 0; i < 50; i++ {
			e := NewEntry("", 0)
			e.Properties["num"] = &PropertyValue{Name: "num", Value: i}
			ct.AddEntry("/test", e)
		}
		done <- true
	}()

	go func() {
		for i := 0; i < 50; i++ {
			e := NewEntry("", 0)
			e.Properties["num"] = &PropertyValue{Name: "num", Value: i}
			ct.AddEntry("/test", e)
		}
		done <- true
	}()

	<-done
	<-done

	entries, _ := ct.GetEntries("/test")
	if len(entries) != 100 {
		t.Errorf("expected 100 entries, got %d", len(entries))
	}
}
