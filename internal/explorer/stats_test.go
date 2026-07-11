package explorer

import (
	"testing"
	"time"

	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/chaincfg/v2"

	"btcd.watch/internal/node"
)

// installChain builds tip headers with the given spacing so the interval
// measurement has data.
func installChain(m *mockBackend, tip int64, spacing time.Duration) {
	now := time.Now().Unix()
	for h := int64(0); h <= tip; h++ {
		hash := hexID("blk" + string(rune('A'+h%26)) + string(rune('a'+h/26%26)))
		m.hashes[h] = hash
		m.headers[hash] = &btcjson.GetBlockHeaderVerboseResult{
			Hash:   hash,
			Height: int32(h),
			Time:   now - (tip-h)*int64(spacing.Seconds()),
		}
	}
	m.tip = tip
}

func TestStats(t *testing.T) {
	m := newMockBackend()
	installChain(m, 20, 60*time.Second)
	m.mempool[hexID("m1")] = btcjson.GetRawMempoolVerboseResult{
		Vsize: 200, Size: 210, Fee: 0.0001, Time: 1,
	}
	m.mempool[hexID("m2")] = btcjson.GetRawMempoolVerboseResult{
		Vsize: 300, Size: 320, Fee: 0.0002, Time: 2,
	}

	stats, err := newTestService(m).computeStats()
	if err != nil {
		t.Fatal(err)
	}

	if stats.Network != "regtest" {
		t.Errorf("network = %q", stats.Network)
	}
	if stats.BlockHeight != 20 {
		t.Errorf("height = %d", stats.BlockHeight)
	}
	if stats.Mempool.TxCount != 2 || stats.Mempool.Bytes != 530 {
		t.Errorf("mempool = %+v", stats.Mempool)
	}
	if stats.AvgBlockIntervalSeconds != 60 {
		t.Errorf("avgInterval = %d, want 60", stats.AvgBlockIntervalSeconds)
	}
	// Tip is fresh, so next block ETA ≈ the full interval.
	if stats.NextBlockEtaSeconds < 5 || stats.NextBlockEtaSeconds > 60 {
		t.Errorf("nextBlockEta = %d, want (5,60]", stats.NextBlockEtaSeconds)
	}
	// Regtest halving interval 150: at height 20, 130 blocks remain.
	if stats.Halving.BlocksRemaining != 130 {
		t.Errorf("halving blocks = %d, want 130", stats.Halving.BlocksRemaining)
	}
	if stats.Halving.EtaSeconds != 130*60 {
		t.Errorf("halving eta = %d, want %d", stats.Halving.EtaSeconds, 130*60)
	}
	if stats.Price == nil || stats.Price.USD != 100_000 || stats.Price.Source != "static" {
		t.Errorf("price = %+v", stats.Price)
	}
	if stats.Syncing {
		t.Error("syncing = true on regtest (detector must be off)")
	}
}

// backdate shifts every header timestamp into the past, simulating a node
// whose tip is far behind the wall clock (IBD).
func backdate(m *mockBackend, age time.Duration) {
	for _, h := range m.headers {
		h.Time -= int64(age.Seconds())
	}
}

func newMainnetService(m node.Backend) *Service {
	return NewService(m, Config{
		Params: &chaincfg.MainNetParams,
		Price:  func() PriceQuote { return PriceQuote{} },
		Floors: FeeFloors{Slow: 1, Standard: 2, Urgent: 5},
	})
}

// The detection tests drive refreshSync directly: in production it runs
// in a background goroutine and readers only ever see the cached answer.
func TestSyncingDetection(t *testing.T) {
	// Mainnet tip a day behind the clock → syncing. Also the assumed
	// state before the first check completes.
	m := newMockBackend()
	installChain(m, 20, 60*time.Second)
	backdate(m, 24*time.Hour)
	s := newMainnetService(m)
	if !s.syncing {
		t.Error("mainnet service must start out assumed syncing")
	}
	s.refreshSync()
	if !s.Syncing() {
		t.Error("stale mainnet tip not detected as syncing")
	}

	// Fresh mainnet tip → synced.
	m2 := newMockBackend()
	installChain(m2, 20, 60*time.Second)
	s2 := newMainnetService(m2)
	s2.refreshSync()
	if s2.Syncing() {
		t.Error("fresh mainnet tip reported as syncing")
	}
	st := s2.SyncStatus()
	if st.TipHeight != 20 || st.CheckedAt.IsZero() {
		t.Errorf("sync status = %+v, want tip 20 and non-zero CheckedAt", st)
	}

	// Regtest with an old tip → never syncing (blocks exist on demand;
	// a paused dev harness is not IBD).
	m3 := newMockBackend()
	installChain(m3, 20, 60*time.Second)
	backdate(m3, 24*time.Hour)
	s3 := newTestService(m3)
	s3.refreshSync()
	if s3.Syncing() {
		t.Error("regtest reported as syncing")
	}
}

// stalledBackend simulates btcd blocking RPC mid-flush: GetBlockCount
// hangs until release is closed.
type stalledBackend struct {
	*mockBackend
	release chan struct{}
	// started, when non-nil, is signalled as a call enters the stall.
	started chan struct{}
}

func (b *stalledBackend) GetBlockCount() (int64, error) {
	if b.started != nil {
		select {
		case b.started <- struct{}{}:
		default:
		}
	}
	<-b.release
	return b.mockBackend.GetBlockCount()
}

// Syncing must serve the cached answer instantly even when the node has
// stopped answering RPC — btcd stalls calls for minutes while flushing
// its UTXO cache, and one stuck probe froze the whole site behind syncMu
// before the check moved off the request path.
func TestSyncingDoesNotBlockOnStalledNode(t *testing.T) {
	m := newMockBackend()
	installChain(m, 20, 60*time.Second)
	b := &stalledBackend{mockBackend: m, release: make(chan struct{})}
	defer close(b.release)

	s := newMainnetService(b)
	done := make(chan bool, 1)
	go func() { done <- s.Syncing() }()

	select {
	case v := <-done:
		if !v {
			t.Error("want assumed syncing=true while the first check is stuck")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Syncing blocked on a stalled node RPC")
	}
}

// OnBlock runs on the rpcclient notification goroutine: if it ever waits
// behind a lock that an in-flight node RPC holds, the whole websocket
// deadlocks — the notification goroutine can't drain the connection, so
// the response the lock-holder is waiting for is never read. This exact
// cycle froze production via intervalMu (avgBlockInterval measured the
// block interval over RPC while holding the mutex OnBlock invalidates).
func TestOnBlockDoesNotBlockBehindIntervalMeasurement(t *testing.T) {
	m := newMockBackend()
	installChain(m, 20, 60*time.Second)
	b := &stalledBackend{
		mockBackend: m,
		release:     make(chan struct{}),
		started:     make(chan struct{}, 1),
	}
	defer close(b.release)

	s := newMainnetService(b)
	go s.avgBlockInterval()
	<-b.started // measurement is now stalled inside the node RPC

	done := make(chan struct{})
	go func() {
		s.OnBlock()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("OnBlock blocked behind an in-flight interval measurement")
	}
}
