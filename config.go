// Copyright (c) 2018 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/decred/dcrd/chaincfg/v2"
	"github.com/decred/dcrd/dcrutil/v2"
	flags "github.com/jessevdk/go-flags"
)

const (
	defaultConfigFilename = "dcrseeder.conf"
	defaultListenPort     = "5354"
)

var (
	// Default configuration options
	defaultConfigFile = filepath.Join(defaultHomeDir, defaultConfigFilename)
	defaultHomeDir    = dcrutil.AppDataDir("dcrseeder", false)
)

// config defines the configuration options for hardforkdemo.
//
// See loadConfig for details on the configuration load process.
type config struct {
	Host       string `short:"H" long:"host" description:"Seed DNS address"`
	Listen     string `long:"listen" short:"l" description:"Listen on address:port"`
	Nameserver string `short:"n" long:"nameserver" description:"hostname of nameserver"`
	Seeder     string `short:"s" long:"default seeder" description:"IP address of a working node"`
	TestNet    bool   `long:"testnet" description:"Use testnet"`

	netParams *chaincfg.Params
}

func loadConfig() (*config, error) {
	err := os.MkdirAll(defaultHomeDir, 0700)
	if err != nil {
		// Show a nicer error message if it's because a symlink is
		// linked to a directory that does not exist (probably because
		// it's not mounted).
		if e, ok := err.(*os.PathError); ok && os.IsExist(err) {
			if link, lerr := os.Readlink(e.Path); lerr == nil {
				str := "is symlink %s -> %s mounted?"
				err = fmt.Errorf(str, e.Path, link)
			}
		}

		return nil, fmt.Errorf("failed to create home directory: %v", err)
	}

	// Default config.
	cfg := config{
		Listen: normalizeAddress("localhost", defaultListenPort),
	}

	preCfg := cfg
	preParser := flags.NewParser(&preCfg, flags.Default)
	_, err = preParser.Parse()
	if err != nil {
		e, ok := err.(*flags.Error)
		if ok && e.Type == flags.ErrHelp {
			os.Exit(0)
		}
		preParser.WriteHelp(os.Stderr)
		return nil, err
	}

	appName := filepath.Base(os.Args[0])
	appName = strings.TrimSuffix(appName, filepath.Ext(appName))
	usageMessage := fmt.Sprintf("Use %s -h to show usage", appName)

	// Load additional config from file.
	parser := flags.NewParser(&cfg, flags.Default)
	err = flags.NewIniParser(parser).ParseFile(defaultConfigFile)
	if err != nil {
		if _, ok := err.(*os.PathError); !ok {
			fmt.Fprintf(os.Stderr, "Error parsing config "+
				"file: %v\n", err)
			fmt.Fprintln(os.Stderr, usageMessage)
			return nil, err
		}
	}

	// Parse command line options again to ensure they take precedence.
	_, err = parser.Parse()
	if err != nil {
		if e, ok := err.(*flags.Error); !ok || e.Type != flags.ErrHelp {
			parser.WriteHelp(os.Stderr)
		}
		return nil, err
	}

	if len(cfg.Host) == 0 {
		return nil, fmt.Errorf("Please specify a hostname")
	}

	if len(cfg.Nameserver) == 0 {
		return nil, fmt.Errorf("Please specify a nameserver")
	}

	if len(cfg.Seeder) == 0 {
		return nil, fmt.Errorf("Please specify a seeder")
	}

	if net.ParseIP(cfg.Seeder) == nil {
		str := "\"%s\" is not a valid textual representation of an IP address"
		return nil, fmt.Errorf(str, cfg.Seeder)
	}

	cfg.Listen = normalizeAddress(cfg.Listen, defaultListenPort)

	if cfg.TestNet {
		cfg.netParams = chaincfg.TestNet3Params()
	} else {
		cfg.netParams = chaincfg.MainNetParams()
	}

	return &cfg, nil
}

// normalizeAddress returns addr with the passed default port appended if
// there is not already a port specified.
func normalizeAddress(addr, defaultPort string) string {
	_, _, err := net.SplitHostPort(addr)
	if err != nil {
		return net.JoinHostPort(addr, defaultPort)
	}
	return addr
}
