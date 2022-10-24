// Copyright (c) 2018-2021 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/netip"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/decred/dcrd/wire"
	"github.com/decred/dcrseeder/api"
)

// Node represents a single node on the Decred network. This struct is json
// encoded to be stored on disk.
type Node struct {
	Services        wire.ServiceFlag
	LastAttempt     time.Time
	FirstSuccess    time.Time
	LastSuccess     time.Time
	LastSeen        time.Time
	ProtocolVersion uint32
	IP              netip.AddrPort
}

type Manager struct {
	mtx sync.RWMutex

	nodes     map[string]*Node
	wg        sync.WaitGroup
	quit      chan struct{}
	peersFile string
}

const (
	// defaultMaxAddresses is the maximum number of addresses to return.
	defaultMaxAddresses = 16

	// defaultStaleTimeout is the time in which a host is considered
	// stale.
	defaultStaleTimeout = time.Hour

	// dumpAddressInterval is the interval used to dump the address
	// cache to disk for future use.
	dumpAddressInterval = time.Minute * 5

	// peersFilename is the name of the file.
	peersFilename = "nodes.json"

	// pruneAddressInterval is the interval used to run the address
	// pruner.
	pruneAddressInterval = time.Minute * 1

	// pruneExpireTimeout is the expire time in which a node is
	// considered dead.
	pruneExpireTimeout = time.Hour * 8
)

func NewManager(dataDir string) (*Manager, error) {
	err := os.MkdirAll(dataDir, 0o700)
	if err != nil {
		return nil, err
	}

	amgr := Manager{
		nodes:     make(map[string]*Node),
		peersFile: filepath.Join(dataDir, peersFilename),
		quit:      make(chan struct{}),
	}

	err = amgr.deserializePeers()
	if err != nil {
		log.Printf("Failed to parse file %s: %v", amgr.peersFile, err)
		// if it is invalid we nuke the old one unconditionally.
		err = os.Remove(amgr.peersFile)
		if err != nil {
			log.Printf("Failed to remove corrupt peers file %s: %v",
				amgr.peersFile, err)
		}
	}

	amgr.wg.Add(1)
	go amgr.addressHandler()

	return &amgr, nil
}

func (m *Manager) Stop() {
	close(m.quit)
	m.wg.Wait() // wait for addressHandler
	log.Print("Address manager done.")
}

func (m *Manager) AddAddresses(addrPorts []netip.AddrPort) int {
	var count int

	m.mtx.Lock()
	now := time.Now()
	for _, addrPortT := range addrPorts {
		// Never use ipv4-wrapped ipv6 addresses.
		addrPort := netip.AddrPortFrom(addrPortT.Addr().Unmap(),
			addrPortT.Port())

		if !isRoutable(addrPort.Addr()) {
			continue
		}

		addrStr := addrPort.String()
		_, exists := m.nodes[addrStr]
		if exists {
			m.nodes[addrStr].LastSeen = now
			continue
		}

		node := Node{
			IP:       addrPort,
			LastSeen: now,
			// FirstSuccess, LastSuccess and LastAttempt are
			// set by Good().
		}
		m.nodes[addrStr] = &node
		count++
	}
	m.mtx.Unlock()

	return count
}

// Addresses returns IPs that need to be tested again.
func (m *Manager) Addresses() []netip.AddrPort {
	addrs := make([]netip.AddrPort, 0, defaultMaxAddresses*8)
	i := defaultMaxAddresses

	m.mtx.RLock()
	now := time.Now()
	for _, node := range m.nodes {
		if i == 0 {
			break
		}
		if now.Sub(node.LastSuccess) < defaultStaleTimeout ||
			now.Sub(node.LastAttempt) < defaultStaleTimeout {
			continue
		}
		addrs = append(addrs, node.IP)
		i--
	}
	m.mtx.RUnlock()

	return addrs
}

func (m *Manager) GoodAddresses(ipversion, pver uint32, services wire.ServiceFlag) []api.Node {
	addrs := make([]api.Node, 0, defaultMaxAddresses)
	i := defaultMaxAddresses

	m.mtx.RLock()
	now := time.Now()
	for _, node := range m.nodes {
		if i == 0 {
			break
		}

		// Skip nodes that aren't known to be be stable yet.
		if node.FirstSuccess.IsZero() ||
			now.Sub(node.FirstSuccess) < defaultStaleTimeout {
			continue
		}

		// Skip nodes that do not seem to be online.
		if node.LastSuccess.IsZero() ||
			now.Sub(node.LastSuccess) >= defaultStaleTimeout {
			continue
		}

		// Filter on ipversion
		switch ipversion {
		case 4:
			if !node.IP.Addr().Is4() {
				continue
			}
		case 6:
			if !node.IP.Addr().Is6() {
				continue
			}
		}

		// Filter on protocol version
		if pver != 0 && node.ProtocolVersion < pver {
			continue
		}

		// Filter on services
		if services != 0 && node.Services&services != services {
			continue
		}

		addr := api.Node{
			Host:            node.IP.String(),
			Services:        uint64(node.Services),
			ProtocolVersion: node.ProtocolVersion,
		}
		addrs = append(addrs, addr)
		i--
	}
	m.mtx.RUnlock()

	return addrs
}

func (m *Manager) Attempt(addrPort netip.AddrPort) {
	m.mtx.Lock()
	node, exists := m.nodes[addrPort.String()]
	if exists {
		node.LastAttempt = time.Now()
	}
	m.mtx.Unlock()
}

func (m *Manager) Good(addrPort netip.AddrPort, services wire.ServiceFlag, pver uint32) {
	m.mtx.Lock()
	node, exists := m.nodes[addrPort.String()]
	if exists {
		now := time.Now()

		node.ProtocolVersion = pver
		node.Services = services
		node.LastSuccess = now
		if node.FirstSuccess.IsZero() {
			node.FirstSuccess = now
		}
	}
	m.mtx.Unlock()
}

// addressHandler is the main handler for the address manager.  It must be run
// as a goroutine.
func (m *Manager) addressHandler() {
	defer m.wg.Done()
	pruneAddressTicker := time.NewTicker(pruneAddressInterval)
	defer pruneAddressTicker.Stop()
	dumpAddressTicker := time.NewTicker(dumpAddressInterval)
	defer dumpAddressTicker.Stop()
out:
	for {
		select {
		case <-dumpAddressTicker.C:
			m.savePeers()
		case <-pruneAddressTicker.C:
			m.prunePeers()
		case <-m.quit:
			break out
		}
	}
	m.savePeers()
}

func (m *Manager) prunePeers() {
	m.mtx.Lock()
	now := time.Now()

	var count int
	for k, node := range m.nodes {
		// do not remove untried nodes
		if node.LastAttempt.IsZero() {
			continue
		}

		// node hasn't been seen via getaddr...
		if now.Sub(node.LastSeen) > pruneExpireTimeout {
			delete(m.nodes, k)
			count++
			continue
		}

		// a successful connection hasn't been made...
		if now.Sub(node.LastSuccess) > pruneExpireTimeout {
			delete(m.nodes, k)
			count++
			continue
		}
	}
	l := len(m.nodes)
	m.mtx.Unlock()

	log.Printf("Pruned %d addresses: %d remaining", count, l)
}

func (m *Manager) deserializePeers() error {
	filePath := m.peersFile
	_, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		return nil
	}
	r, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("%s error opening file: %v", filePath, err)
	}
	defer r.Close()

	var nodes map[string]*Node
	dec := json.NewDecoder(r)
	err = dec.Decode(&nodes)
	if err != nil {
		return fmt.Errorf("error reading %s: %v", filePath, err)
	}

	l := len(nodes)

	m.mtx.Lock()
	m.nodes = nodes
	m.mtx.Unlock()

	log.Printf("%d nodes loaded from %s", l, filePath)
	return nil
}

func (m *Manager) savePeers() {
	m.mtx.RLock()
	defer m.mtx.RUnlock()

	// Write temporary peers file and then move it into place.
	tmpfile := m.peersFile + ".new"
	w, err := os.Create(tmpfile)
	if err != nil {
		log.Printf("Error opening file %s: %v", tmpfile, err)
		return
	}
	enc := json.NewEncoder(w)
	if err := enc.Encode(&m.nodes); err != nil {
		w.Close()
		log.Printf("Failed to encode file %s: %v", tmpfile, err)
		return
	}
	if err := w.Close(); err != nil {
		log.Printf("Error closing file %s: %v", tmpfile, err)
		return
	}
	if err := os.Rename(tmpfile, m.peersFile); err != nil {
		log.Printf("Error writing file %s: %v", m.peersFile, err)
		return
	}

	log.Printf("%d nodes saved to %s", len(m.nodes), m.peersFile)
}
