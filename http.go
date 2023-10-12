// Copyright (c) 2018-2023 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"encoding/json"
	"errors"
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

func httpGetAddrs(w http.ResponseWriter, r *http.Request, amgr *Manager, log *log.Logger) {
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

type server struct {
	srv      *http.Server
	listener net.Listener
	log      *log.Logger
}

func newServer(addr string, amgr *Manager, log *log.Logger) (*server, error) {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}

	mux := http.NewServeMux()
	mux.HandleFunc(api.GetAddrsPath, func(w http.ResponseWriter, r *http.Request) {
		httpGetAddrs(w, r, amgr, log)
	})

	srv := &http.Server{
		Handler:      mux,
		ReadTimeout:  defaultHTTPTimeout, // slow requests should not hold connections opened
		WriteTimeout: defaultHTTPTimeout, // request to response time
	}

	return &server{
		srv:      srv,
		listener: listener,
		log:      log,
	}, nil
}

func (h *server) run(ctx context.Context) {
	var wg sync.WaitGroup

	// Add the graceful shutdown to the waitgroup.
	wg.Add(1)
	go func() {
		defer wg.Done()
		// Wait until context is canceled before shutting down the server.
		<-ctx.Done()
		_ = h.srv.Shutdown(ctx)
	}()

	// Start webserver.
	wg.Add(1)
	go func() {
		defer wg.Done()

		h.log.Printf("Listening on %s", h.listener.Addr())
		err := h.srv.Serve(h.listener)
		// ErrServerClosed is expected from a graceful server shutdown, it can
		// be ignored. Anything else should be logged.
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			h.log.Printf("unexpected (http.Server).Serve error: %v", err)
		}
	}()

	wg.Wait()
}
