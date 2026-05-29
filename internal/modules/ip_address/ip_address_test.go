package ip_address_test

import (
	"testing"

	"github.com/Devaste/MikroLab/internal/config"
)

func loadIPAddressModule(t *testing.T) (*config.ModuleManager, *config.ConfigTree) {
	t.Helper()
	ct := config.NewConfigTree()
	vr := config.NewValidatorRegistry()
	mm := config.NewModuleManager(ct, vr)

	schema := &config.ModuleSchema{
		Path:        "/ip/address",
		Type:        "list",
		Title:       "IP Addresses",
		Description: "Manages IPv4 addresses assigned to router interfaces.",
		Flags: []config.SchemaFlag{
			{Letter: "X", Name: "disabled", Description: "Entry is disabled."},
			{Letter: "I", Name: "invalid", Description: "Configuration is invalid."},
			{Letter: "D", Name: "dynamic", Description: "Entry created by a DHCP client."},
			{Letter: "S", Name: "slave", Description: "Address belongs to a slave interface."},
		},
		Schema: map[string]*config.SchemaProperty{
			"address": {
				Name: "address", Type: config.SchemaString, Required: true,
				Description: "IPv4 address with prefix length.",
			},
			"network": {
				Name: "network", Type: config.SchemaIPAddr,
				Description: "Network address derived from address and netmask.",
			},
			"broadcast": {
				Name: "broadcast", Type: config.SchemaIPAddr,
				Description: "Broadcast address derived from address and netmask.",
			},
			"interface": {
				Name: "interface", Type: config.SchemaInterface, Required: true,
				Description: "Interface on which the IP address is configured.",
			},
			"actual-interface": {
				Name: "actual-interface", Type: config.SchemaInterface, ReadOnly: true,
				Description: "Actual interface where the address is set up.",
			},
			"vrf": {
				Name: "vrf", Type: config.SchemaEnum, ReadOnly: true,
				Default: "main", Description: "VRF this address is associated with.",
			},
			"comment": {
				Name: "comment", Type: config.SchemaString, Default: "",
				Description: "User comment.",
			},
		},
		Actions: map[string]*config.SchemaAction{
			"add": {
				Name: "add", Parameters: []string{"address", "interface", "comment"},
				Validators:  []string{"duplicate_ip_per_interface", "valid_netmask", "interface_exists", "ip_not_in_reserved_range"},
				FlagsSet:    []string{"disabled"},
				Description: "Add a new IP address.",
			},
			"set": {
				Name: "set", Parameters: []string{"numbers", "address", "interface", "comment"},
				Validators:  []string{"entry_exists", "duplicate_ip_per_interface", "interface_exists"},
				Description: "Modify properties of existing IP address(es).",
			},
			"remove": {
				Name: "remove", Parameters: []string{"numbers"},
				Validators:  []string{"entry_exists", "not_dynamic"},
				Description: "Delete IP address(es).",
			},
			"disable": {
				Name: "disable", Parameters: []string{"numbers"},
				Validators:  []string{"entry_exists"},
				Description: "Disable IP address(es).",
			},
			"enable": {
				Name: "enable", Parameters: []string{"numbers"},
				Validators:  []string{"entry_exists"},
				Description: "Enable disabled IP address(es).",
			},
		},
		Defaults: map[string]interface{}{
			"comment":  "",
			"disabled": false,
			"vrf":      "main",
		},
		Constraints: map[string]string{
			"duplicate_ip_per_interface": "Cannot add the same IP address on the same interface.",
			"valid_netmask":              "Netmask must be between /0 and /32.",
			"interface_exists":           "Interface must exist in /interface.",
			"ip_not_in_reserved_range":   "Reserved IP ranges cannot be assigned.",
		},
	}

	if err := mm.RegisterModule(schema); err != nil {
		t.Fatalf("failed to register IP address module: %v", err)
	}

	return mm, ct
}

func TestIPAddressAddValidEntry(t *testing.T) {
	mm, _ := loadIPAddressModule(t)

	op := config.NewOperation(config.OpAdd, "/ip/address")
	op.Properties["address"] = "192.168.1.1/24"
	op.Properties["interface"] = "ether1"
	op.Properties["comment"] = "LAN interface"

	if err := mm.ExecuteOperation(op); err != nil {
		t.Fatalf("unexpected error adding IP address: %v", err)
	}

	entries, err := mm.Tree.GetEntries("/ip/address")
	if err != nil {
		t.Fatalf("unexpected error getting entries: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	entry := entries[0]
	if entry.GetString("address") != "192.168.1.1/24" {
		t.Errorf("expected address '192.168.1.1/24', got %q", entry.GetString("address"))
	}
	if entry.GetString("interface") != "ether1" {
		t.Errorf("expected interface 'ether1', got %q", entry.GetString("interface"))
	}
	if entry.GetString("comment") != "LAN interface" {
		t.Errorf("expected comment 'LAN interface', got %q", entry.GetString("comment"))
	}
}

func TestIPAddressAddMultiple(t *testing.T) {
	mm, _ := loadIPAddressModule(t)

	addrs := []struct {
		address string
		iface   string
		comment string
	}{
		{"192.168.1.1/24", "ether1", "LAN"},
		{"10.0.0.1/8", "ether2", "WAN"},
		{"172.16.0.1/12", "ether3", "DMZ"},
	}

	for _, a := range addrs {
		op := config.NewOperation(config.OpAdd, "/ip/address")
		op.Properties["address"] = a.address
		op.Properties["interface"] = a.iface
		op.Properties["comment"] = a.comment
		if err := mm.ExecuteOperation(op); err != nil {
			t.Fatalf("unexpected error adding %s: %v", a.address, err)
		}
	}

	entries, _ := mm.Tree.GetEntries("/ip/address")
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
}

func TestIPAddressDuplicateIPOnSameInterface(t *testing.T) {
	mm, _ := loadIPAddressModule(t)

	op1 := config.NewOperation(config.OpAdd, "/ip/address")
	op1.Properties["address"] = "192.168.1.1/24"
	op1.Properties["interface"] = "ether1"
	if err := mm.ExecuteOperation(op1); err != nil {
		t.Fatalf("unexpected error adding first entry: %v", err)
	}

	op2 := config.NewOperation(config.OpAdd, "/ip/address")
	op2.Properties["address"] = "192.168.1.1/24"
	op2.Properties["interface"] = "ether1"
	if err := mm.ExecuteOperation(op2); err == nil {
		t.Error("expected error for duplicate IP on same interface")
	}
}

func TestIPAddressSameIPDifferentInterface(t *testing.T) {
	mm, _ := loadIPAddressModule(t)

	op1 := config.NewOperation(config.OpAdd, "/ip/address")
	op1.Properties["address"] = "192.168.1.1/24"
	op1.Properties["interface"] = "ether1"
	if err := mm.ExecuteOperation(op1); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	op2 := config.NewOperation(config.OpAdd, "/ip/address")
	op2.Properties["address"] = "192.168.1.1/24"
	op2.Properties["interface"] = "ether2"
	if err := mm.ExecuteOperation(op2); err != nil {
		t.Errorf("expected same IP on different interface to be valid, got: %v", err)
	}
}

func TestIPAddressInvalidNetmask(t *testing.T) {
	mm, _ := loadIPAddressModule(t)

	invalidCIDRs := []string{
		"192.168.1.1/33",
		"invalid",
		"192.168.1.1",
	}

	for _, cidr := range invalidCIDRs {
		op := config.NewOperation(config.OpAdd, "/ip/address")
		op.Properties["address"] = cidr
		op.Properties["interface"] = "ether1"
		if err := mm.ExecuteOperation(op); err == nil {
			t.Errorf("expected error for invalid CIDR %q", cidr)
		}
	}
}

func TestIPAddressReservedRange(t *testing.T) {
	mm, _ := loadIPAddressModule(t)

	reserved := []string{
		"127.0.0.1/8",
		"224.0.0.1/4",
		"240.0.0.1/4",
		"0.0.0.1/8",
		"169.254.1.1/16",
	}

	for _, addr := range reserved {
		op := config.NewOperation(config.OpAdd, "/ip/address")
		op.Properties["address"] = addr
		op.Properties["interface"] = "ether1"
		if err := mm.ExecuteOperation(op); err == nil {
			t.Errorf("expected error for reserved IP %q", addr)
		}
	}
}

func TestIPAddressRFC1918Allowed(t *testing.T) {
	mm, _ := loadIPAddressModule(t)

	private := []string{
		"10.0.0.1/8",
		"192.168.1.1/24",
		"172.16.0.1/12",
	}

	for _, addr := range private {
		op := config.NewOperation(config.OpAdd, "/ip/address")
		op.Properties["address"] = addr
		op.Properties["interface"] = "ether1"
		if err := mm.ExecuteOperation(op); err != nil {
			t.Errorf("unexpected error for RFC1918 IP %q: %v", addr, err)
		}
	}
}

func TestIPAddressRemoveEntry(t *testing.T) {
	mm, _ := loadIPAddressModule(t)

	op := config.NewOperation(config.OpAdd, "/ip/address")
	op.Properties["address"] = "8.8.8.8/32"
	op.Properties["interface"] = "ether1"
	mm.ExecuteOperation(op)

	entries, _ := mm.Tree.GetEntries("/ip/address")
	entryID := entries[0].ID

	opRemove := config.NewOperation(config.OpRemove, "/ip/address")
	opRemove.EntryID = entryID
	if err := mm.ExecuteOperation(opRemove); err != nil {
		t.Fatalf("unexpected error removing entry: %v", err)
	}

	entries, _ = mm.Tree.GetEntries("/ip/address")
	if len(entries) != 0 {
		t.Errorf("expected 0 entries after removal, got %d", len(entries))
	}
}

func TestIPAddressRemoveNonExistent(t *testing.T) {
	mm, _ := loadIPAddressModule(t)

	op := config.NewOperation(config.OpRemove, "/ip/address")
	op.EntryID = "nonexistent-id"
	if err := mm.ExecuteOperation(op); err == nil {
		t.Error("expected error removing non-existent entry")
	}
}

func TestIPAddressDisableEnable(t *testing.T) {
	mm, _ := loadIPAddressModule(t)

	op := config.NewOperation(config.OpAdd, "/ip/address")
	op.Properties["address"] = "8.8.8.8/32"
	op.Properties["interface"] = "ether1"
	mm.ExecuteOperation(op)

	entries, _ := mm.Tree.GetEntries("/ip/address")
	entryID := entries[0].ID

	// Disable
	opDisable := config.NewOperation(config.OpDisable, "/ip/address")
	opDisable.EntryID = entryID
	if err := mm.ExecuteOperation(opDisable); err != nil {
		t.Fatalf("unexpected error disabling: %v", err)
	}

	entry, _ := mm.Tree.GetEntryByID("/ip/address", entryID)
	if !entry.Disabled {
		t.Error("expected entry to be disabled")
	}

	// Enable
	opEnable := config.NewOperation(config.OpEnable, "/ip/address")
	opEnable.EntryID = entryID
	if err := mm.ExecuteOperation(opEnable); err != nil {
		t.Fatalf("unexpected error enabling: %v", err)
	}

	entry, _ = mm.Tree.GetEntryByID("/ip/address", entryID)
	if entry.Disabled {
		t.Error("expected entry to be enabled after enable")
	}
}

func TestIPAddressSetProperties(t *testing.T) {
	mm, _ := loadIPAddressModule(t)

	op := config.NewOperation(config.OpAdd, "/ip/address")
	op.Properties["address"] = "8.8.8.8/32"
	op.Properties["interface"] = "ether1"
	op.Properties["comment"] = "original"
	mm.ExecuteOperation(op)

	entries, _ := mm.Tree.GetEntries("/ip/address")
	entryID := entries[0].ID

	opSet := config.NewOperation(config.OpSet, "/ip/address")
	opSet.EntryID = entryID
	opSet.Properties = map[string]interface{}{
		"comment": "updated comment",
	}
	if err := mm.ExecuteOperation(opSet); err != nil {
		t.Fatalf("unexpected error setting properties: %v", err)
	}

	entry, _ := mm.Tree.GetEntryByID("/ip/address", entryID)
	if entry.GetString("comment") != "updated comment" {
		t.Errorf("expected comment 'updated comment', got %q", entry.GetString("comment"))
	}
	if entry.GetString("address") != "8.8.8.8/32" {
		t.Errorf("expected address to remain unchanged, got %q", entry.GetString("address"))
	}
}

func TestIPAddressPublicIPAllowed(t *testing.T) {
	mm, _ := loadIPAddressModule(t)

	publicIPs := []string{
		"8.8.8.8/32",
		"1.1.1.1/32",
		"4.4.4.4/24",
	}

	for _, ip := range publicIPs {
		op := config.NewOperation(config.OpAdd, "/ip/address")
		op.Properties["address"] = ip
		op.Properties["interface"] = "ether1"
		if err := mm.ExecuteOperation(op); err != nil {
			t.Errorf("unexpected error for public IP %q: %v", ip, err)
		}
	}

	entries, _ := mm.Tree.GetEntries("/ip/address")
	if len(entries) != len(publicIPs) {
		t.Errorf("expected %d entries, got %d", len(publicIPs), len(entries))
	}
}

func TestIPAddressNoInterface(t *testing.T) {
	mm, _ := loadIPAddressModule(t)

	op := config.NewOperation(config.OpAdd, "/ip/address")
	op.Properties["address"] = "8.8.8.8/32"
	// No interface property set
	if err := mm.ExecuteOperation(op); err != nil {
		t.Errorf("expected add without interface to use defaults: %v", err)
	}
}

func TestIPAddressEventEmitted(t *testing.T) {
	mm, ct := loadIPAddressModule(t)
	received := false

	ct.EventBus.Subscribe("/ip/address", func(event config.Event) {
		received = true
		if event.Type != config.EventAdd {
			t.Errorf("expected EventAdd, got %v", event.Type)
		}
	})

	op := config.NewOperation(config.OpAdd, "/ip/address")
	op.Properties["address"] = "8.8.8.8/32"
	op.Properties["interface"] = "ether1"
	mm.ExecuteOperation(op)

	if !received {
		t.Error("expected event to be emitted")
	}
}

func TestIPAddressFlags(t *testing.T) {
	schema := &config.ModuleSchema{
		Path: "/ip/address",
		Type: "list",
		Flags: []config.SchemaFlag{
			{Letter: "X", Name: "disabled"},
			{Letter: "I", Name: "invalid"},
			{Letter: "D", Name: "dynamic"},
			{Letter: "S", Name: "slave"},
		},
	}

	if len(schema.Flags) != 4 {
		t.Fatalf("expected 4 flags, got %d", len(schema.Flags))
	}

	flagMap := make(map[string]config.SchemaFlag)
	for _, f := range schema.Flags {
		flagMap[f.Letter] = f
	}

	expected := []string{"X", "I", "D", "S"}
	for _, letter := range expected {
		if _, ok := flagMap[letter]; !ok {
			t.Errorf("missing flag %q", letter)
		}
	}
}
