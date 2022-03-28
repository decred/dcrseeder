// Copyright (c) 2021 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"net"
	"testing"
)

func Test_IsRoutable(t *testing.T) {
	ipTests := map[string]struct {
		ip               string
		expectedRoutable bool
	}{

		// IP v4
		"ip4 public": {
			"8.8.8.8",
			true,
		},

		"ip4 unspecified": {
			"0.0.0.0",
			false,
		},
		"ip4 loopback": {
			"127.0.0.1",
			false,
		},

		// RFC1918
		"ip4 inside RFC1918 (10)": {
			"10.0.0.2",
			false,
		},
		"ip4 inside RFC1918 (172)": {
			"172.16.0.2",
			false,
		},
		"ip4 outside RFC1918 (172)": {
			"172.32.0.1",
			true,
		},
		"ip4 inside RFC1918 (192)": {
			"192.168.1.2",
			false,
		},
		"ip4 outside RFC1918 (192)": {
			"192.169.0.1",
			true,
		},

		// IP v6

		"ip6 public": {
			"2001:4860:4860::8888",
			true,
		},
		"ip6 unspecified": {
			"::",
			false,
		},
		"ip6 loopback": {
			"::1",
			false,
		},

		// RFC3964
		"ip6 start RFC3964": {
			"2002:0000:0000:0000:0000:0000:0000:0000",
			false,
		},
		"ip6 end RFC3964": {
			"2002:ffff:ffff:ffff:ffff:ffff:ffff:ffff",
			false,
		},

		// RFC4380
		"ip6 start RFC4380": {
			"2001:0000:0000:0000:0000:0000:0000:0000",
			false,
		},
		"ip6 end RFC4380": {
			"2001:0000:ffff:ffff:ffff:ffff:ffff:ffff",
			false,
		},
		"ip6 outside RFC4380": {
			"2001:0001:0000:0000:0000:0000:0000:0000",
			true,
		},

		// RFC4843
		"ip6 start RFC4843": {
			"2001:0010:0000:0000:0000:0000:0000:0000",
			false,
		},
		"ip6 end RFC4843": {
			"2001:001f:ffff:ffff:ffff:ffff:ffff:ffff",
			false,
		},
		"ip6 outside RFC4843": {
			"2001:0020:0000:0000:0000:0000:0000:0000",
			true,
		},

		// RFC4862
		"ip6 start RFC4862": {
			"fe80:0000:0000:0000:0000:0000:0000:0000",
			false,
		},
		"ip6 end RFC4862": {
			"fe80:0000:0000:0000:ffff:ffff:ffff:ffff",
			false,
		},
		"ip6 outside RFC4862": {
			"fe80:0000:0000:0001:0000:0000:0000:0000",
			true,
		},

		// RFC4193
		"ip6 start RFC4193": {
			"fc00:0000:0000:0000:0000:0000:0000:0000",
			false,
		},
		"ip6 end RFC4193": {
			"fdff:ffff:ffff:ffff:ffff:ffff:ffff:ffff",
			false,
		},
		"ip6 outside RFC4193": {
			"fe00:0000:0000:0000:0000:0000:0000:0000",
			true,
		},
	}

	for testName, test := range ipTests {
		actualRoutable := isRoutable(net.ParseIP(test.ip))
		if actualRoutable != test.expectedRoutable {
			t.Fatalf("%s: expected routable==%t for IP %s",
				testName, test.expectedRoutable, test.ip)
		}
	}
}
