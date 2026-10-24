// Command server is the bootstrapped entrypoint every downstream project
// starts from. It wires: structured logging, OTel traces+metrics, an HTTP
// server, a gRPC server, and a dedicated pprof server for Pyroscope to
// scrape — then waits for SIGINT/SIGTERM and drains cleanly.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joserafaelSH/golang-software-engineer/go-service-template/internal/config"
	"github.com/joserafaelSH/golang-software-engineer/go-service-template/internal/infra/cache"
	"github.com/joserafaelSH/golang-software-engineer/go-service-template/internal/infra/postgres"

	"github.com/joserafaelSH/golang-software-engineer/go-service-template/internal/logger"
	"github.com/joserafaelSH/golang-software-engineer/go-service-template/internal/server"
	"github.com/joserafaelSH/golang-software-engineer/go-service-template/internal/telemetry"
	"github.com/joserafaelSH/golang-software-engineer/go-service-template/internal/worker"
)

func main() {
	cfg := config.Load()
	log := logger.New(cfg.ServiceName, cfg.Environment, cfg.LogLevel)
	db := postgres.NewPostgresClient(cfg)
	redis := cache.NewRedisClient(cfg)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	shutdownTelemetry, err := telemetry.Setup(ctx, cfg.ServiceName, cfg.Environment, cfg.OTLPEndpoint)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to set up telemetry")
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := shutdownTelemetry(shutdownCtx); err != nil {
			log.Error().Err(err).Msg("telemetry shutdown error")
		}
	}()

	pprofSrv := server.NewPprofServer(cfg.PprofPort)

	go func() {
		log.Info().Int("port", cfg.PprofPort).Msg("pprof server listening")
		if err := pprofSrv.ListenAndServe(); err != nil {
			log.Error().Err(err).Msg("pprof server stopped")
		}
	}()

	poller, err := worker.NewPoller(db, redis, &cfg, log)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to set up worker poller")
	}
	poller.Run(ctx)

	<-ctx.Done()
	log.Info().Msg("shutdown signal received, draining")

	drainCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	_ = pprofSrv.Shutdown(drainCtx)

	log.Info().Msg("shutdown complete")
}
