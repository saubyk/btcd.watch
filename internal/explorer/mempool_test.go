package explorer

import (
	"sync"
	"testing"
	"time"

	"github.com/btcsuite/btcd/btcjson"
)

func TestMempoolSnapshotCachesWithinMaxAge(t *testing.T) {
	m := newMockBackend()
	m.mempool[hexID("tx1")] = btcjson.GetRawMempoolVerboseResult{
		Vsize: 200, Fee: 0.001, Time: 100,
	}

	mp := NewMempool(m)
	for range 3 {
		if _, err := mp.Snapshot(); err != nil {
			t.Fatal(err)
		}
	}
	if m.mempoolFetches != 1 {
		t.Errorf("mempool fetched %d times within max age, want 1",
			m.mempoolFetches)
	}

	mp.Invalidate()
	if _, err := mp.Snapshot(); err != nil {
		t.Fatal(err)
	}
	if m.mempoolFetches != 2 {
		t.Errorf("mempool fetched %d times after invalidate, want 2",
			m.mempoolFetches)
	}
}

func TestMempoolEntryDerivation(t *testing.T) {
	m := newMockBackend()
	m.mempool[hexID("tx1")] = btcjson.GetRawMempoolVerboseResult{
		Vsize: 250, Fee: 0.00025, Time: 42,
	}

	snap, err := NewMempool(m).Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	e := snap[hexID("tx1")]
	if e.FeeSats != 25_000 || e.VSize != 250 || e.Time != 42 {
		t.Fatalf("entry = %+v", e)
	}
	if e.FeeRate != 100 {
		t.Errorf("feeRate = %v, want 100 sat/vB", e.FeeRate)
	}
}

// gatedMempoolBackend blocks GetRawMempoolVerbose until a gate token is
// available, signalling started when a fetch begins — simulates a fetch
// stranded by a btcd UTXO-flush stall or a websocket reconnect.
type gatedMempoolBackend struct {
	*mockBackend
	started chan struct{}
	gate    chan struct{}
}

func (b *gatedMempoolBackend) GetRawMempoolVerbose() (map[string]btcjson.GetRawMempoolVerboseResult, error) {
	select {
	case b.started <- struct{}{}:
	default:
	}
	<-b.gate
	return b.mockBackend.GetRawMempoolVerbose()
}

// A fetch stuck on the node must not wedge other readers behind the
// mempool mutex: they serve the stale snapshot instantly. This exact
// wedge froze /api/stats in production — a getrawmempoolverbose stranded
// across a websocket reconnect held m.mu forever.
func TestSnapshotServesStaleDuringStalledFetch(t *testing.T) {
	m := newMockBackend()
	m.mempool[hexID("tx1")] = btcjson.GetRawMempoolVerboseResult{
		Vsize: 200, Fee: 0.001, Time: 100,
	}
	b := &gatedMempoolBackend{
		mockBackend: m,
		started:     make(chan struct{}, 1),
		gate:        make(chan struct{}, 2),
	}

	mp := NewMempool(b)
	b.gate <- struct{}{} // let the warm-up fetch through
	if _, err := mp.Snapshot(); err != nil {
		t.Fatal(err)
	}
	<-b.started // discard the warm-up fetch's signal

	// Stale the snapshot, then strand a second fetch mid-RPC.
	mp.Invalidate()
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, _ = mp.Snapshot()
	}()
	<-b.started

	// The stranded fetch is in flight; this read must return the stale
	// copy immediately instead of queueing behind it.
	done := make(chan int, 1)
	go func() {
		snap, err := mp.Snapshot()
		if err != nil {
			done <- -1
			return
		}
		done <- len(snap)
	}()
	select {
	case n := <-done:
		if n != 1 {
			t.Errorf("stale snapshot has %d entries, want 1", n)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Snapshot wedged behind a stalled mempool fetch")
	}

	b.gate <- struct{}{} // release the stranded fetch
	wg.Wait()
}
