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

// fetchStuck is how long an in-flight mempool fetch may run before
// another caller may fetch alongside it. A getrawmempoolverbose stranded
// by a node stall or a websocket reconnect must never freeze the snapshot
// forever (same watchdog idea as the sync and live-cache refreshes).
const fetchStuck = 2 * time.Minute

// Peak tracking: the queue bar's "capacity track" is the rolling recent
// peak of mempool vbytes, kept as a max per bucket over the last hour.
const (
	peakBucketDur = 10 * time.Minute
	peakBuckets   = 6
)

type peakBucket struct {
	start time.Time
	max   int64
}

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
	// gen advances on every invalidation; an install only marks the
	// snapshot clean when nothing changed while the fetch was in flight.
	gen uint64
	// fetching/fetchStarted single-flight the (lock-free) RPC fetch.
	fetching     bool
	fetchStarted time.Time
	entries      map[string]MempoolEntry
	// queue is the fee-band histogram derived from entries, computed at
	// most once per refresh (a pure function of the snapshot).
	queue *Queue
	// peaks is the rolling window of per-bucket vbytes maxima.
	peaks []peakBucket
}

func NewMempool(backend node.Backend) *Mempool {
	return &Mempool{backend: backend}
}

// Snapshot returns the current entries keyed by txid, refreshing when the
// cached copy is older than snapshotMaxAge. The returned map is shared —
// callers must not mutate it.
func (m *Mempool) Snapshot() (map[string]MempoolEntry, error) {
	if err := m.refresh(); err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.entries, nil
}

// Queue returns the fee-band histogram for the current snapshot. The
// derivation sorts the whole mempool, so it is memoized per refresh
// rather than recomputed on every stats push.
func (m *Mempool) Queue() (*Queue, error) {
	_, queue, err := m.SnapshotAndQueue()
	return queue, err
}

// SnapshotAndQueue returns the entries and their histogram from one
// locked read, so callers joining per-tx data against the queue see a
// single consistent snapshot.
func (m *Mempool) SnapshotAndQueue() (map[string]MempoolEntry, *Queue, error) {
	if err := m.refresh(); err != nil {
		return nil, nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.queue == nil {
		m.queue = queueFromSnapshot(m.entries)
		m.queue.PeakVBytes = m.notePeakLocked(m.queue.TotalVBytes)
	}
	return m.entries, m.queue, nil
}

// notePeakLocked records the current depth in the rolling window and
// returns the recent-peak vbytes (never below the current total). The
// caller must hold m.mu.
func (m *Mempool) notePeakLocked(total int64) int64 {
	now := time.Now()
	bucket := now.Truncate(peakBucketDur)

	if n := len(m.peaks); n > 0 && m.peaks[n-1].start.Equal(bucket) {
		m.peaks[n-1].max = max(m.peaks[n-1].max, total)
	} else {
		m.peaks = append(m.peaks, peakBucket{start: bucket, max: total})
	}

	// Drop buckets outside the window; the slice stays tiny.
	cutoff := bucket.Add(-time.Duration(peakBuckets-1) * peakBucketDur)
	for len(m.peaks) > 0 && m.peaks[0].start.Before(cutoff) {
		m.peaks = m.peaks[1:]
	}

	peak := total
	for _, b := range m.peaks {
		peak = max(peak, b.max)
	}
	return peak
}

// refresh refetches the mempool when the cached copy is stale. The RPC
// runs with NO lock held: btcd can stall getrawmempoolverbose for minutes
// (UTXO cache flushes), and a stranded call must never wedge every future
// reader behind m.mu. While a fetch is in flight, other readers serve the
// stale copy; only a cold start (no snapshot yet) or a fetch stuck past
// fetchStuck fetches alongside it.
func (m *Mempool) refresh() error {
	m.mu.Lock()
	age := time.Since(m.fetched)
	fresh := m.entries != nil && age < snapshotMaxAge &&
		!(m.dirty && age >= snapshotMinAge)
	if fresh {
		m.mu.Unlock()
		return nil
	}
	if m.fetching && time.Since(m.fetchStarted) < fetchStuck &&
		m.entries != nil {

		m.mu.Unlock()
		return nil
	}
	m.fetching = true
	m.fetchStarted = time.Now()
	gen := m.gen
	m.mu.Unlock()

	raw, err := m.backend.GetRawMempoolVerbose()
	if err != nil {
		m.mu.Lock()
		m.fetching = false
		m.mu.Unlock()
		return err
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

	m.mu.Lock()
	defer m.mu.Unlock()
	m.fetching = false
	m.entries = entries
	m.queue = nil
	m.fetched = time.Now()
	if m.gen == gen {
		// Nothing invalidated the snapshot while we fetched; a block or
		// tx that landed mid-fetch keeps it dirty so the next read (past
		// the throttle) refetches.
		m.dirty = false
	}
	return nil
}

// Invalidate forces the next Snapshot call to refresh; used when a block
// connects and the mempool contents change wholesale.
func (m *Mempool) Invalidate() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.fetched = time.Time{}
	m.dirty = true
	m.gen++
}

// MarkDirty requests a refresh on the next read (throttled to
// snapshotMinAge); wired to btcd's tx-accepted notifications.
func (m *Mempool) MarkDirty() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.dirty = true
	m.gen++
}
