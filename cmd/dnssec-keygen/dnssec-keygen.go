// Copyright (c) 2018 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"github.com/decred/dcrseeder/dnssec"
	"github.com/miekg/dns"
	"log"
	"os"
)

func main() {
	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "loadConfig: %v\n", err)
		os.Exit(1)
	}

	err = dnssec.Initialize(defaultHomeDir, cfg.Host, "", "")
	if err != nil {
		log.Printf("initialize failed: %v", err)
		os.Exit(1)
	}
	name := dns.Fqdn(cfg.Host)

	log.Print("Generating ZSK")
	newZsk, err := dnssec.GenerateSigningKeys(name, 1024, 256)
	if err != nil {
		os.Exit(1)
	}
	log.Print("Generating KSK")
	newKsk, err := dnssec.GenerateSigningKeys(name, 1024, 257)
	if err != nil {
		os.Exit(1)
	}

	msgFmt :=
		`# Create/update the DS record in the parent zone:
%s/%s.ds

# Add the below section to %s/dcrseeder.conf:

zsk=%s
ksk=%s

`

	fmt.Printf(msgFmt, defaultHomeDir, newKsk, defaultHomeDir, newZsk, newKsk)

}
