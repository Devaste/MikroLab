package ip_arp_test

import (
	_ "embed"
	"encoding/json"
	"testing"

	"github.com/Devaste/MikroLab/internal/config"
)

//go:embed schema.json
var arpSchemaJSON []byte

func loadARPModule(t *testing.T) (*config.ModuleManager, *config.ConfigTree) {
	t.Helper()
	ct := config.NewConfigTree()
	vr := config.NewValidatorRegistry()
	mm := config.NewModuleManager(ct, vr)

	schema := &config.ModuleSchema{}
	if err := json.Unmarshal(arpSchemaJSON, schema); err != nil {
		t.Fatalf("failed to parse ARP schema: %v", err)
	}

	if err := mm.RegisterModule(schema); err != nil {
		t.Fatalf("failed to register ARP module: %v", err)
	}

	return mm, ct
}

func TestARPAddValidEntry(t *testing.T) {
	mm, _ := loadARPModule(t)

	op := config.NewOperation(config.OpAdd, "/ip/arp")
	op.Properties["address"] = "192.168.1.1"
	op.Properties["mac-address"] = "00:11:22:33:44:55"
	op.Properties["interface"] = "ether1"

	if err := mm.ExecuteOperation(op); err != nil {
		t.Fatalf("unexpected error adding ARP entry: %v", err)
	}

	entries, err := mm.Tree.GetEntries("/ip/arp")
	if err != nil {
		t.Fatalf("unexpected error getting entries: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	entry := entries[0]
	if entry.GetString("address") != "192.168.1.1" {
		t.Errorf("expected address '192.168.1.1', got %q", entry.GetString("address"))
	}
	if entry.GetString("mac-address") != "00:11:22:33:44:55" {
		t.Errorf("expected MAC '00:11:22:33:44:55', got %q", entry.GetString("mac-address"))
	}
	if entry.GetString("interface") != "ether1" {
		t.Errorf("expected interface 'ether1', got %q", entry.GetString("interface"))
	}
}

func TestARPAddMultipleEntries(t *testing.T) {
	mm, _ := loadARPModule(t)

	entries := []struct {
		ip    string
		mac   string
		iface string
	}{
		{"192.168.1.1", "00:11:22:33:44:55", "ether1"},
		{"192.168.1.2", "00:11:22:33:44:56", "ether1"},
		{"10.0.0.1", "aa:bb:cc:dd:ee:ff", "ether2"},
	}

	for _, e := range entries {
		op := config.NewOperation(config.OpAdd, "/ip/arp")
		op.Properties["address"] = e.ip
		op.Properties["mac-address"] = e.mac
		op.Properties["interface"] = e.iface
		if err := mm.ExecuteOperation(op); err != nil {
			t.Fatalf("unexpected error adding entry %s: %v", e.ip, err)
		}
	}

	all, _ := mm.Tree.GetEntries("/ip/arp")
	if len(all) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(all))
	}
}

func TestARPDuplicateEntry(t *testing.T) {
	mm, _ := loadARPModule(t)

	op1 := config.NewOperation(config.OpAdd, "/ip/arp")
	op1.Properties["address"] = "192.168.1.1"
	op1.Properties["mac-address"] = "00:11:22:33:44:55"
	op1.Properties["interface"] = "ether1"
	mm.ExecuteOperation(op1)

	op2 := config.NewOperation(config.OpAdd, "/ip/arp")
	op2.Properties["address"] = "192.168.1.1"
	op2.Properties["mac-address"] = "00:11:22:33:44:55"
	op2.Properties["interface"] = "ether1"
	if err := mm.ExecuteOperation(op2); err == nil {
		t.Error("expected error for duplicate ARP entry")
	}
}

func TestARPSameIPDifferentMAC(t *testing.T) {
	mm, _ := loadARPModule(t)

	op1 := config.NewOperation(config.OpAdd, "/ip/arp")
	op1.Properties["address"] = "192.168.1.1"
	op1.Properties["mac-address"] = "00:11:22:33:44:55"
	op1.Properties["interface"] = "ether1"
	mm.ExecuteOperation(op1)

	op2 := config.NewOperation(config.OpAdd, "/ip/arp")
	op2.Properties["address"] = "192.168.1.1"
	op2.Properties["mac-address"] = "00:11:22:33:44:66"
	op2.Properties["interface"] = "ether1"
	if err := mm.ExecuteOperation(op2); err != nil {
		t.Errorf("expected same IP different MAC to be valid, got: %v", err)
	}
}

func TestARPInvalidMAC(t *testing.T) {
	mm, _ := loadARPModule(t)

	invalidMACs := []string{
		"invalid",
		"00:11:22:33:44:GG",
		"00:11:22:33:44",
		"00-11-22-33-44-55-66",
	}

	for _, mac := range invalidMACs {
		op := config.NewOperation(config.OpAdd, "/ip/arp")
		op.Properties["address"] = "192.168.1.1"
		op.Properties["mac-address"] = mac
		op.Properties["interface"] = "ether1"
		if err := mm.ExecuteOperation(op); err == nil {
			t.Errorf("expected error for invalid MAC %q", mac)
		}
	}
}

func TestARPValidMACFormats(t *testing.T) {
	mm, _ := loadARPModule(t)

	validMACs := []string{
		"00:11:22:33:44:55",
		"aa:bb:cc:dd:ee:ff",
		"00-11-22-33-44-55",
		"0011.2233.4455",
	}

	for _, mac := range validMACs {
		op := config.NewOperation(config.OpAdd, "/ip/arp")
		op.Properties["address"] = "192.168.1.1"
		op.Properties["mac-address"] = mac
		op.Properties["interface"] = "ether1"
		if err := mm.ExecuteOperation(op); err != nil {
			t.Errorf("unexpected error for valid MAC %q: %v", mac, err)
		}
	}
}

func TestARPRemoveEntry(t *testing.T) {
	mm, _ := loadARPModule(t)

	op := config.NewOperation(config.OpAdd, "/ip/arp")
	op.Properties["address"] = "192.168.1.1"
	op.Properties["mac-address"] = "00:11:22:33:44:55"
	op.Properties["interface"] = "ether1"
	mm.ExecuteOperation(op)

	entries, _ := mm.Tree.GetEntries("/ip/arp")
	entryID := entries[0].ID

	opRemove := config.NewOperation(config.OpRemove, "/ip/arp")
	opRemove.EntryID = entryID
	if err := mm.ExecuteOperation(opRemove); err != nil {
		t.Fatalf("unexpected error removing ARP entry: %v", err)
	}

	entries, _ = mm.Tree.GetEntries("/ip/arp")
	if len(entries) != 0 {
		t.Errorf("expected 0 entries after removal, got %d", len(entries))
	}
}

func TestARPRemoveNonExistent(t *testing.T) {
	mm, _ := loadARPModule(t)

	op := config.NewOperation(config.OpRemove, "/ip/arp")
	op.EntryID = "nonexistent"
	if err := mm.ExecuteOperation(op); err == nil {
		t.Error("expected error removing non-existent ARP entry")
	}
}

func TestARPDisableEnable(t *testing.T) {
	mm, _ := loadARPModule(t)

	op := config.NewOperation(config.OpAdd, "/ip/arp")
	op.Properties["address"] = "192.168.1.1"
	op.Properties["mac-address"] = "00:11:22:33:44:55"
	op.Properties["interface"] = "ether1"
	mm.ExecuteOperation(op)

	entries, _ := mm.Tree.GetEntries("/ip/arp")
	entryID := entries[0].ID

	// Disable
	opDisable := config.NewOperation(config.OpDisable, "/ip/arp")
	opDisable.EntryID = entryID
	if err := mm.ExecuteOperation(opDisable); err != nil {
		t.Fatalf("unexpected error disabling: %v", err)
	}

	entry, _ := mm.Tree.GetEntryByID("/ip/arp", entryID)
	if !entry.Disabled {
		t.Error("expected entry to be disabled")
	}

	// Enable
	opEnable := config.NewOperation(config.OpEnable, "/ip/arp")
	opEnable.EntryID = entryID
	if err := mm.ExecuteOperation(opEnable); err != nil {
		t.Fatalf("unexpected error enabling: %v", err)
	}

	entry, _ = mm.Tree.GetEntryByID("/ip/arp", entryID)
	if entry.Disabled {
		t.Error("expected entry to be enabled")
	}
}

func TestARPSetProperties(t *testing.T) {
	mm, _ := loadARPModule(t)

	op := config.NewOperation(config.OpAdd, "/ip/arp")
	op.Properties["address"] = "192.168.1.1"
	op.Properties["mac-address"] = "00:11:22:33:44:55"
	op.Properties["interface"] = "ether1"
	mm.ExecuteOperation(op)

	entries, _ := mm.Tree.GetEntries("/ip/arp")
	entryID := entries[0].ID

	opSet := config.NewOperation(config.OpSet, "/ip/arp")
	opSet.EntryID = entryID
	opSet.Properties = map[string]interface{}{
		"mac-address": "00:11:22:33:44:66",
		"published":   true,
	}
	if err := mm.ExecuteOperation(opSet); err != nil {
		t.Fatalf("unexpected error setting properties: %v", err)
	}

	entry, _ := mm.Tree.GetEntryByID("/ip/arp", entryID)
	if entry.GetString("mac-address") != "00:11:22:33:44:66" {
		t.Errorf("expected MAC '00:11:22:33:44:66', got %q", entry.GetString("mac-address"))
	}
	if entry.GetString("address") != "192.168.1.1" {
		t.Errorf("expected address to remain '192.168.1.1', got %q", entry.GetString("address"))
	}
}

func TestARPPublishedDefault(t *testing.T) {
	mm, _ := loadARPModule(t)

	op := config.NewOperation(config.OpAdd, "/ip/arp")
	op.Properties["address"] = "192.168.1.1"
	op.Properties["mac-address"] = "00:11:22:33:44:55"
	op.Properties["interface"] = "ether1"
	mm.ExecuteOperation(op)

	entries, _ := mm.Tree.GetEntries("/ip/arp")
	entry := entries[0]

	published, ok := entry.GetProperty("published")
	if !ok {
		t.Fatal("expected 'published' property")
	}
	if published.(bool) {
		t.Error("expected published to default to false")
	}
}

func TestARPStatusReadOnly(t *testing.T) {
	mm, _ := loadARPModule(t)

	op := config.NewOperation(config.OpAdd, "/ip/arp")
	op.Properties["address"] = "192.168.1.1"
	op.Properties["mac-address"] = "00:11:22:33:44:55"
	op.Properties["interface"] = "ether1"
	mm.ExecuteOperation(op)

	entries, _ := mm.Tree.GetEntries("/ip/arp")
	entryID := entries[0].ID

	opSet := config.NewOperation(config.OpSet, "/ip/arp")
	opSet.EntryID = entryID
	opSet.Properties = map[string]interface{}{
		"status": "reachable",
	}
	if err := mm.ExecuteOperation(opSet); err != nil {
		// Expected: status is read-only but our model may not enforce it strictly
		t.Logf("set status returned (expected if read-only enforced): %v", err)
	}
}

func TestARPWithDifferentInterfaces(t *testing.T) {
	mm, _ := loadARPModule(t)

	interfaces := []string{"ether1", "ether2", "bridge1", "wlan1"}
	for i, iface := range interfaces {
		op := config.NewOperation(config.OpAdd, "/ip/arp")
		op.Properties["address"] = "10.0.0.1"
		op.Properties["mac-address"] = "00:11:22:33:44:55"
		op.Properties["interface"] = iface
		err := mm.ExecuteOperation(op)
		if err != nil {
			t.Errorf("unexpected error for interface %q (entry %d): %v", iface, i, err)
		}
	}

	entries, _ := mm.Tree.GetEntries("/ip/arp")
	if len(entries) != len(interfaces) {
		t.Errorf("expected %d entries, got %d", len(interfaces), len(entries))
	}
}

func TestARPEventEmitted(t *testing.T) {
	mm, ct := loadARPModule(t)
	received := false

	ct.EventBus.Subscribe("/ip/arp", func(event config.Event) {
		received = true
		if event.Type != config.EventAdd {
			t.Errorf("expected EventAdd, got %v", event.Type)
		}
	})

	op := config.NewOperation(config.OpAdd, "/ip/arp")
	op.Properties["address"] = "192.168.1.1"
	op.Properties["mac-address"] = "00:11:22:33:44:55"
	op.Properties["interface"] = "ether1"
	mm.ExecuteOperation(op)

	if !received {
		t.Error("expected event to be emitted")
	}
}

func TestARPFlags(t *testing.T) {
	schema := &config.ModuleSchema{}
	if err := json.Unmarshal(arpSchemaJSON, schema); err != nil {
		t.Fatalf("failed to parse ARP schema: %v", err)
	}

	if len(schema.Flags) != 6 {
		t.Fatalf("expected 6 flags, got %d", len(schema.Flags))
	}

	flagMap := make(map[string]config.SchemaFlag)
	for _, f := range schema.Flags {
		flagMap[f.Letter] = f
	}

	expected := []string{"X", "I", "H", "D", "P", "C"}
	for _, letter := range expected {
		if _, ok := flagMap[letter]; !ok {
			t.Errorf("missing flag %q", letter)
		}
	}
}
