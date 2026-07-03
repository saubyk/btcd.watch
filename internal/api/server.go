// Package api is the HTTP layer: routing, handlers, and JSON envelopes.
package api

import (
	"net/http"

	"github.com/btcsuite/btcd/chaincfg/v2"

	"btcdwatch.com/internal/explorer"
	"btcdwatch.com/internal/node"
)

// Server routes the /api endpoints.
type Server struct {
	svc     *explorer.Service
	backend node.Backend
	params  *chaincfg.Params
	network string
}

// New builds the HTTP handler for all /api routes.
func New(svc *explorer.Service, backend node.Backend,
	params *chaincfg.Params, network string) http.Handler {

	s := &Server{
		svc:     svc,
		backend: backend,
		params:  params,
		network: network,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/healthz", s.handleHealthz)
	mux.HandleFunc("GET /api/tx/{txid}", s.handleTx)
	mux.HandleFunc("GET /api/search", s.handleSearch)
	mux.HandleFunc("GET /api/fees", s.handleFees)
	mux.HandleFunc("GET /api/stats", s.handleStats)
	mux.HandleFunc("GET /api/examples", s.handleExamples)
	return mux
}
