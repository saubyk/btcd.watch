package explorer

import (
	"fmt"
	"testing"

	"github.com/btcsuite/btcd/btcjson"

	"btcd.watch/internal/node"
)

// installSpendTx installs a confirmed tx with one input and the given
// outputs, returning its txid.
func installSpendTx(m *mockBackend, label string, vouts ...btcjson.Vout) string {
	parent := hexID(label + "parent")
	installParent(m, parent, vout(0, 100, addrA1))

	txid := hexID(label)
	tx := &btcjson.TxRawResult{
		Txid:  txid,
		Vsize: 200,
		Vin:   []btcjson.Vin{vin(parent, 0)},
		Vout:  vouts,
	}
	confirm(m, tx, 512, 1735000000)
	m.txs[txid] = tx
	return txid
}

// TestGetTxOutputSpendState: outputs still in the UTXO set read unspent,
// missing ones spent, OP_RETURN unspendable; inputs carry no state.
func TestGetTxOutputSpendState(t *testing.T) {
	m := newMockBackend()
	txid := installSpendTx(m, "spendstate",
		vout(0, 1.0, addrB1),
		vout(1, 0.5, addrA1),
		voutTyped(2, 0, "nulldata"),
	)
	m.utxos[txid+":0"] = true

	got, err := newTestService(m).GetTx(txid)
	if err != nil {
		t.Fatal(err)
	}

	want := []string{spendUnspent, spendSpent, spendUnspendable}
	if len(got.Outputs) != len(want) {
		t.Fatalf("outputs = %d, want %d", len(got.Outputs), len(want))
	}
	for i, w := range want {
		if got.Outputs[i].Spend != w {
			t.Errorf("output %d spend = %q, want %q", i, got.Outputs[i].Spend, w)
		}
	}
	for i, in := range got.Inputs {
		if in.Spend != "" {
			t.Errorf("input %d spend = %q, want empty", i, in.Spend)
		}
	}
	// The OP_RETURN output needs no lookup.
	if c := m.getTxOutCalls(); c != 2 {
		t.Errorf("gettxout calls = %d, want 2", c)
	}
}

// TestGetTxPendingSkipsSpendState: btcd answers a mempool tx's outputs
// from the mempool, blind to in-mempool spends, so pending txs are not
// labeled at all.
func TestGetTxPendingSkipsSpendState(t *testing.T) {
	m := newMockBackend()
	parent := hexID("pendspendparent")
	installParent(m, parent, vout(0, 1.0, addrA1))
	txid := hexID("pendspend")
	m.txs[txid] = &btcjson.TxRawResult{
		Txid:  txid,
		Vsize: 200,
		Vin:   []btcjson.Vin{vin(parent, 0)},
		Vout:  []btcjson.Vout{vout(0, 0.999, addrB1)},
	}
	m.mempool[txid] = btcjson.GetRawMempoolVerboseResult{Vsize: 200, Fee: 0.001}
	m.utxos[txid+":0"] = true

	got, err := newTestService(m).GetTx(txid)
	if err != nil {
		t.Fatal(err)
	}
	if got.Outputs[0].Spend != "" {
		t.Errorf("pending output spend = %q, want empty", got.Outputs[0].Spend)
	}
	if c := m.getTxOutCalls(); c != 0 {
		t.Errorf("gettxout calls = %d, want 0", c)
	}
}

// TestGetTxSpendLookupFailureDegrades: a failing gettxout leaves rows
// unlabeled instead of failing the whole transaction view.
func TestGetTxSpendLookupFailureDegrades(t *testing.T) {
	m := newMockBackend()
	txid := installSpendTx(m, "spendfail", vout(0, 1.0, addrB1))
	m.txOutErr = node.ErrUnavailable

	got, err := newTestService(m).GetTx(txid)
	if err != nil {
		t.Fatalf("GetTx err = %v, want degraded success", err)
	}
	if got.Outputs[0].Spend != "" {
		t.Errorf("output spend = %q, want empty", got.Outputs[0].Spend)
	}
	if got.AmountSats != 1_0000_0000 {
		t.Errorf("amount = %d, rest of payload should be intact", got.AmountSats)
	}
}

// TestGetTxSpendStateCapped: only the first maxSpendChecks outputs are
// looked up; the rest stay unlabeled.
func TestGetTxSpendStateCapped(t *testing.T) {
	m := newMockBackend()
	n := maxSpendChecks + 5
	vouts := make([]btcjson.Vout, n)
	for i := range vouts {
		vouts[i] = vout(uint32(i), 0.001, fmt.Sprintf("bcrt1qpayee%d", i))
	}
	txid := installSpendTx(m, "spendcap", vouts...)
	for i := range n {
		m.utxos[fmt.Sprintf("%s:%d", txid, i)] = true
	}

	got, err := newTestService(m).GetTx(txid)
	if err != nil {
		t.Fatal(err)
	}
	for i, out := range got.Outputs {
		want := spendUnspent
		if i >= maxSpendChecks {
			want = ""
		}
		if out.Spend != want {
			t.Errorf("output %d spend = %q, want %q", i, out.Spend, want)
		}
	}
	if c := m.getTxOutCalls(); c != maxSpendChecks {
		t.Errorf("gettxout calls = %d, want %d", c, maxSpendChecks)
	}
}

// TestBlockPageSkipsSpendState: block rows reuse the tx derivation but
// must not issue per-output gettxout lookups.
func TestBlockPageSkipsSpendState(t *testing.T) {
	m := newMockBackend()
	installBlock(m)

	if _, err := newTestService(m).BlockByHeight(5, 0, 25); err != nil {
		t.Fatal(err)
	}
	if c := m.getTxOutCalls(); c != 0 {
		t.Errorf("gettxout calls = %d, want 0", c)
	}
}
