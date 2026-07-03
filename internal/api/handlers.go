package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

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
	if query.Kind != chain.QueryTx {
		writeError(w, http.StatusBadRequest, "invalid_txid",
			"txid must be 64 hex characters")
		return
	}

	tx, err := s.svc.GetTx(query.Txid)
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

func (s *Server) handleExamples(w http.ResponseWriter, r *http.Request) {
	examples, err := s.svc.Examples()
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, examples)
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	query := chain.ClassifyQuery(q, s.params)

	switch query.Kind {
	case chain.QueryTx:
		tx, err := s.svc.GetTx(query.Txid)
		if err != nil {
			if errors.Is(err, explorer.ErrTxNotFound) {
				writeJSON(w, http.StatusOK, map[string]any{
					"kind":  "notfound",
					"query": q,
				})
				return
			}
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"kind": "tx",
			"tx":   tx,
		})

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
