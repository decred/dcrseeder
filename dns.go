// Copyright (c) 2018 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"

	"github.com/decred/dcrd/wire"
	"github.com/miekg/dns"
)

type DNSServer struct {
	hostname   string
	listen     string
	nameserver string
}

func (d *DNSServer) Start() {
	defer wg.Done()

	rr := fmt.Sprintf("%s 86400 IN NS %s", d.hostname, d.nameserver)
	authority, err := dns.NewRR(rr)
	if err != nil {
		log.Printf("NewRR: %v", err)
		return
	}

	udpAddr, err := net.ResolveUDPAddr("udp4", d.listen)
	if err != nil {
		log.Printf("ResolveUDPAddr: %v", err)
		return
	}

	udpListen, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		log.Printf("ListenUDP: %v", err)
		return
	}
	defer udpListen.Close()

	minRequiredLabels := dns.CountLabel(d.hostname)
	for {
		b := make([]byte, 512)
		_, addr, err := udpListen.ReadFromUDP(b)
		if err != nil {
			log.Printf("Read: %v", err)
			continue
		}

		go func() {
			dnsMsg := new(dns.Msg)
			err = dnsMsg.Unpack(b)
			if err != nil {
				log.Printf("%s: invalid dns message: %v",
					addr, err)
				return
			}
			if len(dnsMsg.Question) != 1 {
				log.Printf("%s sent more than 1 question: %d",
					addr, len(dnsMsg.Question))
				return
			}

			if !dns.IsSubDomain(d.hostname, dnsMsg.Question[0].Name) {
				log.Printf("%s: invalid hostname: %v", addr,
					dnsMsg.Question[0].Name)
				return
			}
			var wantedSF wire.ServiceFlag
			numLabels := dns.CountLabel(dnsMsg.Question[0].Name) - minRequiredLabels
			switch numLabels {
			case 0:
				wantedSF = wire.SFNodeNetwork
			case 1:
				labels := dns.SplitDomainName(dnsMsg.Question[0].Name)
				label := strings.ToLower(labels[0])
				if len(label) < 2 || label[0] != 'x' {
					log.Printf("invalid name: %s",
						dnsMsg.Question[0].Name)
					return
				}
				wantedSFStr := label[1:]
				u, err := strconv.ParseUint(wantedSFStr, 10, 64)
				if err != nil {
					log.Printf("%s: ParseUint: %v", addr, err)
					return
				}
				wantedSF = wire.ServiceFlag(u)
			default:
				log.Printf("%s: invalid hostname: %v", addr,
					dnsMsg.Question[0].Name)
				return
			}

			var atype string
			qtype := dnsMsg.Question[0].Qtype
			switch qtype {
			case dns.TypeA:
				atype = "A"
			case dns.TypeAAAA:
				atype = "AAAA"
			case dns.TypeNS:
				atype = "NS"
			default:
				log.Printf("%s: invalid qtype: %d", addr,
					dnsMsg.Question[0].Qtype)
				return
			}

			log.Printf("%s: query %d for %v", addr,
				dnsMsg.Question[0].Qtype, wantedSF)

			respMsg := dnsMsg.Copy()
			respMsg.Authoritative = true
			respMsg.Response = true

			if qtype != dns.TypeNS {
				respMsg.Ns = append(respMsg.Ns, authority)
				ips := amgr.GoodAddresses(qtype, wantedSF)
				for _, ip := range ips {
					rr = fmt.Sprintf("%s 30 IN %s %s",
						dnsMsg.Question[0].Name, atype,
						ip.String())
					newRR, err := dns.NewRR(rr)
					if err != nil {
						log.Printf("%s: NewRR: %v",
							addr, err)
						return
					}

					respMsg.Answer = append(respMsg.Answer,
						newRR)
				}
			} else {
				rr = fmt.Sprintf("%s 86400 IN NS %s",
					dnsMsg.Question[0].Name, d.nameserver)
				newRR, err := dns.NewRR(rr)
				if err != nil {
					log.Printf("%s: NewRR: %v", addr, err)
					return
				}

				respMsg.Answer = append(respMsg.Answer, newRR)
			}

			sendBytes, err := respMsg.Pack()
			if err != nil {
				log.Printf("%s: failed to pack response: %v",
					addr, err)
				return
			}

			_, err = udpListen.WriteToUDP(sendBytes, addr)
			if err != nil {
				log.Printf("%s: failed to write response: %v",
					addr, err)
				return
			}
		}()
	}
}

func NewDNSServer(hostname, nameserver, listen string) (*DNSServer, error) {
	if !dns.IsFqdn(hostname) {
		return nil, fmt.Errorf("%s is not a fqdn", hostname)
	}
	if !dns.IsFqdn(nameserver) {
		return nil, fmt.Errorf("%s is not a fqdn", nameserver)
	}
	return &DNSServer{
		hostname:   hostname,
		listen:     listen,
		nameserver: nameserver,
	}, nil
}
