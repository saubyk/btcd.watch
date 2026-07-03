package explorer

import (
	"time"

	"btcdwatch.com/internal/chain"
)

type MempoolStats struct {
	TxCount int   `json:"txCount"`
	Bytes   int64 `json:"bytes"`
}

type HalvingStats struct {
	BlocksRemaining int64 `json:"blocksRemaining"`
	EtaSeconds      int64 `json:"etaSeconds"`
}

type PriceStats struct {
	USD       float64 `json:"usd"`
	Source    string  `json:"source"`
	UpdatedAt int64   `json:"updatedAt"`
}

// Stats is the /api/stats payload. Price is null when no price source is
// available.
type Stats struct {
	Network                 string       `json:"network"`
	BlockHeight             int64        `json:"blockHeight"`
	Mempool                 MempoolStats `json:"mempool"`
	NextBlockEtaSeconds     int64        `json:"nextBlockEtaSeconds"`
	AvgBlockIntervalSeconds int64        `json:"avgBlockIntervalSeconds"`
	Halving                 HalvingStats `json:"halving"`
	Price                   *PriceStats  `json:"price"`
}

// Stats assembles the landing-page dashboard numbers. Mempool count/bytes
// come from the shared snapshot (btcd's rpcclient has no getmempoolinfo
// wrapper, and the snapshot is already warm).
func (s *Service) Stats() (*Stats, error) {
	tip, err := s.backend.GetBlockCount()
	if err != nil {
		return nil, err
	}

	snapshot, err := s.mempool.Snapshot()
	if err != nil {
		return nil, err
	}
	var mempoolBytes int64
	for _, e := range snapshot {
		mempoolBytes += e.SizeBytes
	}

	interval := s.avgBlockInterval()

	// Expected time to the next block: the average interval minus the
	// tip's age, floored so the pill never reads zero/negative when a
	// block is overdue.
	nextEta := interval
	if tipTime, err := s.headerTimeAt(tip); err == nil {
		age := time.Since(time.Unix(tipTime, 0))
		nextEta = max(interval-age, 5*time.Second)
	}

	blocksRemaining := chain.BlocksUntilHalving(tip, s.params)

	stats := &Stats{
		Network:     s.params.Name,
		BlockHeight: tip,
		Mempool: MempoolStats{
			TxCount: len(snapshot),
			Bytes:   mempoolBytes,
		},
		NextBlockEtaSeconds:     int64(nextEta.Seconds()),
		AvgBlockIntervalSeconds: int64(interval.Seconds()),
		Halving: HalvingStats{
			BlocksRemaining: blocksRemaining,
			EtaSeconds:      blocksRemaining * int64(interval.Seconds()),
		},
	}

	if q := s.priceUSD(); q.OK {
		stats.Price = &PriceStats{
			USD:       q.USD,
			Source:    q.Source,
			UpdatedAt: q.UpdatedAt,
		}
	}
	return stats, nil
}
