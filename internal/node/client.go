// Package node wraps btcd's rpcclient in websocket mode behind the narrow,
// mockable Backend interface. All chain access in the explorer goes through
// Backend so derivation logic is unit-testable without a running node.
package node

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/btcsuite/btcd/address/v2"
	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/btcutil/v2"
	"github.com/btcsuite/btcd/chainhash/v2"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/btcsuite/btcd/wire/v2"
)

// ErrUnavailable is returned by all Backend methods while the btcd node is
// unreachable. The API layer maps it to 503.
var ErrUnavailable = errors.New("btcd node unavailable")

// Backend is the set of btcd RPCs the explorer relies on. Method
// signatures mirror rpcclient so the real client is a thin pass-through.
type Backend interface {
	GetRawTransactionVerbose(txHash *chainhash.Hash) (*btcjson.TxRawResult, error)
	GetBlockHeaderVerbose(blockHash *chainhash.Hash) (*btcjson.GetBlockHeaderVerboseResult, error)
	GetBlockHash(height int64) (*chainhash.Hash, error)
	GetBlockCount() (int64, error)
	GetBlockVerbose(blockHash *chainhash.Hash) (*btcjson.GetBlockVerboseResult, error)
	GetRawMempoolVerbose() (map[string]btcjson.GetRawMempoolVerboseResult, error)
	SearchRawTransactionsVerbose(addr address.Address, skip, count int,
		includePrevOut, reverse bool,
		filterAddrs []string) ([]*btcjson.SearchRawTransactionsResult, error)
}

// Handlers are the chain events consumers can subscribe to. Callbacks run
// on the rpcclient notification goroutine — they must return quickly and
// never call back into the node synchronously.
type Handlers struct {
	// OnBlock fires once per newly connected block (deduplicated across
	// btcd's two block-notification variants) with the block's hash hex.
	OnBlock func(height int32, hash string)

	// OnTxAccepted fires when a transaction enters btcd's mempool, with
	// the accepted transaction's verbose payload.
	OnTxAccepted func(raw *btcjson.TxRawResult)

	// OnConnect fires after every (re)connect, once notifications have
	// been registered.
	OnConnect func()
}

// Config holds the btcd connection settings.
type Config struct {
	Host     string
	User     string
	Pass     string
	CertPath string
	// NoTLS connects over plain websocket, matching a btcd running with
	// notls=1 (loopback-only setups). CertPath is ignored when set.
	NoTLS bool
}

// Client implements Backend over a websocket rpcclient connection.
//
// Websocket mode (rather than HTTP POST) is required for btcd push
// notifications. A request issued while the connection is down would block
// inside rpcclient until reconnect, so every method fails fast with
// ErrUnavailable when the client has not yet connected or is currently
// disconnected (auto-reconnect re-establishes the session, re-registers
// notifications, and calls flow again).
type Client struct {
	rpc   *rpcclient.Client
	ready atomic.Bool

	mu       sync.RWMutex
	handlers Handlers

	// lastBlockHash dedupes block events: btcd may deliver both the
	// filtered and legacy block-connected callbacks for the same block
	// depending on version and registration mode.
	lastBlockMu   sync.Mutex
	lastBlockHash string
}

// New creates the client without connecting. Call Start to set event
// handlers and begin connecting in the background; Backend calls return
// ErrUnavailable until the connection is established.
func New(cfg Config) (*Client, error) {
	var certs []byte
	if !cfg.NoTLS && cfg.CertPath != "" {
		var err error
		certs, err = os.ReadFile(cfg.CertPath)
		if err != nil {
			return nil, fmt.Errorf("read rpc cert: %w", err)
		}
	}

	connCfg := &rpcclient.ConnConfig{
		Host:                cfg.Host,
		Endpoint:            "ws",
		User:                cfg.User,
		Pass:                cfg.Pass,
		Certificates:        certs,
		DisableTLS:          cfg.NoTLS,
		DisableConnectOnNew: true,
	}

	c := &Client{}

	ntfns := &rpcclient.NotificationHandlers{
		OnClientConnected: c.onClientConnected,
		OnFilteredBlockConnected: func(height int32,
			header *wire.BlockHeader, _ []*btcutil.Tx) {

			c.onBlock(height, header.BlockHash().String())
		},
		OnBlockConnected: func(hash *chainhash.Hash, height int32,
			_ time.Time) {

			c.onBlock(height, hash.String())
		},
		OnTxAcceptedVerbose: func(raw *btcjson.TxRawResult) {
			if h := c.getHandlers().OnTxAccepted; h != nil && raw != nil {
				h(raw)
			}
		},
	}

	rpc, err := rpcclient.New(connCfg, ntfns)
	if err != nil {
		return nil, fmt.Errorf("create rpc client: %w", err)
	}
	c.rpc = rpc
	return c, nil
}

// Start registers the event handlers and begins connecting. rpcclient
// retries internally with backoff when tries is 0, and auto-reconnects
// (re-registering notifications) after any later drop.
func (c *Client) Start(handlers Handlers) {
	c.mu.Lock()
	c.handlers = handlers
	c.mu.Unlock()

	go func() {
		if err := c.rpc.Connect(0); err != nil &&
			!errors.Is(err, rpcclient.ErrClientAlreadyConnected) {

			slog.Error("btcd connection failed permanently", "err", err)
		}
	}()
}

func (c *Client) getHandlers() Handlers {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.handlers
}

// onClientConnected runs on every (re)connect. Notification registrations
// do not exist on a fresh session, so they are (re)issued here; the RPC
// calls must leave the notification goroutine.
func (c *Client) onClientConnected() {
	go func() {
		if err := c.rpc.NotifyBlocks(); err != nil {
			slog.Error("notifyblocks registration failed", "err", err)
		}
		if err := c.rpc.NotifyNewTransactions(true); err != nil {
			slog.Error("notifynewtransactions registration failed", "err", err)
		}
		c.ready.Store(true)
		slog.Info("connected to btcd, notifications registered")

		if h := c.getHandlers().OnConnect; h != nil {
			h()
		}
	}()
}

func (c *Client) onBlock(height int32, hash string) {
	c.lastBlockMu.Lock()
	dup := hash == c.lastBlockHash
	if !dup {
		c.lastBlockHash = hash
	}
	c.lastBlockMu.Unlock()
	if dup {
		return
	}

	slog.Debug("block connected", "height", height, "hash", hash)
	if h := c.getHandlers().OnBlock; h != nil {
		h(height, hash)
	}
}

// Shutdown tears down the RPC connection.
func (c *Client) Shutdown() {
	c.rpc.Shutdown()
	c.rpc.WaitForShutdown()
}

func (c *Client) available() error {
	if !c.ready.Load() || c.rpc.Disconnected() {
		return ErrUnavailable
	}
	return nil
}

func (c *Client) GetRawTransactionVerbose(txHash *chainhash.Hash) (*btcjson.TxRawResult, error) {
	if err := c.available(); err != nil {
		return nil, err
	}
	return c.rpc.GetRawTransactionVerbose(txHash)
}

func (c *Client) GetBlockHeaderVerbose(blockHash *chainhash.Hash) (*btcjson.GetBlockHeaderVerboseResult, error) {
	if err := c.available(); err != nil {
		return nil, err
	}
	return c.rpc.GetBlockHeaderVerbose(blockHash)
}

func (c *Client) GetBlockHash(height int64) (*chainhash.Hash, error) {
	if err := c.available(); err != nil {
		return nil, err
	}
	return c.rpc.GetBlockHash(height)
}

func (c *Client) GetBlockCount() (int64, error) {
	if err := c.available(); err != nil {
		return 0, err
	}
	return c.rpc.GetBlockCount()
}

func (c *Client) GetBlockVerbose(blockHash *chainhash.Hash) (*btcjson.GetBlockVerboseResult, error) {
	if err := c.available(); err != nil {
		return nil, err
	}
	return c.rpc.GetBlockVerbose(blockHash)
}

func (c *Client) GetRawMempoolVerbose() (map[string]btcjson.GetRawMempoolVerboseResult, error) {
	if err := c.available(); err != nil {
		return nil, err
	}
	return c.rpc.GetRawMempoolVerbose()
}

func (c *Client) SearchRawTransactionsVerbose(addr address.Address, skip,
	count int, includePrevOut, reverse bool,
	filterAddrs []string) ([]*btcjson.SearchRawTransactionsResult, error) {

	if err := c.available(); err != nil {
		return nil, err
	}
	return c.rpc.SearchRawTransactionsVerbose(addr, skip, count,
		includePrevOut, reverse, filterAddrs)
}
