package topology

import (
	"fmt"
	"log"
	"sync"

	"github.com/Devaste/MikroLab/internal/tree"
)

// Topology manages a collection of simulated RouterOS devices and their
// virtual interconnections (links).
type Topology struct {
	devices map[string]*Device
	links   map[string]*Link // link ID -> Link
	mu      sync.RWMutex
	nextID  int
}

// Link represents a virtual cable connecting two interfaces on two devices.
type Link struct {
	ID         string
	DeviceA    string // device ID
	InterfaceA string // interface name on device A
	DeviceB    string
	InterfaceB string
}

// NewTopology creates a new empty topology manager.
func NewTopology() *Topology {
	return &Topology{
		devices: make(map[string]*Device),
		links:   make(map[string]*Link),
		nextID:  1,
	}
}

// Device returns the device with the given ID, or nil if not found.
func (t *Topology) Device(id string) *Device {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.devices[id]
}

// Devices returns a copy of the devices map (for iteration).
func (t *Topology) Devices() map[string]*Device {
	t.mu.RLock()
	defer t.mu.RUnlock()
	devices := make(map[string]*Device, len(t.devices))
	for k, v := range t.devices {
		devices[k] = v
	}
	return devices
}

// Links returns a copy of the links map.
func (t *Topology) Links() map[string]*Link {
	t.mu.RLock()
	defer t.mu.RUnlock()
	links := make(map[string]*Link, len(t.links))
	for k, v := range t.links {
		links[k] = v
	}
	return links
}

// CreateDevice creates a new device with the given name and adds it to the topology.
func (t *Topology) CreateDevice(name string) (*Device, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Generate a unique ID
	id := fmt.Sprintf("device-%d", t.nextID)
	t.nextID++

	device, err := NewDevice(id, name, t)
	if err != nil {
		return nil, fmt.Errorf("failed to create device: %w", err)
	}

	t.devices[id] = device
	log.Printf("[topology] Created device: %s (%s)", id, name)
	return device, nil
}

// CreateDeviceWithID creates a device with a specific ID (used for the default device).
func (t *Topology) CreateDeviceWithID(id, name string) (*Device, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if _, exists := t.devices[id]; exists {
		return nil, fmt.Errorf("device %q already exists", id)
	}

	device, err := NewDevice(id, name, t)
	if err != nil {
		return nil, fmt.Errorf("failed to create device: %w", err)
	}

	t.devices[id] = device
	log.Printf("[topology] Created device: %s (%s) with ID %s", id, name, id)
	return device, nil
}

// DeleteDevice removes a device and all its links from the topology.
func (t *Topology) DeleteDevice(id string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if _, exists := t.devices[id]; !exists {
		return fmt.Errorf("device %q not found", id)
	}

	// Remove all links involving this device
	for linkID, link := range t.links {
		if link.DeviceA == id || link.DeviceB == id {
			delete(t.links, linkID)
			log.Printf("[topology] Removed link %s (device %s removed)", linkID, id)
		}
	}

	delete(t.devices, id)
	log.Printf("[topology] Deleted device: %s", id)
	return nil
}

// Connect creates a virtual cable between two interfaces on two devices.
func (t *Topology) Connect(deviceA, ifaceA, deviceB, ifaceB string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Verify devices exist
	devA, okA := t.devices[deviceA]
	devB, okB := t.devices[deviceB]
	if !okA {
		return fmt.Errorf("device %q not found", deviceA)
	}
	if !okB {
		return fmt.Errorf("device %q not found", deviceB)
	}
	if deviceA == deviceB {
		return fmt.Errorf("cannot connect a device to itself")
	}
	if ifaceA == "" || ifaceB == "" {
		return fmt.Errorf("interface names are required")
	}

	// Check no existing link uses these interfaces
	for _, link := range t.links {
		if (link.DeviceA == deviceA && link.InterfaceA == ifaceA) ||
			(link.DeviceB == deviceA && link.InterfaceB == ifaceA) {
			return fmt.Errorf("interface %s on device %s is already connected", ifaceA, deviceA)
		}
		if (link.DeviceA == deviceB && link.InterfaceA == ifaceB) ||
			(link.DeviceB == deviceB && link.InterfaceB == ifaceB) {
			return fmt.Errorf("interface %s on device %s is already connected", ifaceB, deviceB)
		}
	}

	linkID := fmt.Sprintf("link-%d", len(t.links)+1)
	link := &Link{
		ID:         linkID,
		DeviceA:    deviceA,
		InterfaceA: ifaceA,
		DeviceB:    deviceB,
		InterfaceB: ifaceB,
	}
	t.links[linkID] = link

	log.Printf("[topology] Connected %s:%s <-> %s:%s (link %s)",
		devA.Name, ifaceA, devB.Name, ifaceB, linkID)
	return nil
}

// Disconnect removes a link by its ID.
func (t *Topology) Disconnect(linkID string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if _, exists := t.links[linkID]; !exists {
		return fmt.Errorf("link %q not found", linkID)
	}

	delete(t.links, linkID)
	log.Printf("[topology] Disconnected link %s", linkID)
	return nil
}

// SendPacket sends a packet from a device out through one of its interfaces.
// The topology manager routes it to the connected device.
func (t *Topology) SendPacket(srcDeviceID, outIface string, packet Packet) error {
	t.mu.RLock()
	defer t.mu.RUnlock()

	// Find which link connects this device+interface to another
	var targetDeviceID, recvIface string
	for _, link := range t.links {
		if link.DeviceA == srcDeviceID && link.InterfaceA == outIface {
			targetDeviceID = link.DeviceB
			recvIface = link.InterfaceB
			break
		}
		if link.DeviceB == srcDeviceID && link.InterfaceB == outIface {
			targetDeviceID = link.DeviceA
			recvIface = link.InterfaceA
			break
		}
	}

	if targetDeviceID == "" {
		return fmt.Errorf("no link found for device %s interface %s", srcDeviceID, outIface)
	}

	// Deliver the packet to the target device
	targetDevice, exists := t.devices[targetDeviceID]
	if !exists {
		return fmt.Errorf("target device %s not found", targetDeviceID)
	}

	// Apply packet routing logic
	t.deliverPacket(targetDevice, recvIface, packet)
	return nil
}

// deliverPacket delivers a packet to a device on the specified interface.
// For MVP, it handles ICMP echo requests by generating a reply.
func (t *Topology) deliverPacket(device *Device, recvIface string, packet Packet) {
	log.Printf("[topology] Delivering packet to %s (%s) on interface %s: %s -> %s (proto=%d)",
		device.Name, device.ID, recvIface, packet.SrcIP, packet.DstIP, packet.Protocol)

	// For MVP: handle ICMP echo requests (protocol 1)
	if packet.Protocol == 1 {
		// This is an ICMP packet - generate an echo reply
		reply := Packet{
			SrcIP:    packet.DstIP,
			DstIP:    packet.SrcIP,
			Protocol: 1,
			Payload:  createICMPReply(packet.Payload),
		}
		log.Printf("[topology] Generated ICMP reply: %s -> %s", reply.SrcIP, reply.DstIP)

		// Send reply back through the topology
		// We need to find the outgoing interface on the target device
		// For simplicity, we look up the interface that matches our routing
		err := t.sendReplyBack(device.ID, recvIface, reply)
		if err != nil {
			log.Printf("[topology] Failed to send ICMP reply: %v", err)
		}
	}
}

// sendReplyBack sends the reply packet back through the topology.
func (t *Topology) sendReplyBack(deviceID, inIface string, reply Packet) error {
	// The reply needs to go out on the same interface it came in on
	// Find the link on the other side
	for _, link := range t.links {
		if link.DeviceA == deviceID && link.InterfaceA == inIface {
			log.Printf("[topology] Routing reply from %s:%s to %s:%s",
				deviceID, inIface, link.DeviceB, link.InterfaceB)
			return nil
		}
		if link.DeviceB == deviceID && link.InterfaceB == inIface {
			log.Printf("[topology] Routing reply from %s:%s to %s:%s",
				deviceID, inIface, link.DeviceA, link.InterfaceA)
			return nil
		}
	}
	return fmt.Errorf("no link found for reply on device %s interface %s", deviceID, inIface)
}

// createICMPReply creates a dummy ICMP echo reply payload from an echo request.
func createICMPReply(requestPayload []byte) []byte {
	// For MVP, just return a small reply marker
	reply := make([]byte, len(requestPayload))
	copy(reply, requestPayload)
	// Flip the first byte to mark as reply
	if len(reply) > 0 {
		reply[0] = 0 // ICMP Echo Reply type
	}
	return reply
}

// NewTreeForDevice creates a new empty configuration tree suitable for a device.
// The caller is responsible for populating it with modules.
func NewTreeForDevice() *tree.TreeNode {
	return tree.NewTree()
}
