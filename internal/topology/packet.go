package topology

// Packet represents a minimal network packet for the MVP simulator.
// It does not include full Ethernet/IP headers, only the essential fields
// needed for packet routing between simulated devices.
type Packet struct {
	// SrcIP is the source IP address (e.g., "192.168.1.1")
	SrcIP string
	// DstIP is the destination IP address (e.g., "192.168.1.2")
	DstIP string
	// Protocol is the IP protocol number (1 = ICMP, 6 = TCP, 17 = UDP)
	Protocol int
	// Payload contains the packet payload (e.g., ICMP echo data)
	Payload []byte
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
