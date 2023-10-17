// Copyright (c) 2018-2023 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"strings"

	"github.com/decred/dcrd/chaincfg/v3"
	"github.com/decred/dcrd/dcrutil/v4"
	flags "github.com/jessevdk/go-flags"
)

const (
	appName               = "dcrseeder"
	defaultConfigFilename = appName + ".conf"
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
	Mainnet *netConfig `group:"Mainnet" namespace:"mainnet"`
	Testnet *netConfig `group:"Testnet" namespace:"testnet"`
}

type netConfig struct {
	Enabled bool   `long:"enabled" description:"Enable dcrseeder on this network"`
	Listen  string `long:"listen" description:"HTTP listen on address:port (must be unique per network)"`
	Seeder  string `long:"seeder" description:"IP address of a working node on this network"`

	netParams *chaincfg.Params
	seederIP  netip.AddrPort
	dataDir   string
}

func loadConfig() (*config, error) {
	err := os.MkdirAll(defaultHomeDir, 0o700)
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

	if !cfg.Mainnet.Enabled && !cfg.Testnet.Enabled {
		return nil, fmt.Errorf("no networks enabled")
	}

	parseNet := func(cfg *netConfig, params *chaincfg.Params) error {
		// Only parse params for this network if it is enabled.
		if !cfg.Enabled {
			return nil
		}

		cfg.netParams = params
		cfg.dataDir = filepath.Join(defaultHomeDir, cfg.netParams.Name)

		if cfg.Listen == "" {
			return fmt.Errorf("no listeners specified")
		}
		cfg.Listen = normalizeAddress(cfg.Listen, defaultHTTPPort)

		if len(cfg.Seeder) == 0 {
			return fmt.Errorf("no seeder specified")
		}
		cfg.Seeder = normalizeAddress(cfg.Seeder, cfg.netParams.DefaultPort)

		cfg.seederIP, err = netip.ParseAddrPort(cfg.Seeder)
		if err != nil {
			return fmt.Errorf("invalid seeder ip: %v", err)
		}

		return nil
	}

	err = parseNet(cfg.Mainnet, chaincfg.MainNetParams())
	if err != nil {
		return nil, fmt.Errorf("mainnet params error: %w", err)
	}

	err = parseNet(cfg.Testnet, chaincfg.TestNet3Params())
	if err != nil {
		return nil, fmt.Errorf("testnet params error: %w", err)
	}

	return &cfg, nil
}

// normalizeAddress returns addr with the passed default port appended if
// there is not already a port specified.
func normalizeAddress(addr, defaultPort string) string {
	if _, _, err := net.SplitHostPort(addr); err != nil {
		return net.JoinHostPort(addr, defaultPort)
	}
	return addr
}
