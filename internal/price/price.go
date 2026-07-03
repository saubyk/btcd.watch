// Package price provides the BTC/USD quote: a CoinGecko fetcher with an
// in-memory cache that degrades to a static configured price whenever the
// live source is disabled or unreachable.
package price

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

const coingeckoURL = "https://api.coingecko.com/api/v3/simple/price" +
	"?ids=bitcoin&vs_currencies=usd"

// Quote is the current BTC/USD price. OK is false when no price at all is
// available (live fetch failing and no static fallback configured).
type Quote struct {
	USD       float64
	Source    string // "coingecko" | "static"
	UpdatedAt time.Time
	OK        bool
}

// Service serves cached quotes, refreshing from CoinGecko in the
// background when the live source is enabled.
type Service struct {
	mu    sync.RWMutex
	quote Quote

	client *http.Client
	stop   chan struct{}
}

// New builds the service. source "coingecko" starts a refresh loop; any
// fetch failure leaves the previous quote (or the static fallback) in
// place, so a flaky price API never breaks responses.
func New(source string, staticUSD float64, refreshSeconds int) *Service {
	s := &Service{
		client: &http.Client{Timeout: 10 * time.Second},
		stop:   make(chan struct{}),
	}
	s.quote = Quote{
		USD:       staticUSD,
		Source:    "static",
		UpdatedAt: time.Now(),
		OK:        staticUSD > 0,
	}

	if source == "coingecko" {
		if refreshSeconds < 10 {
			refreshSeconds = 10
		}
		go s.loop(time.Duration(refreshSeconds) * time.Second)
	}
	return s
}

// Quote returns the most recent price.
func (s *Service) Quote() Quote {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.quote
}

// Close stops the refresh loop.
func (s *Service) Close() {
	close(s.stop)
}

func (s *Service) loop(interval time.Duration) {
	s.refresh()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.refresh()
		case <-s.stop:
			return
		}
	}
}

func (s *Service) refresh() {
	resp, err := s.client.Get(coingeckoURL)
	if err != nil {
		slog.Warn("price fetch failed", "err", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		slog.Warn("price fetch failed", "status", resp.StatusCode)
		return
	}

	var body struct {
		Bitcoin struct {
			USD float64 `json:"usd"`
		} `json:"bitcoin"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil ||
		body.Bitcoin.USD <= 0 {

		slog.Warn("price response unusable", "err", err)
		return
	}

	s.mu.Lock()
	s.quote = Quote{
		USD:       body.Bitcoin.USD,
		Source:    "coingecko",
		UpdatedAt: time.Now(),
		OK:        true,
	}
	s.mu.Unlock()
}
