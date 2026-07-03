package explorer

import (
	"testing"

	"github.com/btcsuite/btcd/btcjson"
)

// addMempoolTx installs a mempool entry at the given feerate (sat/vB).
func addMempoolTx(m *mockBackend, label string, rateSatPerVb float64, vsize int32) {
	feeBTC := rateSatPerVb * float64(vsize) / 100_000_000
	m.mempool[hexID(label)] = btcjson.GetRawMempoolVerboseResult{
		Vsize: vsize, Size: vsize, Fee: feeBTC, Time: 1,
	}
}

func TestFeesEmptyMempoolUsesFloors(t *testing.T) {
	m := newMockBackend()
	fees, err := newTestService(m).Fees()
	if err != nil {
		t.Fatal(err)
	}
	if fees.Source != "floor" {
		t.Errorf("source = %q, want floor", fees.Source)
	}
	want := []float64{1, 2, 5} // test floors: slow 1 / standard 2 / urgent 5
	for i, tier := range fees.Tiers {
		if tier.SatPerVb != want[i] {
			t.Errorf("tier %s = %v, want %v", tier.ID, tier.SatPerVb, want[i])
		}
	}
}

func TestFeesWeightedPercentiles(t *testing.T) {
	m := newMockBackend()
	// 10 entries of 100 vB each at rates 10,20,...,100: total 1000 vB.
	// vsize-weighted p25 → 30 (cum 300), p50 → 50, p90 → 90.
	for i := 1; i <= 10; i++ {
		addMempoolTx(m, string(rune('a'+i)), float64(i*10), 100)
	}

	fees, err := newTestService(m).Fees()
	if err != nil {
		t.Fatal(err)
	}
	if fees.Source != "mempool" {
		t.Errorf("source = %q, want mempool", fees.Source)
	}
	want := map[string]float64{"slow": 30, "standard": 50, "urgent": 90}
	for _, tier := range fees.Tiers {
		if tier.SatPerVb != want[tier.ID] {
			t.Errorf("tier %s = %v, want %v",
				tier.ID, tier.SatPerVb, want[tier.ID])
		}
	}
}

// TestFeesWeighting: one huge cheap tx must dominate the weighted
// percentiles even though a count-based percentile would ignore it.
func TestFeesWeighting(t *testing.T) {
	m := newMockBackend()
	addMempoolTx(m, "whale", 2, 9000) // 90% of vbytes at 2 sat/vB
	addMempoolTx(m, "shrimp1", 50, 500)
	addMempoolTx(m, "shrimp2", 100, 500)

	fees, err := newTestService(m).Fees()
	if err != nil {
		t.Fatal(err)
	}
	// p25 and p50 land inside the whale (clamped to floors 1/2), p90 at
	// the whale's upper edge (cum 9000 = target 9000).
	byID := map[string]float64{}
	for _, tier := range fees.Tiers {
		byID[tier.ID] = tier.SatPerVb
	}
	if byID["slow"] != 2 || byID["standard"] != 2 || byID["urgent"] != 5 {
		t.Errorf("tiers = %v, want slow 2 / standard 2 / urgent 5", byID)
	}
}

func TestFeesFloorsAndMonotonic(t *testing.T) {
	m := newMockBackend()
	// Everything at 0.5 sat/vB — below every floor.
	for _, l := range []string{"x", "y", "z"} {
		addMempoolTx(m, l, 0.5, 200)
	}

	fees, err := newTestService(m).Fees()
	if err != nil {
		t.Fatal(err)
	}
	var prev float64
	want := map[string]float64{"slow": 1, "standard": 2, "urgent": 5}
	for _, tier := range fees.Tiers {
		if tier.SatPerVb < prev {
			t.Errorf("tiers not monotonic at %s: %v < %v",
				tier.ID, tier.SatPerVb, prev)
		}
		if tier.SatPerVb != want[tier.ID] {
			t.Errorf("tier %s = %v, want floor %v",
				tier.ID, tier.SatPerVb, want[tier.ID])
		}
		prev = tier.SatPerVb
	}
}
