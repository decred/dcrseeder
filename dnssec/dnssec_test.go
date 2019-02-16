package dnssec

import (
	"github.com/miekg/dns"
	"io/ioutil"
	"testing"
	"time"
)

var (
	zskKeyPrefix = "Ktestnet-seed.stakey.org.+010+40070"
	zskPubStr    = "testnet-seed.stakey.org.	IN	DNSKEY	256 3 10 AwEAAeUV2zcrmU2EDs2DzBWzqD3ymZuhFbTJLos+9MTanaEnO4R0d4VM9kLuPB3hBjRsVvituWnCo8FkDvCOdaH66JVn4riq0dhXLXkIAn1XYSOd/mDUgMDePyDPzzFc2kopvg0zvb+XOH3mK+TZ7xubwc1ZLaO0r+/+F+ZrI9obtu3x"
	zskPrivStr   = `Private-key-format: v1.2
Algorithm: 10 (RSASHA512)
Modulus: 5RXbNyuZTYQOzYPMFbOoPfKZm6EVtMkuiz70xNqdoSc7hHR3hUz2Qu48HeEGNGxW+K25acKjwWQO8I51ofrolWfiuKrR2FcteQgCfVdhI53+YNSAwN4/IM/PMVzaSim+DTO9v5c4feYr5NnvG5vBzVkto7Sv7/4X5msj2hu27fE=
PublicExponent: AQAB
PrivateExponent: KWlNCmkYOln/7wi/MMEcTa54NBjnepnPjx5fUuKOEh6sdKI1JOSns6urNF+EJp/bDPMijErCHWiABt5Jx3E678ecOK2iP0OoK84inCXmpNmK6OTBvjc2EToJ2cYFvbYVxaIn+rJx+hCeto5UteSMksCDCAqD8mWS0oE8koAINRE=
Prime1: 9AupeSX33rtcr8WJ198DK7H4qjVeHGTtuukQB90W3xMDrtWsXUV0/fpXaWdHNPDGqerKpABlAlpvk7JAMcRtLQ==
Prime2: 8E6X34Qj73U2YHDDV7ANCabqu6o/y4xxAqNGyGmTENqgPg87xukxFikOcYTSPDGcdCTdT3Ak+nXfVzcXFLImVQ==
Exponent1: 3h2/IYRtFUtyEIi57MANIrfYmxH3leBGftegv4d6SY4EzButxTZyRLaU2FondQevyPbpeFrjlEC7TLHvu1wMAQ==
Exponent2: xD5qqI4xCoyeK4PrAuEyxH8bksYl8wRuBclxNJmDEHB6DDREjNxCyeYddXcSeTXKns68LPNYP3GjQoYqwyv5QQ==
Coefficient: oLV/tUX4/s4YaVRnLzIN6xekaH+fmK28PcO8gSoigY4Ih8uaxVaIcyb5NZawAVXr+eE6JovZGsbnKk1oJ7GRWw==
`

	kskKeyPrefix = "Ktestnet-seed.stakey.org.+010+35304"
	kskPubStr    = "testnet-seed.stakey.org.	IN	DNSKEY	257 3 10 AwEAAa1S53sZh1ncRpHBtJT6TjqtFs94wnTqGCGm7rrXlvXaiLSxWtyNTbzk7rYWgsBEb0WiRxofKSTd49ud3PmPEtL3g2+mRY2+xQyOKCIVpkkFGdMEMFONKg7GbAu3bD8K/RoQCLoLbp050PpPx2wsTRN5omAY2T65qM5zYJfJM3Sl"
	kskPrivStr   = `Private-key-format: v1.2
Algorithm: 10 (RSASHA512)
Modulus: rVLnexmHWdxGkcG0lPpOOq0Wz3jCdOoYIabuuteW9dqItLFa3I1NvOTuthaCwERvRaJHGh8pJN3j253c+Y8S0veDb6ZFjb7FDI4oIhWmSQUZ0wQwU40qDsZsC7dsPwr9GhAIugtunTnQ+k/HbCxNE3miYBjZPrmoznNgl8kzdKU=
PublicExponent: AQAB
PrivateExponent: glOnaYHNq70ddzYfUjJQpoBGeaUFGyJ3GL7MHcREaAN17eC6QMMjpBjEgji1AluzC7o1Gqg5qNYMIrQ2V5TEgo7+TSPRnEbh42324Pl8fHaL/Hh97AVXzIID/KiCawdlpEnTr0H0z8csJRVSFRPLl3uJ00YCRbKTIbQxAzWk7DU=
Prime1: 1DPoqd5PClbT+1nVtL4vUnIcjkuv08o5MjPGivHXgRVaBqgvDjEqZv+UIFEp+OX40BV+FFvIsqi6VMjOL1uqHw==
Prime2: 0RjDCIY6FX8g+UdwVKOZvepVCbJZo1v4VXpx1kX9faGkKdsYesujynMJmBYqkIicIRK/DoDG5GHfD+yJAlvQuw==
Exponent1: zqjTEQPpNCeFgQdnQgPqMD/joY0Cap9J/qM/27dVamgx6cPHN+oX4oFLcAG7f6PwIi6cQBV3Ks95z/JUIvkBfw==
Exponent2: G5KKVVtt2VvUO0riUybnpRV7dTXhgBsmmg71Z+3+yUxBW4uapMapqI6W20lA/6IkBHB2ZTEyCPem9HCaeIcm9Q==
Coefficient: q9RyLYzWlhSDGYPnsvb1cOHDuoWZzqh4pBJrq7eC0BF/713q31NmLWCnIH8FU41a44K3QQDg1aLEPZbN9YMziA==
`

	zsk2KeyPrefix = "Ktestnet-seed.stakey.org.+010+31082"
	ksk2KeyPrefix = "Ktestnet-seed.stakey.org.+010+34266"
)

// testRR ret	urns the RR from string s. The error is thrown away.
func testRR(s string) dns.RR {
	r, _ := dns.NewRR(s)
	return r
}

func TestZskTagAsString(t *testing.T) {

	xk := testRR(zskPubStr)
	k := xk.(*dns.DNSKEY)
	keyTagStr := KeyTagAsString(k)

	if keyTagStr != zskKeyPrefix {
		t.Error("key tag string invalid", keyTagStr)
	}
}
func TestKskTagAsString(t *testing.T) {

	xk := testRR(kskPubStr)
	k := xk.(*dns.DNSKEY)
	keyTagStr := KeyTagAsString(k)

	if keyTagStr != kskKeyPrefix {
		t.Error("key tag string invalid", keyTagStr)
	}
}

func TestGenerateZSK(t *testing.T) {

	key, privKey, err := generateSigningKey("testnet-seed.stakey.org.", 1024, 256, 3600)

	if err != nil {
		t.Error("generateSigningKey should not return err")
	}
	if key.Flags != 256 {
		t.Error("invalid keyflag")
	}
	if privKey == nil {
		t.Error("pkey should not be nil")
	}
}

func TestGenerateKSK(t *testing.T) {

	key, privKey, err := generateSigningKey("testnet-seed.stakey.org.", 1024, 257, 3600)

	if err != nil {
		t.Error("generateSigningKey should not return err")
	}
	if key.Flags != 257 {
		t.Error("invalid keyflag")
	}
	if privKey == nil {
		t.Error("pkey should not be nil")
	}
	ds := key.ToDS(dns.SHA256)
	if ds == nil {
		t.Error("ToDS shouldn't return nil")
	}
	if len(ds.Digest) < 10 {
		t.Error("can't get DS from key")
	}
}

func TestGenerateSigningKeys(t *testing.T) {
	tmpDir, _ := ioutil.TempDir("/tmp", "dcrseeder")
	err := Initialize(tmpDir, "testnet-seed.stakey.org.", "", "")
	if err != nil {
		t.Errorf("Initialize failed: %v", err)
	}
	zskTag, err := GenerateSigningKeys("testnet-seed.stakey.org.", 1024, 256)
	if zskTag == "" || err != nil {
		t.Errorf("GenerateSigningKeys returns nil: %v", err)
	}
	kskTag, err := GenerateSigningKeys("testnet-seed.stakey.org.", 1024, 257)
	if kskTag == "" || err != nil {
		t.Errorf("GenerateSigningKeys returns nil: %v", err)
	}
	err = Initialize(tmpDir, "testnet-seed.stakey.org.", zskTag, kskTag)
	if err != nil {
		t.Errorf("failed to reload generated keys: %v", err)
	}
	if KeyTagAsString(zsk.PubKey) != zskTag {
		t.Errorf("zsk2KeyPrefix does not match %s", KeyTagAsString(zsk.PubKey))
	}
	if KeyTagAsString(ksk.PubKey) != kskTag {
		t.Errorf("ksk2KeyPrefix does not match %s", KeyTagAsString(ksk.PubKey))
	}
}

func TestInitializeExistingKeysFileNotFound(t *testing.T) {
	err := Initialize("../nonexistentdir", "testnet-seed.stakey.org", zsk2KeyPrefix, ksk2KeyPrefix)
	if err == nil {
		t.Error("Initialize should return err")
	}
}

func TestInitializeExistingKeys(t *testing.T) {
	err := Initialize("../testdata", "testnet-seed.stakey.org", zsk2KeyPrefix, ksk2KeyPrefix)
	if err != nil {
		t.Errorf("Initialize failed: %v", err)
	}
	if KeyTagAsString(zsk.PubKey) != zsk2KeyPrefix {
		t.Errorf("zsk2KeyPrefix does not match %s", zsk2KeyPrefix)
	}
	if KeyTagAsString(ksk.PubKey) != ksk2KeyPrefix {
		t.Errorf("ksk2KeyPrefix does not match %s", ksk2KeyPrefix)
	}
}

func TestInitializeExistingKeysFail(t *testing.T) {
	err := Initialize("../testdata", "testnet-seed.stakey.org", "zsk-noexist", ksk2KeyPrefix)
	if err == nil {
		t.Error("Initialize should fail")
	}
	err = Initialize("../testdata", "testnet-seed.stakey.org", zsk2KeyPrefix, "ksk-noexist")
	if err == nil {
		t.Error("Initialize should fail")
	}
}

func TestMakeRRSIG(t *testing.T) {
	err := Initialize("../testdata", "testnet-seed.stakey.org", "Ktestnet-seed.stakey.org.+010+6257", "Ktestnet-seed.stakey.org.+010+61233")
	if err != nil {
		t.Error("initialize() failed")
	}
	rr := testRR("testnet-seed.stakey.org. 5086	IN	NS	seed.stakey.org.")
	sig, err := SignRRSetWithZsk([]dns.RR{rr})
	if err != nil || sig == nil {
		t.Error("SignRRSetWithZsk returned err")
	}
	if len(sig) < 2 {
		t.Error("missing signature")
	}
	rrSIG := sig[len(sig)-1].(*dns.RRSIG)
	if rrSIG.Signature == "" {
		t.Error("empty signature")
	}
	if rrSIG.ValidityPeriod(time.Now()) == false {
		t.Error("incorrect validity period ")
	}
	if rrSIG.Hdr.Name != "testnet-seed.stakey.org." {
		t.Error("invalid name on signature")
	}
}

func TestSignRRSet(t *testing.T) {
	//	 TODO
}
