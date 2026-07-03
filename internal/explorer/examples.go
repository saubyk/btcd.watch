package explorer

import (
	"github.com/btcsuite/btcd/chainhash/v2"
)

// Examples is real chip data for the landing page — never fabricated.
// Null fields hide the corresponding chip.
type Examples struct {
	PendingTxid   *string `json:"pendingTxid"`
	ConfirmedTxid *string `json:"confirmedTxid"`
	Address       *string `json:"address"`
}

// exampleWalkback bounds how many recent blocks are scanned for a
// non-coinbase example transaction.
const exampleWalkback = 10

// Examples picks a live pending txid (highest feerate in the mempool), a
// recent confirmed non-coinbase txid, and one of its output addresses.
// The confirmed pair is cached per block; the pending pick reads the
// shared snapshot each call.
func (s *Service) Examples() (*Examples, error) {
	out := &Examples{}

	snapshot, err := s.mempool.Snapshot()
	if err != nil {
		return nil, err
	}
	var bestRate float64
	for txid, e := range snapshot {
		if e.FeeRate > bestRate {
			bestRate = e.FeeRate
			id := txid
			out.PendingTxid = &id
		}
	}

	tip, err := s.backend.GetBlockCount()
	if err != nil {
		return nil, err
	}

	s.examplesMu.Lock()
	defer s.examplesMu.Unlock()
	if s.examplesTip != tip {
		confirmed, addr := s.findConfirmedExample(tip)
		s.examplesTip = tip
		s.examplesConfirmed = confirmed
		s.examplesAddress = addr
	}
	out.ConfirmedTxid = s.examplesConfirmed
	out.Address = s.examplesAddress
	return out, nil
}

// findConfirmedExample walks back from the tip looking for the first
// non-coinbase transaction (index 0 of every block is the coinbase) and
// one of its output addresses.
func (s *Service) findConfirmedExample(tip int64) (*string, *string) {
	for height := tip; height > tip-exampleWalkback && height > 0; height-- {
		blockHash, err := s.backend.GetBlockHash(height)
		if err != nil {
			return nil, nil
		}
		block, err := s.backend.GetBlockVerbose(blockHash)
		if err != nil {
			return nil, nil
		}
		if len(block.Tx) < 2 {
			continue
		}

		txid := block.Tx[1]
		var addr *string
		if hash, err := chainhash.NewHashFromStr(txid); err == nil {
			if raw, err := s.backend.GetRawTransactionVerbose(hash); err == nil {
			outer:
				for _, v := range raw.Vout {
					for _, a := range voutAddrs(v) {
						addr = &a
						break outer
					}
				}
			}
		}
		return &txid, addr
	}
	return nil, nil
}
