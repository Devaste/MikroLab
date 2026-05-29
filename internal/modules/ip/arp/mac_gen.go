// Package ip_arp implements the /ip/arp settings directory for ARP table management.
package ip_arp

import (
	"fmt"
	"math/rand"
)

// MikroTik OUIs for MAC address generation
var (
	primaryOUI   = "64:D1:54" // most recent MikroTik hardware
	secondaryOUI = "00:0C:42" // older devices
	ouis         = []string{primaryOUI, secondaryOUI}
)

// GenerateMikroTikMAC returns a randomly generated MikroTik MAC address.
// It randomly picks one of the known MikroTik OUIs and appends 3 random
// bytes (6 hex digits).
func GenerateMikroTikMAC() string {
	oui := ouis[rand.Intn(len(ouis))]
	b1 := rand.Intn(256)
	b2 := rand.Intn(256)
	b3 := rand.Intn(256)
	return fmt.Sprintf("%s:%02X:%02X:%02X", oui, b1, b2, b3)
}
