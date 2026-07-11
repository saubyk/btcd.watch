package explorer

import (
	"time"

	"github.com/btcsuite/btcd/chaincfg/v2"

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
	Syncing                 bool         `json:"syncing"`
	Mempool                 MempoolStats `json:"mempool"`
	Queue                   *Queue       `json:"queue"`
	NextBlockEtaSeconds     int64        `json:"nextBlockEtaSeconds"`
	AvgBlockIntervalSeconds int64        `json:"avgBlockIntervalSeconds"`
	Halving                 HalvingStats `json:"halving"`
	Price                   *PriceStats  `json:"price"`
}

// syncedMaxTipAge is how far the best block's timestamp may lag the wall
// clock before the node is treated as still syncing (or badly stalled).
// A synced mainnet tip is essentially never four hours old, while an IBD
// tip lags by days. Tip age is the signal because btcd's
// getblockchaininfo reports headers == blocks and never sets an
// initialblockdownload field, so the bitcoind-style checks don't work.
const syncedMaxTipAge = 4 * time.Hour

// syncCheckEvery bounds how often Syncing re-queries the node; every
// gated request shares the cached answer in between.
const syncCheckEvery = 10 * time.Second

// Syncing reports whether the node is still catching up to the network.
// On regtest and simnet blocks only exist on demand, so tip age means
// nothing there and the check always passes.
func (s *Service) Syncing() bool {
	switch s.params.Net {
	case chaincfg.RegressionNetParams.Net, chaincfg.SimNetParams.Net:
		return false
	}

	s.syncMu.Lock()
	defer s.syncMu.Unlock()
	if time.Since(s.syncCheckedAt) < syncCheckEvery {
		return s.syncing
	}

	tip, err := s.backend.GetBlockCount()
	if err != nil {
		// Node unreachable: keep the last answer — the request itself
		// will surface node_unavailable.
		return s.syncing
	}
	tipTime, err := s.headerTimeAt(tip)
	if err != nil {
		return s.syncing
	}

	s.syncing = time.Since(time.Unix(tipTime, 0)) > syncedMaxTipAge
	s.syncCheckedAt = time.Now()
	return s.syncing
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
	queue, err := s.mempool.Queue()
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
		Syncing:     s.Syncing(),
		Mempool: MempoolStats{
			TxCount: len(snapshot),
			Bytes:   mempoolBytes,
		},
		Queue:                   queue,
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
