package main

import (
	"net"
	"os"
	"testing"
	"time"

	"github.com/decred/dcrd/wire"
	"github.com/decred/dcrseeder/dnssec"
	"github.com/miekg/dns"
)

const (
	ServerPort  = "55355"
	ListenAddr  = "127.0.0.1"
	testHomeDir = "./testdata"
	seedHost    = "45.77.52.109"
	hostName    = "testnet-seed.decred.org"
	zskPrefix   = "Ktestnet-seed.stakey.org.+010+6257"
	kskPrefix   = "Ktestnet-seed.stakey.org.+010+34266"
)

var (
	dnsClient       *dns.Client
	dnsClientConfig *dns.ClientConfig
	err             error
)

func initializeDnssec(t *testing.T) {
	err := dnssec.Initialize(testHomeDir, hostName, zskPrefix, kskPrefix)
	if err != nil {
		t.Errorf("dnssec initialize failed: %v\n", err)
	}
}

func startServer(t *testing.T) *DNSServer {
	wg.Add(1)
	dnsServer := NewDNSServer(hostName, "ns-test.decred.org", "127.0.0.1:"+ServerPort)
	if dnsServer == nil {
		t.Error("NewDNSServer shouldn't return nil")
	}
	go dnsServer.Start()
	time.Sleep(1 * time.Second)
	return dnsServer
}

func testDnsClient() {

	dnsClient = &dns.Client{
		ReadTimeout: 5 * time.Second,
	}

	dnsClientConfig = &dns.ClientConfig{
		Servers: []string{ListenAddr},
		Port:    ServerPort,
	}
}

func query(t *testing.T, qname string, qtype uint16) (msg *dns.Msg) {

	dnsMessage := &dns.Msg{
		MsgHdr: dns.MsgHdr{
			RecursionDesired: false,
		},
	}
	dnsMessage.SetQuestion(qname, qtype)

	r, _, err := dnsClient.Exchange(dnsMessage, ListenAddr+":"+ServerPort)

	if err != nil {
		t.Error("err should return nil")
	}
	if r == nil {
		t.Error("query should not return nil")
	}
	if r.Rcode != dns.RcodeSuccess {
		t.Error("unexpected Rcode")
	}
	return r
}

func queryEdns(t *testing.T, qname string, qtype uint16) (msg *dns.Msg) {

	dnsMessage := &dns.Msg{
		MsgHdr: dns.MsgHdr{
			RecursionDesired: false,
		},
	}
	dnsMessage.SetEdns0(4096, true)
	dnsMessage.SetQuestion(qname, qtype)

	r, _, err := dnsClient.Exchange(dnsMessage, ListenAddr+":"+ServerPort)
	if err != nil {
		t.Error("err should return nil")
	}
	if r == nil {
		t.Error("query should not return nil")
	}
	if r.Rcode != dns.RcodeSuccess {
		t.Error("unexpected Rcode")
	}
	return r
}

func addrMgr(t *testing.T) {
	amgr, err = NewManager(testHomeDir)
	if err != nil {
		t.Errorf("NewManager: %v\n", err)
	}

	if amgr == nil {
		t.Error("amgr shouldn't be nil")
	}
}

func addGoodAddresses(ipStrs []string) {
	for _, ipStr := range ipStrs {
		ip := net.ParseIP(ipStr)
		amgr.AddAddresses([]net.IP{ip})
		amgr.Good(ip, wire.SFNodeNetwork|wire.SFNodeCF)
	}
}

func TestNewMgr(t *testing.T) {
	_ = os.Remove("./testdata/nodes.json")
	amgr, err = NewManager(testHomeDir)
	if err != nil {
		t.Errorf("NewManager: %v\n", err)
	}
	if amgr == nil {
		t.Error("amgr shouldn't be nil")
	}
}

func TestAddAddresses(t *testing.T) {
	amgr.AddAddresses([]net.IP{net.ParseIP("127.0.0.1")})
}

func TestNewDNSServer(t *testing.T) {
	_ = startServer(t)
}

func TestDnsClient(t *testing.T) {
	testDnsClient()
}

func TestAnswerTypeA(t *testing.T) {
	addGoodAddresses([]string{seedHost})
	r := query(t, dns.Fqdn(hostName), dns.TypeA)
	if r.Answer == nil {
		t.Error("r.Answer shouldn't be nil")
	}
	if len(r.Answer) != 1 {
		t.Error("t.Answer should contain 1 result")
	}
	if len(r.Ns) != 1 {
		t.Error("missing authority data")
	}

	// add a few more seed IPs and see if we get them back via DNS
	addGoodAddresses([]string{"2.2.2.2", "3.3.3.3"})

	r = query(t, dns.Fqdn(hostName), dns.TypeA)
	if r.Answer == nil {
		t.Error("r.Answer shouldn't be nil")
	}
	if len(r.Answer) != 3 {
		t.Error("t.Answer should contain 4 results")
	}
}

func TestAnswerTypeAAAA(t *testing.T) {
	addGoodAddresses([]string{"2a03:b0c0:3:d0::596a:3001"})
	r := query(t, dns.Fqdn(hostName), dns.TypeAAAA)
	if r.Answer == nil {
		t.Error("AAAA r.Answer shouldn't be nil")
	}
	if len(r.Answer) != 1 {
		t.Error("AAAA t.Answer should contain 1 result")
	}
}

func TestEdnsQueryWithoutDnssecInit(t *testing.T) {
	r := queryEdns(t, dns.Fqdn(hostName), dns.TypeA)
	if r.Answer == nil {
		t.Error("r.Answer shouldn't be nil")
	}
	if len(r.Answer) != 3 {
		t.Error("t.Answer should contain 1 RR")
	}
}

func TestDnssecInitialize(t *testing.T) {
	initializeDnssec(t)
}

func TestAnswerTypeARRSIG(t *testing.T) {
	testDnsClient()
	r := queryEdns(t, dns.Fqdn(hostName), dns.TypeA)
	if r.Answer == nil {
		t.Error("r.Answer shouldn't be nil")
	}
	if len(r.Answer) != 4 {
		t.Error("t.Answer should contain 4 RRs")
	}

	rrSig := r.Answer[3].(*dns.RRSIG)
	if rrSig.Signature == "" {
		t.Error("invalid RRSIG")
	}
	if rrSig.SignerName != "testnet-seed.stakey.org." {
		t.Errorf("invalid SignerName %s", rrSig.SignerName)
	}
	if len(r.Ns) != 2 {
		t.Error("missing signed authority data")
	}

	rrSig = r.Ns[1].(*dns.RRSIG)
	if rrSig.Signature == "" {
		t.Error("invalid RRSIG")
	}
	if rrSig.SignerName != "testnet-seed.stakey.org." {
		t.Errorf("invalid SignerName %s", rrSig.SignerName)
	}
}

func TestAnswerTypeDNSKEY(t *testing.T) {
	r := queryEdns(t, dns.Fqdn(hostName), dns.TypeDNSKEY)

	if r.Answer == nil {
		t.Error("r.Answer shouldn't be nil")
	}
	if len(r.Answer) != 3 {
		t.Error("t.Answer should contain 2 RRs")
	}
	if r.Answer[0].(*dns.DNSKEY).String() != "testnet-seed.stakey.org.	3600	IN	DNSKEY	256 3 10 AwEAAcYRTr+FapmO85SOr9uDmQJVmxeuWb8jDd+IjMiU7Y/7ae+Gmo8e0E/EAl8yTSEFutZKLxiM5DWJpobXH2YjfXgDW6CEZ3najAFRXJF8Dl6HGiWTmq8L3DfVvpVlI1D5v08PQglKpM4I4iZNWeQvfLWtqT/8Ak5bhobGaLBxzbXj" {
		t.Error("invalid DNSKEY (ZSK)")
	}
	if r.Answer[1].(*dns.DNSKEY).String() != "testnet-seed.stakey.org.	3600	IN	DNSKEY	257 3 10 AwEAAbefDy94ILgvXAugk7mkOQtX0J8x7tO5U+h+0NCN20mqU/lt63KuOOrAwX7q6izQ5Ym+4qOU+pELIEeoIXEtQKP8UxIDsWb0WnPvUw5lyT+iLF/GSg4Cd4rSSXPCkL6tfr8UFD+s/KytHRLsn4eFWifhiXyc1b4XB1gYFsjbnii3" {
		t.Error("invalid DNSKEY (KSK)")
	}

	rrSig := r.Answer[2].(*dns.RRSIG)
	if rrSig.Signature == "" {
		t.Error("invalid RRSIG")
	}
	if rrSig.SignerName != "testnet-seed.stakey.org." {
		t.Errorf("invalid SignerName %s", rrSig.SignerName)
	}
}

func TestAnswerTypeSOA(t *testing.T) {
	r := queryEdns(t, dns.Fqdn(hostName), dns.TypeSOA)
	if r.Answer == nil {
		t.Error("r.Answer shouldn't be nil")
	}
	if len(r.Answer) != 2 {
		t.Error("t.Answer should contain 2 RRs")
	}

	rrSig := r.Answer[1].(*dns.RRSIG)
	if rrSig.Signature == "" {
		t.Error("invalid RRSIG")
	}
	if rrSig.SignerName != "testnet-seed.stakey.org." {
		t.Errorf("invalid SignerName %s", rrSig.SignerName)
	}
}

func TestAnswerTypeINFO(t *testing.T) {
	r := queryEdns(t, dns.Fqdn(hostName), dns.TypeHINFO)
	if r.Answer == nil {
		t.Error("r.Answer shouldn't be nil")
	}
	if len(r.Answer) != 2 {
		t.Error("t.Answer should contain 2 RRs")
	}

	rrSig := r.Answer[1].(*dns.RRSIG)
	if rrSig.Signature == "" {
		t.Error("invalid RRSIG")
	}
	if rrSig.SignerName != "testnet-seed.stakey.org." {
		t.Errorf("invalid SignerName %s", rrSig.SignerName)
	}
}

func TestAnswerTypeNS(t *testing.T) {
	r := queryEdns(t, dns.Fqdn(hostName), dns.TypeNS)
	if r.Answer == nil {
		t.Error("r.Answer shouldn't be nil")
	}
	if len(r.Answer) != 2 {
		t.Error("t.Answer should contain 2 RRs")
	}

	rrSig := r.Answer[1].(*dns.RRSIG)
	if rrSig.Signature == "" {
		t.Error("invalid RRSIG")
	}
	if rrSig.SignerName != "testnet-seed.stakey.org." {
		t.Errorf("invalid SignerName %s", rrSig.SignerName)
	}
}
