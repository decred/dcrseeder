package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/decred/dcrd/wire"
	"github.com/decred/dcrseeder/api"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
)

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

	w.Header().Set("Transfer-Encoding", "chunked")
	w.WriteHeader(http.StatusOK)
	flush.Flush()

	enc := json.NewEncoder(w)

	ctx := r.Context()
	for _, node := range nodes {
		select {
		case <-ctx.Done():
			log.Printf("client close connection")
			return
		default:
			err := enc.Encode(node)
			if err != nil {
				log.Printf("httpGetAddrs: Encode failed: %v",
					err)
			}
			flush.Flush()
		}
	}
}

func httpServer(listener string) {
	router := mux.NewRouter()
	router.HandleFunc(api.GetAddrsPath,
		httpGetAddrs).Methods(http.MethodGet, http.MethodHead, http.MethodOptions)

	origins := handlers.AllowedOrigins([]string{"*"})
	methods := handlers.AllowedMethods([]string{http.MethodGet, http.MethodHead, http.MethodOptions})

	log.Fatal(http.ListenAndServe(listener, handlers.CORS(origins, methods)(router)))
}
