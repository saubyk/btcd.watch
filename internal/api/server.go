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
	// addrSem bounds concurrent address scans (nil = unlimited); see
	// acquireScan.
	addrSem chan struct{}
}

// Options are the public-exposure knobs. The zero value disables them all,
// which is right for localhost use; a public deployment should set every
// field (see config.example.yaml).
type Options struct {
	// RateLimitPerMin is each client's token budget per minute across
	// /api, weighted by routeCost; 0 disables rate limiting.
	RateLimitPerMin int
	// RateLimitBurst caps how much unused budget accumulates; 0 means a
	// full minute's worth.
	RateLimitBurst int
	// TrustedProxyHeader names the header carrying the real client IP
	// (e.g. "CF-Connecting-IP"); empty trusts the socket address only.
	TrustedProxyHeader string
	// MaxConcurrentScans bounds in-flight address history scans — the
	// most node-expensive operation; 0 is unlimited.
	MaxConcurrentScans int
}

// New builds the HTTP handler: all /api routes plus the SPA on every
// other path, wrapped in the hardening middleware.
func New(svc *explorer.Service, backend node.Backend,
	params *chaincfg.Params, network string, hub *Hub,
	static http.Handler, opts Options) http.Handler {

	s := &Server{
		svc:     svc,
		backend: backend,
		params:  params,
		network: network,
		hub:     hub,
	}
	if opts.MaxConcurrentScans > 0 {
		s.addrSem = make(chan struct{}, opts.MaxConcurrentScans)
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

	var handler http.Handler = mux
	if opts.RateLimitPerMin > 0 {
		limiter := newRateLimiter(opts.RateLimitPerMin, opts.RateLimitBurst)
		handler = withRateLimit(handler, limiter, opts.TrustedProxyHeader)
	}
	return securityHeaders(handler)
}
