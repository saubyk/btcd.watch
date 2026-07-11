package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"btcdwatch.com/internal/chain"
	"btcdwatch.com/internal/explorer"
	"btcdwatch.com/internal/node"
)

// Cache-Control helpers. Responses embedding confirmation counts change
// every block, so even "immutable" data only gets a short public TTL —
// enough for browsers (and an edge cache) to absorb bursts and watch-mode
// polling without ever showing stale confirmations for long.
func cachePublic(w http.ResponseWriter, seconds int) {
	w.Header().Set("Cache-Control", "public, max-age="+strconv.Itoa(seconds))
}

func noStore(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
}

const (
	confirmedTTL = 30 // seconds; tx + block responses
	liveTTL      = 5  // seconds; stats + fees
)

// scanQueueWait bounds how long an address request waits for a scan slot
// before giving up with 503.
const scanQueueWait = 10 * time.Second

// acquireScan takes a slot from the address-scan semaphore, waiting until
// the client hangs up or the queue-wait budget runs out. The returned
// release must be called when the scan finishes. Address history is the
// one endpoint that can hold the node busy for seconds per call
// (searchrawtransactions), so it gets an explicit concurrency ceiling.
func (s *Server) acquireScan(r *http.Request) (release func(), ok bool) {
	if s.addrSem == nil {
		return func() {}, true
	}
	select {
	case s.addrSem <- struct{}{}:
		return func() { <-s.addrSem }, true
	case <-r.Context().Done():
		return nil, false
	case <-time.After(scanQueueWait):
		return nil, false
	}
}

func writeScanBusy(w http.ResponseWriter) {
	w.Header().Set("Retry-After", "10")
	writeError(w, http.StatusServiceUnavailable, "busy",
		"too many address lookups in flight — try again shortly")
}

// gateSyncing rejects lookups while the node is still catching up: the
// indexes only cover the synced portion of the chain, so answers would be
// missing or misleading. Stats/fees stay up so the UI can show the
// syncing state. Returns true when the request was rejected.
func (s *Server) gateSyncing(w http.ResponseWriter) bool {
	if !s.svc.Syncing() {
		return false
	}
	writeError(w, http.StatusServiceUnavailable, "node_syncing",
		"the node is still syncing the blockchain — lookups will work "+
			"once it catches up")
	return true
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		slog.Error("encode response", "err", err)
	}
}

func writeError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, map[string]errorBody{
		"error": {Code: code, Message: msg},
	})
}

// writeServiceError maps explorer/node errors to API errors. Not-found is
// endpoint-specific and handled by callers first.
func writeServiceError(w http.ResponseWriter, err error) {
	if errors.Is(err, node.ErrUnavailable) {
		writeError(w, http.StatusServiceUnavailable, "node_unavailable",
			"the bitcoin node is currently unreachable")
		return
	}
	slog.Error("request failed", "err", err)
	writeError(w, http.StatusInternalServerError, "internal_error",
		"internal error")
}

// healthMaxStale is how long the node may go without answering the sync
// probe before healthz reports degraded. Generous because btcd routinely
// stalls RPC for minutes during UTXO cache flushes — a hiccup that long
// shouldn't page anyone, but a node that has been mute for this long is a
// real outage.
const healthMaxStale = 15 * time.Minute

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	type health struct {
		Status        string `json:"status"`
		Network       string `json:"network"`
		NodeConnected bool   `json:"nodeConnected"`
		Syncing       bool   `json:"syncing,omitempty"`
		BlockHeight   int64  `json:"blockHeight,omitempty"`
	}

	// Served entirely from cached state — healthz must answer instantly
	// even while btcd stalls its RPC interface (UTXO cache flushes block
	// calls for minutes, long past Cloudflare's 100s origin timeout).
	st := s.svc.SyncStatus()

	connected := true
	if c, ok := s.backend.(interface{ Connected() bool }); ok {
		connected = c.Connected()
	}
	if !connected {
		writeJSON(w, http.StatusServiceUnavailable, health{
			Status:  "degraded",
			Network: s.network,
		})
		return
	}

	// Connected but the node hasn't answered a probe in a long while:
	// the RPC interface is wedged. Before the first success the window
	// is anchored at process start so a fresh deploy isn't degraded.
	last := st.CheckedAt
	if last.IsZero() {
		last = s.started
	}
	if time.Since(last) > healthMaxStale {
		writeJSON(w, http.StatusServiceUnavailable, health{
			Status:        "degraded",
			Network:       s.network,
			NodeConnected: true,
			BlockHeight:   st.TipHeight,
		})
		return
	}

	// Syncing is 200, not 503: the service itself is healthy and the UI
	// shows the state — an uptime monitor shouldn't page over IBD.
	status := "ok"
	if st.Syncing {
		status = "syncing"
	}
	writeJSON(w, http.StatusOK, health{
		Status:        status,
		Network:       s.network,
		NodeConnected: true,
		Syncing:       st.Syncing,
		BlockHeight:   st.TipHeight,
	})
}

func (s *Server) handleTx(w http.ResponseWriter, r *http.Request) {
	if s.gateSyncing(w) {
		return
	}
	query := chain.ClassifyQuery(r.PathValue("txid"), s.params)
	if query.Kind != chain.QueryHex {
		writeError(w, http.StatusBadRequest, "invalid_txid",
			"txid must be 64 hex characters")
		return
	}

	tx, err := s.svc.GetTx(query.Hex)
	if err != nil {
		if errors.Is(err, explorer.ErrTxNotFound) {
			writeError(w, http.StatusNotFound, "tx_not_found",
				"transaction not found")
			return
		}
		writeServiceError(w, err)
		return
	}
	// Pending payloads change with every mempool tick; confirmed ones
	// only per block.
	if tx.Status == "confirmed" {
		cachePublic(w, confirmedTTL)
	} else {
		noStore(w)
	}
	writeJSON(w, http.StatusOK, tx)
}

const (
	defaultActivityLimit = 25
	maxActivityLimit     = 100
)

func (s *Server) handleAddress(w http.ResponseWriter, r *http.Request) {
	if s.gateSyncing(w) {
		return
	}
	query := chain.ClassifyQuery(r.PathValue("addr"), s.params)
	if query.Kind != chain.QueryAddress {
		writeError(w, http.StatusBadRequest, "invalid_address",
			"not a valid address for this network")
		return
	}

	offset := intParam(r, "offset", 0, 0, 1<<30)
	limit := intParam(r, "limit", defaultActivityLimit, 1, maxActivityLimit)

	release, ok := s.acquireScan(r)
	if !ok {
		writeScanBusy(w)
		return
	}
	defer release()

	summary, err := s.svc.Address(query.Address, offset, limit)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	noStore(w)
	writeJSON(w, http.StatusOK, summary)
}

func intParam(r *http.Request, name string, def, lo, hi int) int {
	v := r.URL.Query().Get(name)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return min(max(n, lo), hi)
}

func (s *Server) handleFees(w http.ResponseWriter, r *http.Request) {
	fees, err := s.svc.Fees()
	if err != nil {
		writeServiceError(w, err)
		return
	}
	cachePublic(w, liveTTL)
	writeJSON(w, http.StatusOK, fees)
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.svc.Stats()
	if err != nil {
		writeServiceError(w, err)
		return
	}
	cachePublic(w, liveTTL)
	writeJSON(w, http.StatusOK, stats)
}

const defaultBlockTxLimit = 25

func (s *Server) handleBlock(w http.ResponseWriter, r *http.Request) {
	if s.gateSyncing(w) {
		return
	}
	offset := intParam(r, "offset", 0, 0, 1<<30)
	limit := intParam(r, "limit", defaultBlockTxLimit, 1, maxActivityLimit)

	var (
		block *explorer.Block
		err   error
	)
	switch query := chain.ClassifyQuery(r.PathValue("ref"), s.params); query.Kind {
	case chain.QueryBlockHeight:
		block, err = s.svc.BlockByHeight(query.Height, offset, limit)
	case chain.QueryHex:
		block, err = s.svc.BlockByHash(query.Hex, offset, limit)
	default:
		writeError(w, http.StatusBadRequest, "invalid_block",
			"block reference must be a height or a 64-hex block hash")
		return
	}
	if err != nil {
		if errors.Is(err, explorer.ErrBlockNotFound) {
			writeError(w, http.StatusNotFound, "block_not_found",
				"block not found")
			return
		}
		writeServiceError(w, err)
		return
	}
	cachePublic(w, confirmedTTL)
	writeJSON(w, http.StatusOK, block)
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	if s.gateSyncing(w) {
		return
	}
	q := r.URL.Query().Get("q")

	query := chain.ClassifyQuery(q, s.params)
	switch query.Kind {
	case chain.QueryBlockHeight:
		s.searchBlockByHeight(w, q, query.Height)

	case chain.QueryHex:
		s.searchHex(w, q, query.Hex)

	case chain.QueryAddress:
		release, ok := s.acquireScan(r)
		if !ok {
			writeScanBusy(w)
			return
		}
		defer release()

		summary, err := s.svc.Address(query.Address, 0, defaultActivityLimit)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		noStore(w)
		writeJSON(w, http.StatusOK, map[string]any{
			"kind":    "address",
			"address": summary,
		})

	default:
		noStore(w)
		writeJSON(w, http.StatusOK, map[string]any{
			"kind":  "invalid",
			"query": q,
		})
	}
}

func (s *Server) searchBlockByHeight(w http.ResponseWriter, q string, height int64) {
	block, err := s.svc.BlockByHeight(height, 0, defaultBlockTxLimit)
	if err != nil {
		if errors.Is(err, explorer.ErrBlockNotFound) {
			writeNotFound(w, q)
			return
		}
		writeServiceError(w, err)
		return
	}
	cachePublic(w, confirmedTTL)
	writeJSON(w, http.StatusOK, map[string]any{
		"kind":  "block",
		"block": block,
	})
}

// searchHex resolves a 64-hex query, which may be a txid or a block hash.
// The leading-zeros form of a mined block hash is checked as a block
// first; otherwise the txid interpretation wins and the block hash lookup
// is the fallback (regtest hashes rarely carry visible leading zeros).
func (s *Server) searchHex(w http.ResponseWriter, q, hex string) {
	first, second := s.txResult, s.blockResult
	if strings.HasPrefix(hex, "00000000") {
		first, second = second, first
	}

	result, err := first(hex)
	if isNotFound(err) {
		result, err = second(hex)
	}
	switch {
	case err == nil:
		if result.maxAge > 0 {
			cachePublic(w, result.maxAge)
		} else {
			noStore(w)
		}
		writeJSON(w, http.StatusOK, result.payload)
	case isNotFound(err):
		writeNotFound(w, q)
	default:
		writeServiceError(w, err)
	}
}

func isNotFound(err error) bool {
	return errors.Is(err, explorer.ErrTxNotFound) ||
		errors.Is(err, explorer.ErrBlockNotFound)
}

// searchResult pairs a search payload with its cache TTL (0 = no-store),
// which only the resolver knows (pending vs confirmed).
type searchResult struct {
	payload any
	maxAge  int
}

func (s *Server) txResult(txid string) (searchResult, error) {
	tx, err := s.svc.GetTx(txid)
	if err != nil {
		return searchResult{}, err
	}
	maxAge := 0
	if tx.Status == "confirmed" {
		maxAge = confirmedTTL
	}
	return searchResult{
		payload: map[string]any{"kind": "tx", "tx": tx},
		maxAge:  maxAge,
	}, nil
}

func (s *Server) blockResult(hash string) (searchResult, error) {
	block, err := s.svc.BlockByHash(hash, 0, defaultBlockTxLimit)
	if err != nil {
		return searchResult{}, err
	}
	return searchResult{
		payload: map[string]any{"kind": "block", "block": block},
		maxAge:  confirmedTTL,
	}, nil
}

func writeNotFound(w http.ResponseWriter, q string) {
	noStore(w)
	writeJSON(w, http.StatusOK, map[string]any{
		"kind":  "notfound",
		"query": q,
	})
}
