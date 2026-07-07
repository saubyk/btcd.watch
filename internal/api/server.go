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
	hub     *Hub
}

// New builds the HTTP handler: all /api routes plus the SPA on every
// other path.
func New(svc *explorer.Service, backend node.Backend,
	params *chaincfg.Params, network string, hub *Hub,
	static http.Handler) http.Handler {

	s := &Server{
		svc:     svc,
		backend: backend,
		params:  params,
		network: network,
		hub:     hub,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/healthz", s.handleHealthz)
	mux.HandleFunc("GET /api/tx/{txid}", s.handleTx)
	mux.HandleFunc("GET /api/address/{addr}", s.handleAddress)
	mux.HandleFunc("GET /api/search", s.handleSearch)
	mux.HandleFunc("GET /api/block/{ref}", s.handleBlock)
	mux.HandleFunc("GET /api/fees", s.handleFees)
	mux.HandleFunc("GET /api/stats", s.handleStats)
	mux.HandleFunc("GET /api/ws", s.handleWS)
	// Unknown /api paths are JSON 404s, never the SPA fallback (every
	// specific /api route above wins over this catch-all).
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "unknown_endpoint",
			"unknown API endpoint")
	})
	if static != nil {
		// "/" is the least specific pattern, so every route above still
		// wins.
		mux.Handle("/", static)
	}
	return mux
}
