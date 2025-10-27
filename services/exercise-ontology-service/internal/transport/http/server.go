package httptransport

import (
    "net/http"
    "time"
)

// ServerConfig configures the HTTP server.
type ServerConfig struct {
    Address      string
    ReadTimeout  time.Duration
    WriteTimeout time.Duration
    IdleTimeout  time.Duration
}

// NewServer instantiates an http.Server with timeouts.
func NewServer(cfg ServerConfig, handler http.Handler) *http.Server {
    return &http.Server{
        Addr:         cfg.Address,
        Handler:      handler,
        ReadTimeout:  cfg.ReadTimeout,
        WriteTimeout: cfg.WriteTimeout,
        IdleTimeout:  cfg.IdleTimeout,
    }
}
