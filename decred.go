// Copyright (c) 2020 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"time"

	"github.com/decred/dcrd/chaincfg/v3"
	"github.com/decred/dcrd/peer/v2"
	"github.com/decred/dcrd/wire"
)

const (
	// defaultAddressTimeout defines the duration to wait for new addresses.
	defaultAddressTimeout = time.Minute * 10

	// defaultNodeTimeout defines the timeout on responses from a node.
	defaultNodeTimeout = time.Second * 3
)

var amgr *Manager

func creep(ctx context.Context, netParams *chaincfg.Params) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
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
			onaddr := make(chan struct{}, 1)
			verack := make(chan struct{}, 1)
			config := peer.Config{
				UserAgentName:    appName,
				UserAgentVersion: "0.0.1",
				Net:              netParams.Net,
				DisableRelayTx:   true,

				Listeners: peer.MessageListeners{
					OnAddr: func(p *peer.Peer, msg *wire.MsgAddr) {
						n := make([]net.IP, 0, len(msg.AddrList))
						for _, addr := range msg.AddrList {
							n = append(n, addr.IP)
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

			go func(ip net.IP) {
				defer wg.Done()

				host := net.JoinHostPort(ip.String(), netParams.DefaultPort)
				p, err := peer.NewOutboundPeer(&config, host)
				if err != nil {
					log.Printf("NewOutboundPeer on %v: %v", host, err)
					return
				}

				amgr.Attempt(ip)
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
					amgr.Good(p.NA().IP, p.Services(), p.ProtocolVersion())

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
			}(ip)
		}
		wg.Wait()
	}
}

func main() {
	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "loadConfig: %v\n", err)
		os.Exit(1)
	}
	amgr, err = NewManager(filepath.Join(defaultHomeDir,
		cfg.netParams.Name), cfg.netParams.DefaultPort)
	if err != nil {
		fmt.Fprintf(os.Stderr, "NewManager: %v\n", err)
		os.Exit(1)
	}

	amgr.AddAddresses([]net.IP{net.ParseIP(cfg.Seeder)})

	ctx, shutdown := context.WithCancel(context.Background())
	killSwitch := make(chan os.Signal, 1)
	signal.Notify(killSwitch, os.Interrupt)
	go func() {
		<-killSwitch
		log.Print("Shutting down...")
		shutdown()
	}()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		creep(ctx, cfg.netParams) // only returns on context cancellation
		log.Print("Crawler done.")
	}()

	if cfg.HTTPListen != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := serveHTTP(ctx, cfg.HTTPListen); err != nil {
				log.Fatal(err)
				shutdown()
			}
			log.Print("HTTP server done.")
		}()
	}

	if cfg.DNSListen != "" {
		dnsServer := NewDNSServer(cfg.Host, cfg.Nameserver, cfg.DNSListen)
		go dnsServer.Start() // no graceful shutdown
	}

	// Wait for crawler and http server, then stop address manager.
	wg.Wait()
	amgr.Stop()
	log.Print("Bye!")
}
