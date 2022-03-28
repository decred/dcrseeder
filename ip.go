// Copyright (c) 2018-2021 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import "net"

var (
	// rfc3964Net specifies the IPv6 to IPv4 encapsulation address block as
	// defined by RFC3964 (2002::/16).
	rfc3964Net = ipNet("2002::", 16)

	// rfc4380Net specifies the IPv6 teredo tunneling over UDP address block
	// as defined by RFC4380 (2001::/32).
	rfc4380Net = ipNet("2001::", 32)

	// rfc4843Net specifies the IPv6 ORCHID address block as defined by
	// RFC4843 (2001:10::/28).
	rfc4843Net = ipNet("2001:10::", 28)

	// rfc4862Net specifies the IPv6 stateless address autoconfiguration
	// address block as defined by RFC4862 (FE80::/64).
	rfc4862Net = ipNet("FE80::", 64)
)

// ipNet returns a net.IPNet struct given the passed IP address string, number
// of one bits to include at the start of the mask, and the total number of bits
// for the mask.
func ipNet(ip string, ones int) net.IPNet {
	const bits = 128
	return net.IPNet{IP: net.ParseIP(ip), Mask: net.CIDRMask(ones, bits)}
}

func isRoutable(addr net.IP) bool {
	if addr.IsLoopback() {
		return false
	}

	if addr.IsUnspecified() {
		return false
	}

	if addr.IsPrivate() {
		return false
	}

	if rfc3964Net.Contains(addr) ||
		rfc4380Net.Contains(addr) ||
		rfc4843Net.Contains(addr) ||
		rfc4862Net.Contains(addr) {
		return false
	}

	return true
}
