package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"example.com/activity/internal/config"
	"example.com/activity/internal/outbox"
)

const (
	defaultDLQBatchSize = 50
)

func main() {
	cfg := config.Load()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool, err := pgxpool.New(ctx, cfg.PostgresURL)
	if err != nil {
		log.Fatalf("failed to connect to postgres: %v", err)
	}
	defer pool.Close()

	manager := outbox.NewDLQManager(pool, cfg.DLQMaxRetries, cfg.DLQBaseDelay)

	metricsSrv := &http.Server{Addr: cfg.MetricsAddress, Handler: promhttp.Handler()}
	go func() {
		log.Printf("dlq manager metrics listening on %s", cfg.MetricsAddress)
		if err := metricsSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("metrics server error: %v", err)
		}
	}()

	ticker := time.NewTicker(cfg.DLQPollInterval)
	defer ticker.Stop()

	log.Printf("DLQ manager started (interval=%s, maxRetries=%d)", cfg.DLQPollInterval, cfg.DLQMaxRetries)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case <-ctx.Done():
			goto shutdown
		case <-ticker.C:
			processed, err := manager.RunOnce(ctx, defaultDLQBatchSize)
			if err != nil {
				log.Printf("dlq manager error: %v", err)
			} else if processed > 0 {
				log.Printf("dlq manager processed %d entries", processed)
			}
		case <-stop:
			log.Println("dlq manager received shutdown signal")
			cancel()
			goto shutdown
		}
	}

shutdown:
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := metricsSrv.Shutdown(shutdownCtx); err != nil {
		log.Printf("metrics server shutdown error: %v", err)
	}
}
