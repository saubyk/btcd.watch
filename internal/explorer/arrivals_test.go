package explorer

import (
	"fmt"
	"testing"

	"github.com/btcsuite/btcd/btcjson"
)

func noteArrival(svc *Service, m *mockBackend, label string,
	rate float64, vsize int32, amountBTC float64) {

	addMempoolTx(m, label, rate, vsize)
	svc.NoteArrival(&btcjson.TxRawResult{
		Txid: hexID(label),
		Vout: []btcjson.Vout{vout(0, amountBTC, addrB1)},
	})
}

func TestMempoolUpdateResolvesArrivals(t *testing.T) {
	m := newMockBackend()
	svc := newTestService(m)

	noteArrival(svc, m, "first", 8, 140, 0.5)
	noteArrival(svc, m, "second", 3, 200, 1.25)

	update, err := svc.computeMempoolUpdate()
	if err != nil {
		t.Fatal(err)
	}

	if update.Queue == nil || update.Queue.TxCount != 2 {
		t.Fatalf("queue = %+v", update.Queue)
	}
	if len(update.Arrivals) != 2 {
		t.Fatalf("arrivals = %d, want 2", len(update.Arrivals))
	}
	// Both accepts were counted moments ago, so the young-window floor
	// (10s) yields 2 tx over 10s.
	if update.InflowTxPerMin != 12 {
		t.Errorf("inflow = %v tx/min, want 12", update.InflowTxPerMin)
	}
	// Newest first.
	got := update.Arrivals[0]
	if got.Txid != hexID("second") || got.FeeRateSatPerVb != 3 ||
		got.VSize != 200 || got.AmountSats != 1_2500_0000 {

		t.Errorf("newest arrival = %+v", got)
	}
}

func TestMempoolUpdateSkipsUnresolvedArrivals(t *testing.T) {
	m := newMockBackend()
	svc := newTestService(m)

	// Announced but not (yet) in the mempool — e.g. mined immediately.
	svc.NoteArrival(&btcjson.TxRawResult{
		Txid: hexID("ghost"),
		Vout: []btcjson.Vout{vout(0, 1, addrB1)},
	})
	noteArrival(svc, m, "real", 5, 150, 0.25)

	update, err := svc.computeMempoolUpdate()
	if err != nil {
		t.Fatal(err)
	}
	if len(update.Arrivals) != 1 || update.Arrivals[0].Txid != hexID("real") {
		t.Fatalf("arrivals = %+v, want only the real tx", update.Arrivals)
	}
}

func TestNoteArrivalCapsAndDedupes(t *testing.T) {
	m := newMockBackend()
	svc := newTestService(m)

	for i := range arrivalsCap + 4 {
		noteArrival(svc, m, fmt.Sprintf("tx%02d", i), 5, 100, 0.1)
	}
	// Rebroadcast of the newest keeps a single entry.
	svc.NoteArrival(&btcjson.TxRawResult{
		Txid: hexID(fmt.Sprintf("tx%02d", arrivalsCap+3)),
		Vout: []btcjson.Vout{vout(0, 0.1, addrB1)},
	})

	svc.arrivalsMu.Lock()
	n := len(svc.arrivals)
	newest := svc.arrivals[0].txid
	seen := map[string]int{}
	for _, a := range svc.arrivals {
		seen[a.txid]++
	}
	svc.arrivalsMu.Unlock()

	if n != arrivalsCap {
		t.Errorf("buffer = %d entries, want %d", n, arrivalsCap)
	}
	if want := hexID(fmt.Sprintf("tx%02d", arrivalsCap+3)); newest != want {
		t.Errorf("newest = %s, want %s", newest, want)
	}
	for txid, count := range seen {
		if count > 1 {
			t.Errorf("duplicate buffer entry for %s", txid)
		}
	}
}

func TestQueuePeakTracksRollingMax(t *testing.T) {
	m := newMockBackend()
	svc := newTestService(m)

	addMempoolTx(m, "big", 5, 2000)
	q1, err := svc.Mempool().Queue()
	if err != nil {
		t.Fatal(err)
	}
	if q1.PeakVBytes != 2000 {
		t.Errorf("peak = %d, want 2000", q1.PeakVBytes)
	}

	// The mempool drains; the peak remembers the recent high while the
	// total drops.
	delete(m.mempool, hexID("big"))
	addMempoolTx(m, "small", 5, 300)
	svc.Mempool().Invalidate()

	q2, err := svc.Mempool().Queue()
	if err != nil {
		t.Fatal(err)
	}
	if q2.TotalVBytes != 300 {
		t.Errorf("total = %d, want 300", q2.TotalVBytes)
	}
	if q2.PeakVBytes != 2000 {
		t.Errorf("peak = %d, want 2000 (rolling max)", q2.PeakVBytes)
	}
}
