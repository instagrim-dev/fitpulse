package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"example.com/exerciseontology/internal/api"
	"example.com/exerciseontology/internal/auth"
	"example.com/exerciseontology/internal/cache"
	"example.com/exerciseontology/internal/config"
	"example.com/exerciseontology/internal/domain"
	"example.com/exerciseontology/internal/knowledge"
	httptransport "example.com/exerciseontology/internal/transport/http"
)

func main() {
	cfg := config.Load()

	repo := buildRepository(cfg)
	var invalidator cache.Invalidator = cache.NoopInvalidator{}
	if cfg.CacheInvalidationURL != "" {
		invalidator = cache.NewHTTPInvalidator(cfg.CacheInvalidationURL, cfg.CacheInvalidationToken, cfg.HTTPTimeout)
		log.Printf("cache invalidator enabled -> %s", cfg.CacheInvalidationURL)
	}

	service := domain.NewService(repo, invalidator)

	handler := api.NewHandler(service)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	mux.Handle("/metrics", promhttp.Handler())

	// Simple CORS middleware for local dev
	cors := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "http://localhost:5173")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}

	middleware := auth.NewMiddleware(auth.Config{Secret: cfg.JWTSecret, Issuer: cfg.JWTIssuer})

	// Basic request logger
	logger := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.Printf("%s %s", r.Method, r.URL.Path)
			next.ServeHTTP(w, r)
		})
	}

	server := httptransport.NewServer(httptransport.ServerConfig{
		Address:      cfg.HTTPAddress,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}, cors(logger(middleware.Wrap(mux))))

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Printf("exercise-ontology-service listening on %s", cfg.HTTPAddress)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}
}

func buildRepository(cfg config.Config) domain.Repository {
	if cfg.DgraphURL != "" {
		log.Printf("using Dgraph repository at %s", cfg.DgraphURL)
		return knowledge.NewDgraphRepository(cfg.DgraphURL, cfg.HTTPTimeout)
	}
	log.Printf("DGRAPH_URL not set, using in-memory repository")
	return knowledge.NewInMemoryRepository()
}
