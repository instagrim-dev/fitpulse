// Package config centralises configuration parsing for the activity service.
package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Config captures runtime configuration values for the activity service.
type Config struct {
	HTTPAddress        string
	PostgresURL        string
	KafkaBrokers       []string
	SchemaRegistryURL  string
	OutboxPollInterval time.Duration
	OutboxBatchSize    int
	JWTSecret          string
	JWTIssuer          string
	DLQPollInterval    time.Duration // Interval between DLQ polling iterations.
	DLQMaxRetries      int           // Maximum number of DLQ retry attempts before quarantine.
	DLQBaseDelay       time.Duration // Base delay used for exponential backoff.
}

// Load reads environment variables into Config, applying sensible defaults for local dev.
func Load() Config {
	cfg := Config{
		HTTPAddress:        getEnv("HTTP_ADDRESS", ":8080"),
		PostgresURL:        getEnv("POSTGRES_URL", "postgres://platform:platform@postgres:5432/fitness?sslmode=disable"),
		SchemaRegistryURL:  getEnv("SCHEMA_REGISTRY_URL", "http://schema-registry:8081"),
		OutboxPollInterval: getDurationEnv("OUTBOX_POLL_INTERVAL", 2*time.Second),
		OutboxBatchSize:    getIntEnv("OUTBOX_BATCH_SIZE", 25),
		JWTSecret:          getEnv("JWT_SECRET", "dev-secret-change-me"),
		JWTIssuer:          getEnv("JWT_ISSUER", "i5e.identity"),
		DLQPollInterval:    getDurationEnv("DLQ_POLL_INTERVAL", 30*time.Second),
		DLQMaxRetries:      getIntEnv("DLQ_MAX_RETRIES", 5),
		DLQBaseDelay:       getDurationEnv("DLQ_BASE_DELAY", time.Minute),
	}

	brokers := getEnv("KAFKA_BROKERS", "kafka:9092")
	cfg.KafkaBrokers = splitAndTrim(brokers)
	return cfg
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		return value
	}
	return fallback
}

func splitAndTrim(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func getDurationEnv(key string, fallback time.Duration) time.Duration {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		if parsed, err := time.ParseDuration(value); err == nil {
			return parsed
		}
	}
	return fallback
}

func getIntEnv(key string, fallback int) int {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			return parsed
		}
	}
	return fallback
}
