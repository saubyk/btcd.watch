package api

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"btcdwatch.com/internal/explorer"
)

// fakeClient builds a wsClient without a real connection; writePump never
// runs, so messages accumulate in the send buffer.
func fakeClient(buffer int) *wsClient {
	return &wsClient{
		send:    make(chan []byte, buffer),
		done:    make(chan struct{}),
		watched: make(map[string]bool),
	}
}

func testHub(t *testing.T, txs map[string]*explorer.Tx) (*Hub, context.CancelFunc) {
	t.Helper()
	stats := func() (*explorer.Stats, error) {
		return &explorer.Stats{Network: "regtest", BlockHeight: 42}, nil
	}
	tx := func(txid string) (*explorer.Tx, error) {
		if tx, ok := txs[txid]; ok {
			return tx, nil
		}
		return nil, explorer.ErrTxNotFound
	}
	h := NewHub(stats, tx)
	ctx, cancel := context.WithCancel(context.Background())
	go h.Run(ctx)
	return h, cancel
}

// recv waits for one message on the client's buffer.
func recv(t *testing.T, c *wsClient) map[string]any {
	t.Helper()
	select {
	case raw, ok := <-c.send:
		if !ok {
			t.Fatal("send channel closed")
		}
		var msg map[string]any
		if err := json.Unmarshal(raw, &msg); err != nil {
			t.Fatal(err)
		}
		return msg
	case <-time.After(2 * time.Second):
		t.Fatal("no message received")
		return nil
	}
}

func TestHubStatsOnConnectAndBlock(t *testing.T) {
	h, cancel := testHub(t, nil)
	defer cancel()

	c := fakeClient(8)
	h.register <- c

	if msg := recv(t, c); msg["type"] != "stats" {
		t.Fatalf("first message = %v, want stats", msg["type"])
	}

	h.NotifyBlock()
	if msg := recv(t, c); msg["type"] != "stats" {
		t.Fatalf("block message = %v, want stats", msg["type"])
	}
}

func TestHubWatchFanOutTargetsOnlyWatchers(t *testing.T) {
	txid := "aa11"
	pending := &explorer.Tx{
		Txid:   txid,
		Status: "pending",
		Pending: &explorer.TxPending{
			TxsAhead:   3,
			EtaSeconds: 120,
		},
	}
	h, cancel := testHub(t, map[string]*explorer.Tx{txid: pending})
	defer cancel()

	watcher, bystander := fakeClient(8), fakeClient(8)
	h.register <- watcher
	h.register <- bystander
	recv(t, watcher)   // connect stats
	recv(t, bystander) // connect stats

	h.commands <- wsCommand{client: watcher, watch: true, txid: txid}

	msg := recv(t, watcher)
	if msg["type"] != "tx" || msg["txid"] != txid {
		t.Fatalf("watcher got %v", msg)
	}
	data := msg["data"].(map[string]any)
	if data["status"] != "pending" || data["txsAhead"].(float64) != 3 {
		t.Fatalf("tx data = %v", data)
	}

	// A block pushes stats to everyone but tx updates only to watchers.
	h.NotifyBlock()
	sawTx := map[bool]int{}
	for range 2 {
		m := recv(t, watcher)
		sawTx[m["type"] == "tx"]++
	}
	if sawTx[true] != 1 || sawTx[false] != 1 {
		t.Fatalf("watcher after block: %v (want 1 stats + 1 tx)", sawTx)
	}
	if m := recv(t, bystander); m["type"] != "stats" {
		t.Fatalf("bystander got %v, want stats only", m["type"])
	}
	select {
	case raw := <-bystander.send:
		t.Fatalf("bystander got extra message: %s", raw)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestHubUnwatchStopsUpdates(t *testing.T) {
	txid := "bb22"
	h, cancel := testHub(t, map[string]*explorer.Tx{
		txid: {Txid: txid, Status: "pending"},
	})
	defer cancel()

	c := fakeClient(8)
	h.register <- c
	recv(t, c)

	h.commands <- wsCommand{client: c, watch: true, txid: txid}
	recv(t, c)
	h.commands <- wsCommand{client: c, watch: false, txid: txid}

	h.NotifyBlock()
	if m := recv(t, c); m["type"] != "stats" {
		t.Fatalf("got %v, want stats only after unwatch", m["type"])
	}
	select {
	case raw := <-c.send:
		t.Fatalf("unexpected message after unwatch: %s", raw)
	case <-time.After(100 * time.Millisecond):
	}
}

// TestHubDropsSlowClient: a client with a full buffer is disconnected
// instead of stalling the hub.
func TestHubDropsSlowClient(t *testing.T) {
	h, cancel := testHub(t, nil)
	defer cancel()

	slow := fakeClient(1)
	healthy := fakeClient(64)
	h.register <- slow
	h.register <- healthy
	recv(t, healthy)
	// slow's single buffer slot now holds its connect-stats message and
	// is never drained.

	// Two more pushes: the second trySend to slow fails → dropped.
	h.NotifyBlock()
	time.Sleep(50 * time.Millisecond)
	h.NotifyBlock()

	select {
	case <-slow.done:
		// Dropped, as required.
	case <-time.After(2 * time.Second):
		t.Fatal("slow client was never dropped")
	}
}
