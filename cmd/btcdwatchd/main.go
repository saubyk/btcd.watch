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

	hub := api.NewHub(svc.Stats, svc.GetTx)
	hubCtx, stopHub := context.WithCancel(context.Background())
	defer stopHub()
	go hub.Run(hubCtx)

	backend.Start(node.Handlers{
		OnBlock: func(height int32) {
			svc.OnBlock()
			hub.NotifyBlock()
		},
		OnTxAccepted: svc.Mempool().MarkDirty,
	})

	static, err := api.StaticHandler(*staticDir)
	if err != nil {
		return err
	}

	server := &http.Server{
		Addr:              cfg.Server.Listen,
		Handler:           api.New(svc, backend, params, cfg.Node.Network, hub, static),
		ReadHeaderTimeout: 5 * time.Second,
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
