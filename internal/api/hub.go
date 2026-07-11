package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"btcdwatch.com/internal/explorer"
)

// tickInterval drives periodic pushes: watched-pending queue positions and
// throttled stats refreshes (covers price updates and mempool churn
// between blocks).
const tickInterval = 10 * time.Second

// mempoolPushInterval throttles live-mempool pushes: tx-accepted
// notifications mark the hub dirty, and at most one mempool push per
// interval goes out (plus an immediate one per block, for the bar
// contraction).
const mempoolPushInterval = 2 * time.Second

// StatsFunc / TxFunc / MempoolFunc / BlockFlashFunc decouple the hub from
// the explorer service so tests can inject fakes.
type (
	StatsFunc      func() (*explorer.Stats, error)
	TxFunc         func(txid string) (*explorer.Tx, error)
	MempoolFunc    func() (*explorer.MempoolUpdate, error)
	BlockFlashFunc func(hash string) (*explorer.BlockFlash, error)
)

// TxUpdate is the compact per-transaction push payload.
type TxUpdate struct {
	Status              string   `json:"status"`
	Confirmations       int64    `json:"confirmations"`
	BlockHeight         *int64   `json:"blockHeight"`
	TxsAhead            *int     `json:"txsAhead"`
	EtaSeconds          *int64   `json:"etaSeconds"`
	QueueVbytesFraction *float64 `json:"queueVbytesFraction"`
}

type wsCommand struct {
	client *wsClient
	watch  bool
	txid   string
}

// maxWatchedPerClient caps watch registrations per connection. The UI
// watches one transaction at a time; the cap only exists so a hostile
// client cannot grow the watchers map without bound.
const maxWatchedPerClient = 32

// Hub fans chain events out to WebSocket clients. A single event-loop
// goroutine owns all state (clients and watch registrations) — RPC-heavy
// work happens in short-lived goroutines that only touch clients through
// their buffered send channels.
type Hub struct {
	stats      StatsFunc
	tx         TxFunc
	mempool    MempoolFunc
	blockFlash BlockFlashFunc

	// MaxClients rejects further WebSocket registrations once reached
	// (0 = unlimited). Set before Run.
	MaxClients int

	register   chan *wsClient
	unregister chan *wsClient
	commands   chan wsCommand
	blocks     chan string
	mempoolCh  chan struct{}

	// mpInterval is the mempool push throttle (mempoolPushInterval;
	// shortened by tests).
	mpInterval time.Duration

	// Loop-owned state — never touched outside run().
	clients      map[*wsClient]bool
	watchers     map[string]map[*wsClient]bool
	mempoolDirty bool
}

func NewHub(stats StatsFunc, tx TxFunc, mempool MempoolFunc,
	blockFlash BlockFlashFunc) *Hub {

	return &Hub{
		stats:      stats,
		tx:         tx,
		mempool:    mempool,
		blockFlash: blockFlash,
		register:   make(chan *wsClient),
		unregister: make(chan *wsClient, 8),
		commands:   make(chan wsCommand),
		blocks:     make(chan string, 1),
		mempoolCh:  make(chan struct{}, 1),
		mpInterval: mempoolPushInterval,
		clients:    make(map[*wsClient]bool),
		watchers:   make(map[string]map[*wsClient]bool),
	}
}

// NotifyBlock wakes the hub after a new block; with rapid blocks the
// latest wins. There is a single notifier (the rpcclient callback), so
// drain-then-send never races.
func (h *Hub) NotifyBlock(hash string) {
	select {
	case <-h.blocks:
	default:
	}
	h.blocks <- hash
}

// NotifyMempool requests a live-mempool push; bursts coalesce and the run
// loop throttles to one push per mempoolPushInterval.
func (h *Hub) NotifyMempool() {
	select {
	case h.mempoolCh <- struct{}{}:
	default:
	}
}

// Run is the hub event loop; cancel the context to stop it.
func (h *Hub) Run(ctx context.Context) {
	ticker := time.NewTicker(tickInterval)
	defer ticker.Stop()
	mpTicker := time.NewTicker(h.mpInterval)
	defer mpTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			for c := range h.clients {
				c.shutdown()
			}
			return

		case c := <-h.register:
			if h.MaxClients > 0 && len(h.clients) >= h.MaxClients {
				// Full house: close the connection instead of letting
				// the client map grow without bound.
				c.shutdown()
				continue
			}
			h.clients[c] = true
			go h.pushStats(c)
			go h.pushMempool(c)

		case c := <-h.unregister:
			h.drop(c)

		case cmd := <-h.commands:
			if !h.clients[cmd.client] {
				continue
			}
			if cmd.watch {
				if len(cmd.client.watched) >= maxWatchedPerClient &&
					!cmd.client.watched[cmd.txid] {

					continue
				}
				set := h.watchers[cmd.txid]
				if set == nil {
					set = make(map[*wsClient]bool)
					h.watchers[cmd.txid] = set
				}
				set[cmd.client] = true
				cmd.client.watched[cmd.txid] = true
				go h.pushTx(cmd.txid, cmd.client)
			} else {
				delete(cmd.client.watched, cmd.txid)
				h.removeWatcher(cmd.txid, cmd.client)
			}

		case hash := <-h.blocks:
			h.pushAll()
			// Immediate mempool push so the bar contraction lands with
			// the flash, not on the next throttle tick.
			h.mempoolDirty = false
			clients := h.clientList()
			if len(clients) > 0 {
				go h.pushMempool(clients...)
				go h.pushBlockFlash(hash, clients...)
			}

		case <-h.mempoolCh:
			h.mempoolDirty = true

		case <-mpTicker.C:
			if !h.mempoolDirty {
				continue
			}
			h.mempoolDirty = false
			if clients := h.clientList(); len(clients) > 0 {
				go h.pushMempool(clients...)
			}

		case <-ticker.C:
			h.pushAll()
		}
	}
}

// clientList snapshots the loop-owned client set for use outside the loop.
func (h *Hub) clientList() []*wsClient {
	clients := make([]*wsClient, 0, len(h.clients))
	for c := range h.clients {
		clients = append(clients, c)
	}
	return clients
}

// pushAll refreshes stats for every client and state for every watched
// txid. Snapshots of the loop-owned maps are taken here; the fetches and
// sends run outside the loop.
func (h *Hub) pushAll() {
	clients := h.clientList()
	if len(clients) > 0 {
		go h.pushStats(clients...)
	}
	for txid, set := range h.watchers {
		targets := make([]*wsClient, 0, len(set))
		for c := range set {
			targets = append(targets, c)
		}
		go h.pushTx(txid, targets...)
	}
}

func (h *Hub) drop(c *wsClient) {
	if !h.clients[c] {
		return
	}
	delete(h.clients, c)
	for txid := range c.watched {
		h.removeWatcher(txid, c)
	}
	c.shutdown()
}

func (h *Hub) removeWatcher(txid string, c *wsClient) {
	if set := h.watchers[txid]; set != nil {
		delete(set, c)
		if len(set) == 0 {
			delete(h.watchers, txid)
		}
	}
}

func (h *Hub) pushStats(clients ...*wsClient) {
	stats, err := h.stats()
	if err != nil {
		return // node down — clients keep their last numbers
	}
	h.send(clients, map[string]any{"type": "stats", "data": stats})
}

func (h *Hub) pushMempool(clients ...*wsClient) {
	update, err := h.mempool()
	if err != nil {
		return // node down — clients keep their last numbers
	}
	h.send(clients, map[string]any{"type": "mempool", "data": update})
}

func (h *Hub) pushBlockFlash(hash string, clients ...*wsClient) {
	flash, err := h.blockFlash(hash)
	if err != nil {
		return // node down — no banner
	}
	h.send(clients, map[string]any{"type": "block", "data": flash})
}

func (h *Hub) pushTx(txid string, clients ...*wsClient) {
	tx, err := h.tx(txid)
	if err != nil {
		return // evicted or node down — nothing sensible to push
	}

	update := TxUpdate{
		Status:        tx.Status,
		Confirmations: tx.Confirmations,
	}
	if tx.Block != nil {
		update.BlockHeight = &tx.Block.Height
	}
	if tx.Pending != nil {
		update.TxsAhead = &tx.Pending.TxsAhead
		update.EtaSeconds = &tx.Pending.EtaSeconds
		update.QueueVbytesFraction = &tx.Pending.QueueVbytesFraction
	}
	h.send(clients, map[string]any{
		"type": "tx",
		"txid": txid,
		"data": update,
	})
}

// send marshals once and fan-outs non-blockingly; a client whose buffer is
// full is disconnected rather than allowed to stall everyone else.
func (h *Hub) send(clients []*wsClient, payload any) {
	msg, err := json.Marshal(payload)
	if err != nil {
		slog.Error("marshal ws payload", "err", err)
		return
	}
	for _, c := range clients {
		if !c.trySend(msg) {
			select {
			case h.unregister <- c:
			default:
				// Unregister queue full — the loop will drop the
				// client when its read pump fails instead.
			}
		}
	}
}
