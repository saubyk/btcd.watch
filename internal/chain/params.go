// Package chain provides the network abstraction: mapping config network
// names to chain parameters, query classification, and halving math. All
// network-dependent constants flow from chaincfg.Params — nothing is
// hardcoded per network.
package chain

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/btcsuite/btcd/address/v2"
	"github.com/btcsuite/btcd/chaincfg/v2"
)

// ParamsForNetwork maps a config network name to its chain parameters.
func ParamsForNetwork(name string) (*chaincfg.Params, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "mainnet":
		return &chaincfg.MainNetParams, nil
	case "testnet", "testnet3":
		return &chaincfg.TestNet3Params, nil
	case "regtest":
		return &chaincfg.RegressionNetParams, nil
	case "signet":
		return &chaincfg.SigNetParams, nil
	case "simnet":
		return &chaincfg.SimNetParams, nil
	}
	return nil, fmt.Errorf("unknown network %q", name)
}

// QueryKind is the classification of a raw search query.
type QueryKind string

const (
	// QueryHex is a 64-char hex string — a txid or a block hash; the
	// caller resolves the ambiguity by lookup.
	QueryHex         QueryKind = "hex"
	QueryBlockHeight QueryKind = "block-height"
	QueryAddress     QueryKind = "address"
	QueryInvalid     QueryKind = "invalid"
)

// Query is the result of classifying a raw search string.
type Query struct {
	Kind QueryKind

	// Hex is the normalized (lowercase) 64-hex id when Kind == QueryHex.
	Hex string

	// Height is the parsed block height when Kind == QueryBlockHeight.
	Height int64

	// Address is the decoded address when Kind == QueryAddress. It is
	// guaranteed to be valid for the network it was classified against.
	Address address.Address
}

// ClassifyQuery decides whether a raw search string is a block height, a
// 64-hex id (txid or block hash), an address valid for the given network,
// or none of those. The backend owns classification so that address
// validation is always network-correct (a mainnet bc1 address is rejected
// on regtest, whose bech32 HRP is different). Height is checked first: an
// all-digit query can never be a txid or address.
func ClassifyQuery(raw string, params *chaincfg.Params) Query {
	q := strings.TrimSpace(raw)

	if height, ok := ParseBlockHeight(q); ok {
		return Query{Kind: QueryBlockHeight, Height: height}
	}

	if len(q) == 64 {
		if _, err := hex.DecodeString(q); err == nil {
			return Query{Kind: QueryHex, Hex: strings.ToLower(q)}
		}
	}

	if addr, err := address.DecodeAddress(q, params); err == nil &&
		addr.IsForNet(params) {

		return Query{Kind: QueryAddress, Address: addr}
	}

	return Query{Kind: QueryInvalid}
}

// BlocksUntilHalving returns how many blocks remain after the given height
// until the next subsidy halving. Halvings occur at heights that are
// multiples of params.SubsidyReductionInterval.
func BlocksUntilHalving(height int64, params *chaincfg.Params) int64 {
	interval := int64(params.SubsidyReductionInterval)
	return interval - height%interval
}

// BlockSubsidy returns the coinbase subsidy in satoshis at the given
// height: 50 BTC halved once per SubsidyReductionInterval (mirrors
// blockchain.CalcBlockSubsidy, including the interval-0 no-halving guard).
func BlockSubsidy(height int64, params *chaincfg.Params) int64 {
	const initial = int64(50 * 1e8)
	if params.SubsidyReductionInterval == 0 {
		return initial
	}
	halvings := height / int64(params.SubsidyReductionInterval)
	if halvings >= 64 {
		return 0
	}
	return initial >> uint(halvings)
}

// ParseBlockHeight reports whether a raw search string is a block height:
// all digits (thousands-separator commas allowed), short enough to be a
// plausible height.
func ParseBlockHeight(raw string) (int64, bool) {
	q := strings.ReplaceAll(strings.TrimSpace(raw), ",", "")
	if len(q) == 0 || len(q) > 10 {
		return 0, false
	}
	// ParseUint rejects signs and non-digits; 10 decimal digits always
	// fit in int64.
	height, err := strconv.ParseUint(q, 10, 64)
	if err != nil {
		return 0, false
	}
	return int64(height), true
}
