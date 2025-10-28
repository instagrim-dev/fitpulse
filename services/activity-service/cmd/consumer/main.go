package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/segmentio/kafka-go"

	"example.com/activity/internal/config"
	"example.com/activity/internal/consumer"
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

	handler := consumer.NewPersistenceHandler(pool)

	metricsSrv := &http.Server{Addr: cfg.MetricsAddress, Handler: promhttp.Handler()}

	go func() {
		log.Printf("consumer metrics listening on %s", cfg.MetricsAddress)
		if err := metricsSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("metrics server error: %v", err)
		}
	}()

	var wg sync.WaitGroup
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	for _, topic := range cfg.ConsumerTopics {
		reader := kafka.NewReader(kafka.ReaderConfig{
			Brokers:         cfg.KafkaBrokers,
			GroupID:         cfg.ConsumerGroupID,
			Topic:           topic,
			MinBytes:        1e3,
			MaxBytes:        10e6,
			CommitInterval:  time.Second,
			RetentionTime:   24 * time.Hour,
			ReadLagInterval: -1,
		})

		proc := consumer.NewProcessor(reader, handler)

		wg.Add(1)
		go func(topic string, r *kafka.Reader) {
			defer wg.Done()
			defer r.Close()

			log.Printf("consumer started (topic=%s, group=%s)", topic, cfg.ConsumerGroupID)
			if err := proc.Run(ctx); err != nil && err != context.Canceled {
				log.Printf("consumer stopped with error (topic=%s): %v", topic, err)
			}
		}(topic, reader)
	}

	<-stop
	log.Println("consumer shutdown requested")
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := metricsSrv.Shutdown(shutdownCtx); err != nil {
		log.Printf("metrics server shutdown error: %v", err)
	}

	wg.Wait()
}
