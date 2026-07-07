package explorer

import (
	"errors"
	"fmt"

	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/chainhash/v2"

	"btcdwatch.com/internal/chain"
)

// ErrBlockNotFound is returned for unknown heights and hashes.
var ErrBlockNotFound = errors.New("block not found")

// BlockTx is one row of a block's transaction list.
type BlockTx struct {
	Txid       string `json:"txid"`
	AmountSats int64  `json:"amountSats"`
	// FeeRateSatPerVb is null for the coinbase.
	FeeRateSatPerVb *float64 `json:"feeRateSatPerVb"`
	IsCoinbase      bool     `json:"isCoinbase"`
}

// Block is the /api/block response payload.
type Block struct {
	Height        int64  `json:"height"`
	Hash          string `json:"hash"`
	Time          int64  `json:"time"`
	Confirmations int64  `json:"confirmations"`
	TxCount       int    `json:"txCount"`
	// AvgFeeSatPerVb is derived from the coinbase (outputs minus
	// subsidy) over the block's non-coinbase vsize, so it needs no scan
	// of the full transaction list.
	AvgFeeSatPerVb float64 `json:"avgFeeSatPerVb"`
	SizeBytes      int64   `json:"sizeBytes"`
	// NextHeight is null at the chain tip.
	NextHeight *int64    `json:"nextHeight"`
	Txs        []BlockTx `json:"txs"`
	Offset     int       `json:"offset"`
	Limit      int       `json:"limit"`
	HasMore    bool      `json:"hasMore"`
}

// BlockByHeight resolves a height to its hash and derives the payload.
func (s *Service) BlockByHeight(height int64, offset, limit int) (*Block, error) {
	if height < 0 {
		return nil, ErrBlockNotFound
	}
	hash, err := s.backend.GetBlockHash(height)
	if err != nil {
		if isBlockMissing(err) {
			return nil, ErrBlockNotFound
		}
		return nil, err
	}
	return s.block(hash, offset, limit)
}

// BlockByHash derives the payload for a block hash in hex.
func (s *Service) BlockByHash(hashStr string, offset, limit int) (*Block, error) {
	hash, err := chainhash.NewHashFromStr(hashStr)
	if err != nil {
		return nil, ErrBlockNotFound
	}
	return s.block(hash, offset, limit)
}

func (s *Service) block(hash *chainhash.Hash, offset, limit int) (*Block, error) {
	raw, err := s.backend.GetBlockVerbose(hash)
	if err != nil {
		if isBlockMissing(err) {
			return nil, ErrBlockNotFound
		}
		return nil, err
	}

	block := &Block{
		Height:        raw.Height,
		Hash:          raw.Hash,
		Time:          raw.Time,
		Confirmations: raw.Confirmations,
		TxCount:       len(raw.Tx),
		SizeBytes:     int64(raw.Size),
		Offset:        offset,
		Limit:         limit,
	}
	if raw.NextHash != "" {
		next := raw.Height + 1
		block.NextHeight = &next
	}

	// Page of transaction rows. Each row reuses GetTx so amounts match
	// the transaction view (change heuristic included) and fee rates come
	// from cached prevouts.
	end := min(offset+limit, len(raw.Tx))
	var coinbase *Tx
	block.Txs = make([]BlockTx, 0, max(end-offset, 0))
	for i := offset; i < end; i++ {
		tx, err := s.GetTx(raw.Tx[i])
		if err != nil {
			return nil, fmt.Errorf("block tx %s: %w", raw.Tx[i], err)
		}
		if tx.IsCoinbase {
			coinbase = tx
		}
		block.Txs = append(block.Txs, BlockTx{
			Txid:            tx.Txid,
			AmountSats:      tx.AmountSats,
			FeeRateSatPerVb: tx.FeeRateSatPerVb,
			IsCoinbase:      tx.IsCoinbase,
		})
	}
	block.HasMore = end < len(raw.Tx)

	if err := s.deriveAvgFee(block, raw, coinbase); err != nil {
		return nil, err
	}
	return block, nil
}

// deriveAvgFee computes the block's average feerate from the coinbase:
// total fees = coinbase outputs − subsidy, spread over the block's
// non-coinbase vsize. The first page already fetched the coinbase as a
// row; later pages fetch it separately.
func (s *Service) deriveAvgFee(block *Block,
	raw *btcjson.GetBlockVerboseResult, coinbase *Tx) error {

	if len(raw.Tx) < 2 {
		return nil // only a coinbase — no fees paid
	}

	if coinbase == nil {
		var err error
		if coinbase, err = s.GetTx(raw.Tx[0]); err != nil {
			return fmt.Errorf("coinbase tx %s: %w", raw.Tx[0], err)
		}
	}

	// A coinbase has no change heuristic, so AmountSats is the full
	// output total: subsidy + fees.
	fees := coinbase.AmountSats - chain.BlockSubsidy(block.Height, s.params)

	blockVsize := int64(raw.Weight+3) / 4
	if blockVsize == 0 {
		blockVsize = int64(raw.Size)
	}

	if vsize := blockVsize - coinbase.VSize; fees > 0 && vsize > 0 {
		block.AvgFeeSatPerVb = float64(fees) / float64(vsize)
	}
	return nil
}

func isBlockMissing(err error) bool {
	code, ok := rpcErrCode(err)
	return ok && (code == btcjson.ErrRPCBlockNotFound ||
		code == btcjson.ErrRPCOutOfRange ||
		code == btcjson.ErrRPCInvalidAddressOrKey)
}
