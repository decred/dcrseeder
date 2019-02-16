package dnssec

import (
	"bufio"
	"crypto"
	"crypto/rsa"
	"errors"
	"fmt"
	"github.com/miekg/dns"
	"log"
	"os"
	"path"
	"strings"
	"time"
)

type SigningKey struct {
	KeyFile string
	PrivKey *crypto.PrivateKey
	PubKey  *dns.DNSKEY
}

var (
	zsk                *SigningKey
	ksk                *SigningKey
	zone               string
	keyStorageDir      string
	rrSigInceptionTime time.Time
)

func KeyTagAsString(k *dns.DNSKEY) string {
	return fmt.Sprintf("K%s+0%d+%d", dns.Fqdn(k.Hdr.Name), k.Algorithm, k.KeyTag())
}

func generateSigningKey(name string, bits int, flags uint16, ttl uint32) (newKey *dns.DNSKEY, privKey crypto.PrivateKey, error error) {
	key := new(dns.DNSKEY)
	key.Hdr.Name = name
	key.Hdr.Rrtype = dns.TypeDNSKEY
	key.Hdr.Class = dns.ClassINET
	key.Hdr.Ttl = ttl
	key.Flags = flags
	key.Protocol = 3
	key.Algorithm = dns.RSASHA512
	privkey, err := key.Generate(bits)

	if privkey != nil {
		log.Printf("Private key generated, keytag: %s", KeyTagAsString(key))
	}
	return key, privkey, err
}

func GenerateSigningKeys(name string, bits int, keyFlags uint16) (retKeyTag string, err error) {

	key, privKey, err := generateSigningKey(name, bits, keyFlags, 3600) // ZSK

	if err != nil {
		log.Printf("Unable to generate key: %s\n", err)
		return "", err
	}

	keyTag := KeyTagAsString(key)

	writeKey := func(keyName, suffix, data string) (err error) {

		keyFile := fmt.Sprintf("%s.%s", keyTag, suffix)
		fullKeyFile := path.Join(keyStorageDir, keyFile)
		f, err := os.OpenFile(fullKeyFile, os.O_CREATE|os.O_WRONLY, 0400)
		defer f.Close()

		if err != nil {
			log.Printf("Failed to create key file: %s", keyFile)
			return err
		}
		w := bufio.NewWriter(f)
		_, err = w.WriteString(data)
		err = w.Flush()

		if err != nil {
			log.Printf("Failed to save key file %s\n", fullKeyFile)
			return err
		}

		log.Printf("Created key %s\n", fullKeyFile)
		return nil
	}

	err = writeKey(keyTag, "private", key.PrivateKeyString(privKey))
	if err != nil {
		return "", err
	}
	err = writeKey(keyTag, "key", key.String())
	if err != nil {
		return "", err
	}

	// if this is a KSK, we'll output the DS record as well
	if keyFlags == 257 {
		dsRR := []string{
			key.ToDS(dns.SHA256).String(), ""}
		err := writeKey(keyTag, "ds", strings.Join(dsRR, "\n"))
		if err != nil {
			return "", err
		}
	}

	return keyTag, nil
}

func loadSigningKey(keyFileBasename string) (signingKey *SigningKey, err error) {

	keyFilename := strings.Join([]string{keyFileBasename, ".key"}, "")
	f, err := os.Open(keyFilename)
	if err != nil {
		log.Printf("Cannot open key file: %s\n", keyFilename)
		return nil, err
	}
	rr, err := dns.ReadRR(bufio.NewReader(f), keyFilename)
	if err != nil {
		log.Printf("Unable to load public key from %s\n", keyFilename)
		return nil, err
	}
	f.Close()

	k := rr.(*dns.DNSKEY)

	keyFilename = strings.Join([]string{keyFileBasename, ".private"}, "")
	fPriv, err := os.Open(keyFilename)
	if err != nil {
		log.Printf("Cannot open key file: %s\n", keyFilename)
		return nil, err
	}
	privKey, err := k.ReadPrivateKey(bufio.NewReader(fPriv), keyFilename)
	if err != nil {
		log.Printf("Unable to load private key from %s\n", keyFilename)
		return nil, err
	}
	fPriv.Close()

	return &SigningKey{
		PubKey:  rr.(*dns.DNSKEY),
		PrivKey: &privKey,
	}, nil
}

func makeRRSIG(k *dns.DNSKEY, ttl uint32) *dns.RRSIG {

	sig := new(dns.RRSIG)
	sig.Hdr = dns.RR_Header{
		Name:     zone,
		Rrtype:   dns.TypeRRSIG,
		Class:    dns.ClassINET,
		Ttl:      ttl,
		Rdlength: 0}
	sig.Inception = uint32(rrSigInceptionTime.Unix() - 300) // clock skew safe, 5 minutes ago
	sig.Expiration = sig.Inception + 30*24*60*60            // inception + 30 days
	sig.KeyTag = k.KeyTag()
	sig.SignerName = k.Hdr.Name
	sig.Algorithm = k.Algorithm
	return sig
}

func SignRRSetWithZsk(rrSet []dns.RR) ([]dns.RR, error) {
	return signRRSet(zsk, rrSet)
}

func SignRRSetWithKsk(rrSet []dns.RR) ([]dns.RR, error) {
	return signRRSet(ksk, rrSet)
}

func signRRSet(signingKey *SigningKey, rrSet []dns.RR) ([]dns.RR, error) {
	if rrSet == nil {
		return rrSet, nil
	}
	log.Printf("signing RRSET %v", rrSet[0])

	// TODO should we raise an error if the RRset is empty,
	// or just silently get over the fact and return nil
	if len(rrSet) < 1 {
		return rrSet, nil
	}

	if signingKey == nil {
		err := errors.New("DNSSEC is not initialized")
		return rrSet, err
	}

	p := *signingKey.PrivKey
	ttl := rrSet[0].Header().Ttl
	sig := makeRRSIG(signingKey.PubKey, ttl)
	err := sig.Sign(p.(*rsa.PrivateKey), rrSet)
	if err != nil {
		log.Printf("DNSSEC: cannot sign RR %v", err)
		return rrSet, err
	}

	rrSet = append(rrSet, sig)
	return rrSet, err
}

func GetDNSKEY() (rrset []dns.RR, err error) {
	zsk, err := dns.NewRR(zsk.PubKey.String())
	if err != nil {
		return nil, err
	}
	ksk, err := dns.NewRR(ksk.PubKey.String())
	if err != nil {
		return nil, err
	}
	return []dns.RR{zsk, ksk}, nil
}

func Initialize(appDatadir string, hostname string, zskPrefix string, kskPrefix string) (err error) {

	zone = dns.Fqdn(hostname)
	keyStorageDir = appDatadir
	rrSigInceptionTime = time.Now()

	if len(zskPrefix) < 1 || len(kskPrefix) < 1 {
		log.Printf("DNSSEC signing keys not loaded")
		return nil
	}

	zsk, err = loadSigningKey(path.Join(appDatadir, zskPrefix))
	if err != nil {
		return err
	}

	ksk, err = loadSigningKey(path.Join(appDatadir, kskPrefix))
	if err != nil {
		return err
	}

	log.Printf("DNSSEC initialized")
	return nil
}
