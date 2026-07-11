package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRateLimiterSpendAndRefill(t *testing.T) {
	l := newRateLimiter(60, 10) // 1 token/sec, burst 10
	now := time.Unix(1000, 0)

	// Burst spends down to zero, then denies.
	for i := range 10 {
		if !l.allow("a", 1, now) {
			t.Fatalf("request %d within burst denied", i)
		}
	}
	if l.allow("a", 1, now) {
		t.Fatal("request beyond burst allowed")
	}

	// 5 seconds refills 5 tokens — a cost-8 call still doesn't fit, a
	// cost-4 one does.
	now = now.Add(5 * time.Second)
	if l.allow("a", 8, now) {
		t.Fatal("cost above refilled balance allowed")
	}
	if !l.allow("a", 4, now) {
		t.Fatal("cost within refilled balance denied")
	}

	// Refill never exceeds burst.
	now = now.Add(time.Hour)
	for i := range 10 {
		if !l.allow("a", 1, now) {
			t.Fatalf("request %d after long idle denied", i)
		}
	}
	if l.allow("a", 1, now) {
		t.Fatal("burst cap not enforced after long idle")
	}
}

func TestRateLimiterIsolatesClients(t *testing.T) {
	l := newRateLimiter(60, 2)
	now := time.Unix(1000, 0)

	l.allow("a", 2, now)
	if l.allow("a", 1, now) {
		t.Fatal("exhausted client allowed")
	}
	if !l.allow("b", 1, now) {
		t.Fatal("fresh client denied by another client's spending")
	}
}

func TestRateLimiterSweepDropsIdleBuckets(t *testing.T) {
	l := newRateLimiter(60, 10)
	now := time.Unix(1000, 0)

	l.allow("a", 1, now)
	l.allow("b", 1, now)

	// Past the sweep interval and past full refill, idle buckets go.
	now = now.Add(sweepEvery + time.Minute)
	l.allow("c", 1, now)
	if len(l.buckets) != 1 {
		t.Fatalf("idle buckets not swept: %d left", len(l.buckets))
	}
}

func TestClientIP(t *testing.T) {
	r := httptest.NewRequest("GET", "/api/stats", nil)
	r.RemoteAddr = "192.0.2.7:4242"
	r.Header.Set("X-Forwarded-For", "203.0.113.9, 198.51.100.2")

	// Forwarding headers are ignored unless explicitly trusted.
	if got := clientIP(r, ""); got != "192.0.2.7" {
		t.Fatalf("untrusted header honored: %q", got)
	}
	if got := clientIP(r, "X-Forwarded-For"); got != "203.0.113.9" {
		t.Fatalf("trusted header first entry = %q", got)
	}
	// Trusted header absent → socket address.
	if got := clientIP(r, "CF-Connecting-IP"); got != "192.0.2.7" {
		t.Fatalf("missing trusted header fallback = %q", got)
	}
}

func TestRouteCostOrdering(t *testing.T) {
	addr := routeCost("/api/address/bc1qexample")
	search := routeCost("/api/search")
	tx := routeCost("/api/tx/abcd")
	stats := routeCost("/api/stats")
	if !(addr > search && search > tx && tx > stats) {
		t.Fatalf("cost ordering violated: addr=%v search=%v tx=%v stats=%v",
			addr, search, tx, stats)
	}
}

func TestWithRateLimitRejectsAndExempts(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	limiter := newRateLimiter(60, 1) // one cost-1 request, then dry
	h := withRateLimit(inner, limiter, "")

	do := func(path string) int {
		r := httptest.NewRequest("GET", path, nil)
		r.RemoteAddr = "192.0.2.7:4242"
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code
	}

	if code := do("/api/stats"); code != http.StatusOK {
		t.Fatalf("first request = %d", code)
	}
	if code := do("/api/stats"); code != http.StatusTooManyRequests {
		t.Fatalf("exhausted request = %d, want 429", code)
	}
	// The SPA and its assets are never limited.
	if code := do("/"); code != http.StatusOK {
		t.Fatalf("static path limited: %d", code)
	}
}

func TestSecurityHeaders(t *testing.T) {
	h := securityHeaders(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {}))

	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))

	for _, header := range []string{
		"Content-Security-Policy",
		"X-Content-Type-Options",
		"X-Frame-Options",
		"Referrer-Policy",
	} {
		if w.Header().Get(header) == "" {
			t.Errorf("%s not set", header)
		}
	}
	if got := w.Header().Get("Strict-Transport-Security"); got != "" {
		t.Errorf("HSTS set by app (%q) — that's the TLS proxy's job", got)
	}
}
