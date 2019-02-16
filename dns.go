// Copyright (c) 2018 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"github.com/decred/dcrseeder/dnssec"
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

	udpAddr, err := net.ResolveUDPAddr("udp4", d.listen)
	if err != nil {
		log.Printf("ResolveUDPAddr: %v\n", err)
		return
	}

	udpListen, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		log.Printf("ListenUDP: %v\n", err)
		return
	}
	defer udpListen.Close()

	for {
		b := make([]byte, 512)
		_, addr, err := udpListen.ReadFromUDP(b)
		if err != nil {
			log.Printf("Read: %v", err)
			continue
		}

		go func() {
			dnsMsg := new(dns.Msg)
			err = dnsMsg.Unpack(b[:])
			if err != nil {
				log.Printf("%s: invalid dns message: %v\n",
					addr, err)
				return
			}
			if len(dnsMsg.Question) != 1 {
				log.Printf("%s sent more than 1 question: %d\n",
					addr, len(dnsMsg.Question))
				return
			}
			domainName := strings.ToLower(dnsMsg.Question[0].Name)

			isDnssecQuery := false
			if dnsMsg.IsEdns0() != nil {
				isDnssecQuery = dnsMsg.IsEdns0().Do()
			}

			// TODO respond back with empty response
			//ff := strings.LastIndex(domainName, d.hostname)
			//if ff < 0 {
			//	log.Printf("invalid name: %s",
			//		dnsMsg.Question[0].Name)
			//	return
			//}

			respMsg := dnsMsg.Copy()
			respMsg.Authoritative = true
			respMsg.Response = true

			wantedSF := getWantedSF(domainName, addr)
			if wantedSF == 0 {
				sendResponseBytes(respMsg, addr, udpListen)
				return
			}

			rr := fmt.Sprintf("%s 86400 IN NS %s", d.hostname, d.nameserver)
			authority, err := dns.NewRR(rr)
			if err != nil {
				log.Printf("NewRR: %v", err)
				return
			}

			log.Printf("%s: query %d for %v", addr,
				dnsMsg.Question[0].Qtype, wantedSF)

			aResponse := func(qtype uint16, atype string) {
				respMsg.Ns = append(respMsg.Ns, authority)
				if isDnssecQuery {
					respMsg.Ns, err = dnssec.SignRRSetWithZsk(respMsg.Ns)
					if err != nil {
						log.Printf("unable to sign %s response\n", atype)
					}
				}
				ips := amgr.GoodAddresses(qtype, wantedSF)
				for _, ip := range ips {
					rr = fmt.Sprintf("%s 30 IN %s %s",
						dnsMsg.Question[0].Name, atype,
						ip.String())
					newRR, err := dns.NewRR(rr)
					if err != nil {
						log.Printf("%s: NewRR: %v\n", addr, err)
						return
					}
					respMsg.Answer = append(respMsg.Answer, newRR)
				}

				if respMsg.Answer != nil && isDnssecQuery {
					respMsg.Answer, err = dnssec.SignRRSetWithZsk(respMsg.Answer)
					if err != nil {
						log.Printf("DNSSEC: cannot sign %s RR: %v\n", atype, err)
						return
					}
				}
			}

			qtype := dnsMsg.Question[0].Qtype
			switch qtype {
			case dns.TypeA:
				aResponse(qtype, "A")

			case dns.TypeAAAA:
				aResponse(qtype, "AAAA")

			case dns.TypeNS:
				rr = fmt.Sprintf("%s 86400 IN NS %s",
					dnsMsg.Question[0].Name, d.nameserver)
				newRR, err := dns.NewRR(rr)
				if err != nil {
					log.Printf("%s: NewRR: %v", addr, err)
					return
				}
				respMsg.Answer = append(respMsg.Answer, newRR)
				if isDnssecQuery {
					respMsg.Answer, err = dnssec.SignRRSetWithZsk(respMsg.Answer)
					if err != nil {
						log.Printf("Error signing RR: %s\n", err.Error())
					}
				}

			case dns.TypeDNSKEY:
				respMsg.Answer, err = dnssec.GetDNSKEY()
				if err != nil {
					log.Printf("invalid DNSKEY: %v", err)
					return
				}
				if isDnssecQuery {
					respMsg.Answer, err = dnssec.SignRRSetWithKsk(respMsg.Answer)
				}

			case dns.TypeSOA:
				soa := new(dns.SOA)
				soa.Hdr = dns.RR_Header{
					Name:     d.hostname,
					Rrtype:   dns.TypeSOA,
					Class:    dns.ClassINET,
					Ttl:      14400,
					Rdlength: 0,
				}
				soa.Ns = d.nameserver
				soa.Mbox = "hostmaster.stakey.org."
				soa.Serial = 1164162633
				soa.Refresh = 14400
				soa.Retry = 3600
				soa.Expire = 1209600
				soa.Minttl = 86400
				respMsg.Answer = append(respMsg.Answer, soa)
				if isDnssecQuery {
					respMsg.Answer, err = dnssec.SignRRSetWithZsk(respMsg.Answer)
					if err != nil {
						return
					}
				}

			case dns.TypeHINFO:
				hinfo := &dns.HINFO{
					Hdr: dns.RR_Header{
						Name:   d.hostname,
						Rrtype: dns.TypeHINFO,
						Class:  dns.ClassINET,
						Ttl:    3600,
					},
					Cpu: "dcrseeder",
					Os:  "",
				}
				respMsg.Answer = append(respMsg.Answer, hinfo)
				if isDnssecQuery {
					respMsg.Answer, err = dnssec.SignRRSetWithZsk(respMsg.Answer)
					if err != nil {
						return
					}
				}

			case dns.TypeDS:

			default:
				log.Printf("%s: invalid qtype: %d\n", addr, dnsMsg.Question[0].Qtype)
			}

			//done:
			sendResponseBytes(respMsg, addr, udpListen)
		}()
	}
}

func sendResponseBytes(respMsg *dns.Msg, addr *net.UDPAddr, udpListen *net.UDPConn) {
	sendBytes, err := respMsg.Pack()
	if err != nil {
		log.Printf("%s: failed to pack response: %v", addr, err)
		return
	}
	_, err = udpListen.WriteToUDP(sendBytes, addr)
	if err != nil {
		log.Printf("%s: failed to write response: %v", addr, err)
		return
	}
}

func getWantedSF(domainName string, addr *net.UDPAddr) (wantedSF wire.ServiceFlag) {
	wantedSF = wire.SFNodeNetwork
	labels := dns.SplitDomainName(domainName)
	if labels == nil {
		return 0
	}
	if labels[0][0] == 'x' && len(labels[0]) > 1 {
		wantedSFStr := labels[0][1:]
		u, err := strconv.ParseUint(wantedSFStr, 10, 64)
		if err != nil {
			log.Printf("%s: ParseUint: %v", addr, err)
			return 0
		}
		wantedSF = wire.ServiceFlag(u)
	}
	return wantedSF
}

func NewDNSServer(hostname, nameserver, listen string) *DNSServer {
	if hostname[len(hostname)-1] != '.' {
		hostname = hostname + "."
	}
	if nameserver[len(nameserver)-1] != '.' {
		nameserver = nameserver + "."
	}

	return &DNSServer{
		hostname:   hostname,
		listen:     listen,
		nameserver: nameserver,
	}
}
