package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

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

	ticker := time.NewTicker(cfg.DLQPollInterval)
	defer ticker.Stop()

	log.Printf("DLQ manager started (interval=%s, maxRetries=%d)", cfg.DLQPollInterval, cfg.DLQMaxRetries)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			processed, err := manager.RunOnce(ctx, defaultDLQBatchSize)
			if err != nil {
				log.Printf("dlq manager error: %v", err)
			} else if processed > 0 {
				log.Printf("dlq manager processed %d entries", processed)
			}
		case <-stop:
			log.Println("dlq manager received shutdown signal")
			return
		}
	}
}
