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

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/segmentio/kafka-go"

	"example.com/exerciseontology/internal/cache"
	"example.com/exerciseontology/internal/config"
	"example.com/exerciseontology/internal/consumer"
	"example.com/exerciseontology/internal/domain"
	"example.com/exerciseontology/internal/knowledge"
)

func main() {
	cfg := config.Load()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	metricsSrv := &http.Server{Addr: cfg.MetricsAddress, Handler: promhttp.Handler()}
	go func() {
		log.Printf("ontology consumer metrics listening on %s", cfg.MetricsAddress)
		if err := metricsSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("metrics server error: %v", err)
		}
	}()

	repo := knowledge.NewDgraphRepository(cfg.DgraphURL, cfg.HTTPTimeout)
	service := domain.NewService(repo, cache.NoopInvalidator{})
	handler := consumer.NewEnrichmentHandler(service)
	var wg sync.WaitGroup

	for _, topic := range cfg.ConsumerTopics {
		reader := kafka.NewReader(kafka.ReaderConfig{
			Brokers:        cfg.KafkaBrokers,
			GroupID:        cfg.ConsumerGroup,
			Topic:          topic,
			MinBytes:       1e3,
			MaxBytes:       10e6,
			CommitInterval: time.Second,
		})
		proc := consumer.NewProcessor(reader, handler)

		wg.Add(1)
		go func(tp string, r *kafka.Reader) {
			defer wg.Done()
			defer r.Close()
			if err := proc.Run(ctx); err != nil && err != context.Canceled {
				log.Printf("consumer stopped with error (topic=%s): %v", tp, err)
			}
		}(topic, reader)
	}

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	<-signals
	log.Println("ontology consumer shutting down")
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := metricsSrv.Shutdown(shutdownCtx); err != nil {
		log.Printf("metrics shutdown error: %v", err)
	}

	wg.Wait()
}
