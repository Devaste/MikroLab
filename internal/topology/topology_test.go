package topology

import (
	"testing"
)

// TestNewTopology verifies that a new topology is created empty.
func TestNewTopology(t *testing.T) {
	topo := NewTopology()
	if topo == nil {
		t.Fatal("NewTopology() returned nil")
	}
	if len(topo.Devices()) != 0 {
		t.Errorf("expected 0 devices, got %d", len(topo.Devices()))
	}
	if len(topo.Links()) != 0 {
		t.Errorf("expected 0 links, got %d", len(topo.Links()))
	}
}

// TestCreateDevice verifies device creation.
func TestCreateDevice(t *testing.T) {
	topo := NewTopology()

	dev, err := topo.CreateDevice("Router1")
	if err != nil {
		t.Fatalf("CreateDevice failed: %v", err)
	}
	if dev == nil {
		t.Fatal("CreateDevice returned nil device")
	}
	if dev.Name != "Router1" {
		t.Errorf("expected name Router1, got %s", dev.Name)
	}
	if dev.ID == "" {
		t.Errorf("expected non-empty ID")
	}

	// Verify it's in the topology
	if len(topo.Devices()) != 1 {
		t.Errorf("expected 1 device, got %d", len(topo.Devices()))
	}
}

// TestCreateDeviceWithID verifies creating a device with a specific ID.
func TestCreateDeviceWithID(t *testing.T) {
	topo := NewTopology()

	dev, err := topo.CreateDeviceWithID("my-device", "MyRouter")
	if err != nil {
		t.Fatalf("CreateDeviceWithID failed: %v", err)
	}
	if dev.ID != "my-device" {
		t.Errorf("expected ID my-device, got %s", dev.ID)
	}
	if dev.Name != "MyRouter" {
		t.Errorf("expected name MyRouter, got %s", dev.Name)
	}
}

// TestCreateDuplicateDevice verifies that duplicate IDs are rejected.
func TestCreateDuplicateDevice(t *testing.T) {
	topo := NewTopology()

	_, err := topo.CreateDeviceWithID("device-1", "Router1")
	if err != nil {
		t.Fatalf("First creation failed: %v", err)
	}

	_, err = topo.CreateDeviceWithID("device-1", "Router2")
	if err == nil {
		t.Fatal("Expected error for duplicate device ID, got nil")
	}
}

// TestDeleteDevice verifies device deletion.
func TestDeleteDevice(t *testing.T) {
	topo := NewTopology()

	dev, err := topo.CreateDevice("Router1")
	if err != nil {
		t.Fatalf("CreateDevice failed: %v", err)
	}

	// Delete the device
	err = topo.DeleteDevice(dev.ID)
	if err != nil {
		t.Fatalf("DeleteDevice failed: %v", err)
	}

	if len(topo.Devices()) != 0 {
		t.Errorf("expected 0 devices after deletion, got %d", len(topo.Devices()))
	}

	// Deleting non-existent device should fail
	err = topo.DeleteDevice("nonexistent")
	if err == nil {
		t.Fatal("Expected error for deleting non-existent device, got nil")
	}
}

// TestConnectDevices verifies connecting two devices.
func TestConnectDevices(t *testing.T) {
	topo := NewTopology()

	devA, err := topo.CreateDevice("RouterA")
	if err != nil {
		t.Fatalf("CreateDevice failed: %v", err)
	}
	devB, err := topo.CreateDevice("RouterB")
	if err != nil {
		t.Fatalf("CreateDevice failed: %v", err)
	}

	err = topo.Connect(devA.ID, "ether1", devB.ID, "ether1")
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	if len(topo.Links()) != 1 {
		t.Errorf("expected 1 link, got %d", len(topo.Links()))
	}
}

// TestConnectInvalidDevice verifies connecting non-existent devices fails.
func TestConnectInvalidDevice(t *testing.T) {
	topo := NewTopology()

	err := topo.Connect("nonexistent", "ether1", "other", "ether1")
	if err == nil {
		t.Fatal("Expected error for connecting non-existent device, got nil")
	}
}

// TestConnectSelf verifies connecting a device to itself fails.
func TestConnectSelf(t *testing.T) {
	topo := NewTopology()

	dev, err := topo.CreateDevice("RouterA")
	if err != nil {
		t.Fatalf("CreateDevice failed: %v", err)
	}

	err = topo.Connect(dev.ID, "ether1", dev.ID, "ether2")
	if err == nil {
		t.Fatal("Expected error for self-connect, got nil")
	}
}

// TestConnectSameInterface verifies connecting the same interface twice fails.
func TestConnectSameInterface(t *testing.T) {
	topo := NewTopology()

	devA, _ := topo.CreateDevice("RouterA")
	devB, _ := topo.CreateDevice("RouterB")
	devC, _ := topo.CreateDevice("RouterC")

	// First connection should succeed
	err := topo.Connect(devA.ID, "ether1", devB.ID, "ether1")
	if err != nil {
		t.Fatalf("First connect failed: %v", err)
	}

	// Second connection using the same interface on devA should fail
	err = topo.Connect(devA.ID, "ether1", devC.ID, "ether1")
	if err == nil {
		t.Fatal("Expected error for re-using interface, got nil")
	}
}

// TestDisconnect verifies disconnecting a link.
func TestDisconnect(t *testing.T) {
	topo := NewTopology()

	devA, _ := topo.CreateDevice("RouterA")
	devB, _ := topo.CreateDevice("RouterB")

	err := topo.Connect(devA.ID, "ether1", devB.ID, "ether1")
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	// Get the link ID
	var linkID string
	for id := range topo.Links() {
		linkID = id
		break
	}

	err = topo.Disconnect(linkID)
	if err != nil {
		t.Fatalf("Disconnect failed: %v", err)
	}

	if len(topo.Links()) != 0 {
		t.Errorf("expected 0 links after disconnect, got %d", len(topo.Links()))
	}

	// Disconnecting non-existent link should fail
	err = topo.Disconnect("nonexistent")
	if err == nil {
		t.Fatal("Expected error for disconnecting non-existent link, got nil")
	}
}

// TestDeleteDeviceRemovesLinks verifies that deleting a device removes its links.
func TestDeleteDeviceRemovesLinks(t *testing.T) {
	topo := NewTopology()

	devA, _ := topo.CreateDevice("RouterA")
	devB, _ := topo.CreateDevice("RouterB")

	err := topo.Connect(devA.ID, "ether1", devB.ID, "ether1")
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	// Delete device A
	err = topo.DeleteDevice(devA.ID)
	if err != nil {
		t.Fatalf("DeleteDevice failed: %v", err)
	}

	// Links should be removed
	if len(topo.Links()) != 0 {
		t.Errorf("expected 0 links after device deletion, got %d", len(topo.Links()))
	}
}

// TestDeviceHasTree verifies each device has its own tree.
func TestDeviceHasTree(t *testing.T) {
	topo := NewTopology()

	devA, _ := topo.CreateDevice("RouterA")
	devB, _ := topo.CreateDevice("RouterB")

	if devA.Tree == nil {
		t.Fatal("Device A has nil tree")
	}
	if devB.Tree == nil {
		t.Fatal("Device B has nil tree")
	}

	// They should be different tree instances
	if devA.Tree == devB.Tree {
		t.Fatal("Devices should have different tree instances")
	}
}

// TestPacketCreation verifies Packet struct works correctly.
func TestPacketCreation(t *testing.T) {
	pkt := NewICMPEchoRequest("192.168.1.1", "192.168.1.2", []byte{8, 0, 0, 0})
	if pkt.SrcIP != "192.168.1.1" {
		t.Errorf("expected src 192.168.1.1, got %s", pkt.SrcIP)
	}
	if pkt.DstIP != "192.168.1.2" {
		t.Errorf("expected dst 192.168.1.2, got %s", pkt.DstIP)
	}
	if pkt.Protocol != 1 {
		t.Errorf("expected protocol 1 (ICMP), got %d", pkt.Protocol)
	}

	reply := NewICMPEchoReply("192.168.1.2", "192.168.1.1", []byte{0, 0, 0, 0})
	if reply.SrcIP != "192.168.1.2" {
		t.Errorf("expected src 192.168.1.2, got %s", reply.SrcIP)
	}
}

// TestSendPacket verifies sending a packet between connected devices.
func TestSendPacket(t *testing.T) {
	topo := NewTopology()

	devA, _ := topo.CreateDevice("RouterA")
	devB, _ := topo.CreateDevice("RouterB")

	err := topo.Connect(devA.ID, "ether1", devB.ID, "ether1")
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	// Send an ICMP request from A to B
	pkt := NewICMPEchoRequest("192.168.1.1", "192.168.1.2", []byte{8, 0, 0, 0})
	err = topo.SendPacket(devA.ID, "ether1", pkt)
	if err != nil {
		t.Fatalf("SendPacket failed: %v", err)
	}
}

// TestSendPacketNoLink verifies sending to a non-existent link fails.
func TestSendPacketNoLink(t *testing.T) {
	topo := NewTopology()

	dev, err := topo.CreateDevice("RouterA")
	if err != nil {
		t.Fatalf("CreateDevice failed: %v", err)
	}

	pkt := NewICMPEchoRequest("192.168.1.1", "192.168.1.2", nil)
	err = topo.SendPacket(dev.ID, "ether1", pkt)
	if err == nil {
		t.Fatal("Expected error for sending on non-connected interface, got nil")
	}
}
