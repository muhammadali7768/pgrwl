package cmd

import (
	"context"
	"log"
	"log/slog"
	"os/signal"
	"sync"
	"syscall"

	"github.com/hashmap-kz/pgrwl/internal/opt/supervisor"

	"github.com/hashmap-kz/pgrwl/config"
	"github.com/hashmap-kz/pgrwl/internal/opt/httpsrv"
)

type ServeModeOpts struct {
	Directory  string
	ListenPort int
	Verbose    bool
}

func RunServeMode(opts *ServeModeOpts) {
	var err error
	cfg := config.Cfg()
	loggr := slog.With("component", "serve-mode-runner")

	// setup context
	ctx, cancel := context.WithCancel(context.Background())
	ctx, signalCancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer signalCancel()

	stor, err := supervisor.SetupStorage(opts.Directory)
	if err != nil {
		//nolint:gocritic
		log.Fatal(err)
	}
	if err := supervisor.CheckManifest(cfg); err != nil {
		log.Fatal(err)
	}

	// Use WaitGroup to wait for all goroutines to finish
	var wg sync.WaitGroup

	// HTTP server
	wg.Add(1)
	go func() {
		defer wg.Done()

		defer func() {
			if r := recover(); r != nil {
				loggr.Info("http server panicked",
					slog.Any("panic", r),
					slog.String("goroutine", "http-server"),
				)
			}
		}()

		handlers := httpsrv.InitHTTPHandlers(&httpsrv.HTTPHandlersOpts{
			BaseDir:     opts.Directory,
			Verbose:     opts.Verbose,
			RunningMode: config.ModeServe,
			Storage:     stor,
		})
		srv := httpsrv.NewHTTPSrv(opts.ListenPort, handlers)
		if err := srv.Run(ctx); err != nil {
			loggr.Info("http server failed", slog.Any("err", err))
			cancel()
		}
	}()

	// Wait for signal (context cancellation)
	<-ctx.Done()
	loggr.Info("shutting down, waiting for goroutines...")

	// Wait for all goroutines to finish
	wg.Wait()
	loggr.Info("all components shut down cleanly")
}
