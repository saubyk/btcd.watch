package explorer

import (
	"testing"
	"time"
)

var inflowT0 = time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)

func TestInflowEmptyIsZero(t *testing.T) {
	var c inflowCounter
	if rate := c.ratePerMin(inflowT0); rate != 0 {
		t.Errorf("rate = %v, want 0", rate)
	}
}

func TestInflowStartupWindowScaling(t *testing.T) {
	var c inflowCounter

	// Three events in the first ten seconds: the window is floored at
	// one bucket width, not diluted across the full minute.
	c.note(inflowT0)
	c.note(inflowT0.Add(2 * time.Second))
	c.note(inflowT0.Add(4 * time.Second))

	if rate := c.ratePerMin(inflowT0.Add(5 * time.Second)); rate != 18 {
		t.Errorf("rate = %v, want 18 (3 tx over a 10s floor)", rate)
	}

	// Half a minute in, the same three events measure over 30s.
	if rate := c.ratePerMin(inflowT0.Add(30 * time.Second)); rate != 6 {
		t.Errorf("rate = %v, want 6 (3 tx over 30s)", rate)
	}
}

func TestInflowFullWindow(t *testing.T) {
	var c inflowCounter

	// One event per bucket across the whole window.
	for i := range inflowBuckets {
		c.note(inflowT0.Add(time.Duration(i) * inflowBucketSeconds * time.Second))
	}

	// 59s in, all six events are still inside the window.
	if rate := c.ratePerMin(inflowT0.Add(59 * time.Second)); rate != 6.1 {
		t.Errorf("rate = %v, want 6.1 (6 tx over 59s)", rate)
	}

	// A second later the oldest bucket rotates out of the minute.
	if rate := c.ratePerMin(inflowT0.Add(60 * time.Second)); rate != 5 {
		t.Errorf("rate = %v, want 5 (oldest bucket expired)", rate)
	}
}

func TestInflowOldBucketsExpire(t *testing.T) {
	var c inflowCounter

	c.note(inflowT0)
	c.note(inflowT0.Add(time.Second))
	c.note(inflowT0.Add(30 * time.Second))

	// 70s later the t0 burst has rotated out; only the 30s event is in
	// the window.
	if rate := c.ratePerMin(inflowT0.Add(70 * time.Second)); rate != 1 {
		t.Errorf("rate = %v, want 1 (one tx left in the window)", rate)
	}

	// Far beyond the window everything expires.
	if rate := c.ratePerMin(inflowT0.Add(10 * time.Minute)); rate != 0 {
		t.Errorf("rate = %v, want 0 after the window drained", rate)
	}
}
