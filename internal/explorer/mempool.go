package explorer

import (
	"sync"
	"time"

	"github.com/btcsuite/btcd/btcutil/v2"

	"btcdwatch.com/internal/node"
)

// snapshotMaxAge bounds how stale the shared mempool snapshot may be before
// a read triggers a refresh.
const snapshotMaxAge = 5 * time.Second

// MempoolEntry is one mempool transaction in the shared snapshot.
type MempoolEntry struct {
	FeeSats   int64
	VSize     int64
	SizeBytes int64
	Time      int64
	// FeeRate is sat/vB.
	FeeRate float64
}

// Mempool is the lazily refreshed, shared snapshot of getrawmempool
// verbose=true. Fee tiers, pending-queue position, and example chips all
// read the same snapshot so the node never sees redundant full-mempool
// calls.
type Mempool struct {
	backend node.Backend

	mu      sync.Mutex
	fetched time.Time
	entries map[string]MempoolEntry
}

func NewMempool(backend node.Backend) *Mempool {
	return &Mempool{backend: backend}
}

// Snapshot returns the current entries keyed by txid, refreshing when the
// cached copy is older than snapshotMaxAge. The returned map is shared —
// callers must not mutate it.
func (m *Mempool) Snapshot() (map[string]MempoolEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.entries != nil && time.Since(m.fetched) < snapshotMaxAge {
		return m.entries, nil
	}

	raw, err := m.backend.GetRawMempoolVerbose()
	if err != nil {
		return nil, err
	}

	entries := make(map[string]MempoolEntry, len(raw))
	for txid, e := range raw {
		fee, err := btcutil.NewAmount(e.Fee)
		if err != nil {
			continue
		}
		vsize := int64(e.Vsize)
		if vsize == 0 {
			vsize = int64(e.Size)
		}
		entry := MempoolEntry{
			FeeSats:   int64(fee),
			VSize:     vsize,
			SizeBytes: int64(e.Size),
			Time:      e.Time,
		}
		if vsize > 0 {
			entry.FeeRate = float64(entry.FeeSats) / float64(vsize)
		}
		entries[txid] = entry
	}

	m.entries = entries
	m.fetched = time.Now()
	return m.entries, nil
}

// Invalidate forces the next Snapshot call to refresh. Wired to block and
// mempool notifications in the live-update milestone.
func (m *Mempool) Invalidate() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.fetched = time.Time{}
}
