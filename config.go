// Copyright (c) 2020 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/decred/dcrd/chaincfg/v3"
	"github.com/decred/dcrd/dcrutil/v3"
	flags "github.com/jessevdk/go-flags"
)

const (
	appName               = "dcrseeder"
	defaultConfigFilename = appName + ".conf"
	defaultDNSPort        = "5354"
	defaultHTTPPort       = "8000"
)

var (
	// Default configuration options
	defaultConfigFile = filepath.Join(defaultHomeDir, defaultConfigFilename)
	defaultHomeDir    = dcrutil.AppDataDir(appName, false)
)

// config defines the configuration options for dcrseeder.
//
// See loadConfig for details on the configuration load process.
type config struct {
	Host       string `short:"H" long:"host" description:"DEPRECATED: Seed DNS address"`
	DNSListen  string `long:"dnslisten" description:"DEPRECATED: DNS listen on address:port"`
	HTTPListen string `long:"httplisten" description:"HTTP listen on address:port"`
	Nameserver string `short:"n" long:"nameserver" description:"DEPRECATED: hostname of nameserver"`
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
		var e *os.PathError
		if errors.As(err, &e) && os.IsExist(err) {
			if link, lerr := os.Readlink(e.Path); lerr == nil {
				str := "is symlink %s -> %s mounted?"
				err = fmt.Errorf(str, e.Path, link)
			}
		}

		return nil, fmt.Errorf("failed to create home directory: %v", err)
	}

	// Default config.
	cfg := config{}

	preCfg := cfg
	preParser := flags.NewParser(&preCfg, flags.Default)
	_, err = preParser.Parse()
	if err != nil {
		var e *flags.Error
		if errors.As(err, &e) && e.Type == flags.ErrHelp {
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
		var e *os.PathError
		if !errors.As(err, &e) {
			fmt.Fprintf(os.Stderr, "Error parsing config "+
				"file: %v\n", err)
			fmt.Fprintln(os.Stderr, usageMessage)
			return nil, err
		}
	}

	// Parse command line options again to ensure they take precedence.
	_, err = parser.Parse()
	if err != nil {
		var e *flags.Error
		if !errors.As(err, &e) || e.Type != flags.ErrHelp {
			parser.WriteHelp(os.Stderr)
		}
		return nil, err
	}

	if len(cfg.Seeder) == 0 {
		return nil, fmt.Errorf("no seeder specified")
	}

	if net.ParseIP(cfg.Seeder) == nil {
		str := "\"%s\" is not a valid textual representation of an IP address"
		return nil, fmt.Errorf(str, cfg.Seeder)
	}

	if cfg.DNSListen == "" && cfg.HTTPListen == "" {
		return nil, fmt.Errorf("no listeners specified")
	}
	if cfg.DNSListen != "" {
		fmt.Fprintln(os.Stderr, "The --dnslisten option is deprecated: use --httplisten")
		if len(cfg.Host) == 0 {
			return nil, fmt.Errorf("no hostname specified")
		}

		if len(cfg.Nameserver) == 0 {
			return nil, fmt.Errorf("no nameserver specified")
		}

		cfg.DNSListen = normalizeAddress(cfg.DNSListen, defaultDNSPort)
	}
	if cfg.HTTPListen != "" {
		cfg.HTTPListen = normalizeAddress(cfg.HTTPListen, defaultHTTPPort)
	}

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
