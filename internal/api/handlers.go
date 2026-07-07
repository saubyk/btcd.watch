package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"btcdwatch.com/internal/chain"
	"btcdwatch.com/internal/explorer"
	"btcdwatch.com/internal/node"
)

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

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	type health struct {
		Status        string `json:"status"`
		Network       string `json:"network"`
		NodeConnected bool   `json:"nodeConnected"`
		BlockHeight   int64  `json:"blockHeight,omitempty"`
	}

	height, err := s.backend.GetBlockCount()
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, health{
			Status:        "degraded",
			Network:       s.network,
			NodeConnected: false,
		})
		return
	}
	writeJSON(w, http.StatusOK, health{
		Status:        "ok",
		Network:       s.network,
		NodeConnected: true,
		BlockHeight:   height,
	})
}

func (s *Server) handleTx(w http.ResponseWriter, r *http.Request) {
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
	writeJSON(w, http.StatusOK, tx)
}

const (
	defaultActivityLimit = 25
	maxActivityLimit     = 100
)

func (s *Server) handleAddress(w http.ResponseWriter, r *http.Request) {
	query := chain.ClassifyQuery(r.PathValue("addr"), s.params)
	if query.Kind != chain.QueryAddress {
		writeError(w, http.StatusBadRequest, "invalid_address",
			"not a valid address for this network")
		return
	}

	offset := intParam(r, "offset", 0, 0, 1<<30)
	limit := intParam(r, "limit", defaultActivityLimit, 1, maxActivityLimit)

	summary, err := s.svc.Address(query.Address, offset, limit)
	if err != nil {
		writeServiceError(w, err)
		return
	}
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
	writeJSON(w, http.StatusOK, fees)
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.svc.Stats()
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

const defaultBlockTxLimit = 25

func (s *Server) handleBlock(w http.ResponseWriter, r *http.Request) {
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
	writeJSON(w, http.StatusOK, block)
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")

	query := chain.ClassifyQuery(q, s.params)
	switch query.Kind {
	case chain.QueryBlockHeight:
		s.searchBlockByHeight(w, q, query.Height)

	case chain.QueryHex:
		s.searchHex(w, q, query.Hex)

	case chain.QueryAddress:
		summary, err := s.svc.Address(query.Address, 0, defaultActivityLimit)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"kind":    "address",
			"address": summary,
		})

	default:
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
		writeJSON(w, http.StatusOK, result)
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

func (s *Server) txResult(txid string) (any, error) {
	tx, err := s.svc.GetTx(txid)
	if err != nil {
		return nil, err
	}
	return map[string]any{"kind": "tx", "tx": tx}, nil
}

func (s *Server) blockResult(hash string) (any, error) {
	block, err := s.svc.BlockByHash(hash, 0, defaultBlockTxLimit)
	if err != nil {
		return nil, err
	}
	return map[string]any{"kind": "block", "block": block}, nil
}

func writeNotFound(w http.ResponseWriter, q string) {
	writeJSON(w, http.StatusOK, map[string]any{
		"kind":  "notfound",
		"query": q,
	})
}
