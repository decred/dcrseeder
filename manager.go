// Copyright (c) 2018-2021 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/decred/dcrd/wire"
	"github.com/decred/dcrseeder/api"
	"github.com/miekg/dns"
)

// Node represents a single node on the Decred network. This struct is json
// encoded to be stored on disk.
type Node struct {
	Services        wire.ServiceFlag
	LastAttempt     time.Time
	LastSuccess     time.Time
	LastSeen        time.Time
	ProtocolVersion uint32
	IP              net.IP
}

type Manager struct {
	mtx sync.RWMutex

	port      string
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

func NewManager(dataDir string, defaultPort string) (*Manager, error) {
	err := os.MkdirAll(dataDir, 0700)
	if err != nil {
		return nil, err
	}

	amgr := Manager{
		port:      defaultPort,
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

func (m *Manager) AddAddresses(addrs []net.IP) int {
	var count int

	m.mtx.Lock()
	for _, addr := range addrs {
		if !isRoutable(addr) {
			continue
		}
		addrStr := addr.String()

		_, exists := m.nodes[addrStr]
		if exists {
			m.nodes[addrStr].LastSeen = time.Now()
			continue
		}
		node := Node{
			IP:       addr,
			LastSeen: time.Now(),
		}
		m.nodes[addrStr] = &node
		count++
	}
	m.mtx.Unlock()

	return count
}

// Addresses returns IPs that need to be tested again.
func (m *Manager) Addresses() []net.IP {
	addrs := make([]net.IP, 0, defaultMaxAddresses*8)
	now := time.Now()
	i := defaultMaxAddresses

	m.mtx.RLock()
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

	now := time.Now()
	m.mtx.RLock()
	for _, node := range m.nodes {
		if i == 0 {
			break
		}

		if node.LastSuccess.IsZero() ||
			now.Sub(node.LastSuccess) > defaultStaleTimeout {
			continue
		}
		switch ipversion {
		case 4:
			if node.IP.To4() == nil {
				continue
			}
		case 6:
			if node.IP.To4() != nil {
				continue
			}
		}
		if pver != 0 && node.ProtocolVersion < pver {
			continue
		}
		if services != 0 && node.Services&services != services {
			continue
		}
		addr := api.Node{
			Host:            net.JoinHostPort(node.IP.String(), m.port),
			Services:        uint64(node.Services),
			ProtocolVersion: node.ProtocolVersion,
		}
		addrs = append(addrs, addr)
		i--
	}
	m.mtx.RUnlock()

	return addrs
}

// GoodDNSAddresses returns good working IPs that match both the
// passed DNS query type and have the requested services.
func (m *Manager) GoodDNSAddresses(qtype uint16, services wire.ServiceFlag) []net.IP {
	addrs := make([]net.IP, 0, defaultMaxAddresses)
	i := defaultMaxAddresses

	if qtype != dns.TypeA && qtype != dns.TypeAAAA {
		return addrs
	}

	now := time.Now()
	m.mtx.RLock()
	for _, node := range m.nodes {
		if i == 0 {
			break
		}

		if qtype == dns.TypeA && node.IP.To4() == nil {
			continue
		} else if qtype == dns.TypeAAAA && node.IP.To4() != nil {
			continue
		}

		if node.LastSuccess.IsZero() ||
			now.Sub(node.LastSuccess) > defaultStaleTimeout {
			continue
		}

		// Does the node have the requested services?
		if node.Services&services != services {
			continue
		}
		addrs = append(addrs, node.IP)
		i--
	}
	m.mtx.RUnlock()

	return addrs
}

func (m *Manager) Attempt(ip net.IP) {
	m.mtx.Lock()
	node, exists := m.nodes[ip.String()]
	if exists {
		node.LastAttempt = time.Now()
	}
	m.mtx.Unlock()
}

func (m *Manager) Good(ip net.IP, services wire.ServiceFlag, pver uint32) {
	m.mtx.Lock()
	node, exists := m.nodes[ip.String()]
	if exists {
		node.ProtocolVersion = pver
		node.Services = services
		node.LastSuccess = time.Now()
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
	var count int
	now := time.Now()
	m.mtx.Lock()
	for k, node := range m.nodes {
		if now.Sub(node.LastSeen) > pruneExpireTimeout {
			delete(m.nodes, k)
			count++
			continue
		}
		if !node.LastSuccess.IsZero() &&
			now.Sub(node.LastSuccess) > pruneExpireTimeout {
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
