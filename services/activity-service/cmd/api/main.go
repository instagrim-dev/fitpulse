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

	"example.com/activity/internal/api"
	"example.com/activity/internal/auth"
	"example.com/activity/internal/config"
	"example.com/activity/internal/domain"
	"example.com/activity/internal/outbox"
	persistence "example.com/activity/internal/persistence/postgres"
	httptransport "example.com/activity/internal/transport/http"
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

	repo := persistence.NewRepository(pool)
	producer := outbox.NewKafkaProducer(cfg.KafkaBrokers)
	defer producer.Close()

	registry := outbox.NewSchemaRegistryClient(cfg.SchemaRegistryURL)
	dispatcher := outbox.NewDispatcher(pool, producer, registry, cfg.OutboxPollInterval, cfg.OutboxBatchSize)

	go dispatcher.Start(ctx)

	service := domain.NewService(repo)

	handler := api.NewHandler(service)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	mux.Handle("/metrics", promhttp.Handler())

	authMiddleware := auth.NewMiddleware(auth.Config{Secret: cfg.JWTSecret, Issuer: cfg.JWTIssuer})

	server := httptransport.NewServer(httptransport.ServerConfig{
		Address:      cfg.HTTPAddress,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}, authMiddleware.Wrap(mux))

	shutdownCh := make(chan os.Signal, 1)
	signal.Notify(shutdownCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Printf("activity-service listening on %s", cfg.HTTPAddress)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	<-shutdownCh
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}

	dispatcher.Wait()
}
