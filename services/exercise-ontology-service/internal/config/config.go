package config

import (
	"os"
	"strings"
	"time"
)

// Config captures runtime configuration values for the ontology service.
type Config struct {
	HTTPAddress            string
	DgraphURL              string
	JWTSecret              string
	JWTIssuer              string
	HTTPTimeout            time.Duration
	CacheInvalidationURL   string
	CacheInvalidationToken string
	KafkaBrokers           []string
	ConsumerGroup          string
	ConsumerTopics         []string
	MetricsAddress         string
}

// Load reads environment variables and applies defaults.
func Load() Config {
	return Config{
		HTTPAddress:            getEnv("HTTP_ADDRESS", ":8090"),
		DgraphURL:              getEnv("DGRAPH_URL", "http://dgraph-alpha:8080"),
		JWTSecret:              getEnv("JWT_SECRET", "dev-secret-change-me"),
		JWTIssuer:              getEnv("JWT_ISSUER", "i5e.identity"),
		HTTPTimeout:            getDurationEnv("HTTP_TIMEOUT", 5*time.Second),
		CacheInvalidationURL:   getEnv("CACHE_INVALIDATION_URL", ""),
		CacheInvalidationToken: getEnv("CACHE_INVALIDATION_TOKEN", ""),
		KafkaBrokers:           splitAndTrim(getEnv("KAFKA_BROKERS", "kafka:9092")),
		ConsumerGroup:          getEnv("CONSUMER_GROUP_ID", "exercise-ontology-consumer"),
		ConsumerTopics:         splitAndTrim(getEnv("CONSUMER_TOPICS", "activity_events")),
		MetricsAddress:         getEnv("METRICS_ADDRESS", ":9195"),
	}
}

func splitAndTrim(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		return value
	}
	return fallback
}

func getDurationEnv(key string, fallback time.Duration) time.Duration {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		if parsed, err := time.ParseDuration(value); err == nil {
			return parsed
		}
	}
	return fallback
}
