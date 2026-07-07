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
	mempool := func() (*explorer.MempoolUpdate, error) {
		return &explorer.MempoolUpdate{
			Queue:    &explorer.Queue{TxCount: 2},
			Arrivals: []explorer.Arrival{},
		}, nil
	}
	blockFlash := func(hash string) (*explorer.BlockFlash, error) {
		return &explorer.BlockFlash{Height: 43, TxCount: 7}, nil
	}
	h := NewHub(stats, tx, mempool, blockFlash)
	h.mpInterval = 50 * time.Millisecond // don't make tests wait 2s
	ctx, cancel := context.WithCancel(context.Background())
	go h.Run(ctx)
	return h, cancel
}

// recvConnect drains the two connect pushes (stats + mempool, in either
// order since they run in separate goroutines).
func recvConnect(t *testing.T, c *wsClient) {
	t.Helper()
	types := map[any]bool{}
	for range 2 {
		types[recv(t, c)["type"]] = true
	}
	if !types["stats"] || !types["mempool"] {
		t.Fatalf("connect pushes = %v, want stats + mempool", types)
	}
}

// recvTypes collects the next n messages' types.
func recvTypes(t *testing.T, c *wsClient, n int) map[string]int {
	t.Helper()
	got := map[string]int{}
	for range n {
		got[recv(t, c)["type"].(string)]++
	}
	return got
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
	recvConnect(t, c)

	// A block pushes stats, a fresh mempool state, and the block flash.
	h.NotifyBlock("hash43")
	got := recvTypes(t, c, 3)
	if got["stats"] != 1 || got["mempool"] != 1 || got["block"] != 1 {
		t.Fatalf("block pushes = %v, want stats + mempool + block", got)
	}
}

func TestHubMempoolPushThrottled(t *testing.T) {
	h, cancel := testHub(t, nil)
	defer cancel()

	c := fakeClient(8)
	h.register <- c
	recvConnect(t, c)

	// A burst of tx-accepted notifications coalesces into one push on
	// the next throttle tick.
	for range 5 {
		h.NotifyMempool()
	}
	if msg := recv(t, c); msg["type"] != "mempool" {
		t.Fatalf("got %v, want mempool", msg["type"])
	}
	select {
	case raw := <-c.send:
		t.Fatalf("extra push after burst: %s", raw)
	case <-time.After(100 * time.Millisecond):
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
	recvConnect(t, watcher)
	recvConnect(t, bystander)

	h.commands <- wsCommand{client: watcher, watch: true, txid: txid}

	msg := recv(t, watcher)
	if msg["type"] != "tx" || msg["txid"] != txid {
		t.Fatalf("watcher got %v", msg)
	}
	data := msg["data"].(map[string]any)
	if data["status"] != "pending" || data["txsAhead"].(float64) != 3 {
		t.Fatalf("tx data = %v", data)
	}

	// A block pushes stats/mempool/flash to everyone but tx updates only
	// to watchers.
	h.NotifyBlock("hash43")
	got := recvTypes(t, watcher, 4)
	if got["tx"] != 1 {
		t.Fatalf("watcher after block: %v (want a tx push too)", got)
	}
	if got := recvTypes(t, bystander, 3); got["tx"] != 0 {
		t.Fatalf("bystander after block: %v (want no tx push)", got)
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
	recvConnect(t, c)

	h.commands <- wsCommand{client: c, watch: true, txid: txid}
	recv(t, c)
	h.commands <- wsCommand{client: c, watch: false, txid: txid}

	h.NotifyBlock("hash43")
	if got := recvTypes(t, c, 3); got["tx"] != 0 {
		t.Fatalf("got %v, want no tx push after unwatch", got)
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
	recvConnect(t, healthy)
	// slow's single buffer slot now holds its first connect push and is
	// never drained; the second connect push already fails to send.

	// Further pushes guarantee the drop.
	h.NotifyBlock("hash43")
	time.Sleep(50 * time.Millisecond)
	h.NotifyBlock("hash44")

	select {
	case <-slow.done:
		// Dropped, as required.
	case <-time.After(2 * time.Second):
		t.Fatal("slow client was never dropped")
	}
}
