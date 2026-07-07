package explorer

import (
	"time"

	"github.com/btcsuite/btcd/address/v2"
	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/btcutil/v2"

	"btcdwatch.com/internal/chain"
)

// scanChunk is the page size used when walking address history for
// totals; btcd serves searchrawtransactions pages of at most 100.
const scanChunk = 100

// totalsTTL bounds how stale a cached address summary may be. (The
// live-update milestone additionally invalidates on new blocks.)
const totalsTTL = 30 * time.Second

type AddressActivity struct {
	Txid      string `json:"txid"`
	Direction string `json:"direction"` // "received" | "sent" | "self"
	// AmountSats is the net effect on the address, always positive;
	// Direction carries the sign.
	AmountSats    int64  `json:"amountSats"`
	Status        string `json:"status"` // "pending" | "confirmed"
	Confirmations int64  `json:"confirmations"`
	// Time is the block time, or first-seen for mempool entries (0 when
	// unknown).
	Time int64 `json:"time"`
}

type AddressSummary struct {
	Address string `json:"address"`
	// Type is the script-type code (P2WPKH, P2TR, ...); empty hides the
	// type chips.
	Type         string   `json:"type"`
	BalanceSats  int64    `json:"balanceSats"`
	ReceivedSats int64    `json:"receivedSats"`
	SentSats     int64    `json:"sentSats"`
	FiatUSD      *float64 `json:"fiatUsd"`
	TxCount      int      `json:"txCount"`
	// Approximate is true when history exceeded the scan cap, so totals
	// only cover the most recent MaxScanTxs transactions.
	Approximate bool              `json:"approximate"`
	Activity    []AddressActivity `json:"activity"`
	Offset      int               `json:"offset"`
	Limit       int               `json:"limit"`
	HasMore     bool              `json:"hasMore"`
}

// addressTotals is the cached full-history aggregate.
type addressTotals struct {
	received    int64
	sent        int64
	txCount     int
	approximate bool
	fetchedAt   time.Time
}

// Address builds the address summary: cached full-history totals plus one
// page of activity (newest first). btcd's searchrawtransactions with
// includePrevOut inlines every input's origin, so no follow-up RPCs are
// needed.
func (s *Service) Address(addr address.Address, offset, limit int) (*AddressSummary, error) {
	encoded := addr.EncodeAddress()

	totals, err := s.addressTotals(addr, encoded)
	if err != nil {
		return nil, err
	}

	page, err := s.searchPage(addr, offset, limit)
	if err != nil {
		return nil, err
	}

	activity := make([]AddressActivity, 0, len(page))
	for _, tx := range page {
		activity = append(activity, activityFor(encoded, tx))
	}

	summary := &AddressSummary{
		Address:      encoded,
		Type:         chain.ScriptTypeOf(addr),
		BalanceSats:  totals.received - totals.sent,
		ReceivedSats: totals.received,
		SentSats:     totals.sent,
		TxCount:      totals.txCount,
		Approximate:  totals.approximate,
		Activity:     activity,
		Offset:       offset,
		Limit:        limit,
	}
	if totals.approximate {
		summary.HasMore = len(activity) == limit
	} else {
		summary.HasMore = offset+len(activity) < totals.txCount
	}

	if q := s.priceUSD(); q.OK {
		fiat := btcutil.Amount(summary.BalanceSats).ToBTC() * q.USD
		summary.FiatUSD = &fiat
	}
	return summary, nil
}

// addressTotals aggregates the full (capped) history, cached briefly —
// pagination through an address must not rescan everything per page.
func (s *Service) addressTotals(addr address.Address, encoded string) (addressTotals, error) {
	if cached, ok := s.totals.get(encoded); ok &&
		time.Since(cached.fetchedAt) < totalsTTL {

		return cached, nil
	}

	var t addressTotals
	for skip := 0; skip < s.maxScan; skip += scanChunk {
		count := min(scanChunk, s.maxScan-skip)
		page, err := s.searchPage(addr, skip, count)
		if err != nil {
			return addressTotals{}, err
		}
		for _, tx := range page {
			recv, sent := addressDelta(encoded, tx)
			t.received += recv
			t.sent += sent
		}
		t.txCount += len(page)
		if len(page) < count {
			t.fetchedAt = time.Now()
			s.totals.put(encoded, t)
			return t, nil
		}
	}

	// The cap was reached with a full final page — more history exists.
	t.approximate = true
	t.fetchedAt = time.Now()
	s.totals.put(encoded, t)
	return t, nil
}

// searchPage fetches one page of history, mapping btcd's "no information"
// error (an address with no transactions) to an empty page.
func (s *Service) searchPage(addr address.Address, skip, count int) ([]*btcjson.SearchRawTransactionsResult, error) {
	page, err := s.backend.SearchRawTransactionsVerbose(
		addr, skip, count, true, true, nil,
	)
	if err != nil {
		if isNoTxInfo(err) {
			return nil, nil
		}
		return nil, err
	}
	return page, nil
}

// addressDelta sums what a transaction paid to (recv) and spent from
// (sent) the address.
func addressDelta(addr string, tx *btcjson.SearchRawTransactionsResult) (recv, sent int64) {
	for _, v := range tx.Vout {
		for _, a := range voutAddrs(v) {
			if a == addr {
				recv += satsFromBTC(v.Value)
			}
		}
	}
	for _, in := range tx.Vin {
		if in.PrevOut == nil {
			continue
		}
		for _, a := range in.PrevOut.Addresses {
			if a == addr {
				sent += satsFromBTC(in.PrevOut.Value)
			}
		}
	}
	return recv, sent
}

// activityFor classifies one transaction's effect on the address.
//
// When the address appears on both sides, the transaction fee decides:
// if the net outflow is no more than the fee, the money merely moved back
// to the same address (a self transfer); otherwise it is a spend and the
// change coming back is excluded from the displayed amount.
func activityFor(addr string, tx *btcjson.SearchRawTransactionsResult) AddressActivity {
	recv, sent := addressDelta(addr, tx)

	a := AddressActivity{
		Txid:          tx.Txid,
		Confirmations: int64(tx.Confirmations),
	}

	switch {
	case sent == 0:
		a.Direction = "received"
		a.AmountSats = recv
	case recv == 0:
		a.Direction = "sent"
		a.AmountSats = sent
	default:
		net := sent - recv
		if fee, ok := txFee(tx); ok && net <= fee {
			a.Direction = "self"
			a.AmountSats = net
		} else {
			a.Direction = "sent"
			a.AmountSats = net
		}
	}

	if tx.BlockHash != "" {
		a.Status = "confirmed"
		a.Time = tx.Blocktime
	} else {
		a.Status = "pending"
		a.Time = tx.Time
	}
	return a
}

// txFee computes fee = Σ inputs − Σ outputs from the inlined prevouts;
// ok is false for coinbase or when any prevout is missing.
func txFee(tx *btcjson.SearchRawTransactionsResult) (int64, bool) {
	var in, out int64
	for _, vin := range tx.Vin {
		if vin.PrevOut == nil {
			return 0, false
		}
		in += satsFromBTC(vin.PrevOut.Value)
	}
	for _, v := range tx.Vout {
		out += satsFromBTC(v.Value)
	}
	return in - out, true
}
