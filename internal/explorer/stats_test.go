package explorer

import (
	"testing"
	"time"

	"github.com/btcsuite/btcd/btcjson"
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
}
