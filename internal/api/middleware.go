package api

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// securityHeaders sets browser hardening headers on every response. The
// CSP allows the Google Fonts stylesheets index.html loads, React's inline
// style attributes, and same-origin WebSocket upgrades — nothing else.
// HSTS is deliberately not set here: it is the TLS proxy's job, and
// emitting it from the app would poison plain-HTTP localhost use.
func securityHeaders(next http.Handler) http.Handler {
	const csp = "default-src 'self'; script-src 'self'; " +
		"style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; " +
		"font-src https://fonts.gstatic.com; img-src 'self' data:; " +
		"connect-src 'self' ws: wss:; frame-ancestors 'none'; " +
		"base-uri 'none'"

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", csp)
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}

// clientIP resolves the caller's address for rate limiting. When the
// operator declares a trusted proxy header (e.g. CF-Connecting-IP behind
// Cloudflare, X-Forwarded-For behind a local reverse proxy) its first
// entry wins; otherwise the socket address is authoritative. Never honor
// forwarding headers by default — anyone can forge them when no proxy
// strips inbound copies.
func clientIP(r *http.Request, trustedHeader string) string {
	if trustedHeader != "" {
		if v := r.Header.Get(trustedHeader); v != "" {
			if i := strings.IndexByte(v, ','); i >= 0 {
				v = v[:i]
			}
			return strings.TrimSpace(v)
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// routeCost weights API calls by what they cost the node. Address lookups
// fan out into searchrawtransactions scans (the expensive path); search
// may resolve to one; tx/block are single lookups; stats/fees/healthz are
// served from caches.
func routeCost(path string) float64 {
	switch {
	case strings.HasPrefix(path, "/api/address/"):
		return 8
	case path == "/api/search", path == "/api/ws":
		return 4
	case strings.HasPrefix(path, "/api/tx/"),
		strings.HasPrefix(path, "/api/block/"):
		return 2
	default:
		return 1
	}
}

// rateLimiter is a per-client token bucket. Buckets refill continuously at
// perMin tokens per minute up to burst; a request spends its route cost or
// is rejected. Idle buckets are swept periodically so memory stays bounded
// by recent distinct clients, not lifetime ones.
type rateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	perSec  float64
	burst   float64
	sweepAt time.Time
}

type bucket struct {
	tokens  float64
	updated time.Time
}

const sweepEvery = 5 * time.Minute

func newRateLimiter(perMin, burst int) *rateLimiter {
	if burst <= 0 {
		burst = perMin
	}
	return &rateLimiter{
		buckets: make(map[string]*bucket),
		perSec:  float64(perMin) / 60,
		burst:   float64(burst),
	}
}

func (l *rateLimiter) allow(key string, cost float64, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	if now.After(l.sweepAt) {
		l.sweep(now)
		l.sweepAt = now.Add(sweepEvery)
	}

	b, ok := l.buckets[key]
	if !ok {
		b = &bucket{tokens: l.burst, updated: now}
		l.buckets[key] = b
	} else {
		b.tokens = min(l.burst,
			b.tokens+now.Sub(b.updated).Seconds()*l.perSec)
		b.updated = now
	}

	if b.tokens < cost {
		return false
	}
	b.tokens -= cost
	return true
}

// sweep drops buckets that have been idle long enough to refill fully —
// indistinguishable from a fresh bucket, so nothing is lost.
func (l *rateLimiter) sweep(now time.Time) {
	refillTime := time.Duration(l.burst / l.perSec * float64(time.Second))
	for key, b := range l.buckets {
		if now.Sub(b.updated) > refillTime {
			delete(l.buckets, key)
		}
	}
}

// withRateLimit rejects /api requests from clients that exceed the token
// budget. Static assets and the SPA shell are never limited — they are the
// CDN's problem, and charging them would drain budgets on every page load.
func withRateLimit(next http.Handler, limiter *rateLimiter,
	trustedHeader string) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}
		ip := clientIP(r, trustedHeader)
		if !limiter.allow(ip, routeCost(r.URL.Path), time.Now()) {
			w.Header().Set("Retry-After", "10")
			writeError(w, http.StatusTooManyRequests, "rate_limited",
				"too many requests — please slow down")
			return
		}
		next.ServeHTTP(w, r)
	})
}
