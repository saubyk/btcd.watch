// Package explorer derives everything the UI shows from raw btcd RPC
// results: fees, from/to heuristics, amounts, queue position, and (in later
// milestones) address aggregation, fee tiers, and network stats. It talks
// only to node.Backend so all derivation is unit-testable against fixtures.
package explorer

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/btcutil/v2"
	"github.com/btcsuite/btcd/chaincfg/v2"
	"github.com/btcsuite/btcd/chainhash/v2"

	"btcdwatch.com/internal/node"
)

// ErrTxNotFound is returned when the node has no information about a txid.
var ErrTxNotFound = errors.New("transaction not found")

// nonStandardLabel stands in for outputs whose script decodes to no
// address.
const nonStandardLabel = "(non-standard output)"

// vbytesPerBlock approximates the block space ahead of a pending tx that
// one block clears (1 MvB of standard weight).
const vbytesPerBlock = 1_000_000

// TxBlock identifies the block containing a confirmed transaction.
type TxBlock struct {
	Height int64  `json:"height"`
	Hash   string `json:"hash"`
	Time   int64  `json:"time"`
}

// TxPending describes a pending transaction's position in the mempool
// queue.
type TxPending struct {
	TxsAhead      int     `json:"txsAhead"`
	VBytesAhead   int64   `json:"vbytesAhead"`
	EtaBlocks     int64   `json:"etaBlocks"`
	EtaSeconds    int64   `json:"etaSeconds"`
	QueueFraction float64 `json:"queueFraction"`
}

// Tx is the /api/tx response payload. Amounts are satoshis; nullable
// fields are pointers.
type Tx struct {
	Txid            string     `json:"txid"`
	Status          string     `json:"status"`
	AmountSats      int64      `json:"amountSats"`
	FiatUSD         *float64   `json:"fiatUsd"`
	From            []string   `json:"from"`
	To              []string   `json:"to"`
	IsCoinbase      bool       `json:"isCoinbase"`
	Confirmations   int64      `json:"confirmations"`
	Block           *TxBlock   `json:"block"`
	FeeSats         *int64     `json:"feeSats"`
	FeeRateSatPerVb *float64   `json:"feeRateSatPerVb"`
	VSize           int64      `json:"vsize"`
	FirstSeen       int64      `json:"firstSeen"`
	Pending         *TxPending `json:"pending"`
}

// prevout is the cached slice of a parent transaction an input spends.
type prevout struct {
	valueSats int64
	addrs     []string
}

// PriceQuote is the current BTC/USD price as seen by the explorer. OK is
// false when no price is available (fiat fields are then null).
type PriceQuote struct {
	USD       float64
	Source    string
	UpdatedAt int64
	OK        bool
}

// PriceFunc reports the current BTC/USD price.
type PriceFunc func() PriceQuote

// Service derives explorer responses from a node backend.
type Service struct {
	backend  node.Backend
	params   *chaincfg.Params
	mempool  *Mempool
	priceUSD PriceFunc
	floors   FeeFloors

	prevouts *lruCache[prevout]
	headers  *lruCache[*btcjson.GetBlockHeaderVerboseResult]

	intervalMu   sync.Mutex
	intervalAt   time.Time
	intervalMean time.Duration

	examplesMu        sync.Mutex
	examplesTip       int64
	examplesConfirmed *string
	examplesAddress   *string
}

func NewService(backend node.Backend, params *chaincfg.Params,
	mempool *Mempool, price PriceFunc, floors FeeFloors) *Service {

	return &Service{
		backend:  backend,
		params:   params,
		mempool:  mempool,
		priceUSD: price,
		floors:   floors,
		prevouts: newLRU[prevout](4096),
		headers:  newLRU[*btcjson.GetBlockHeaderVerboseResult](1024),
	}
}

// GetTx looks up a transaction and derives the full API payload.
func (s *Service) GetTx(txid string) (*Tx, error) {
	hash, err := chainhash.NewHashFromStr(txid)
	if err != nil {
		return nil, ErrTxNotFound
	}

	raw, err := s.backend.GetRawTransactionVerbose(hash)
	if err != nil {
		if isNoTxInfo(err) {
			return nil, ErrTxNotFound
		}
		return nil, err
	}

	tx := &Tx{
		Txid:          raw.Txid,
		Confirmations: int64(raw.Confirmations),
		VSize:         int64(raw.Vsize),
		From:          []string{},
		To:            []string{},
	}
	if tx.VSize == 0 {
		tx.VSize = int64(raw.Size)
	}

	isCoinbase := len(raw.Vin) > 0 && raw.Vin[0].IsCoinBase()
	tx.IsCoinbase = isCoinbase

	var sumInSats int64
	if !isCoinbase {
		ins, err := s.resolvePrevouts(raw.Vin)
		if err != nil {
			return nil, fmt.Errorf("resolve inputs: %w", err)
		}
		seen := make(map[string]bool)
		for _, in := range ins {
			sumInSats += in.valueSats
			for _, a := range in.addrs {
				if !seen[a] {
					seen[a] = true
					tx.From = append(tx.From, a)
				}
			}
		}
	}

	s.deriveOutputs(tx, raw.Vout, isCoinbase, sumInSats)

	if q := s.priceUSD(); q.OK {
		fiat := btcutil.Amount(tx.AmountSats).ToBTC() * q.USD
		tx.FiatUSD = &fiat
	}

	if raw.BlockHash != "" {
		tx.Status = "confirmed"
		header, err := s.header(raw.BlockHash)
		if err != nil {
			return nil, fmt.Errorf("block header: %w", err)
		}
		tx.Block = &TxBlock{
			Height: int64(header.Height),
			Hash:   header.Hash,
			Time:   header.Time,
		}
		tx.FirstSeen = header.Time
		return tx, nil
	}

	tx.Status = "pending"
	if err := s.derivePending(tx); err != nil {
		return nil, fmt.Errorf("pending position: %w", err)
	}
	return tx, nil
}

// deriveOutputs fills amount, to, fee, and feerate. From-addresses must
// already be populated (change heuristic: a vout whose addresses are all in
// the from-set is treated as change and excluded; if that excludes every
// output — a self-send — all outputs count).
func (s *Service) deriveOutputs(tx *Tx, vouts []btcjson.Vout,
	isCoinbase bool, sumInSats int64) {

	fromSet := make(map[string]bool, len(tx.From))
	for _, a := range tx.From {
		fromSet[a] = true
	}

	isChange := func(addrs []string) bool {
		if isCoinbase || len(addrs) == 0 {
			return false
		}
		for _, a := range addrs {
			if !fromSet[a] {
				return false
			}
		}
		return true
	}

	var sumOutSats, amountSats int64
	var toAddrs []string
	seen := make(map[string]bool)
	addTo := func(addrs []string) {
		if len(addrs) == 0 {
			addrs = []string{nonStandardLabel}
		}
		for _, a := range addrs {
			if !seen[a] {
				seen[a] = true
				toAddrs = append(toAddrs, a)
			}
		}
	}

	for _, v := range vouts {
		sats := satsFromBTC(v.Value)
		sumOutSats += sats
		if isChange(voutAddrs(v)) {
			continue
		}
		amountSats += sats
		addTo(voutAddrs(v))
	}

	// Self-send: every output looked like change. Count them all.
	if len(toAddrs) == 0 {
		amountSats = sumOutSats
		for _, v := range vouts {
			addTo(voutAddrs(v))
		}
	}

	tx.AmountSats = amountSats
	tx.To = toAddrs
	if tx.To == nil {
		tx.To = []string{}
	}

	if !isCoinbase {
		fee := sumInSats - sumOutSats
		tx.FeeSats = &fee
		if tx.VSize > 0 {
			rate := float64(fee) / float64(tx.VSize)
			tx.FeeRateSatPerVb = &rate
		}
	}
}

// resolvePrevouts returns value and addresses for each input, fetching
// every referenced parent transaction at most once and serving repeats from
// the outpoint LRU.
func (s *Service) resolvePrevouts(vins []btcjson.Vin) ([]prevout, error) {
	out := make([]prevout, 0, len(vins))
	parents := make(map[string]*btcjson.TxRawResult)

	for _, in := range vins {
		key := fmt.Sprintf("%s:%d", in.Txid, in.Vout)
		if p, ok := s.prevouts.get(key); ok {
			out = append(out, p)
			continue
		}

		parent, ok := parents[in.Txid]
		if !ok {
			hash, err := chainhash.NewHashFromStr(in.Txid)
			if err != nil {
				return nil, fmt.Errorf("input txid %s: %w", in.Txid, err)
			}
			parent, err = s.backend.GetRawTransactionVerbose(hash)
			if err != nil {
				return nil, fmt.Errorf("input tx %s: %w", in.Txid, err)
			}
			parents[in.Txid] = parent
		}

		if int(in.Vout) >= len(parent.Vout) {
			return nil, fmt.Errorf("input %s:%d out of range", in.Txid, in.Vout)
		}
		v := parent.Vout[in.Vout]
		p := prevout{
			valueSats: satsFromBTC(v.Value),
			addrs:     voutAddrs(v),
		}
		s.prevouts.put(key, p)
		out = append(out, p)
	}
	return out, nil
}

// derivePending computes queue position from the shared mempool snapshot.
func (s *Service) derivePending(tx *Tx) error {
	snapshot, err := s.mempool.Snapshot()
	if err != nil {
		return err
	}

	// The tx may have entered the mempool after the snapshot was taken.
	if _, ok := snapshot[tx.Txid]; !ok {
		s.mempool.Invalidate()
		if snapshot, err = s.mempool.Snapshot(); err != nil {
			return err
		}
	}

	var myRate float64
	if entry, ok := snapshot[tx.Txid]; ok {
		tx.FirstSeen = entry.Time
		myRate = entry.FeeRate
	} else if tx.FeeRateSatPerVb != nil {
		myRate = *tx.FeeRateSatPerVb
	}

	pending := &TxPending{}
	for txid, e := range snapshot {
		if txid == tx.Txid {
			continue
		}
		if e.FeeRate > myRate {
			pending.TxsAhead++
			pending.VBytesAhead += e.VSize
		}
	}

	pending.EtaBlocks = pending.VBytesAhead/vbytesPerBlock + 1
	interval := s.avgBlockInterval()
	pending.EtaSeconds = pending.EtaBlocks * int64(interval.Seconds())
	if n := len(snapshot); n > 1 {
		pending.QueueFraction = float64(pending.TxsAhead) / float64(n-1)
	}

	tx.Pending = pending
	return nil
}

// avgBlockInterval measures the mean spacing of the last 10 blocks,
// cached for 30 seconds. Falls back to the network's target time when the
// chain is too short or timestamps are unusable.
func (s *Service) avgBlockInterval() time.Duration {
	s.intervalMu.Lock()
	defer s.intervalMu.Unlock()

	if !s.intervalAt.IsZero() && time.Since(s.intervalAt) < 30*time.Second {
		return s.intervalMean
	}

	mean := s.params.TargetTimePerBlock
	if measured, err := s.measureBlockInterval(); err == nil && measured > 0 {
		mean = measured
	}

	s.intervalMean = mean
	s.intervalAt = time.Now()
	return mean
}

func (s *Service) measureBlockInterval() (time.Duration, error) {
	tip, err := s.backend.GetBlockCount()
	if err != nil {
		return 0, err
	}
	span := min(int64(9), tip)
	if span < 1 {
		return 0, errors.New("chain too short")
	}

	tipTime, err := s.headerTimeAt(tip)
	if err != nil {
		return 0, err
	}
	baseTime, err := s.headerTimeAt(tip - span)
	if err != nil {
		return 0, err
	}
	if tipTime <= baseTime {
		return 0, errors.New("non-monotonic timestamps")
	}
	return time.Duration((tipTime-baseTime)/span) * time.Second, nil
}

func (s *Service) headerTimeAt(height int64) (int64, error) {
	hash, err := s.backend.GetBlockHash(height)
	if err != nil {
		return 0, err
	}
	header, err := s.header(hash.String())
	if err != nil {
		return 0, err
	}
	return header.Time, nil
}

// header fetches a block header by hash hex, cached (headers are
// immutable).
func (s *Service) header(hashStr string) (*btcjson.GetBlockHeaderVerboseResult, error) {
	if h, ok := s.headers.get(hashStr); ok {
		return h, nil
	}
	hash, err := chainhash.NewHashFromStr(hashStr)
	if err != nil {
		return nil, err
	}
	header, err := s.backend.GetBlockHeaderVerbose(hash)
	if err != nil {
		return nil, err
	}
	s.headers.put(hashStr, header)
	return header, nil
}

// voutAddrs extracts output addresses, tolerating both btcd result shapes
// (the addresses array and Core's newer singular address field).
func voutAddrs(v btcjson.Vout) []string {
	if len(v.ScriptPubKey.Addresses) > 0 {
		return v.ScriptPubKey.Addresses
	}
	if v.ScriptPubKey.Address != "" {
		return []string{v.ScriptPubKey.Address}
	}
	return nil
}

// satsFromBTC converts a btcjson BTC float to satoshis with proper
// rounding.
func satsFromBTC(btc float64) int64 {
	amt, err := btcutil.NewAmount(btc)
	if err != nil {
		return 0
	}
	return int64(amt)
}

func isNoTxInfo(err error) bool {
	var rpcErr *btcjson.RPCError
	return errors.As(err, &rpcErr) && rpcErr.Code == btcjson.ErrRPCNoTxInfo
}
