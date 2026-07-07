package explorer

import (
	"time"

	"github.com/btcsuite/btcd/btcjson"
)

// arrivalsCap bounds the server-side buffer of recent arrivals (the UI
// shows the newest six).
const arrivalsCap = 12

// arrivalMaxAge prunes stale buffer entries that never resolved (e.g.
// mined or evicted before the next snapshot read).
const arrivalMaxAge = 5 * time.Minute

// Arrival is one recently accepted transaction in the landing feed.
// AmountSats is the gross output total (the raw payload carries no
// prevouts, so no change heuristic); the row's pending view shows the
// derived net amount.
type Arrival struct {
	Txid            string  `json:"txid"`
	AmountSats      int64   `json:"amountSats"`
	FeeRateSatPerVb float64 `json:"feeRateSatPerVb"`
	VSize           int64   `json:"vsize"`
	// Time is the mempool acceptance time (unix seconds).
	Time int64 `json:"time"`
}

// arrival is the unresolved buffer entry; fee data joins from the mempool
// snapshot at read time.
type arrival struct {
	txid       string
	amountSats int64
	seen       time.Time
}

// NoteArrival records a tx-accepted notification (newest first) and marks
// the mempool snapshot dirty. Fee rate and vsize are not in the
// notification payload — they resolve from the next snapshot refresh in
// MempoolUpdate.
func (s *Service) NoteArrival(raw *btcjson.TxRawResult) {
	var amount int64
	for _, v := range raw.Vout {
		amount += satsFromBTC(v.Value)
	}

	now := time.Now()
	s.arrivalsMu.Lock()
	buf := make([]arrival, 0, arrivalsCap)
	buf = append(buf, arrival{txid: raw.Txid, amountSats: amount, seen: now})
	for _, a := range s.arrivals {
		if len(buf) == arrivalsCap || now.Sub(a.seen) > arrivalMaxAge {
			break
		}
		if a.txid == raw.Txid {
			continue // rebroadcast — keep the newest entry only
		}
		buf = append(buf, a)
	}
	s.arrivals = buf
	s.arrivalsMu.Unlock()

	s.mempool.MarkDirty()
}

// MempoolUpdate is the live-mempool WS payload: the queue histogram plus
// the recent arrivals whose fee data has resolved.
type MempoolUpdate struct {
	Queue    *Queue    `json:"queue"`
	Arrivals []Arrival `json:"arrivals"`
}

// MempoolUpdate assembles the payload from one consistent snapshot read.
// Buffered arrivals not (yet) in the snapshot are skipped: either the
// refresh throttle hasn't caught up (they appear on the next push) or the
// tx was already mined or evicted.
func (s *Service) MempoolUpdate() (*MempoolUpdate, error) {
	snapshot, queue, err := s.mempool.SnapshotAndQueue()
	if err != nil {
		return nil, err
	}

	s.arrivalsMu.Lock()
	defer s.arrivalsMu.Unlock()

	arrivals := make([]Arrival, 0, len(s.arrivals))
	for _, a := range s.arrivals {
		entry, ok := snapshot[a.txid]
		if !ok {
			continue
		}
		arrivals = append(arrivals, Arrival{
			Txid:            a.txid,
			AmountSats:      a.amountSats,
			FeeRateSatPerVb: entry.FeeRate,
			VSize:           entry.VSize,
			Time:            entry.Time,
		})
	}
	return &MempoolUpdate{Queue: queue, Arrivals: arrivals}, nil
}
