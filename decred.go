// Copyright (c) 2018-2023 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/decred/dcrd/chaincfg/v3"
	"github.com/decred/dcrd/peer/v3"
	"github.com/decred/dcrd/wire"
)

const (
	// defaultAddressTimeout defines the duration to wait for new addresses.
	defaultAddressTimeout = time.Minute * 10

	// defaultNodeTimeout defines the timeout on responses from a node.
	defaultNodeTimeout = time.Second * 3
)

var amgr *Manager

func testPeer(ctx context.Context, ip netip.AddrPort, netParams *chaincfg.Params) {
	onaddr := make(chan struct{}, 1)
	verack := make(chan struct{}, 1)
	config := peer.Config{
		UserAgentName:    appName,
		UserAgentVersion: "0.0.1",
		Net:              netParams.Net,
		DisableRelayTx:   true,

		Listeners: peer.MessageListeners{
			OnAddr: func(p *peer.Peer, msg *wire.MsgAddr) {
				n := make([]netip.AddrPort, 0, len(msg.AddrList))
				for _, entry := range msg.AddrList {
					if addr, ok := netip.AddrFromSlice(entry.IP); ok {
						addrPort := netip.AddrPortFrom(addr, entry.Port)
						n = append(n, addrPort)
					}
				}
				added := amgr.AddAddresses(n)
				log.Printf("Peer %v sent %v addresses, %d new",
					p.Addr(), len(msg.AddrList), added)
				onaddr <- struct{}{}
			},
			OnVerAck: func(p *peer.Peer, msg *wire.MsgVerAck) {
				log.Printf("Adding peer %v with services %v pver %d",
					p.NA().IP.String(), p.Services(), p.ProtocolVersion())
				verack <- struct{}{}
			},
		},
	}

	host := ip.String()
	p, err := peer.NewOutboundPeer(&config, host)
	if err != nil {
		log.Printf("NewOutboundPeer on %v: %v", host, err)
		return
	}

	// Time stamp the attempt after disconnect or dial error so we don't prune
	// this peer before or during its test.
	defer amgr.Attempt(ip)

	ctxTimeout, cancel := context.WithTimeout(ctx, defaultNodeTimeout)
	defer cancel()
	var dialer net.Dialer
	conn, err := dialer.DialContext(ctxTimeout, "tcp", p.Addr())
	if err != nil {
		return
	}
	p.AssociateConnection(conn)
	defer p.Disconnect()

	// Wait for the verack message or timeout in case of failure.
	select {
	case <-verack:
		// Mark this peer as a good node.
		amgr.Good(ip, p.Services(), p.ProtocolVersion())

		// Ask peer for some addresses.
		p.QueueMessage(wire.NewMsgGetAddr(), nil)

	case <-time.After(defaultNodeTimeout):
		log.Printf("verack timeout on peer %v", p.Addr())
		return
	case <-ctx.Done():
		return
	}

	select {
	case <-onaddr:
	case <-time.After(defaultNodeTimeout):
		log.Printf("getaddr timeout on peer %v", p.Addr())
	case <-ctx.Done():
	}
}

func creep(ctx context.Context, netParams *chaincfg.Params) {
	for {
		if ctx.Err() != nil {
			return
		}

		ips := amgr.Addresses()
		if len(ips) == 0 {
			log.Printf("No stale addresses -- sleeping for %v", defaultAddressTimeout)
			select {
			case <-time.After(defaultAddressTimeout):
			case <-ctx.Done():
				return
			}
			continue
		}

		var wg sync.WaitGroup
		wg.Add(len(ips))
		for _, ip := range ips {
			go func(ip netip.AddrPort) {
				defer wg.Done()
				testPeer(ctx, ip, netParams)
			}(ip)
		}
		wg.Wait()
	}
}

func main() {
	ctx := shutdownListener()

	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "loadConfig: %v\n", err)
		os.Exit(1)
	}

	dataDir := filepath.Join(defaultHomeDir, cfg.netParams.Name)
	amgr, err = NewManager(dataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "NewManager: %v\n", err)
		os.Exit(1)
	}

	seeder, err := netip.ParseAddrPort(cfg.Seeder)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid seeder ip: %v\n", err)
		os.Exit(1)
	}
	amgr.AddAddresses([]netip.AddrPort{seeder})

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		amgr.run(ctx) // only returns on context cancellation
		log.Print("Address manager done.")
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		creep(ctx, cfg.netParams) // only returns on context cancellation
		log.Print("Crawler done.")
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := serveHTTP(ctx, cfg.Listen); err != nil {
			log.Fatal(err)
		}
		log.Print("HTTP server done.")
	}()

	// Wait for crawler and http server, then stop address manager.
	wg.Wait()
	log.Print("Bye!")
}
