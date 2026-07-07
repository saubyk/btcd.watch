package explorer

import (
	"testing"

	"github.com/btcsuite/btcd/btcjson"
)

// voutTyped is vout() with an explicit scriptPubKey type.
func voutTyped(n uint32, valueBTC float64, class string, addrs ...string) btcjson.Vout {
	v := vout(n, valueBTC, addrs...)
	v.ScriptPubKey.Type = class
	return v
}

// vinSeq is vin() with an explicit sequence (vin()'s zero value signals
// RBF per BIP-125, which real btcd wallets rarely emit by accident).
func vinSeq(txid string, n uint32, seq uint32) btcjson.Vin {
	v := vin(txid, n)
	v.Sequence = seq
	return v
}

func TestGetTxTypeAndRBF(t *testing.T) {
	m := newMockBackend()

	// Parent: one P2WPKH output and one P2PKH output.
	parent := hexID("typeparent")
	m.txs[parent] = &btcjson.TxRawResult{
		Txid: parent,
		Vout: []btcjson.Vout{
			voutTyped(0, 1.0, "witness_v0_keyhash", addrA1),
			voutTyped(1, 1.0, "pubkeyhash", addrA2),
		},
	}

	// Two SegWit inputs + one legacy input → dominant input P2WPKH.
	// Outputs pay taproot. Final sequences → no RBF signal.
	txid := hexID("typed")
	m.txs[txid] = &btcjson.TxRawResult{
		Txid:  txid,
		Vsize: 300,
		Vin: []btcjson.Vin{
			vinSeq(parent, 0, 0xffffffff),
			vinSeq(parent, 0, 0xfffffffe),
			vinSeq(parent, 1, 0xffffffff),
		},
		Vout: []btcjson.Vout{
			voutTyped(0, 2.9, "witness_v1_taproot", addrB1),
		},
	}
	confirm(m, m.txs[txid], 3, 1735000000)
	installChain(m, 3, 60)

	tx, err := newTestService(m).GetTx(txid)
	if err != nil {
		t.Fatal(err)
	}
	if tx.Type == nil {
		t.Fatal("type = nil")
	}
	if tx.Type.Code != "P2WPKH" || tx.Type.In != "P2WPKH" || tx.Type.Out != "P2TR" {
		t.Errorf("type = %+v, want P2WPKH/P2WPKH/P2TR", tx.Type)
	}
	if tx.Rbf {
		t.Error("rbf = true for final sequences")
	}
}

// TestGetTxTypeExcludesChange: the "out" side classifies the actual
// recipient, not the change output, whatever the vout order.
func TestGetTxTypeExcludesChange(t *testing.T) {
	m := newMockBackend()
	parent := hexID("chgparent")
	installParent(m, parent, voutTyped(0, 1.0, "witness_v0_keyhash", addrA1))

	// Change (P2WPKH back to the sender) listed FIRST, taproot payment
	// second — position must not decide the type.
	txid := hexID("chgtx")
	m.txs[txid] = &btcjson.TxRawResult{
		Txid:  txid,
		Vsize: 200,
		Vin:   []btcjson.Vin{vinSeq(parent, 0, 0xffffffff)},
		Vout: []btcjson.Vout{
			voutTyped(0, 0.4, "witness_v0_keyhash", addrA1), // change
			voutTyped(1, 0.59, "witness_v1_taproot", addrB1),
		},
	}
	confirm(m, m.txs[txid], 3, 1735000000)
	installChain(m, 3, 60)

	tx, err := newTestService(m).GetTx(txid)
	if err != nil {
		t.Fatal(err)
	}
	if tx.Type == nil || tx.Type.Out != "P2TR" {
		t.Errorf("type = %+v, want out P2TR (change excluded)", tx.Type)
	}
}

func TestDominantCodeFirstSeenTie(t *testing.T) {
	got := dominantCode([]string{"P2SH", "P2WPKH", "P2WPKH", "P2SH"})
	if got != "P2SH" {
		t.Errorf("dominantCode tie = %q, want first-seen P2SH", got)
	}
}

func TestGetTxRBFSignaled(t *testing.T) {
	m := newMockBackend()
	parent := hexID("rbfparent")
	installParent(m, parent, vout(0, 1.0, addrA1))

	txid := hexID("rbftx")
	m.txs[txid] = &btcjson.TxRawResult{
		Txid:  txid,
		Vsize: 140,
		Vin: []btcjson.Vin{
			vinSeq(parent, 0, 0xfffffffd), // BIP-125 opt-in
		},
		Vout: []btcjson.Vout{vout(0, 0.999, addrB1)},
	}
	addMempoolTx(m, "rbftx", 5, 140)

	tx, err := newTestService(m).GetTx(txid)
	if err != nil {
		t.Fatal(err)
	}
	if !tx.Rbf {
		t.Error("rbf = false for sequence 0xfffffffd")
	}
	// vout() fixtures are witness_v0_keyhash → P2WPKH both sides.
	if tx.Type == nil || tx.Type.Code != "P2WPKH" {
		t.Errorf("type = %+v, want P2WPKH", tx.Type)
	}
}

func TestGetTxTypeCoinbaseAndNonStandard(t *testing.T) {
	m := newMockBackend()

	coinbase := hexID("cbtyped")
	m.txs[coinbase] = &btcjson.TxRawResult{
		Txid:  coinbase,
		Vsize: 100,
		Vin:   []btcjson.Vin{{Coinbase: "abcd"}},
		Vout:  []btcjson.Vout{voutTyped(0, 50, "witness_v0_keyhash", addrB1)},
	}
	confirm(m, m.txs[coinbase], 2, 1735000000)
	installChain(m, 2, 60)

	tx, err := newTestService(m).GetTx(coinbase)
	if err != nil {
		t.Fatal(err)
	}
	if tx.Rbf {
		t.Error("coinbase signals rbf")
	}
	if tx.Type == nil || tx.Type.Code != "P2WPKH" || tx.Type.In != "" {
		t.Errorf("coinbase type = %+v, want out-only P2WPKH", tx.Type)
	}

	// Nothing classifiable on either side → nil.
	weird := hexID("weird")
	m.txs[weird] = &btcjson.TxRawResult{
		Txid:  weird,
		Vsize: 90,
		Vin:   []btcjson.Vin{{Coinbase: "abcd"}},
		Vout:  []btcjson.Vout{voutTyped(0, 50, "nulldata")},
	}
	confirm(m, m.txs[weird], 2, 1735000000)

	tx, err = newTestService(m).GetTx(weird)
	if err != nil {
		t.Fatal(err)
	}
	if tx.Type != nil {
		t.Errorf("non-standard type = %+v, want nil", tx.Type)
	}
}
