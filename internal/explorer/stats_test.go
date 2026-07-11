package explorer

import (
	"testing"
	"time"

	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/chaincfg/v2"
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

	stats, err := newTestService(m).Stats()
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

func newMainnetService(m *mockBackend) *Service {
	return NewService(m, Config{
		Params: &chaincfg.MainNetParams,
		Price:  func() PriceQuote { return PriceQuote{} },
		Floors: FeeFloors{Slow: 1, Standard: 2, Urgent: 5},
	})
}

func TestSyncingDetection(t *testing.T) {
	// Mainnet tip a day behind the clock → syncing.
	m := newMockBackend()
	installChain(m, 20, 60*time.Second)
	backdate(m, 24*time.Hour)
	if !newMainnetService(m).Syncing() {
		t.Error("stale mainnet tip not detected as syncing")
	}

	// Fresh mainnet tip → synced.
	m2 := newMockBackend()
	installChain(m2, 20, 60*time.Second)
	if newMainnetService(m2).Syncing() {
		t.Error("fresh mainnet tip reported as syncing")
	}

	// Regtest with an old tip → never syncing (blocks exist on demand;
	// a paused dev harness is not IBD).
	m3 := newMockBackend()
	installChain(m3, 20, 60*time.Second)
	backdate(m3, 24*time.Hour)
	if newTestService(m3).Syncing() {
		t.Error("regtest reported as syncing")
	}
}
