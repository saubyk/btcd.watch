package api

import (
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/btcsuite/btcd/chaincfg/v2"
	"github.com/gorilla/websocket"

	"btcdwatch.com/internal/chain"
)

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = 30 * time.Second
	// maxInboundBytes bounds client frames; watch/unwatch messages are
	// tiny.
	maxInboundBytes = 256
	sendBufferSize  = 32
)

// The API serves localhost tooling and, in production, the same origin as
// the SPA; in development the page origin is the Vite server, so the
// default same-origin check would reject the proxied upgrade.
var upgrader = websocket.Upgrader{
	CheckOrigin: func(*http.Request) bool { return true },
}

// wsInbound is a client→server frame.
type wsInbound struct {
	Type string `json:"type"` // "watch" | "unwatch"
	Txid string `json:"txid"`
}

// wsClient is one WebSocket connection. The hub owns `watched`; the pumps
// own the conn.
type wsClient struct {
	conn *websocket.Conn
	// send is never closed — detached push goroutines may hold a
	// reference at any time. Shutdown is signalled via done instead.
	send chan []byte
	done chan struct{}

	// watched is only touched by the hub loop.
	watched map[string]bool

	closeOnce sync.Once
}

// trySend queues a message without blocking; false means the client is too
// slow and should be dropped.
func (c *wsClient) trySend(msg []byte) bool {
	select {
	case <-c.done:
		// Already being dropped; swallow quietly so senders don't
		// re-trigger the unregister path.
		return true
	case c.send <- msg:
		return true
	default:
		return false
	}
}

// shutdown is called by the hub when the client is dropped.
func (c *wsClient) shutdown() {
	c.closeOnce.Do(func() {
		close(c.done)
		if c.conn != nil {
			c.conn.Close()
		}
	})
}

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Debug("ws upgrade failed", "err", err)
		return
	}

	c := &wsClient{
		conn:    conn,
		send:    make(chan []byte, sendBufferSize),
		done:    make(chan struct{}),
		watched: make(map[string]bool),
	}
	s.hub.register <- c

	go c.writePump()
	go c.readPump(s.hub, s.params)
}

// readPump parses watch/unwatch commands until the connection dies, then
// unregisters the client.
func (c *wsClient) readPump(hub *Hub, params *chaincfg.Params) {
	defer func() {
		hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxInboundBytes)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		var msg wsInbound
		if err := c.conn.ReadJSON(&msg); err != nil {
			return
		}
		if msg.Type != "watch" && msg.Type != "unwatch" {
			continue
		}
		query := chain.ClassifyQuery(msg.Txid, params)
		if query.Kind != chain.QueryHex {
			continue
		}
		hub.commands <- wsCommand{
			client: c,
			watch:  msg.Type == "watch",
			txid:   query.Hex,
		}
	}
}

// writePump drains the send channel and keeps the connection alive with
// pings; it exits when the hub closes the channel or a write fails.
func (c *wsClient) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case <-c.done:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			c.conn.WriteMessage(websocket.CloseMessage, nil)
			return
		case msg := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
