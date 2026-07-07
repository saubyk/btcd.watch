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

// StatsFunc / TxFunc decouple the hub from the explorer service so tests
// can inject fakes.
type (
	StatsFunc func() (*explorer.Stats, error)
	TxFunc    func(txid string) (*explorer.Tx, error)
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

// Hub fans chain events out to WebSocket clients. A single event-loop
// goroutine owns all state (clients and watch registrations) — RPC-heavy
// work happens in short-lived goroutines that only touch clients through
// their buffered send channels.
type Hub struct {
	stats StatsFunc
	tx    TxFunc

	register   chan *wsClient
	unregister chan *wsClient
	commands   chan wsCommand
	blocks     chan struct{}

	// Loop-owned state — never touched outside run().
	clients  map[*wsClient]bool
	watchers map[string]map[*wsClient]bool
}

func NewHub(stats StatsFunc, tx TxFunc) *Hub {
	return &Hub{
		stats:      stats,
		tx:         tx,
		register:   make(chan *wsClient),
		unregister: make(chan *wsClient, 8),
		commands:   make(chan wsCommand),
		blocks:     make(chan struct{}, 1),
		clients:    make(map[*wsClient]bool),
		watchers:   make(map[string]map[*wsClient]bool),
	}
}

// NotifyBlock wakes the hub after a new block; multiple rapid blocks
// coalesce into one wake-up.
func (h *Hub) NotifyBlock() {
	select {
	case h.blocks <- struct{}{}:
	default:
	}
}

// Run is the hub event loop; cancel the context to stop it.
func (h *Hub) Run(ctx context.Context) {
	ticker := time.NewTicker(tickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			for c := range h.clients {
				c.shutdown()
			}
			return

		case c := <-h.register:
			h.clients[c] = true
			go h.pushStats(c)

		case c := <-h.unregister:
			h.drop(c)

		case cmd := <-h.commands:
			if !h.clients[cmd.client] {
				continue
			}
			if cmd.watch {
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

		case <-h.blocks:
			h.pushAll()

		case <-ticker.C:
			h.pushAll()
		}
	}
}

// pushAll refreshes stats for every client and state for every watched
// txid. Snapshots of the loop-owned maps are taken here; the fetches and
// sends run outside the loop.
func (h *Hub) pushAll() {
	clients := make([]*wsClient, 0, len(h.clients))
	for c := range h.clients {
		clients = append(clients, c)
	}
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
