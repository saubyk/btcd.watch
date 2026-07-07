package explorer

import (
	"testing"
	"time"

	"github.com/btcsuite/btcd/btcjson"
)

func TestQueueEmptyMempool(t *testing.T) {
	q := queueFromSnapshot(nil)

	if q.TxCount != 0 || q.TotalVBytes != 0 {
		t.Errorf("counts = %d/%d, want 0/0", q.TxCount, q.TotalVBytes)
	}
	if len(q.Bands) != 5 {
		t.Fatalf("bands = %d, want 5", len(q.Bands))
	}
	if q.CutoffFraction != 1 || q.NextBlockRate != 1 {
		t.Errorf("cutoff = %v rate = %v, want 1/1",
			q.CutoffFraction, q.NextBlockRate)
	}
}

func TestQueueBandAssignment(t *testing.T) {
	snap := map[string]MempoolEntry{
		"a": {FeeRate: 20, VSize: 100},  // 15+
		"b": {FeeRate: 12, VSize: 200},  // 10–15
		"c": {FeeRate: 7, VSize: 300},   // 6–10
		"d": {FeeRate: 5, VSize: 400},   // 4–6
		"e": {FeeRate: 2, VSize: 500},   // 1–4
		"f": {FeeRate: 0.5, VSize: 100}, // below 1 lumps into the last band
	}
	q := queueFromSnapshot(snap)

	if q.TxCount != 6 || q.TotalVBytes != 1600 {
		t.Errorf("counts = %d/%d, want 6/1600", q.TxCount, q.TotalVBytes)
	}
	want := []int64{100, 200, 300, 400, 600}
	for i, b := range q.Bands {
		if b.VBytes != want[i] {
			t.Errorf("band %d vbytes = %d, want %d", i, b.VBytes, want[i])
		}
	}
	if q.Bands[0].MaxSatPerVb != 0 || q.Bands[0].MinSatPerVb != 15 {
		t.Errorf("front band = %+v, want open-ended 15+", q.Bands[0])
	}
	if q.Bands[4].MinSatPerVb != 1 || q.Bands[4].MaxSatPerVb != 4 {
		t.Errorf("back band = %+v, want 1–4", q.Bands[4])
	}

	// Everything fits in one block: cutoff at the end, threshold is the
	// cheapest rate present.
	if q.CutoffFraction != 1 {
		t.Errorf("cutoff = %v, want 1", q.CutoffFraction)
	}
	if q.NextBlockRate != 0.5 {
		t.Errorf("nextBlockRate = %v, want 0.5", q.NextBlockRate)
	}
}

func TestQueueCutoffInsideBar(t *testing.T) {
	// 1.2 MvB waiting: the 1 MvB cutoff lands at 5/6 of the bar, inside
	// the cheaper entry.
	snap := map[string]MempoolEntry{
		"rich": {FeeRate: 50, VSize: 600_000},
		"poor": {FeeRate: 3, VSize: 600_000},
	}
	q := queueFromSnapshot(snap)

	if got, want := q.CutoffFraction, 1_000_000.0/1_200_000.0; got != want {
		t.Errorf("cutoff = %v, want %v", got, want)
	}
	if q.NextBlockRate != 3 {
		t.Errorf("nextBlockRate = %v, want 3", q.NextBlockRate)
	}
}

func TestPendingQueueVbytesFraction(t *testing.T) {
	m := newMockBackend()
	installChain(m, 3, 60*time.Second)

	// Our tx pays 5 sat/vB; 300 of the 500 total vB (self included) pay
	// more.
	addMempoolTx(m, "mine", 5, 100)
	addMempoolTx(m, "richer", 20, 300)
	addMempoolTx(m, "poorer", 2, 100)

	parent := hexID("parent")
	installParent(m, parent, vout(0, 1.0, addrA1))
	m.txs[hexID("mine")] = &btcjson.TxRawResult{
		Txid:  hexID("mine"),
		Vsize: 100,
		Vin:   []btcjson.Vin{vin(parent, 0)},
		Vout:  []btcjson.Vout{vout(0, 0.999, addrB1)},
	}

	tx, err := newTestService(m).GetTx(hexID("mine"))
	if err != nil {
		t.Fatal(err)
	}
	if tx.Pending == nil {
		t.Fatal("pending = nil")
	}
	if got, want := tx.Pending.QueueVbytesFraction, 0.6; got != want {
		t.Errorf("queueVbytesFraction = %v, want %v", got, want)
	}
}
