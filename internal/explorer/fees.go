package explorer

import (
	"fmt"
	"math"
	"sort"
	"time"
)

// FeeFloors are the configured minimum sat/vB per tier, used verbatim when
// the mempool is empty and as clamps otherwise.
type FeeFloors struct {
	Slow     float64
	Standard float64
	Urgent   float64
}

type FeeTier struct {
	ID       string  `json:"id"`
	SatPerVb float64 `json:"satPerVb"`
	// EtaBlocks is how many blocks a tx at this rate typically waits.
	EtaBlocks int64  `json:"etaBlocks"`
	Label     string `json:"label"`
}

type FeeEstimate struct {
	Tiers  []FeeTier `json:"tiers"`
	Source string    `json:"source"` // "mempool" | "floor"
}

// Fees derives the three tiers. btcd has no estimatesmartfee, so rates are
// vsize-weighted feerate percentiles of the mempool snapshot — p25 (slow),
// p50 (standard), p90 (urgent) — clamped to the configured floors, forced
// monotonic, minimum 1 sat/vB.
func (s *Service) Fees() (*FeeEstimate, error) {
	snapshot, err := s.mempool.Snapshot()
	if err != nil {
		return nil, err
	}

	slow, standard, urgent, source := s.tierRates(snapshot)

	interval := s.avgBlockInterval()
	tier := func(id string, rate float64, blocks int64) FeeTier {
		return FeeTier{
			ID:        id,
			SatPerVb:  rate,
			EtaBlocks: blocks,
			Label:     humanEta(time.Duration(blocks) * interval),
		}
	}

	return &FeeEstimate{
		Tiers: []FeeTier{
			tier("slow", slow, 6),
			tier("standard", standard, 3),
			tier("urgent", urgent, 1),
		},
		Source: source,
	}, nil
}

func (s *Service) tierRates(snapshot map[string]MempoolEntry) (slow, standard, urgent float64, source string) {
	if len(snapshot) == 0 {
		slow, standard, urgent = s.floors.Slow, s.floors.Standard, s.floors.Urgent
	} else {
		type weighted struct {
			rate  float64
			vsize int64
		}
		entries := make([]weighted, 0, len(snapshot))
		var totalVsize int64
		for _, e := range snapshot {
			entries = append(entries, weighted{rate: e.FeeRate, vsize: e.VSize})
			totalVsize += e.VSize
		}
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].rate < entries[j].rate
		})

		// percentile p: the lowest feerate at which p of the mempool's
		// vbytes sit at or below — weighting by size approximates queue
		// position by block space.
		percentile := func(p float64) float64 {
			target := p * float64(totalVsize)
			var cum int64
			for _, e := range entries {
				cum += e.vsize
				if float64(cum) >= target {
					return e.rate
				}
			}
			return entries[len(entries)-1].rate
		}

		slow = math.Max(percentile(0.25), s.floors.Slow)
		standard = math.Max(percentile(0.50), s.floors.Standard)
		urgent = math.Max(percentile(0.90), s.floors.Urgent)
		source = "mempool"
	}

	slow = math.Max(math.Round(slow*10)/10, 1)
	standard = math.Max(math.Round(standard*10)/10, slow)
	urgent = math.Max(math.Round(urgent*10)/10, standard)
	if source == "" {
		source = "floor"
	}
	return slow, standard, urgent, source
}

// humanEta renders a duration the way the design copy does ("~30 min",
// "~2 hrs").
func humanEta(d time.Duration) string {
	switch {
	case d < 90*time.Second:
		return "~1 min"
	case d < 90*time.Minute:
		return fmt.Sprintf("~%d min", int(math.Round(d.Minutes())))
	case d < 48*time.Hour:
		return fmt.Sprintf("~%d hrs", int(math.Round(d.Hours())))
	default:
		return fmt.Sprintf("~%d days", int(math.Round(d.Hours()/24)))
	}
}
