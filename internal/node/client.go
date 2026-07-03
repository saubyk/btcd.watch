// Package node wraps btcd's rpcclient in websocket mode behind the narrow,
// mockable Backend interface. All chain access in the explorer goes through
// Backend so derivation logic is unit-testable without a running node.
package node

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync/atomic"

	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/chainhash/v2"
	"github.com/btcsuite/btcd/rpcclient"
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
}

// Config holds the btcd connection settings.
type Config struct {
	Host     string
	User     string
	Pass     string
	CertPath string
}

// Client implements Backend over a websocket rpcclient connection.
//
// Websocket mode (rather than HTTP POST) is required because later
// milestones rely on btcd push notifications. A request issued while the
// connection is down would block inside rpcclient until reconnect, so every
// method fails fast with ErrUnavailable when the client has not yet
// connected or is currently disconnected (auto-reconnect re-establishes the
// session and calls flow again).
type Client struct {
	rpc   *rpcclient.Client
	ready atomic.Bool
}

// New creates the client and starts connecting in the background. A btcd
// node that is down at startup does not fail the server; Backend calls
// return ErrUnavailable until the connection is established.
func New(cfg Config) (*Client, error) {
	var certs []byte
	if cfg.CertPath != "" {
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
		DisableConnectOnNew: true,
	}

	// Notification handlers are wired in the live-update milestone.
	rpc, err := rpcclient.New(connCfg, nil)
	if err != nil {
		return nil, fmt.Errorf("create rpc client: %w", err)
	}

	c := &Client{rpc: rpc}
	go c.connect(cfg.Host)
	return c, nil
}

// connect blocks until the websocket session is established. rpcclient
// retries internally with backoff when tries is 0.
func (c *Client) connect(host string) {
	if err := c.rpc.Connect(0); err != nil &&
		!errors.Is(err, rpcclient.ErrClientAlreadyConnected) {

		slog.Error("btcd connection failed permanently",
			"host", host, "err", err)
		return
	}
	c.ready.Store(true)
	slog.Info("connected to btcd", "host", host)
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
