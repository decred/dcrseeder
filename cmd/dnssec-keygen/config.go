// Copyright (c) 2018 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"github.com/decred/dcrd/chaincfg"
	"github.com/decred/dcrd/dcrutil"
	"github.com/jessevdk/go-flags"
	"os"
	"path/filepath"
)

const (
	defaultConfigFilename = "dcrseeder.conf"
)

var (
	// Default network parameters
	activeNetParams = &chaincfg.MainNetParams

	// Default configuration options
	defaultConfigFile = filepath.Join(defaultHomeDir, defaultConfigFilename)
	defaultHomeDir    = dcrutil.AppDataDir("dcrseeder", false)
)

// config defines the configuration options for hardforkdemo.
//
// See loadConfig for details on the configuration load process.
type config struct {
	Host string `short:"H" long:"host" description:"Seed DNS address"`
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

		str := "failed to create home directory: %v"
		err := fmt.Errorf(str, err)
		fmt.Fprintln(os.Stderr, err)
		return nil, err
	}

	// Default config.
	cfg := config{}

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

	// Load additional config from file.
	parser := flags.NewParser(&cfg, flags.Default)

	// Parse command line options again to ensure they take precedence.
	_, err = parser.Parse()
	if err != nil {
		if e, ok := err.(*flags.Error); !ok || e.Type != flags.ErrHelp {
			parser.WriteHelp(os.Stderr)
		}
		return nil, err
	}

	if len(cfg.Host) == 0 {
		str := "Please specify a hostname"
		err := fmt.Errorf(str)
		fmt.Fprintln(os.Stderr, err)
		return nil, err
	}

	return &cfg, nil
}
