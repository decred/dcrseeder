// Copyright (c) 2021 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"net"
	"testing"
)

func Test_IsRoutable(t *testing.T) {
	ipTests := []struct {
		ip               string
		expectedRoutable bool
	}{
		{
			"10.0.0.2",
			false,
		},
		{
			"172.16.0.2",
			false,
		},
		{
			"192.168.1.2",
			false,
		},
		{
			"8.8.8.8",
			true,
		},
	}

	for _, test := range ipTests {
		actualRoutable := isRoutable(net.ParseIP(test.ip))
		if actualRoutable != test.expectedRoutable {
			t.Fatalf("expected routable==%t for IP %s", test.expectedRoutable, test.ip)
		}
	}
}
