package topology

// EthernetFrame represents a minimal Ethernet frame used for bridge forwarding.
type EthernetFrame struct {
	// SrcMAC is the source MAC address (e.g., "00:11:22:33:44:55")
	SrcMAC string
	// DstMAC is the destination MAC address (e.g., "00:11:22:33:44:66")
	DstMAC string
	// EtherType is the Ethernet type field (e.g., 0x0800 for IPv4, 0x0806 for ARP)
	EtherType uint16
	// Payload contains the raw frame payload
	Payload []byte
}

// Packet represents a minimal network packet for the MVP simulator.
// It does not include full Ethernet/IP headers, only the essential fields
// needed for packet routing between simulated devices.
type Packet struct {
	// Eth contains the Ethernet frame information
	Eth EthernetFrame
	// SrcIP is the source IP address (e.g., "192.168.1.1")
	SrcIP string
	// DstIP is the destination IP address (e.g., "192.168.1.2")
	DstIP string
	// Protocol is the IP protocol number (1 = ICMP, 6 = TCP, 17 = UDP)
	Protocol int
	// Payload contains the packet payload (e.g., ICMP echo data)
	Payload []byte
}

// Common Ethernet types
const (
	EtherTypeIPv4 = 0x0800
	EtherTypeARP  = 0x0806
	EtherTypeIPv6 = 0x86DD
)

// BroadcastMAC is the Ethernet broadcast address
const BroadcastMAC = "FF:FF:FF:FF:FF:FF"

// IsBroadcast checks if the destination MAC is the broadcast address.
func (f EthernetFrame) IsBroadcast() bool {
	return f.DstMAC == BroadcastMAC
}

// IsMulticast checks if the destination MAC is a multicast address.
func (f EthernetFrame) IsMulticast() bool {
	return len(f.DstMAC) > 1 && (f.DstMAC[0] == '0' || f.DstMAC[0] == '1')
}

// NewICMPEchoRequest creates a new ICMP echo request packet.
func NewICMPEchoRequest(srcIP, dstIP string, payload []byte) Packet {
	return Packet{
		SrcIP:    srcIP,
		DstIP:    dstIP,
		Protocol: 1, // ICMP
		Payload:  payload,
	}
}

// NewICMPEchoReply creates a new ICMP echo reply packet.
func NewICMPEchoReply(srcIP, dstIP string, payload []byte) Packet {
	return Packet{
		SrcIP:    srcIP,
		DstIP:    dstIP,
		Protocol: 1, // ICMP
		Payload:  payload,
	}
}
