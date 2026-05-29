package topology

import (
	"fmt"
	"log"
	"sync"

	"github.com/Devaste/MikroLab/internal/tree"
)

// FirewallEvaluator defines the interface for firewall rule evaluation.
// The filter module implements this to check packets against firewall rules.
type FirewallEvaluator interface {
	Evaluate(deviceID, chain string, packet Packet) (action string, logMsg string)
}

// BridgeHandler defines the interface for bridge packet processing.
// The bridge module implements this to handle L2 forwarding.
type BridgeHandler interface {
	// GetBridgeByPort returns the bridge name that contains the given port.
	GetBridgeByPort(portName string) (bridgeName string, found bool)

	// HandlePacket processes a packet arriving on a bridged interface.
	HandlePacket(bridgeName string, inPort string, packet Packet) []ForwardAction

	// GetBridgePorts returns all port names for a bridge.
	GetBridgePorts(bridgeName string) []string

	// AddMAC adds/updates a MAC address in the bridge's forwarding table.
	AddMAC(bridgeName, mac, portName string)
}

// ForwardAction represents a forwarding decision from the bridge.
type ForwardAction struct {
	OutIface string
}

// Topology manages a collection of simulated RouterOS devices and their
// virtual interconnections (links).
type Topology struct {
	devices           map[string]*Device
	links             map[string]*Link // link ID -> Link
	mu                sync.RWMutex
	nextID            int
	bridgeHandler     BridgeHandler
	firewallEvaluator FirewallEvaluator
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

// SetBridgeHandler sets the bridge handler for L2 forwarding.
func (t *Topology) SetBridgeHandler(h BridgeHandler) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.bridgeHandler = h
}

// SetFirewallEvaluator sets the firewall evaluator for packet filtering.
func (t *Topology) SetFirewallEvaluator(f FirewallEvaluator) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.firewallEvaluator = f
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

// CreateDeviceWithID creates a device with a specific ID.
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

// findLinkForDevice finds the link that connects this device+interface to another device.
func (t *Topology) findLinkForDevice(srcDeviceID, outIface string) (targetDeviceID, recvIface string) {
	for _, link := range t.links {
		if link.DeviceA == srcDeviceID && link.InterfaceA == outIface {
			return link.DeviceB, link.InterfaceB
		}
		if link.DeviceB == srcDeviceID && link.InterfaceB == outIface {
			return link.DeviceA, link.InterfaceA
		}
	}
	return "", ""
}

// routePacket forwards a packet directly to the connected device without
// re-processing it through ICMP echo generation. This prevents infinite loops.
func (t *Topology) routePacket(srcDeviceID, outIface string, packet Packet) error {
	t.mu.RLock()
	targetDeviceID, recvIface := t.findLinkForDevice(srcDeviceID, outIface)
	t.mu.RUnlock()

	if targetDeviceID == "" {
		return fmt.Errorf("no link found for device %s interface %s", srcDeviceID, outIface)
	}

	t.mu.RLock()
	targetDevice, exists := t.devices[targetDeviceID]
	t.mu.RUnlock()

	if !exists {
		return fmt.Errorf("target device %s not found", targetDeviceID)
	}

	// Forward to the connected device (just log, no more ICMP processing)
	log.Printf("[topology] Forwarding packet to %s (%s) on interface %s: %s -> %s (proto=%d)",
		targetDevice.Name, targetDevice.ID, recvIface, packet.SrcIP, packet.DstIP, packet.Protocol)

	// Perform MAC learning on the receiving interface if bridge handler is active
	t.mu.RLock()
	bridgeHandler := t.bridgeHandler
	t.mu.RUnlock()
	if bridgeHandler != nil {
		if packet.Eth.SrcMAC != "" {
			if bridgeName, found := bridgeHandler.GetBridgeByPort(recvIface); found {
				bridgeHandler.AddMAC(bridgeName, packet.Eth.SrcMAC, recvIface)
			}
		}
	}

	return nil
}

// SendPacket sends a packet from a device out through one of its interfaces.
func (t *Topology) SendPacket(srcDeviceID, outIface string, packet Packet) error {
	t.mu.RLock()
	bridgeHandler := t.bridgeHandler
	t.mu.RUnlock()

	// Check if this interface belongs to a bridge
	if bridgeHandler != nil {
		if bridgeName, found := bridgeHandler.GetBridgeByPort(outIface); found {
			// The bridge handler processes the packet
			actions := bridgeHandler.HandlePacket(bridgeName, outIface, packet)

			// Forward to all determined output interfaces
			for _, action := range actions {
				t.mu.RLock()
				targetDeviceID, recvIface := t.findLinkForDevice(srcDeviceID, action.OutIface)
				t.mu.RUnlock()

				if targetDeviceID == "" {
					log.Printf("[topology] No link found for bridge port %s on device %s",
						action.OutIface, srcDeviceID)
					continue
				}

				t.mu.RLock()
				targetDevice, exists := t.devices[targetDeviceID]
				t.mu.RUnlock()

				if !exists {
					log.Printf("[topology] Target device %s not found", targetDeviceID)
					continue
				}

				// Apply firewall on forward chain (traversing the bridge)
				packet.OutIface = action.OutIface
				packet.InIface = outIface
				if !t.applyFirewall(srcDeviceID, ChainForward, &packet) {
					continue
				}

				// Route the packet directly to the target device
				log.Printf("[topology] Bridge forwarding packet to %s (%s) on interface %s: %s -> %s (proto=%d)",
					targetDevice.Name, targetDevice.ID, recvIface, packet.SrcIP, packet.DstIP, packet.Protocol)

				// Perform MAC learning on the receiving side
				t.mu.RLock()
				bridgeHandler2 := t.bridgeHandler
				t.mu.RUnlock()
				if bridgeHandler2 != nil {
					if packet.Eth.SrcMAC != "" {
						if bridgeName2, found := bridgeHandler2.GetBridgeByPort(recvIface); found {
							bridgeHandler2.AddMAC(bridgeName2, packet.Eth.SrcMAC, recvIface)
						}
					}
				}
			}
			return nil
		}
	}

	// Normal (non-bridge) forwarding
	t.mu.RLock()
	targetDeviceID, recvIface := t.findLinkForDevice(srcDeviceID, outIface)
	t.mu.RUnlock()

	if targetDeviceID == "" {
		return fmt.Errorf("no link found for device %s interface %s", srcDeviceID, outIface)
	}

	t.mu.RLock()
	targetDevice, exists := t.devices[targetDeviceID]
	t.mu.RUnlock()

	if !exists {
		return fmt.Errorf("target device %s not found", targetDeviceID)
	}

	// Apply firewall on output chain for packet leaving the device
	packet.OutIface = outIface
	if !t.applyFirewall(srcDeviceID, ChainOutput, &packet) {
		return nil
	}

	// Apply routing logic (only process ICMP echo requests once)
	t.handleICMPPacket(targetDevice, recvIface, packet)
	return nil
}

// handleICMPPacket processes an ICMP echo request and generates a single reply.
func (t *Topology) handleICMPPacket(device *Device, recvIface string, packet Packet) {
	log.Printf("[topology] Delivering packet to %s (%s) on interface %s: %s -> %s (proto=%d)",
		device.Name, device.ID, recvIface, packet.SrcIP, packet.DstIP, packet.Protocol)

	// Perform MAC learning
	t.mu.RLock()
	bridgeHandler := t.bridgeHandler
	t.mu.RUnlock()

	if bridgeHandler != nil {
		if packet.Eth.SrcMAC != "" {
			if bridgeName, found := bridgeHandler.GetBridgeByPort(recvIface); found {
				bridgeHandler.AddMAC(bridgeName, packet.Eth.SrcMAC, recvIface)
			}
		}
	}

	// Apply firewall on input chain for packet arriving at the device
	packet.InIface = recvIface
	if !t.applyFirewall(device.ID, ChainInput, &packet) {
		return
	}

	// Only generate a reply for ICMP Echo Requests (type 8), not replies (type 0)
	if packet.Protocol == 1 && len(packet.Payload) > 0 && packet.Payload[0] == 8 {
		reply := Packet{
			Eth: EthernetFrame{
				SrcMAC:    packet.Eth.DstMAC,
				DstMAC:    packet.Eth.SrcMAC,
				EtherType: packet.Eth.EtherType,
				Payload:   packet.Eth.Payload,
			},
			SrcIP:    packet.DstIP,
			DstIP:    packet.SrcIP,
			Protocol: 1,
			Payload:  createICMPReply(packet.Payload),
		}

		log.Printf("[topology] Generated ICMP reply: %s -> %s", reply.SrcIP, reply.DstIP)

		// Apply firewall on output chain for the reply
		reply.OutIface = recvIface
		if !t.applyFirewall(device.ID, ChainOutput, &reply) {
			return
		}

		// Send reply back directly (no further ICMP processing)
		err := t.routePacketBack(device.ID, recvIface, reply)
		if err != nil {
			log.Printf("[topology] Failed to send ICMP reply: %v", err)
		}
	}
}

// routePacketBack sends the reply packet back directly.
func (t *Topology) routePacketBack(deviceID, inIface string, reply Packet) error {
	for _, link := range t.links {
		if link.DeviceA == deviceID && link.InterfaceA == inIface {
			log.Printf("[topology] Routing reply from %s:%s to %s:%s",
				deviceID, inIface, link.DeviceB, link.InterfaceB)
			return t.routePacket(deviceID, inIface, reply)
		}
		if link.DeviceB == deviceID && link.InterfaceB == inIface {
			log.Printf("[topology] Routing reply from %s:%s to %s:%s",
				deviceID, inIface, link.DeviceA, link.InterfaceA)
			return t.routePacket(deviceID, inIface, reply)
		}
	}
	return fmt.Errorf("no link found for reply on device %s interface %s", deviceID, inIface)
}

// createICMPReply creates a dummy ICMP echo reply payload from an echo request.
func createICMPReply(requestPayload []byte) []byte {
	reply := make([]byte, len(requestPayload))
	copy(reply, requestPayload)
	if len(reply) > 0 {
		reply[0] = 0 // ICMP Echo Reply type
	}
	return reply
}

// Chain constants for firewall evaluation
const (
	ChainInput   = "input"
	ChainForward = "forward"
	ChainOutput  = "output"
)

// applyFirewall evaluates the packet against firewall rules for the given chain.
// Returns true if the packet should be accepted, false if dropped.
// If the action is "log", the log message is recorded.
func (t *Topology) applyFirewall(deviceID, chain string, packet *Packet) bool {
	t.mu.RLock()
	fw := t.firewallEvaluator
	t.mu.RUnlock()

	if fw == nil {
		return true // no firewall configured - accept
	}

	action, logMsg := fw.Evaluate(deviceID, chain, *packet)

	switch action {
	case "drop":
		log.Printf("[firewall] DROP: device=%s chain=%s %s -> %s (proto=%d)",
			deviceID, chain, packet.SrcIP, packet.DstIP, packet.Protocol)
		return false
	case "log":
		log.Printf("[firewall] LOG: %s", logMsg)
		return true // log and continue
	default:
		return true // accept
	}
}

// NewTreeForDevice creates a new empty configuration tree suitable for a device.
func NewTreeForDevice() *tree.TreeNode {
	return tree.NewTree()
}
