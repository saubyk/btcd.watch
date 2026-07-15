package explorer

import (
	"math"
	"sync"
	"time"
)

// The inflow rate measures how fast transactions enter the mempool. It is
// counted where every tx-accepted notification lands (NoteArrival) — the
// arrivals buffer is capped and fee-joined, so it undercounts bursts; this
// counter sees the raw stream.

// inflowBucketSeconds x inflowBuckets = the rolling measurement window.
const (
	inflowBucketSeconds = 10
	inflowBuckets       = 6
)

// inflowCounter counts events in rotating fixed-width buckets, yielding a
// per-minute rate over the last minute. Fixed memory and O(1) per event,
// so a transaction flood cannot grow it.
type inflowCounter struct {
	mu sync.Mutex
	// started anchors the rate window while the process is younger than
	// the full window, so early rates aren't diluted by empty buckets.
	started time.Time
	slot    int64
	counts  [inflowBuckets]int
}

// note records one accepted transaction.
func (c *inflowCounter) note(now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.started.IsZero() {
		c.started = now
	}
	slot := now.Unix() / inflowBucketSeconds
	c.rotate(slot)
	c.counts[int(slot%inflowBuckets)]++
}

// ratePerMin is the rate over the last minute (or since start, floored at
// one bucket width, while the counter is younger than that).
func (c *inflowCounter) ratePerMin(now time.Time) float64 {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.started.IsZero() {
		return 0
	}
	c.rotate(now.Unix() / inflowBucketSeconds)

	total := 0
	for _, n := range c.counts {
		total += n
	}
	window := min(now.Sub(c.started), time.Minute)
	window = max(window, inflowBucketSeconds*time.Second)
	rate := float64(total) / window.Seconds() * 60
	return math.Round(rate*10) / 10
}

// rotate zeroes the buckets skipped since the last event so stale counts
// fall out of the window. Same-slot (or a backwards clock step) is a no-op.
func (c *inflowCounter) rotate(slot int64) {
	if slot <= c.slot {
		return
	}
	steps := min(slot-c.slot, inflowBuckets)
	for i := int64(1); i <= steps; i++ {
		c.counts[int((c.slot+i)%inflowBuckets)] = 0
	}
	c.slot = slot
}
