package explorer

import (
	"errors"
	"testing"
	"time"

	"github.com/btcsuite/btcd/btcjson"
)

// installBlock builds a block at height 5 holding a coinbase and two
// spends. Coinbase outputs are subsidy (50 BTC on young regtest) plus
// 4,000 sats of fees; non-coinbase vsize is 400 (weight 2000 → vsize 500,
// coinbase 100), so the average feerate is 10 sat/vB.
func installBlock(m *mockBackend) (blockHash string, txids []string) {
	installChain(m, 6, 60*time.Second)
	blockHash = m.hashes[5]

	coinbase := hexID("coinbase5")
	spend1, spend2 := hexID("spend5a"), hexID("spend5b")
	parent := hexID("parentblk")
	installParent(m, parent,
		vout(0, 1.0, addrA1), vout(1, 0.5, addrA1))

	m.txs[coinbase] = &btcjson.TxRawResult{
		Txid:      coinbase,
		Vsize:     100,
		BlockHash: blockHash,
		Vin:       []btcjson.Vin{{Coinbase: "03abcdef"}},
		Vout:      []btcjson.Vout{vout(0, 50.00004, addrB1)},
	}
	m.txs[spend1] = &btcjson.TxRawResult{
		Txid:      spend1,
		Vsize:     200,
		BlockHash: blockHash,
		Vin:       []btcjson.Vin{vin(parent, 0)},
		Vout:      []btcjson.Vout{vout(0, 0.99997, addrB1)},
	}
	m.txs[spend2] = &btcjson.TxRawResult{
		Txid:      spend2,
		Vsize:     200,
		BlockHash: blockHash,
		Vin:       []btcjson.Vin{vin(parent, 1)},
		Vout:      []btcjson.Vout{vout(0, 0.49999, addrB1)},
	}

	txids = []string{coinbase, spend1, spend2}
	m.blocks[blockHash] = &btcjson.GetBlockVerboseResult{
		Hash:          blockHash,
		Height:        5,
		Time:          1_700_000_000,
		Confirmations: 2,
		Size:          520,
		Weight:        2000,
		Tx:            txids,
		NextHash:      m.hashes[6],
	}
	return blockHash, txids
}

func TestBlockByHeight(t *testing.T) {
	m := newMockBackend()
	blockHash, txids := installBlock(m)

	block, err := newTestService(m).BlockByHeight(5, 0, 25)
	if err != nil {
		t.Fatal(err)
	}

	if block.Height != 5 || block.Hash != blockHash {
		t.Errorf("block = %d %s", block.Height, block.Hash)
	}
	if block.TxCount != 3 || block.Confirmations != 2 || block.SizeBytes != 520 {
		t.Errorf("count/conf/size = %d/%d/%d",
			block.TxCount, block.Confirmations, block.SizeBytes)
	}
	if block.NextHeight == nil || *block.NextHeight != 6 {
		t.Errorf("nextHeight = %v, want 6", block.NextHeight)
	}
	if block.AvgFeeSatPerVb != 10 {
		t.Errorf("avgFee = %v, want 10", block.AvgFeeSatPerVb)
	}

	if len(block.Txs) != 3 || block.HasMore {
		t.Fatalf("txs = %d hasMore = %v", len(block.Txs), block.HasMore)
	}
	cb := block.Txs[0]
	if !cb.IsCoinbase || cb.FeeRateSatPerVb != nil || cb.Txid != txids[0] {
		t.Errorf("coinbase row = %+v", cb)
	}
	if cb.AmountSats != 50_00004000 {
		t.Errorf("coinbase amount = %d", cb.AmountSats)
	}
	// spend1: 1.0 in, 0.99997 out → 3,000 sats fee over 200 vB.
	row := block.Txs[1]
	if row.FeeRateSatPerVb == nil || *row.FeeRateSatPerVb != 15 {
		t.Errorf("spend1 rate = %v, want 15", row.FeeRateSatPerVb)
	}
}

func TestBlockPagination(t *testing.T) {
	m := newMockBackend()
	_, txids := installBlock(m)
	svc := newTestService(m)

	page1, err := svc.BlockByHeight(5, 0, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(page1.Txs) != 2 || !page1.HasMore {
		t.Errorf("page1 = %d rows hasMore=%v", len(page1.Txs), page1.HasMore)
	}

	page2, err := svc.BlockByHeight(5, 2, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(page2.Txs) != 1 || page2.HasMore {
		t.Errorf("page2 = %d rows hasMore=%v", len(page2.Txs), page2.HasMore)
	}
	if page2.Txs[0].Txid != txids[2] {
		t.Errorf("page2 row = %s, want %s", page2.Txs[0].Txid, txids[2])
	}
}

func TestBlockByHashAndNotFound(t *testing.T) {
	m := newMockBackend()
	blockHash, _ := installBlock(m)
	svc := newTestService(m)

	block, err := svc.BlockByHash(blockHash, 0, 25)
	if err != nil {
		t.Fatal(err)
	}
	if block.Height != 5 {
		t.Errorf("height = %d, want 5", block.Height)
	}

	if _, err := svc.BlockByHeight(99, 0, 25); !errors.Is(err, ErrBlockNotFound) {
		t.Errorf("unknown height err = %v, want ErrBlockNotFound", err)
	}
	if _, err := svc.BlockByHash(hexID("nothere"), 0, 25); !errors.Is(err, ErrBlockNotFound) {
		t.Errorf("unknown hash err = %v, want ErrBlockNotFound", err)
	}
}
