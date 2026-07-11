// btcdwatchd is the btcdwatch.com server: a REST (and, in later
// milestones, WebSocket) API over a local btcd node.
package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/btcsuite/btcd/btcjson"

	"btcdwatch.com/internal/api"
	"btcdwatch.com/internal/chain"
	"btcdwatch.com/internal/config"
	"btcdwatch.com/internal/explorer"
	"btcdwatch.com/internal/node"
	"btcdwatch.com/internal/price"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
	configPath := flag.String("config", "",
		"path to YAML config (default: config.yaml if present)")
	staticDir := flag.String("static-dir", "",
		"serve the SPA from this directory instead of the embedded build")
	flag.Parse()

	path := *configPath
	if path == "" {
		if _, err := os.Stat("config.yaml"); err == nil {
			path = "config.yaml"
		}
	}

	cfg, err := config.Load(path)
	if err != nil {
		return err
	}

	params, err := chain.ParamsForNetwork(cfg.Node.Network)
	if err != nil {
		return err
	}

	// Created first (the service needs it), but connection starts below,
	// once the notification consumers exist.
	backend, err := node.New(node.Config{
		Host:     cfg.Node.RPCHost,
		User:     cfg.Node.RPCUser,
		Pass:     cfg.Node.RPCPass,
		CertPath: cfg.Node.RPCCert,
	})
	if err != nil {
		return err
	}
	defer backend.Shutdown()

	prices := price.New(cfg.Price.Source, cfg.Price.StaticUSD,
		cfg.Price.RefreshSeconds)
	defer prices.Close()
	quote := func() explorer.PriceQuote {
		q := prices.Quote()
		return explorer.PriceQuote{
			USD:       q.USD,
			Source:    q.Source,
			UpdatedAt: q.UpdatedAt.Unix(),
			OK:        q.OK,
		}
	}

	svc := explorer.NewService(backend, explorer.Config{
		Params: params,
		Price:  quote,
		Floors: explorer.FeeFloors{
			Slow:     cfg.Fees.FloorSlow,
			Standard: cfg.Fees.FloorStandard,
			Urgent:   cfg.Fees.FloorUrgent,
		},
		MaxScanTxs: cfg.Address.MaxScanTxs,
	})

	hub := api.NewHub(svc.Stats, svc.GetTx, svc.MempoolUpdate, svc.BlockFlash)
	// Must be set before Run starts — the hub loop reads it.
	hub.MaxClients = cfg.Server.MaxWSClients
	hubCtx, stopHub := context.WithCancel(context.Background())
	defer stopHub()
	go hub.Run(hubCtx)

	backend.Start(node.Handlers{
		OnBlock: func(_ int32, hash string) {
			svc.OnBlock()
			hub.NotifyBlock(hash)
		},
		OnTxAccepted: func(raw *btcjson.TxRawResult) {
			svc.NoteArrival(raw)
			hub.NotifyMempool()
		},
	})

	static, err := api.StaticHandler(*staticDir)
	if err != nil {
		return err
	}

	handler := api.New(svc, backend, params, cfg.Node.Network, hub, static,
		api.Options{
			RateLimitPerMin:    cfg.Server.RateLimitPerMin,
			RateLimitBurst:     cfg.Server.RateLimitBurst,
			TrustedProxyHeader: cfg.Server.TrustedProxyHeader,
			MaxConcurrentScans: cfg.Address.MaxConcurrentScans,
		})

	// The write timeout is generous because address scans can hold a
	// response open for a while. WebSocket connections are unaffected:
	// gorilla clears the connection deadlines on upgrade and the pumps
	// manage their own per-frame deadlines.
	server := &http.Server{
		Addr:              cfg.Server.Listen,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    16 << 10,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("btcdwatchd listening",
			"addr", cfg.Server.Listen, "network", cfg.Node.Network)
		if err := server.ListenAndServe(); err != nil &&
			!errors.Is(err, http.ErrServerClosed) {

			errCh <- err
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-errCh:
		return err
	case sig := <-stop:
		slog.Info("shutting down", "signal", sig)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return server.Shutdown(ctx)
}
