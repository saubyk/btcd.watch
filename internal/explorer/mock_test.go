package explorer

import (
	"strings"

	"github.com/btcsuite/btcd/address/v2"
	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/chainhash/v2"
)

// mockBackend implements node.Backend from in-memory fixtures.
type mockBackend struct {
	txs     map[string]*btcjson.TxRawResult
	headers map[string]*btcjson.GetBlockHeaderVerboseResult
	hashes  map[int64]string
	blocks  map[string]*btcjson.GetBlockVerboseResult
	tip     int64
	mempool map[string]btcjson.GetRawMempoolVerboseResult
	// history is per-address, newest first (as reverse=true returns).
	history map[string][]*btcjson.SearchRawTransactionsResult

	txFetches      map[string]int
	mempoolFetches int
	searchFetches  int
}

func newMockBackend() *mockBackend {
	return &mockBackend{
		txs:       make(map[string]*btcjson.TxRawResult),
		headers:   make(map[string]*btcjson.GetBlockHeaderVerboseResult),
		hashes:    make(map[int64]string),
		blocks:    make(map[string]*btcjson.GetBlockVerboseResult),
		mempool:   make(map[string]btcjson.GetRawMempoolVerboseResult),
		history:   make(map[string][]*btcjson.SearchRawTransactionsResult),
		txFetches: make(map[string]int),
	}
}

func (m *mockBackend) GetRawTransactionVerbose(txHash *chainhash.Hash) (*btcjson.TxRawResult, error) {
	id := txHash.String()
	m.txFetches[id]++
	tx, ok := m.txs[id]
	if !ok {
		return nil, &btcjson.RPCError{
			Code:    btcjson.ErrRPCNoTxInfo,
			Message: "No information available about transaction",
		}
	}
	return tx, nil
}

func (m *mockBackend) GetBlockHeaderVerbose(blockHash *chainhash.Hash) (*btcjson.GetBlockHeaderVerboseResult, error) {
	h, ok := m.headers[blockHash.String()]
	if !ok {
		return nil, &btcjson.RPCError{
			Code:    btcjson.ErrRPCBlockNotFound,
			Message: "Block not found",
		}
	}
	return h, nil
}

func (m *mockBackend) GetBlockHash(height int64) (*chainhash.Hash, error) {
	s, ok := m.hashes[height]
	if !ok {
		return nil, &btcjson.RPCError{
			Code:    btcjson.ErrRPCOutOfRange,
			Message: "Block number out of range",
		}
	}
	return chainhash.NewHashFromStr(s)
}

func (m *mockBackend) GetBlockCount() (int64, error) {
	return m.tip, nil
}

func (m *mockBackend) GetBlockVerbose(blockHash *chainhash.Hash) (*btcjson.GetBlockVerboseResult, error) {
	b, ok := m.blocks[blockHash.String()]
	if !ok {
		return nil, &btcjson.RPCError{
			Code:    btcjson.ErrRPCBlockNotFound,
			Message: "Block not found",
		}
	}
	return b, nil
}

func (m *mockBackend) GetRawMempoolVerbose() (map[string]btcjson.GetRawMempoolVerboseResult, error) {
	m.mempoolFetches++
	return m.mempool, nil
}

func (m *mockBackend) SearchRawTransactionsVerbose(addr address.Address, skip,
	count int, includePrevOut, reverse bool,
	filterAddrs []string) ([]*btcjson.SearchRawTransactionsResult, error) {

	m.searchFetches++
	list, ok := m.history[addr.EncodeAddress()]
	if !ok {
		return nil, &btcjson.RPCError{
			Code:    btcjson.ErrRPCNoTxInfo,
			Message: "No information available about address",
		}
	}
	if skip >= len(list) {
		return nil, nil
	}
	return list[skip:min(skip+count, len(list))], nil
}

// hexID builds a deterministic 64-hex id from a short label.
func hexID(label string) string {
	const hexDigits = "0123456789abcdef"
	var b strings.Builder
	for i := range 64 {
		b.WriteByte(hexDigits[(int(label[i%len(label)])+i)%16])
	}
	return b.String()
}

func vout(n uint32, valueBTC float64, addrs ...string) btcjson.Vout {
	return btcjson.Vout{
		N:     n,
		Value: valueBTC,
		ScriptPubKey: btcjson.ScriptPubKeyResult{
			Type:      "witness_v0_keyhash",
			Addresses: addrs,
		},
	}
}

func vin(txid string, n uint32) btcjson.Vin {
	return btcjson.Vin{Txid: txid, Vout: n}
}
