// Copyright (c) 2020 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/decred/dcrd/wire"
	"github.com/decred/dcrseeder/api"
)

const defaultHTTPTimeout = 10 * time.Second

func httpGetAddrs(w http.ResponseWriter, r *http.Request) {
	var wantedIP uint32
	var wantedPV uint32
	var wantedSF wire.ServiceFlag

	query := r.URL.Query()

	requestedIP := query.Get(api.IPVersion)
	if requestedIP != "" {
		u, _ := strconv.ParseUint(requestedIP, 10, 32)
		if u == 4 || u == 6 {
			wantedIP = uint32(u)
		}
	}

	requestedPV := query.Get(api.ProtocolVersion)
	if requestedPV != "" {
		u, _ := strconv.ParseUint(requestedPV, 10, 32)
		wantedPV = uint32(u)
	}

	requestedSF := query.Get(api.ServiceFlag)
	if requestedSF != "" {
		u, _ := strconv.ParseUint(requestedSF, 10, 64)
		wantedSF = wire.ServiceFlag(u)
	}

	nodes := amgr.GoodAddresses(wantedIP, wantedPV, wantedSF)

	flush, ok := w.(http.Flusher)
	if !ok {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8") // not a json array
	// Replace the Server response header. When used with nginx's "server_tokens
	// off;" and "proxy_pass_header Server;" options.
	w.Header().Set("Server", appName)
	w.WriteHeader(http.StatusOK)
	flush.Flush()

	enc := json.NewEncoder(w)

	ctx := r.Context()
	for _, node := range nodes {
		select {
		case <-ctx.Done():
			return
		default:
			err := enc.Encode(node)
			if err != nil {
				log.Printf("httpGetAddrs: Encode failed: %v", err)
			}
			flush.Flush()
		}
	}
}

func serveHTTP(ctx context.Context, addr string) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("can't listen on %s. web server quitting: %w", addr, err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc(api.GetAddrsPath, httpGetAddrs)

	srv := &http.Server{
		Handler:      mux,
		ReadTimeout:  defaultHTTPTimeout, // slow requests should not hold connections opened
		WriteTimeout: defaultHTTPTimeout, // request to response time
	}

	// Shutdown the server on context cancellation.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-ctx.Done()
		ctxShutdown, cancel := context.WithTimeout(context.Background(), defaultHTTPTimeout)
		defer cancel()
		err := srv.Shutdown(ctxShutdown)
		if err != nil {
			log.Printf("Trouble shutting down HTTP server: %v", err)
		}
	}()
	defer wg.Wait()

	err = srv.Serve(listener) // blocking
	if !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("unexpected (http.Server).Serve error: %w", err)
	}
	return nil // Shutdown called
}
