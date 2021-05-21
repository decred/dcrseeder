// Copyright (c) 2018-2021 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

const (
	// GetAddrsPath is the URL path to fetch a list of public nodes
	GetAddrsPath = "/api/addrs"

	ipVersion       = "ipversion"
	serviceFlag     = "services"
	protocolVersion = "pver"
)

type apiNode struct {
	Host            string `json:"host"`
	Services        uint64 `json:"services"`
	ProtocolVersion uint32 `json:"pver"`
}
