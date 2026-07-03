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

func TestExamples(t *testing.T) {
	m := newMockBackend()
	installChain(m, 5, 60*time.Second)

	// Tip block only has a coinbase; block 4 has a spend.
	coinbaseTip, coinbase4 := hexID("cbTip"), hexID("cb4")
	spend := hexID("spend4")
	m.blocks[m.hashes[5]] = &btcjson.GetBlockVerboseResult{
		Hash: m.hashes[5], Height: 5, Tx: []string{coinbaseTip},
	}
	m.blocks[m.hashes[4]] = &btcjson.GetBlockVerboseResult{
		Hash: m.hashes[4], Height: 4, Tx: []string{coinbase4, spend},
	}
	m.txs[spend] = &btcjson.TxRawResult{
		Txid: spend,
		Vout: []btcjson.Vout{vout(0, 1.0, addrB1)},
	}

	// Highest-feerate mempool entry becomes the pending example.
	addMempoolTx(m, "cheap", 2, 200)
	addMempoolTx(m, "rich", 80, 200)

	examples, err := newTestService(m).Examples()
	if err != nil {
		t.Fatal(err)
	}

	if examples.PendingTxid == nil || *examples.PendingTxid != hexID("rich") {
		t.Errorf("pending = %v, want rich tx", examples.PendingTxid)
	}
	if examples.ConfirmedTxid == nil || *examples.ConfirmedTxid != spend {
		t.Errorf("confirmed = %v, want %s", examples.ConfirmedTxid, spend)
	}
	if examples.Address == nil || *examples.Address != addrB1 {
		t.Errorf("address = %v, want %s", examples.Address, addrB1)
	}
}
