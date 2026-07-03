package explorer

import (
	"errors"
	"testing"

	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/chaincfg/v2"
)

const (
	addrA1 = "bcrt1qsender1"
	addrA2 = "bcrt1qsender2"
	addrB1 = "bcrt1qrecipient"
)

func newTestService(m *mockBackend) *Service {
	price := func() PriceQuote {
		return PriceQuote{USD: 100_000, Source: "static", OK: true}
	}
	floors := FeeFloors{Slow: 1, Standard: 2, Urgent: 5}
	return NewService(m, &chaincfg.RegressionNetParams, NewMempool(m), price, floors)
}

// installParent registers a parent tx whose vouts fund the tx under test.
func installParent(m *mockBackend, txid string, vouts ...btcjson.Vout) {
	m.txs[txid] = &btcjson.TxRawResult{Txid: txid, Vout: vouts}
}

// confirm installs a block at the given height and stamps the tx into it.
func confirm(m *mockBackend, tx *btcjson.TxRawResult, height int64, time int64) {
	blockHash := hexID("block")
	tx.BlockHash = blockHash
	tx.Confirmations = 3
	m.headers[blockHash] = &btcjson.GetBlockHeaderVerboseResult{
		Hash:   blockHash,
		Height: int32(height),
		Time:   time,
	}
}

// TestGetTxPayment covers the standard 2-in/2-out payment with change:
// fee, from/to heuristics, and amount.
func TestGetTxPayment(t *testing.T) {
	m := newMockBackend()
	p1, p2 := hexID("parent1"), hexID("parent2")
	installParent(m, p1, vout(0, 1.0, addrA1))
	installParent(m, p2, vout(0, 0.5, addrA2))

	txid := hexID("payment")
	tx := &btcjson.TxRawResult{
		Txid:  txid,
		Vsize: 208,
		Vin:   []btcjson.Vin{vin(p1, 0), vin(p2, 0)},
		Vout: []btcjson.Vout{
			vout(0, 1.2, addrB1),  // payment
			vout(1, 0.29, addrA1), // change back to a sender address
		},
	}
	confirm(m, tx, 512, 1735000000)
	m.txs[txid] = tx

	got, err := newTestService(m).GetTx(txid)
	if err != nil {
		t.Fatal(err)
	}

	if got.Status != "confirmed" {
		t.Errorf("status = %q, want confirmed", got.Status)
	}
	if got.Block == nil || got.Block.Height != 512 || got.Block.Time != 1735000000 {
		t.Errorf("block = %+v", got.Block)
	}
	if got.Confirmations != 3 {
		t.Errorf("confirmations = %d", got.Confirmations)
	}

	// fee = 1.5 BTC in − 1.49 BTC out = 0.01 BTC = 1_000_000 sats.
	if got.FeeSats == nil || *got.FeeSats != 1_000_000 {
		t.Errorf("feeSats = %v, want 1000000", got.FeeSats)
	}
	wantRate := 1_000_000.0 / 208.0
	if got.FeeRateSatPerVb == nil || *got.FeeRateSatPerVb != wantRate {
		t.Errorf("feeRate = %v, want %v", got.FeeRateSatPerVb, wantRate)
	}

	if len(got.From) != 2 || got.From[0] != addrA1 || got.From[1] != addrA2 {
		t.Errorf("from = %v", got.From)
	}
	// Change output back to addrA1 must be excluded.
	if len(got.To) != 1 || got.To[0] != addrB1 {
		t.Errorf("to = %v", got.To)
	}
	if got.AmountSats != 120_000_000 {
		t.Errorf("amountSats = %d, want 120000000", got.AmountSats)
	}
	// 1.2 BTC at $100k.
	if got.FiatUSD == nil || *got.FiatUSD != 120_000 {
		t.Errorf("fiatUsd = %v, want 120000", got.FiatUSD)
	}
	if got.IsCoinbase || got.Pending != nil {
		t.Errorf("isCoinbase=%v pending=%v", got.IsCoinbase, got.Pending)
	}
}

func TestGetTxCoinbase(t *testing.T) {
	m := newMockBackend()
	txid := hexID("coinbase")
	tx := &btcjson.TxRawResult{
		Txid:  txid,
		Vsize: 110,
		Vin:   []btcjson.Vin{{Coinbase: "0341f80004"}},
		Vout: []btcjson.Vout{
			vout(0, 50.0, addrA1),
			vout(1, 0.0), // OP_RETURN style, no addresses
		},
	}
	confirm(m, tx, 150, 1735000100)
	m.txs[txid] = tx

	got, err := newTestService(m).GetTx(txid)
	if err != nil {
		t.Fatal(err)
	}

	if !got.IsCoinbase {
		t.Error("isCoinbase = false")
	}
	if got.FeeSats != nil || got.FeeRateSatPerVb != nil {
		t.Errorf("coinbase fee should be null, got %v / %v",
			got.FeeSats, got.FeeRateSatPerVb)
	}
	// Coinbase amount = sum of ALL outputs.
	if got.AmountSats != 50_0000_0000 {
		t.Errorf("amountSats = %d, want 5000000000", got.AmountSats)
	}
	if len(got.From) != 0 {
		t.Errorf("from = %v, want empty", got.From)
	}
	// Zero-address vout renders as the placeholder.
	if len(got.To) != 2 || got.To[0] != addrA1 || got.To[1] != nonStandardLabel {
		t.Errorf("to = %v", got.To)
	}
}

// TestGetTxSelfSend: every output pays a from-address, so the change
// heuristic would exclude everything — all outputs must count instead.
func TestGetTxSelfSend(t *testing.T) {
	m := newMockBackend()
	p1 := hexID("selfparent")
	installParent(m, p1, vout(0, 1.0, addrA1))

	txid := hexID("selfsend")
	tx := &btcjson.TxRawResult{
		Txid:  txid,
		Vsize: 141,
		Vin:   []btcjson.Vin{vin(p1, 0)},
		Vout:  []btcjson.Vout{vout(0, 0.9999, addrA1)},
	}
	confirm(m, tx, 100, 1735000200)
	m.txs[txid] = tx

	got, err := newTestService(m).GetTx(txid)
	if err != nil {
		t.Fatal(err)
	}

	if got.AmountSats != 99_990_000 {
		t.Errorf("amountSats = %d, want 99990000", got.AmountSats)
	}
	if len(got.To) != 1 || got.To[0] != addrA1 {
		t.Errorf("to = %v", got.To)
	}
	if got.FeeSats == nil || *got.FeeSats != 10_000 {
		t.Errorf("feeSats = %v, want 10000", got.FeeSats)
	}
}

func TestGetTxPending(t *testing.T) {
	m := newMockBackend()
	p1 := hexID("pendparent")
	installParent(m, p1, vout(0, 1.0, addrA1))

	txid := hexID("pending")
	m.txs[txid] = &btcjson.TxRawResult{
		Txid:  txid,
		Vsize: 200,
		Vin:   []btcjson.Vin{vin(p1, 0)},
		Vout:  []btcjson.Vout{vout(0, 0.999, addrB1)},
	}

	// Our tx pays 100000 sats / 200 vB = 500 sat/vB. One richer entry
	// (600 sat/vB, 400 vB) and one poorer (1 sat/vB).
	m.mempool[txid] = btcjson.GetRawMempoolVerboseResult{
		Vsize: 200, Fee: 0.001, Time: 1735000300,
	}
	m.mempool[hexID("richer")] = btcjson.GetRawMempoolVerboseResult{
		Vsize: 400, Fee: 0.0024, Time: 1735000301,
	}
	m.mempool[hexID("poorer")] = btcjson.GetRawMempoolVerboseResult{
		Vsize: 150, Fee: 0.0000015, Time: 1735000302,
	}

	got, err := newTestService(m).GetTx(txid)
	if err != nil {
		t.Fatal(err)
	}

	if got.Status != "pending" || got.Block != nil {
		t.Fatalf("status=%q block=%v", got.Status, got.Block)
	}
	if got.Pending == nil {
		t.Fatal("pending = nil")
	}
	if got.Pending.TxsAhead != 1 || got.Pending.VBytesAhead != 400 {
		t.Errorf("txsAhead=%d vbytesAhead=%d, want 1/400",
			got.Pending.TxsAhead, got.Pending.VBytesAhead)
	}
	if got.Pending.EtaBlocks != 1 {
		t.Errorf("etaBlocks = %d, want 1", got.Pending.EtaBlocks)
	}
	// Empty mock chain (tip 0) → target time per block fallback.
	wantEta := int64(chaincfg.RegressionNetParams.TargetTimePerBlock.Seconds())
	if got.Pending.EtaSeconds != wantEta {
		t.Errorf("etaSeconds = %d, want %d", got.Pending.EtaSeconds, wantEta)
	}
	if got.Pending.QueueFraction != 0.5 {
		t.Errorf("queueFraction = %v, want 0.5", got.Pending.QueueFraction)
	}
	if got.FirstSeen != 1735000300 {
		t.Errorf("firstSeen = %d", got.FirstSeen)
	}
}

func TestGetTxNotFound(t *testing.T) {
	m := newMockBackend()
	_, err := newTestService(m).GetTx(hexID("missing"))
	if !errors.Is(err, ErrTxNotFound) {
		t.Fatalf("err = %v, want ErrTxNotFound", err)
	}
}

// TestPrevoutCache: a second lookup of a tx spending the same outpoint must
// not refetch the parent.
func TestPrevoutCache(t *testing.T) {
	m := newMockBackend()
	p1 := hexID("cachedparent")
	installParent(m, p1, vout(0, 1.0, addrA1))

	txid := hexID("cachehit")
	tx := &btcjson.TxRawResult{
		Txid:  txid,
		Vsize: 141,
		Vin:   []btcjson.Vin{vin(p1, 0)},
		Vout:  []btcjson.Vout{vout(0, 0.999, addrB1)},
	}
	confirm(m, tx, 10, 1735000400)
	m.txs[txid] = tx

	svc := newTestService(m)
	for range 2 {
		if _, err := svc.GetTx(txid); err != nil {
			t.Fatal(err)
		}
	}
	if n := m.txFetches[p1]; n != 1 {
		t.Errorf("parent fetched %d times, want 1", n)
	}
}
