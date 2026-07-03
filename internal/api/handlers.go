package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

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
		// Address detail lands in the address-view milestone; the
		// classification lets the frontend route already.
		writeJSON(w, http.StatusOK, map[string]any{
			"kind":  "address",
			"query": query.Address.EncodeAddress(),
		})

	default:
		writeJSON(w, http.StatusOK, map[string]any{
			"kind":  "invalid",
			"query": q,
		})
	}
}
