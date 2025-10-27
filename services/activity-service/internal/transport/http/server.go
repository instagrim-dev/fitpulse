package httptransport

import (
	"net/http"
	"time"
)

// ServerConfig contains tunables for the HTTP server.
type ServerConfig struct {
	Address      string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

// NewServer creates *http.Server with provided handler.
func NewServer(cfg ServerConfig, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:         cfg.Address,
		Handler:      handler,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
	}
}
