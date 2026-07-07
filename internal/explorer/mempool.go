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

// snapshotMinAge throttles dirty-flag refreshes: a busy txgen marks the
// snapshot dirty constantly, and refetching the whole mempool on every
// read would defeat the point of sharing it.
const snapshotMinAge = time.Second

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
	dirty   bool
	entries map[string]MempoolEntry
	// queue is the fee-band histogram derived from entries, computed at
	// most once per refresh (a pure function of the snapshot).
	queue *Queue
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

	if err := m.refreshLocked(); err != nil {
		return nil, err
	}
	return m.entries, nil
}

// Queue returns the fee-band histogram for the current snapshot. The
// derivation sorts the whole mempool, so it is memoized per refresh
// rather than recomputed on every stats push.
func (m *Mempool) Queue() (*Queue, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.refreshLocked(); err != nil {
		return nil, err
	}
	if m.queue == nil {
		m.queue = queueFromSnapshot(m.entries)
	}
	return m.queue, nil
}

// refreshLocked refetches the mempool when the cached copy is stale; the
// caller must hold m.mu.
func (m *Mempool) refreshLocked() error {
	age := time.Since(m.fetched)
	fresh := m.entries != nil && age < snapshotMaxAge &&
		!(m.dirty && age >= snapshotMinAge)
	if fresh {
		return nil
	}

	raw, err := m.backend.GetRawMempoolVerbose()
	if err != nil {
		return err
	}
	m.dirty = false

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
	m.queue = nil
	m.fetched = time.Now()
	return nil
}

// Invalidate forces the next Snapshot call to refresh; used when a block
// connects and the mempool contents change wholesale.
func (m *Mempool) Invalidate() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.fetched = time.Time{}
}

// MarkDirty requests a refresh on the next read (throttled to
// snapshotMinAge); wired to btcd's tx-accepted notifications.
func (m *Mempool) MarkDirty() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.dirty = true
}
