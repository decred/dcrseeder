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

type crawler struct {
	params *chaincfg.Params
	amgr   *Manager
	log    *log.Logger
}

func newCrawler(params *chaincfg.Params, amgr *Manager, log *log.Logger) *crawler {
	return &crawler{
		params: params,
		amgr:   amgr,
		log:    log,
	}
}

func (c *crawler) testPeer(ctx context.Context, ip netip.AddrPort) {
	onaddr := make(chan struct{}, 1)
	verack := make(chan struct{}, 1)
	config := peer.Config{
		UserAgentName:    appName,
		UserAgentVersion: "0.0.1",
		Net:              c.params.Net,
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
				added := c.amgr.AddAddresses(n)
				c.log.Printf("Peer %v sent %v addresses, %d new",
					p.Addr(), len(msg.AddrList), added)
				onaddr <- struct{}{}
			},
			OnVerAck: func(p *peer.Peer, msg *wire.MsgVerAck) {
				c.log.Printf("Adding peer %v with services %v pver %d",
					p.NA().IP.String(), p.Services(), p.ProtocolVersion())
				verack <- struct{}{}
			},
		},
	}

	host := ip.String()
	p, err := peer.NewOutboundPeer(&config, host)
	if err != nil {
		c.log.Printf("NewOutboundPeer on %v: %v", host, err)
		return
	}

	// Time stamp the attempt after disconnect or dial error so we don't prune
	// this peer before or during its test.
	defer c.amgr.Attempt(ip)

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
		c.amgr.Good(ip, p.Services(), p.ProtocolVersion())

		// Ask peer for some addresses.
		p.QueueMessage(wire.NewMsgGetAddr(), nil)

	case <-time.After(defaultNodeTimeout):
		c.log.Printf("verack timeout on peer %v", p.Addr())
		return
	case <-ctx.Done():
		return
	}

	select {
	case <-onaddr:
	case <-time.After(defaultNodeTimeout):
		c.log.Printf("getaddr timeout on peer %v", p.Addr())
	case <-ctx.Done():
	}
}

func (c *crawler) run(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			return
		}

		ips := c.amgr.Addresses()
		if len(ips) == 0 {
			c.log.Printf("No stale addresses -- sleeping for %v", defaultAddressTimeout)
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
				c.testPeer(ctx, ip)
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

	// Prefix log lines with current network, e.g. "[mainnet]" or "[testnet]".
	logPrefix := fmt.Sprintf("[%.7s] ", cfg.netParams.Name)
	log := log.New(os.Stdout, logPrefix, log.LstdFlags|log.Lmsgprefix)

	dataDir := filepath.Join(defaultHomeDir, cfg.netParams.Name)
	amgr, err := NewManager(dataDir, log)
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

	c := newCrawler(cfg.netParams, amgr, log)

	server, err := newServer(cfg.Listen, amgr, log)
	if err != nil {
		log.Fatal(err)
	}

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		amgr.run(ctx) // Only returns on context cancellation.
		log.Print("Address manager done.")
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		c.run(ctx) // Only returns on context cancellation.
		log.Print("Crawler done.")
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		server.run(ctx) // Only returns on context cancellation.
		log.Print("HTTP server done.")
	}()

	// Wait for crawler and http server, then stop address manager.
	wg.Wait()
	log.Print("Bye!")
}
